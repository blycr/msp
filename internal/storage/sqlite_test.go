package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*SQLite, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	sq, err := InitSQLite(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		sq.Close()
	}
	return sq, cleanup
}

func TestInitSQLite(t *testing.T) {
	t.Run("creates new database", func(t *testing.T) {
		sq, cleanup := setupTestDB(t)
		defer cleanup()

		assert.NotNil(t, sq.DB())
	})

	t.Run("creates directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir", "nested")
		dbPath := filepath.Join(subDir, "test.db")

		sq, err := InitSQLite(dbPath)
		require.NoError(t, err)
		defer sq.Close()

		_, err = os.Stat(subDir)
		assert.NoError(t, err)
	})
}

func TestProgress(t *testing.T) {
	sq, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get non-existent progress", func(t *testing.T) {
		val, err := sq.GetProgress(ctx, "non-existent-id")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), val)
	})

	t.Run("save and get progress", func(t *testing.T) {
		mediaID := "test-media-123"
		progress := 123.45

		err := sq.SetProgress(ctx, mediaID, progress)
		require.NoError(t, err)

		val, err := sq.GetProgress(ctx, mediaID)
		assert.NoError(t, err)
		assert.Equal(t, progress, val)
	})

	t.Run("update progress", func(t *testing.T) {
		mediaID := "test-media-update"

		err := sq.SetProgress(ctx, mediaID, 50.0)
		require.NoError(t, err)

		err = sq.SetProgress(ctx, mediaID, 75.5)
		require.NoError(t, err)

		val, err := sq.GetProgress(ctx, mediaID)
		assert.NoError(t, err)
		assert.Equal(t, 75.5, val)
	})

	t.Run("empty ID", func(t *testing.T) {
		err := sq.SetProgress(ctx, "", 100.0)
		assert.NoError(t, err)

		val, err := sq.GetProgress(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), val)
	})
}

func TestScanMeta(t *testing.T) {
	sq, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get non-existent scan meta", func(t *testing.T) {
		meta, found, err := sq.GetScanMeta(ctx, "non-existent-key")
		assert.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, meta.CacheKey)
	})

	t.Run("save and get scan meta", func(t *testing.T) {
		cacheKey := "scan-2024-01-01"
		meta := domain.MediaScan{
			CacheKey: cacheKey,
			ScanID:   1,
			BuiltAt:  time.Now().Unix(),
			Complete: true,
		}

		err := sq.SetScanMeta(ctx, nil, cacheKey, meta)
		require.NoError(t, err)

		retrieved, found, err := sq.GetScanMeta(ctx, cacheKey)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, meta.ScanID, retrieved.ScanID)
		assert.Equal(t, meta.Complete, retrieved.Complete)
	})

	t.Run("update scan meta", func(t *testing.T) {
		cacheKey := "scan-update-test"

		meta1 := domain.MediaScan{
			CacheKey: cacheKey,
			ScanID:   1,
			BuiltAt:  time.Now().Unix(),
			Complete: false,
		}
		err := sq.SetScanMeta(ctx, nil, cacheKey, meta1)
		require.NoError(t, err)

		meta2 := domain.MediaScan{
			CacheKey: cacheKey,
			ScanID:   2,
			BuiltAt:  time.Now().Unix(),
			Complete: true,
		}
		err = sq.SetScanMeta(ctx, nil, cacheKey, meta2)
		require.NoError(t, err)

		retrieved, found, err := sq.GetScanMeta(ctx, cacheKey)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(2), retrieved.ScanID)
		assert.True(t, retrieved.Complete)
	})
}

