package media

import (
	"context"
	"fmt"
	"time"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/scanner"
	"msp/internal/util"

	"gorm.io/gorm"
)

// LoadMediaFromDB 从数据库加载媒体列表。
func (mp *MediaProcessor) LoadMediaFromDB(ctx context.Context, cacheKey string, shares []domain.Share) (domain.MediaResponse, time.Time, bool, error) {
	if !mp.IsDBAvailable() {
		return domain.MediaResponse{}, time.Time{}, false, nil
	}
	scan, ok, err := mp.db.GetScanMeta(ctx, cacheKey)
	if err != nil || !ok || scan.ScanID <= 0 || scan.BuiltAt <= 0 {
		return domain.MediaResponse{}, time.Time{}, false, err
	}
	resp, err := mp.LoadMediaResponseFromDBScan(ctx, scan.ScanID, shares)
	if err != nil {
		return domain.MediaResponse{}, time.Time{}, false, err
	}
	return resp, time.Unix(0, scan.BuiltAt), true, nil
}

func (mp *MediaProcessor) ReindexAndLoadMedia(ctx context.Context, cacheKey string, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) (domain.MediaResponse, time.Time, error) {
	if !mp.IsDBAvailable() {
		return domain.MediaResponse{}, time.Time{}, nil
	}
	scanID, builtAt, _, err := mp.IndexMediaToDB(ctx, cacheKey, shares, blacklist, maxItems)
	if err != nil {
		return domain.MediaResponse{}, time.Time{}, err
	}
	resp, err := mp.LoadMediaResponseFromDBScan(ctx, scanID, shares)
	if err != nil {
		return domain.MediaResponse{}, time.Time{}, err
	}
	mp.notifyScanComplete(resp)
	return resp, builtAt, nil
}

// SetPostScanHook 注册扫描回调：每次扫描成功完成后以本次扫描的媒体条目
// （视频/音频/图片）调用；新扫描开始或服务关闭时以 nil 调用，用于停止上一轮
// 扫描派生的后台工作（如缩略图预热）。传 nil 可注销。
func (mp *MediaProcessor) SetPostScanHook(hook func(items []domain.MediaItem)) {
	if mp == nil {
		return
	}
	mp.postScan.mu.Lock()
	mp.postScan.hook = hook
	mp.postScan.mu.Unlock()
}

// CancelPostScanHook 以 nil 调用已注册的扫描回调，通知其停止上一轮扫描派生
// 的后台工作。在新扫描开始时和服务关闭时调用。
func (mp *MediaProcessor) CancelPostScanHook() {
	if mp == nil {
		return
	}
	mp.postScan.mu.RLock()
	hook := mp.postScan.hook
	mp.postScan.mu.RUnlock()
	if hook != nil {
		hook(nil)
	}
}

// notifyScanComplete 以本次扫描的视频/音频/图片条目调用已注册的扫描回调。
func (mp *MediaProcessor) notifyScanComplete(resp domain.MediaResponse) {
	mp.postScan.mu.RLock()
	hook := mp.postScan.hook
	mp.postScan.mu.RUnlock()
	if hook == nil {
		return
	}
	items := make([]domain.MediaItem, 0, len(resp.Videos)+len(resp.Audios)+len(resp.Images))
	items = append(items, resp.Videos...)
	items = append(items, resp.Audios...)
	items = append(items, resp.Images...)
	hook(items)
}

