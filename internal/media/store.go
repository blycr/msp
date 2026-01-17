package media

import (
	"context"
	"time"

	"msp/internal/config"
	"msp/internal/db"
	"msp/internal/types"
	"msp/internal/util"
)

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

func IndexMediaToDB(ctx context.Context, cacheKey string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) (scanID int64, builtAt time.Time, complete bool, err error) {
	if db.DB == nil {
		return 0, time.Time{}, false, nil
	}

	builtAt = time.Now()
	scanID = builtAt.UnixNano()

	shareRoots := make([]string, 0, len(shares))
	validShares := make([]config.Share, 0, len(shares))
	for _, sh := range shares {
		root := util.NormalizePath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}
		shareRoots = append(shareRoots, root)
		sh.Path = root
		validShares = append(validShares, sh)
	}

	tx := db.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, time.Time{}, false, tx.Error
	}
	defer func() {
		_ = tx.Rollback()
	}()

	seen := 0
	limit := maxItems
	if limit <= 0 {
		limit = 1000000000
	}

	cb := func(item types.MediaItem, path string, root string) error {
		item.ScanID = scanID
		item.ShareRoot = root
		item.Path = path
		if err := db.UpsertMediaItem(ctx, tx, &item); err != nil {
			return err
		}
		seen++
		return nil
	}

	if err := WalkShares(ctx, validShares, blacklist, limit, cb); err != nil {
		return 0, time.Time{}, false, err
	}

	complete = seen < limit
	if complete {
		if err := db.DeleteStaleByScan(ctx, tx, scanID, shareRoots); err != nil {
			return 0, time.Time{}, false, err
		}
		if err := db.DeleteByShareRootsNotIn(ctx, tx, shareRoots); err != nil {
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