func TestMediaItems(t *testing.T) {
	sq, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("upsert single media item", func(t *testing.T) {
		item := &domain.MediaItem{
			ID:         "video-001",
			ScanID:     1,
			Kind:       "video",
			Name:       "Test Video",
			Path:       "/media/videos/test.mp4",
			ShareRoot:  "/media/videos",
			ShareLabel: "Videos",
			Size:       1024 * 1024 * 100,
		}

		err := sq.UpsertMediaItem(ctx, nil, item)
		assert.NoError(t, err)
	})

	t.Run("batch upsert media items", func(t *testing.T) {
		items := []domain.MediaItem{
			{ID: "audio-001", ScanID: 2, Kind: "audio", Name: "Test Audio 1", Path: "/media/music/song1.mp3", ShareRoot: "/media/music", ShareLabel: "Music", Size: 1024 * 1024 * 5},
			{ID: "audio-002", ScanID: 2, Kind: "audio", Name: "Test Audio 2", Path: "/media/music/song2.mp3", ShareRoot: "/media/music", ShareLabel: "Music", Size: 1024 * 1024 * 4},
		}

		err := sq.UpsertMediaItems(ctx, nil, items)
		assert.NoError(t, err)
	})

	t.Run("query media items", func(t *testing.T) {
		items := []domain.MediaItem{
			{ID: "q-video-1", ScanID: 10, Kind: "video", Name: "Video A", Path: "/v/a.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "q-video-2", ScanID: 10, Kind: "video", Name: "Video B", Path: "/v/b.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "q-audio-1", ScanID: 10, Kind: "audio", Name: "Audio A", Path: "/a/a.mp3", ShareRoot: "/a", ShareLabel: "A"},
		}
		err := sq.UpsertMediaItems(ctx, nil, items)
		require.NoError(t, err)

		videos, err := sq.QueryMediaItems(ctx, 10, "video")
		assert.NoError(t, err)
		assert.Len(t, videos, 2)

		audios, err := sq.QueryMediaItems(ctx, 10, "audio")
		assert.NoError(t, err)
		assert.Len(t, audios, 1)

		images, err := sq.QueryMediaItems(ctx, 10, "image")
		assert.NoError(t, err)
		assert.Len(t, images, 0)
	})
}

func TestUserPrefs(t *testing.T) {
	sq, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get empty prefs", func(t *testing.T) {
		prefs, err := sq.GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, prefs)
	})

	t.Run("save and get prefs", func(t *testing.T) {
		kv := map[string]string{
			"theme":     "dark",
			"language":  "zh-CN",
			"auto_play": "true",
		}

		err := sq.SetPrefs(ctx, kv)
		assert.NoError(t, err)

		prefs, err := sq.GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "dark", prefs["theme"])
		assert.Equal(t, "zh-CN", prefs["language"])
		assert.Equal(t, "true", prefs["auto_play"])
	})

	t.Run("empty key ignored", func(t *testing.T) {
		err := sq.SetPrefs(ctx, map[string]string{"": "empty-key", "valid": "value"})
		assert.NoError(t, err)

		prefs, err := sq.GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.NotContains(t, prefs, "")
		assert.Equal(t, "value", prefs["valid"])
	})
}

func TestTransaction(t *testing.T) {
	sq, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("operations in transaction", func(t *testing.T) {
		err := sq.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			item := &domain.MediaItem{
				ID: "tx-1", ScanID: 999, Kind: "video", Name: "Transaction Test",
				Path: "/tx/test.mp4", ShareRoot: "/tx", ShareLabel: "TX",
			}
			return sq.UpsertMediaItem(ctx, tx, item)
		})
		assert.NoError(t, err)

		items, err := sq.QueryMediaItems(ctx, 999, "video")
		assert.NoError(t, err)
		assert.Len(t, items, 1)
	})
}

func TestStoreWithNilDB(t *testing.T) {
	store := NewStore(nil)
	ctx := context.Background()

	t.Run("GetProgress returns 0", func(t *testing.T) {
		val, err := store.GetProgress(ctx, "test")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), val)
	})

	t.Run("SetProgress returns nil", func(t *testing.T) {
		err := store.SetProgress(ctx, "test", 100.0)
		assert.NoError(t, err)
	})

	t.Run("GetAllPrefs returns empty map", func(t *testing.T) {
		prefs, err := store.GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, prefs)
	})
}