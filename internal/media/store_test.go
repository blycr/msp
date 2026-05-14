package media

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMediaStore(t *testing.T) (*MediaProcessor, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	sq, err := storage.InitSQLite(dbPath)
	require.NoError(t, err)

	mp := NewMediaProcessor(sq)
	return mp, func() {
		sq.Close()
	}
}

func TestIsDBAvailable(t *testing.T) {
	t.Run("nil DB", func(t *testing.T) {
		mp := NewMediaProcessor(nil)
		assert.False(t, mp.IsDBAvailable())
	})

	t.Run("valid DB", func(t *testing.T) {
		mp, cleanup := setupTestMediaStore(t)
		defer cleanup()
		assert.True(t, mp.IsDBAvailable())
	})
}

func TestLoadMediaFromDBNoDB(t *testing.T) {
	mp := NewMediaProcessor(nil)
	ctx := context.Background()
	_, _, ok, err := mp.LoadMediaFromDB(ctx, "key", nil)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestLoadMediaFromDB(t *testing.T) {
	mp, cleanup := setupTestMediaStore(t)
	defer cleanup()

	ctx := context.Background()
	sq := mp.db
	scanID := time.Now().UnixNano()

	items := []domain.MediaItem{
		{ID: "v1", ScanID: scanID, Kind: "video", Name: "Video1", Path: "/v/1.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "a1", ScanID: scanID, Kind: "audio", Name: "Audio1", Path: "/a/1.mp3", ShareRoot: "/a", ShareLabel: "A"},
	}
	err := sq.UpsertMediaItems(ctx, nil, items)
	require.NoError(t, err)

	builtAt := time.Now()
	err = sq.SetScanMeta(ctx, nil, "test-key", domain.MediaScan{CacheKey: "test-key", ScanID: scanID, BuiltAt: builtAt.UnixNano(), Complete: true})
	require.NoError(t, err)

	shares := []domain.Share{{Label: "V", Path: "/v"}, {Label: "A", Path: "/a"}}
	resp, bt, ok, err := mp.LoadMediaFromDB(ctx, "test-key", shares)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, bt.IsZero())
	assert.Len(t, resp.Videos, 1)
	assert.Len(t, resp.Audios, 1)
}

func TestLoadMediaFromDBNoScanMeta(t *testing.T) {
	mp, cleanup := setupTestMediaStore(t)
	defer cleanup()

	ctx := context.Background()
	_, _, ok, err := mp.LoadMediaFromDB(ctx, "nonexistent-key", nil)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestLoadMediaResponseFromDBScan(t *testing.T) {
	mp, cleanup := setupTestMediaStore(t)
	defer cleanup()

	ctx := context.Background()
	sq := mp.db
	scanID := int64(12345)

	items := []domain.MediaItem{
		{ID: "v1", ScanID: scanID, Kind: "video", Name: "Video1", Path: "/v1.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "v2", ScanID: scanID, Kind: "video", Name: "Video2", Path: "/v2.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "a1", ScanID: scanID, Kind: "audio", Name: "Audio1", Path: "/a1.mp3", ShareRoot: "/a", ShareLabel: "A"},
		{ID: "i1", ScanID: scanID, Kind: "image", Name: "Image1", Path: "/i1.jpg", ShareRoot: "/i", ShareLabel: "I"},
		{ID: "o1", ScanID: scanID, Kind: "other", Name: "Other1", Path: "/o1.txt", ShareRoot: "/o", ShareLabel: "O"},
	}
	err := sq.UpsertMediaItems(ctx, nil, items)
	require.NoError(t, err)

	shares := []domain.Share{{Label: "V", Path: "/v"}}
	resp, err := mp.LoadMediaResponseFromDBScan(ctx, scanID, shares)
	assert.NoError(t, err)
	assert.Len(t, resp.Videos, 2)
	assert.Len(t, resp.Audios, 1)
	assert.Len(t, resp.Images, 1)
	assert.Len(t, resp.Others, 1)
	assert.Len(t, resp.Shares, 1)
}

func TestLoadMediaResponseFromDBScanEmpty(t *testing.T) {
	mp, cleanup := setupTestMediaStore(t)
	defer cleanup()

	ctx := context.Background()
	resp, err := mp.LoadMediaResponseFromDBScan(ctx, 999999, nil)
	assert.NoError(t, err)
	assert.Len(t, resp.Videos, 0)
	assert.Len(t, resp.Audios, 0)
}

func TestReindexAndLoadMediaNoDB(t *testing.T) {
	mp := NewMediaProcessor(nil)
	ctx := context.Background()
	_, _, err := mp.ReindexAndLoadMedia(ctx, "key", nil, config.BlacklistConfig{}, 0)
	assert.NoError(t, err)
}

func TestIndexMediaToDBNoDB(t *testing.T) {
	mp := NewMediaProcessor(nil)
	ctx := context.Background()
	_, _, _, err := mp.IndexMediaToDB(ctx, "key", nil, config.BlacklistConfig{}, 0)
	assert.NoError(t, err)
}
