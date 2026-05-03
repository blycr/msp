package storage

import (
	"context"
	"log"
	"msp/internal/constants"
	"msp/internal/domain"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type SQLite struct {
	db *gorm.DB
}

func NewGormLogger() logger.Interface {
	return logger.New(
		log.New(log.Writer(), "", log.LstdFlags|log.Lmicroseconds),
		logger.Config{
			SlowThreshold:             2 * time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

func InitSQLite(dbPath string) (*SQLite, error) {
	if dbPath == "" {
		dbPath = "msp.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, constants.DirPerm); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger:                 NewGormLogger(),
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(1)
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

	if err := db.AutoMigrate(&domain.MediaItem{}, &domain.MediaScan{}, &domain.UserPref{}, &domain.PlaybackProgress{}); err != nil {
		return nil, err
	}

	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

func (s *SQLite) DB() *gorm.DB {
	return s.db
}

// ProgressStore implementation

func (s *SQLite) GetProgress(ctx context.Context, mediaID string) (float64, error) {
	if s.db == nil || mediaID == "" {
		return 0, nil
	}
	var p domain.PlaybackProgress
	err := s.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).WithContext(ctx).First(&p, "media_id = ?", mediaID).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return p.Time, err
}

func (s *SQLite) SetProgress(ctx context.Context, mediaID string, t float64) error {
	if s.db == nil || mediaID == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&domain.PlaybackProgress{
		MediaID: mediaID,
		Time:    t,
	}).Error
}

// PrefsStore implementation

func (s *SQLite) GetAllPrefs(ctx context.Context) (map[string]string, error) {
	if s.db == nil {
		return map[string]string{}, nil
	}
	var prefs []domain.UserPref
	if err := s.db.WithContext(ctx).Find(&prefs).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(prefs))
	for _, p := range prefs {
		out[p.Key] = p.Value
	}
	return out, nil
}

func (s *SQLite) SetPrefs(ctx context.Context, kv map[string]string) error {
	if s.db == nil || len(kv) == 0 {
		return nil
	}

	prefs := make([]domain.UserPref, 0, len(kv))
	for k, v := range kv {
		if k == "" {
			continue
		}
		prefs = append(prefs, domain.UserPref{Key: k, Value: v})
	}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&prefs).Error
}

// Media-related database operations

func (s *SQLite) GetScanMeta(ctx context.Context, cacheKey string) (domain.MediaScan, bool, error) {
	if s.db == nil || cacheKey == "" {
		return domain.MediaScan{}, false, nil
	}
	var scan domain.MediaScan
	err := s.db.WithContext(ctx).First(&scan, "cache_key = ?", cacheKey).Error
	if err == gorm.ErrRecordNotFound {
		return domain.MediaScan{}, false, nil
	}
	return scan, true, err
}

func (s *SQLite) SetScanMeta(ctx context.Context, tx *gorm.DB, cacheKey string, meta domain.MediaScan) error {
	dbConn := s.db
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

func (s *SQLite) UpsertMediaItem(ctx context.Context, tx *gorm.DB, item *domain.MediaItem) error {
	dbConn := s.db
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil {
		return nil
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(item).Error
}

func (s *SQLite) UpsertMediaItems(ctx context.Context, tx *gorm.DB, items []domain.MediaItem) error {
	dbConn := s.db
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil || len(items) == 0 {
		return nil
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&items).Error
}

func (s *SQLite) DeleteStaleByScan(ctx context.Context, tx *gorm.DB, scanID int64, shareRoots []string) error {
	dbConn := s.db
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil || scanID <= 0 || len(shareRoots) == 0 {
		return nil
	}
	return dbConn.WithContext(ctx).Where("scan_id != ? AND share_root IN ?", scanID, shareRoots).Delete(&domain.MediaItem{}).Error
}

func (s *SQLite) DeleteByShareRootsNotIn(ctx context.Context, tx *gorm.DB, shareRoots []string) error {
	dbConn := s.db
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil {
		return nil
	}
	if len(shareRoots) == 0 {
		return dbConn.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&domain.MediaItem{}).Error
	}
	return dbConn.WithContext(ctx).Where("share_root NOT IN ?", shareRoots).Delete(&domain.MediaItem{}).Error
}

func (s *SQLite) QueryMediaItems(ctx context.Context, scanID int64, kind string) ([]domain.MediaItem, error) {
	if s.db == nil || scanID <= 0 || kind == "" {
		return nil, nil
	}
	var items []domain.MediaItem
	err := s.db.WithContext(ctx).
		Scopes(ByScan(scanID), ByKind(kind)).
		Order("share_label, lower(name)").
		Find(&items).Error
	return items, err
}

func (s *SQLite) CountMediaItems(ctx context.Context, scanID int64, kind string) (int, error) {
	if s.db == nil || scanID <= 0 || kind == "" {
		return 0, nil
	}
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.MediaItem{}).
		Scopes(ByScan(scanID), ByKind(kind)).
		Count(&count).Error
	return int(count), err
}

// Scopes

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