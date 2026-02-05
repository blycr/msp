package media

import (
	"context"
	"fmt"
	"time"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/db"
	"msp/internal/types"
	"msp/internal/util"

	"gorm.io/gorm"
)

// LoadMediaFromDB 从数据库加载媒体列表。
func LoadMediaFromDB(ctx context.Context, cacheKey string, shares []config.Share) (types.MediaResponse, time.Time, bool, error) {
	if db.DB == nil {
		return types.MediaResponse{}, time.Time{}, false, nil
	}
	scan, ok, err := db.GetScanMeta(ctx, cacheKey)
	if err != nil || !ok || scan.ScanID <= 0 || scan.BuiltAt <= 0 {
		return types.MediaResponse{}, time.Time{}, false, err
	}
	resp, err := LoadMediaResponseFromDBScan(ctx, scan.ScanID, shares)
	if err != nil {
		return types.MediaResponse{}, time.Time{}, false, err
	}
	return resp, time.Unix(0, scan.BuiltAt), true, nil
}

// ReindexAndLoadMedia 重新索引媒体文件并加载结果。
func ReindexAndLoadMedia(ctx context.Context, cacheKey string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) (types.MediaResponse, time.Time, error) {
	if db.DB == nil {
		return types.MediaResponse{}, time.Time{}, nil
	}
	scanID, builtAt, _, err := IndexMediaToDB(ctx, cacheKey, shares, blacklist, maxItems)
	if err != nil {
		return types.MediaResponse{}, time.Time{}, err
	}
	resp, err := LoadMediaResponseFromDBScan(ctx, scanID, shares)
	if err != nil {
		return types.MediaResponse{}, time.Time{}, err
	}
	return resp, builtAt, nil
}

// IndexMediaToDB scans all shares and indexes media files into the database.
// It returns the scan ID, build time, completion status, and any error encountered.
func IndexMediaToDB(ctx context.Context, cacheKey string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) (scanID int64, builtAt time.Time, complete bool, err error) {
	if db.DB == nil {
		return 0, time.Time{}, false, nil
	}

	builtAt = time.Now()
	scanID = builtAt.UnixNano()

	validShares, shareRoots := prepareShares(shares)

	tx := db.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, time.Time{}, false, tx.Error
	}
	defer func() {
		_ = tx.Rollback()
	}()

	seen, err := performScan(ctx, tx, scanID, validShares, blacklist, maxItems)
	if err != nil {
		return 0, time.Time{}, false, err
	}

	limit := maxItems
	if limit <= 0 {
		limit = constants.DBScanLimit
	}
	complete = seen < limit

	if complete {
		if err := cleanupStaleData(ctx, tx, scanID, shareRoots); err != nil {
			return 0, time.Time{}, false, err
		}
	}

	if err := db.SetScanMeta(ctx, tx, cacheKey, types.MediaScan{ScanID: scanID, BuiltAt: builtAt.UnixNano(), Complete: complete}); err != nil {
		return 0, time.Time{}, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return 0, time.Time{}, false, err
	}
	return scanID, builtAt, complete, nil
}

func prepareShares(shares []config.Share) (validShares []config.Share, shareRoots []string) {
	shareRoots = make([]string, 0, len(shares))
	validShares = make([]config.Share, 0, len(shares))
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

func performScan(ctx context.Context, tx *gorm.DB, scanID int64, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) (int, error) {
	seen := 0
	limit := maxItems
	if limit <= 0 {
		limit = constants.DBScanLimit
	}

	// 使用批量插入缓冲区，提升性能
	const batchSize = 100
	batch := make([]types.MediaItem, 0, batchSize)

	cb := func(item types.MediaItem, path string, root string) error {
		item.ScanID = scanID
		item.ShareRoot = root
		item.Path = path
		batch = append(batch, item)

		// 批量写入
		if len(batch) >= batchSize {
			if err := db.UpsertMediaItems(ctx, tx, batch); err != nil {
				return fmt.Errorf("batch upsert media items: %w", err)
			}
			batch = batch[:0] // 清空切片但保留容量
		}

		seen++
		return nil
	}

	if err := WalkShares(ctx, shares, blacklist, limit, cb); err != nil {
		return 0, fmt.Errorf("walk shares: %w", err)
	}

	// 写入剩余数据
	if len(batch) > 0 {
		if err := db.UpsertMediaItems(ctx, tx, batch); err != nil {
			return 0, fmt.Errorf("final batch upsert: %w", err)
		}
	}

	return seen, nil
}

func cleanupStaleData(ctx context.Context, tx *gorm.DB, scanID int64, shareRoots []string) error {
	if err := db.DeleteStaleByScan(ctx, tx, scanID, shareRoots); err != nil {
		return fmt.Errorf("delete stale scan data: %w", err)
	}
	if err := db.DeleteByShareRootsNotIn(ctx, tx, shareRoots); err != nil {
		return fmt.Errorf("delete orphaned share data: %w", err)
	}
	return nil
}

// LoadMediaResponseFromDBScan 从指定的扫描会话加载媒体响应。
func LoadMediaResponseFromDBScan(ctx context.Context, scanID int64, shares []config.Share) (types.MediaResponse, error) {
	resp := types.MediaResponse{
		Shares: make([]config.Share, len(shares)),
		Videos: []types.MediaItem{},
		Audios: []types.MediaItem{},
		Images: []types.MediaItem{},
		Others: []types.MediaItem{},
	}
	copy(resp.Shares, shares)

	videos, err := db.QueryMediaItems(ctx, scanID, "video")
	if err != nil {
		return types.MediaResponse{}, err
	}
	audios, err := db.QueryMediaItems(ctx, scanID, "audio")
	if err != nil {
		return types.MediaResponse{}, err
	}
	images, err := db.QueryMediaItems(ctx, scanID, "image")
	if err != nil {
		return types.MediaResponse{}, err
	}
	others, err := db.QueryMediaItems(ctx, scanID, "other")
	if err != nil {
		return types.MediaResponse{}, err
	}

	resp.Videos = videos
	resp.Audios = audios
	resp.Images = images
	resp.Others = others
	return resp, nil
}
