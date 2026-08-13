package storage

import (
	"context"
	"errors"
	"log"
	"log/slog"
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

// ErrUnavailable is returned by storage methods when the database is not
// initialized. Callers can use errors.Is to detect explicit degradation.
var ErrUnavailable = errors.New("storage: database unavailable")

type SQLite struct {
	db *gorm.DB
}

// sqliteDSN puts per-connection PRAGMAs in the DSN so every pooled
// connection gets busy_timeout (and the rest), not just the first Exec.
func sqliteDSN(dbPath string) string {
	return "file:" + filepath.ToSlash(dbPath) +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=cache_size(-16000)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=mmap_size(268435456)"
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

	db, err := gorm.Open(sqlite.Open(sqliteDSN(dbPath)), &gorm.Config{
		Logger:                 NewGormLogger(),
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err == nil {
		// busy_timeout and friends are per-connection; they are in the DSN so
		// pooled conns inherit them. Cap the pool — GOMAXPROCS writers fight
		// SQLite's single-writer lock during a scan.
		sqlDB.SetMaxOpenConns(4)
		sqlDB.SetMaxIdleConns(2)

		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			slog.Warn("failed to set WAL mode", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
			slog.Warn("failed to set synchronous mode", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA cache_size=-16000;"); err != nil {
			slog.Warn("failed to set cache size", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000;"); err != nil {
			slog.Warn("failed to set busy timeout", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA temp_store=MEMORY;"); err != nil {
			slog.Warn("failed to set temp store", "err", err)
		}
		if _, err := sqlDB.Exec("PRAGMA mmap_size=268435456;"); err != nil {
			slog.Warn("failed to set mmap size", "err", err)
		}
	}

	if err := db.AutoMigrate(&domain.MediaItem{}, &domain.MediaScan{}, &domain.UserPref{}, &domain.PlaybackProgress{}, &domain.Favorite{}); err != nil {
		return nil, err
	}

	// Expression indexes for common sort/filter patterns.
	db.Exec("CREATE INDEX IF NOT EXISTS idx_media_item_lower_name ON media_items(LOWER(name))")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_media_item_sort ON media_items(scan_id, kind, share_label, name)")

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

// guard returns the database connection if initialized.
func (s *SQLite) guard(name string) (*gorm.DB, bool) {
	if s.db == nil {
		slog.Warn("database not initialized", "op", "SQLite."+name)
		return nil, false
	}
	return s.db, true
}

// guardTx returns the transaction if provided, otherwise the main DB connection.
func (s *SQLite) guardTx(tx *gorm.DB, name string) (*gorm.DB, bool) {
	dbConn := s.db
	if tx != nil {
		dbConn = tx
	}
	if dbConn == nil {
		slog.Warn("database not initialized", "op", "SQLite."+name)
		return nil, false
	}
	return dbConn, true
}

func (s *SQLite) DB() *gorm.DB {
	return s.db
}

// ProgressStore implementation

func (s *SQLite) GetProgress(ctx context.Context, mediaID string) (float64, error) {
	dbConn, ok := s.guard("GetProgress")
	if !ok {
		return 0, ErrUnavailable
	}
	if mediaID == "" {
		return 0, nil
	}
	var p domain.PlaybackProgress
	err := dbConn.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).WithContext(ctx).First(&p, "media_id = ?", mediaID).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return p.Time, err
}

func (s *SQLite) SetProgress(ctx context.Context, mediaID string, t float64) error {
	dbConn, ok := s.guard("SetProgress")
	if !ok {
		return ErrUnavailable
	}
	if mediaID == "" {
		return nil
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&domain.PlaybackProgress{
		MediaID: mediaID,
		Time:    t,
	}).Error
}

func (s *SQLite) DeleteProgress(ctx context.Context, mediaID string) error {
	dbConn, ok := s.guard("DeleteProgress")
	if !ok {
		return ErrUnavailable
	}
	if mediaID == "" {
		return nil
	}
	return dbConn.WithContext(ctx).Delete(&domain.PlaybackProgress{}, "media_id = ?", mediaID).Error
}

func (s *SQLite) ListAllProgress(ctx context.Context) ([]domain.PlaybackProgress, error) {
	dbConn, ok := s.guard("ListAllProgress")
	if !ok {
		return nil, ErrUnavailable
	}
	var list []domain.PlaybackProgress
	if err := dbConn.WithContext(ctx).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *SQLite) ListRecentProgress(ctx context.Context, limit int) ([]domain.PlaybackProgress, error) {
	dbConn, ok := s.guard("ListRecentProgress")
	if !ok {
		return nil, ErrUnavailable
	}
	if limit <= 0 {
		limit = 10
	}
	var items []domain.PlaybackProgress
	err := dbConn.WithContext(ctx).
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// PrefsStore implementation

func (s *SQLite) GetAllPrefs(ctx context.Context) (map[string]string, error) {
	dbConn, ok := s.guard("GetAllPrefs")
	if !ok {
		return nil, ErrUnavailable
	}
	var prefs []domain.UserPref
	if err := dbConn.WithContext(ctx).Find(&prefs).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(prefs))
	for _, p := range prefs {
		out[p.Key] = p.Value
	}
	return out, nil
}

func (s *SQLite) SetPrefs(ctx context.Context, kv map[string]string) error {
	dbConn, ok := s.guard("SetPrefs")
	if !ok {
		return ErrUnavailable
	}
	if len(kv) == 0 {
		return nil
	}

	prefs := make([]domain.UserPref, 0, len(kv))
	for k, v := range kv {
		if k == "" {
			continue
		}
		prefs = append(prefs, domain.UserPref{Key: k, Value: v})
	}

	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&prefs).Error
}

// Media-related database operations

func (s *SQLite) GetScanMeta(ctx context.Context, cacheKey string) (domain.MediaScan, bool, error) {
	dbConn, ok := s.guard("GetScanMeta")
	if !ok {
		return domain.MediaScan{}, false, ErrUnavailable
	}
	if cacheKey == "" {
		return domain.MediaScan{}, false, nil
	}
	var scan domain.MediaScan
	err := dbConn.WithContext(ctx).First(&scan, "cache_key = ?", cacheKey).Error
	if err == gorm.ErrRecordNotFound {
		return domain.MediaScan{}, false, nil
	}
	return scan, true, err
}

func (s *SQLite) SetScanMeta(ctx context.Context, tx *gorm.DB, cacheKey string, meta domain.MediaScan) error {
	dbConn, ok := s.guardTx(tx, "SetScanMeta")
	if !ok {
		return ErrUnavailable
	}
	if cacheKey == "" {
		return nil
	}
	meta.CacheKey = cacheKey
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&meta).Error
}

func (s *SQLite) UpsertMediaItem(ctx context.Context, tx *gorm.DB, item *domain.MediaItem) error {
	dbConn, ok := s.guardTx(tx, "UpsertMediaItem")
	if !ok {
		return ErrUnavailable
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(item).Error
}

func (s *SQLite) UpsertMediaItems(ctx context.Context, tx *gorm.DB, items []domain.MediaItem) error {
	dbConn, ok := s.guardTx(tx, "UpsertMediaItems")
	if !ok {
		return ErrUnavailable
	}
	if len(items) == 0 {
		return nil
	}
	return dbConn.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&items).Error
}

func (s *SQLite) DeleteStaleByScan(ctx context.Context, tx *gorm.DB, scanID int64, shareRoots []string) error {
	dbConn, ok := s.guardTx(tx, "DeleteStaleByScan")
	if !ok {
		return ErrUnavailable
	}
	if scanID <= 0 || len(shareRoots) == 0 {
		return nil
	}
	return dbConn.WithContext(ctx).Where("scan_id != ? AND share_root IN ?", scanID, shareRoots).Delete(&domain.MediaItem{}).Error
}

func (s *SQLite) DeleteByShareRootsNotIn(ctx context.Context, tx *gorm.DB, shareRoots []string) error {
	dbConn, ok := s.guardTx(tx, "DeleteByShareRootsNotIn")
	if !ok {
		return ErrUnavailable
	}
	if len(shareRoots) == 0 {
		slog.Warn("SQLite.DeleteByShareRootsNotIn: deleting all media items (no active shares)")
		return dbConn.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&domain.MediaItem{}).Error
	}
	return dbConn.WithContext(ctx).Where("share_root NOT IN ?", shareRoots).Delete(&domain.MediaItem{}).Error
}

func (s *SQLite) QueryMediaItems(ctx context.Context, scanID int64, kind string) ([]domain.MediaItem, error) {
	dbConn, ok := s.guard("QueryMediaItems")
	if !ok {
		return nil, ErrUnavailable
	}
	if scanID <= 0 || kind == "" {
		return nil, nil
	}
	var items []domain.MediaItem
	err := dbConn.WithContext(ctx).
		Scopes(ByScan(scanID), ByKind(kind)).
		Order("share_label, lower(name)").
		Find(&items).Error
	return items, err
}

func (s *SQLite) CountMediaItems(ctx context.Context, scanID int64, kind string) (int, error) {
	dbConn, ok := s.guard("CountMediaItems")
	if !ok {
		return 0, ErrUnavailable
	}
	if scanID <= 0 || kind == "" {
		return 0, nil
	}
	var count int64
	err := dbConn.WithContext(ctx).Model(&domain.MediaItem{}).
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

// FavoriteStore implementation

func (s *SQLite) ListFavorites(ctx context.Context) ([]domain.Favorite, error) {
	dbConn, ok := s.guard("ListFavorites")
	if !ok {
		return nil, ErrUnavailable
	}
	var items []domain.Favorite
	err := dbConn.WithContext(ctx).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (s *SQLite) AddFavorite(ctx context.Context, mediaID string) error {
	dbConn, ok := s.guard("AddFavorite")
	if !ok {
		return ErrUnavailable
	}
	return dbConn.WithContext(ctx).
		Where(domain.Favorite{MediaID: mediaID}).
		FirstOrCreate(&domain.Favorite{MediaID: mediaID}).Error
}

func (s *SQLite) RemoveFavorite(ctx context.Context, mediaID string) error {
	dbConn, ok := s.guard("RemoveFavorite")
	if !ok {
		return ErrUnavailable
	}
	return dbConn.WithContext(ctx).Delete(&domain.Favorite{}, "media_id = ?", mediaID).Error
}

func (s *SQLite) IsFavorite(ctx context.Context, mediaID string) (bool, error) {
	dbConn, ok := s.guard("IsFavorite")
	if !ok {
		return false, ErrUnavailable
	}
	var count int64
	err := dbConn.WithContext(ctx).Model(&domain.Favorite{}).Where("media_id = ?", mediaID).Count(&count).Error
	return count > 0, err
}