func (mp *MediaProcessor) IndexMediaToDB(ctx context.Context, cacheKey string, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) (scanID int64, builtAt time.Time, complete bool, err error) {
	if !mp.IsDBAvailable() {
		return 0, time.Time{}, false, nil
	}

	// 新扫描开始：停止上一轮扫描派生的后台工作（如缩略图预热）。
	mp.CancelPostScanHook()

	builtAt = time.Now()
	scanID = builtAt.UnixNano()

	validShares, shareRoots := prepareShares(shares)

	tx := mp.db.DB().WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, time.Time{}, false, tx.Error
	}
	defer func() {
		_ = tx.Rollback()
	}()

	limit := maxItems
	if limit <= 0 {
		limit = constants.DBScanLimit
	}

	if err := mp.cleanupStaleData(ctx, tx, scanID, shareRoots); err != nil {
		return 0, time.Time{}, false, err
	}

	seen, err := mp.performScan(ctx, tx, scanID, validShares, blacklist, maxItems)
	if err != nil {
		return 0, time.Time{}, false, err
	}
	complete = seen < limit

	if err := mp.db.SetScanMeta(ctx, tx, cacheKey, domain.MediaScan{ScanID: scanID, BuiltAt: builtAt.UnixNano(), Complete: complete}); err != nil {
		return 0, time.Time{}, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return 0, time.Time{}, false, err
	}
	return scanID, builtAt, complete, nil
}

func prepareShares(shares []domain.Share) (validShares []domain.Share, shareRoots []string) {
	shareRoots = make([]string, 0, len(shares))
	validShares = make([]domain.Share, 0, len(shares))
	for _, sh := range shares {
		root := util.NormalizePath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}
		shareRoots = append(shareRoots, root)
		sh.Path = root
		validShares = append(validShares, sh)
	}
	return validShares, shareRoots
}

func (mp *MediaProcessor) performScan(ctx context.Context, tx *gorm.DB, scanID int64, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) (int, error) {
	seen := 0
	limit := maxItems
	if limit <= 0 {
		limit = constants.DBScanLimit
	}

	const batchSize = 100
	batch := make([]domain.MediaItem, 0, batchSize)
	batchPaths := make(map[string]bool, batchSize)

	cb := func(item domain.MediaItem, path string, root string) error {
		item.ScanID = scanID
		item.ShareRoot = root
		item.Path = path

		if batchPaths[path] {
			return nil
		}
		batchPaths[path] = true
		batch = append(batch, item)

		if len(batch) >= batchSize {
			if err := mp.db.UpsertMediaItems(ctx, tx, batch); err != nil {
				return fmt.Errorf("batch upsert media items: %w", err)
			}
			batch = batch[:0]
			for k := range batchPaths {
				delete(batchPaths, k)
			}
		}

		seen++
		return nil
	}

	if err := scanner.WalkShares(ctx, shares, blacklist, limit, cb, mp.idCodec); err != nil {
		return 0, fmt.Errorf("walk shares: %w", err)
	}

	if len(batch) > 0 {
		if err := mp.db.UpsertMediaItems(ctx, tx, batch); err != nil {
			return 0, fmt.Errorf("final batch upsert: %w", err)
		}
	}

	return seen, nil
}

func (mp *MediaProcessor) cleanupStaleData(ctx context.Context, tx *gorm.DB, scanID int64, shareRoots []string) error {
	if err := mp.db.DeleteStaleByScan(ctx, tx, scanID, shareRoots); err != nil {
		return fmt.Errorf("delete stale scan data: %w", err)
	}
	if err := mp.db.DeleteByShareRootsNotIn(ctx, tx, shareRoots); err != nil {
		return fmt.Errorf("delete orphaned share data: %w", err)
	}
	return nil
}

// LoadMediaResponseFromDBScan 从指定的扫描会话加载媒体响应。
func (mp *MediaProcessor) LoadMediaResponseFromDBScan(ctx context.Context, scanID int64, shares []domain.Share) (domain.MediaResponse, error) {
	resp := newMediaResponse(shares)
	copy(resp.Shares, shares)

	videos, err := mp.db.QueryMediaItems(ctx, scanID, "video")
	if err != nil {
		return domain.MediaResponse{}, err
	}
	audios, err := mp.db.QueryMediaItems(ctx, scanID, "audio")
	if err != nil {
		return domain.MediaResponse{}, err
	}
	images, err := mp.db.QueryMediaItems(ctx, scanID, "image")
	if err != nil {
		return domain.MediaResponse{}, err
	}
	others, err := mp.db.QueryMediaItems(ctx, scanID, "other")
	if err != nil {
		return domain.MediaResponse{}, err
	}

	resp.Videos = videos
	resp.Audios = audios
	resp.Images = images
	resp.Others = others
	return resp, nil
}
