package db

import (
	"context"
	"log"
	"msp/internal/types"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dbPath string) error {
	if dbPath == "" {
		dbPath = "msp.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		PrepareStmt:            true, // 缓存预编译语句
		SkipDefaultTransaction: true, // 禁用默认事务以提高写入性能（我们在业务中手动控制事务）
	})
	if err != nil {
		return err
	}

	// 连接池性能调优
	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(1) // SQLite 建议单连接以避免过多的锁竞争（除非开启 WAL）
		sqlDB.SetMaxIdleConns(1)

		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			log.Printf("DB Warn: failed to set WAL mode: %v", err)
		}
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
			log.Printf("DB Warn: failed to set synchronous mode: %v", err)
		}
		if _, err := sqlDB.Exec("PRAGMA cache_size=-2000;"); err != nil {
			log.Printf("DB Warn: failed to set cache size: %v", err)
		}
	}

	return DB.AutoMigrate(&types.MediaItem{}, &types.MediaScan{}, &types.UserPref{}, &types.PlaybackProgress{})
}

func GetProgress(ctx context.Context, mediaID string) (float64, error) {
	if DB == nil || mediaID == "" {
		return 0, nil
	}
	var p types.PlaybackProgress
	// Use silent logger to avoid "record not found" spam in logs
	err := DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).WithContext(ctx).First(&p, "media_id = ?", mediaID).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return p.Time, err
}

func SetProgress(ctx context.Context, mediaID string, t float64) error {
	if DB == nil || mediaID == "" {
		return nil
	}
	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&types.PlaybackProgress{
		MediaID: mediaID,
		Time:    t,
	}).Error
}

// Scopes 提供可复用的查询逻辑
func ByScan(scanID int64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("scan_id = ?", scanID)
	}
}

func ByKind(kind string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("kind = ?", kind)
	}
}

func GetScanMeta(ctx context.Context, cacheKey string) (types.MediaScan, bool, error) {
	if DB == nil || cacheKey == "" {
		return types.MediaScan{}, false, nil
	}
	var scan types.MediaScan
	err := DB.WithContext(ctx).First(&scan, "cache_key = ?", cacheKey).Error
	if err == gorm.ErrRecordNotFound {
		return types.MediaScan{}, false, nil
	}
	return scan, true, err
}

func SetScanMeta(ctx context.Context, tx *gorm.DB, cacheKey string, meta types.MediaScan) error {
	dbConn := DB
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil || cacheKey == "" {
		return nil
	}
	meta.CacheKey = cacheKey
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&meta).Error
}

func UpsertMediaItem(ctx context.Context, tx *gorm.DB, item *types.MediaItem) error {
	dbConn := DB
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil {
		return nil
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "path"}},
		UpdateAll: true,
	}).Create(item).Error
}

func DeleteStaleByScan(ctx context.Context, tx *gorm.DB, scanID int64, shareRoots []string) error {
	dbConn := DB
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil || scanID <= 0 || len(shareRoots) == 0 {
		return nil
	}
	return dbConn.WithContext(ctx).Where("scan_id != ? AND share_root IN ?", scanID, shareRoots).Delete(&types.MediaItem{}).Error
}

func DeleteByShareRootsNotIn(ctx context.Context, tx *gorm.DB, shareRoots []string) error {
	dbConn := DB
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil {
		return nil
	}
	if len(shareRoots) == 0 {
		return dbConn.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&types.MediaItem{}).Error
	}
	return dbConn.WithContext(ctx).Where("share_root NOT IN ?", shareRoots).Delete(&types.MediaItem{}).Error
}

func QueryMediaItems(ctx context.Context, scanID int64, kind string) ([]types.MediaItem, error) {
	if DB == nil || scanID <= 0 || kind == "" {
		return nil, nil
	}
	var items []types.MediaItem
	err := DB.WithContext(ctx).
		Scopes(ByScan(scanID), ByKind(kind)).
		Order("share_label, lower(name)").
		Find(&items).Error
	return items, err
}

func CountMediaItems(ctx context.Context, scanID int64, kind string) (int, error) {
	if DB == nil || scanID <= 0 || kind == "" {
		return 0, nil
	}
	var count int64
	err := DB.WithContext(ctx).Model(&types.MediaItem{}).
		Scopes(ByScan(scanID), ByKind(kind)).
		Count(&count).Error
	return int(count), err
}

func GetAllPrefs(ctx context.Context) (map[string]string, error) {
	if DB == nil {
		return map[string]string{}, nil
	}
	var prefs []types.UserPref
	if err := DB.WithContext(ctx).Find(&prefs).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(prefs))
	for _, p := range prefs {
		out[p.Key] = p.Value
	}
	return out, nil
}

func SetPrefs(ctx context.Context, kv map[string]string) error {
	if DB == nil || len(kv) == 0 {
		return nil
	}

	prefs := make([]types.UserPref, 0, len(kv))
	for k, v := range kv {
		if k == "" {
			continue
		}
		prefs = append(prefs, types.UserPref{Key: k, Value: v})
	}

	return DB.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&prefs).Error
}

func Close() {
	if DB != nil {
		sqlDB, _ := DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}
