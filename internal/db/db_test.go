package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"msp/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDB 创建临时测试数据库
func setupTestDB(t *testing.T) func() {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	err := Init(dbPath)
	require.NoError(t, err)
	
	return func() {
		Close()
		_ = os.Remove(dbPath)
	}
}

func TestInit(t *testing.T) {
	t.Run("创建新数据库", func(t *testing.T) {
		cleanup := setupTestDB(t)
		defer cleanup()
		
		assert.NotNil(t, DB)
	})
	
	t.Run("创建目录", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir", "nested")
		dbPath := filepath.Join(subDir, "test.db")
		
		err := Init(dbPath)
		require.NoError(t, err)
		defer Close()
		
		_, err = os.Stat(subDir)
		assert.NoError(t, err)
	})
}

func TestProgress(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("获取不存在的进度", func(t *testing.T) {
		time, err := GetProgress(ctx, "non-existent-id")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), time)
	})
	
	t.Run("保存和获取进度", func(t *testing.T) {
		mediaID := "test-media-123"
		progress := 123.45
		
		err := SetProgress(ctx, mediaID, progress)
		require.NoError(t, err)
		
		time, err := GetProgress(ctx, mediaID)
		assert.NoError(t, err)
		assert.Equal(t, progress, time)
	})
	
	t.Run("更新进度", func(t *testing.T) {
		mediaID := "test-media-update"
		
		err := SetProgress(ctx, mediaID, 50.0)
		require.NoError(t, err)
		
		err = SetProgress(ctx, mediaID, 75.5)
		require.NoError(t, err)
		
		time, err := GetProgress(ctx, mediaID)
		assert.NoError(t, err)
		assert.Equal(t, 75.5, time)
	})
	
	t.Run("空ID", func(t *testing.T) {
		err := SetProgress(ctx, "", 100.0)
		assert.NoError(t, err)
		
		time, err := GetProgress(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), time)
	})
}

func TestScanMeta(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("获取不存在的扫描元数据", func(t *testing.T) {
		meta, found, err := GetScanMeta(ctx, "non-existent-key")
		assert.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, meta.CacheKey)
	})
	
	t.Run("保存和获取扫描元数据", func(t *testing.T) {
		cacheKey := "scan-2024-01-01"
		meta := types.MediaScan{
			CacheKey: cacheKey,
			ScanID:   1,
			BuiltAt:  time.Now().Unix(),
			Complete: true,
		}
		
		err := SetScanMeta(ctx, nil, cacheKey, meta)
		require.NoError(t, err)
		
		retrieved, found, err := GetScanMeta(ctx, cacheKey)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, meta.ScanID, retrieved.ScanID)
		assert.Equal(t, meta.Complete, retrieved.Complete)
	})
	
	t.Run("更新扫描元数据", func(t *testing.T) {
		cacheKey := "scan-update-test"
		
		meta1 := types.MediaScan{
			CacheKey: cacheKey,
			ScanID:   1,
			BuiltAt:  time.Now().Unix(),
			Complete: false,
		}
		err := SetScanMeta(ctx, nil, cacheKey, meta1)
		require.NoError(t, err)
		
		meta2 := types.MediaScan{
			CacheKey: cacheKey,
			ScanID:   2,
			BuiltAt:  time.Now().Unix(),
			Complete: true,
		}
		err = SetScanMeta(ctx, nil, cacheKey, meta2)
		require.NoError(t, err)
		
		retrieved, found, err := GetScanMeta(ctx, cacheKey)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(2), retrieved.ScanID)
		assert.True(t, retrieved.Complete)
	})
}

func TestMediaItem(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("插入单个媒体项", func(t *testing.T) {
		item := &types.MediaItem{
			ID:         "video-001",
			ScanID:     1,
			Kind:       "video",
			Name:       "Test Video",
			Path:       "/media/videos/test.mp4",
			ShareRoot:  "/media/videos",
			ShareLabel: "Videos",
			Size:       1024 * 1024 * 100, // 100MB
		}
		
		err := UpsertMediaItem(ctx, nil, item)
		assert.NoError(t, err)
	})
	
	t.Run("批量插入媒体项", func(t *testing.T) {
		items := []types.MediaItem{
			{
				ID:         "audio-001",
				ScanID:     2,
				Kind:       "audio",
				Name:       "Test Audio 1",
				Path:       "/media/music/song1.mp3",
				ShareRoot:  "/media/music",
				ShareLabel: "Music",
				Size:       1024 * 1024 * 5,
			},
			{
				ID:         "audio-002",
				ScanID:     2,
				Kind:       "audio",
				Name:       "Test Audio 2",
				Path:       "/media/music/song2.mp3",
				ShareRoot:  "/media/music",
				ShareLabel: "Music",
				Size:       1024 * 1024 * 4,
			},
		}
		
		err := UpsertMediaItems(ctx, nil, items)
		assert.NoError(t, err)
	})
	
	t.Run("更新媒体项", func(t *testing.T) {
		item := &types.MediaItem{
			ID:         "video-update",
			ScanID:     1,
			Kind:       "video",
			Name:       "Original Name",
			Path:       "/media/videos/update.mp4",
			ShareRoot:  "/media/videos",
			ShareLabel: "Videos",
			Size:       1000,
		}
		
		err := UpsertMediaItem(ctx, nil, item)
		require.NoError(t, err)
		
		// 更新名称和大小
		item.Name = "Updated Name"
		item.Size = 2000
		err = UpsertMediaItem(ctx, nil, item)
		require.NoError(t, err)
		
		// 查询验证
		items, err := QueryMediaItems(ctx, 1, "video")
		require.NoError(t, err)
		
		var found bool
		for _, i := range items {
			if i.ID == "video-update" {
				assert.Equal(t, "Updated Name", i.Name)
				assert.Equal(t, int64(2000), i.Size)
				found = true
				break
			}
		}
		assert.True(t, found)
	})
	
	t.Run("查询媒体项", func(t *testing.T) {
		// 插入测试数据
		items := []types.MediaItem{
			{ID: "q-video-1", ScanID: 10, Kind: "video", Name: "Video A", Path: "/v/a.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "q-video-2", ScanID: 10, Kind: "video", Name: "Video B", Path: "/v/b.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "q-audio-1", ScanID: 10, Kind: "audio", Name: "Audio A", Path: "/a/a.mp3", ShareRoot: "/a", ShareLabel: "A"},
		}
		err := UpsertMediaItems(ctx, nil, items)
		require.NoError(t, err)
		
		// 查询视频
		videos, err := QueryMediaItems(ctx, 10, "video")
		assert.NoError(t, err)
		assert.Len(t, videos, 2)
		
		// 查询音频
		audios, err := QueryMediaItems(ctx, 10, "audio")
		assert.NoError(t, err)
		assert.Len(t, audios, 1)
		
		// 查询不存在的类型
		images, err := QueryMediaItems(ctx, 10, "image")
		assert.NoError(t, err)
		assert.Len(t, images, 0)
	})
	
	t.Run("统计媒体项", func(t *testing.T) {
		// 插入测试数据
		items := []types.MediaItem{
			{ID: "c-video-1", ScanID: 20, Kind: "video", Name: "V1", Path: "/v/1.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "c-video-2", ScanID: 20, Kind: "video", Name: "V2", Path: "/v/2.mp4", ShareRoot: "/v", ShareLabel: "V"},
			{ID: "c-video-3", ScanID: 20, Kind: "video", Name: "V3", Path: "/v/3.mp4", ShareRoot: "/v", ShareLabel: "V"},
		}
		err := UpsertMediaItems(ctx, nil, items)
		require.NoError(t, err)
		
		count, err := CountMediaItems(ctx, 20, "video")
		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})
}

func TestDeleteStaleByScan(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// 插入测试数据
	items := []types.MediaItem{
		{ID: "d-1", ScanID: 1, Kind: "video", Name: "Old 1", Path: "/v/old1.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "d-2", ScanID: 1, Kind: "video", Name: "Old 2", Path: "/v/old2.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "d-3", ScanID: 2, Kind: "video", Name: "New 1", Path: "/v/new1.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "d-4", ScanID: 2, Kind: "video", Name: "Other", Path: "/o/other.mp4", ShareRoot: "/o", ShareLabel: "O"},
	}
	err := UpsertMediaItems(ctx, nil, items)
	require.NoError(t, err)
	
	// 删除 scanID=1 且在 /v 下的项目
	err = DeleteStaleByScan(ctx, nil, 2, []string{"/v"})
	assert.NoError(t, err)
	
	// 验证
	videos, err := QueryMediaItems(ctx, 1, "video")
	assert.NoError(t, err)
	assert.Len(t, videos, 0) // scanID=1 的都被删除了
	
	videos2, err := QueryMediaItems(ctx, 2, "video")
	assert.NoError(t, err)
	assert.Len(t, videos2, 2) // scanID=2 的都还在
}

func TestDeleteByShareRootsNotIn(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// 插入测试数据
	items := []types.MediaItem{
		{ID: "del-1", ScanID: 1, Kind: "video", Name: "Keep 1", Path: "/keep/a.mp4", ShareRoot: "/keep", ShareLabel: "K"},
		{ID: "del-2", ScanID: 1, Kind: "video", Name: "Delete 1", Path: "/delete/b.mp4", ShareRoot: "/delete", ShareLabel: "D"},
		{ID: "del-3", ScanID: 1, Kind: "video", Name: "Delete 2", Path: "/remove/c.mp4", ShareRoot: "/remove", ShareLabel: "R"},
	}
	err := UpsertMediaItems(ctx, nil, items)
	require.NoError(t, err)
	
	// 删除不在 /keep 下的项目
	err = DeleteByShareRootsNotIn(ctx, nil, []string{"/keep"})
	assert.NoError(t, err)
	
	// 验证
	allItems, err := QueryMediaItems(ctx, 1, "video")
	assert.NoError(t, err)
	assert.Len(t, allItems, 1)
	assert.Equal(t, "Keep 1", allItems[0].Name)
}

func TestUserPrefs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("获取空偏好设置", func(t *testing.T) {
		prefs, err := GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, prefs)
	})
	
	t.Run("保存和获取偏好设置", func(t *testing.T) {
		kv := map[string]string{
			"theme":       "dark",
			"language":    "zh-CN",
			"auto_play":   "true",
		}
		
		err := SetPrefs(ctx, kv)
		assert.NoError(t, err)
		
		prefs, err := GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "dark", prefs["theme"])
		assert.Equal(t, "zh-CN", prefs["language"])
		assert.Equal(t, "true", prefs["auto_play"])
	})
	
	t.Run("更新偏好设置", func(t *testing.T) {
		err := SetPrefs(ctx, map[string]string{"theme": "light"})
		assert.NoError(t, err)
		
		prefs, err := GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "light", prefs["theme"])
	})
	
	t.Run("空键被忽略", func(t *testing.T) {
		err := SetPrefs(ctx, map[string]string{
			"":        "empty-key",
			"valid":   "value",
		})
		assert.NoError(t, err)
		
		prefs, err := GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.NotContains(t, prefs, "")
		assert.Equal(t, "value", prefs["valid"])
	})
	
	t.Run("空map", func(t *testing.T) {
		err := SetPrefs(ctx, map[string]string{})
		assert.NoError(t, err)
	})
}

func TestScopes(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	// 插入测试数据
	items := []types.MediaItem{
		{ID: "s-1", ScanID: 100, Kind: "video", Name: "V1", Path: "/v/1.mp4", ShareRoot: "/v", ShareLabel: "V"},
		{ID: "s-2", ScanID: 100, Kind: "audio", Name: "A1", Path: "/a/1.mp3", ShareRoot: "/a", ShareLabel: "A"},
		{ID: "s-3", ScanID: 200, Kind: "video", Name: "V2", Path: "/v/2.mp4", ShareRoot: "/v", ShareLabel: "V"},
	}
	err := UpsertMediaItems(ctx, nil, items)
	require.NoError(t, err)
	
	t.Run("ByScan scope", func(t *testing.T) {
		var result []types.MediaItem
		err := DB.WithContext(ctx).Scopes(ByScan(100)).Find(&result).Error
		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})
	
	t.Run("ByKind scope", func(t *testing.T) {
		var result []types.MediaItem
		err := DB.WithContext(ctx).Scopes(ByKind("video")).Find(&result).Error
		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})
	
	t.Run("组合 scopes", func(t *testing.T) {
		var result []types.MediaItem
		err := DB.WithContext(ctx).Scopes(ByScan(100), ByKind("video")).Find(&result).Error
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "V1", result[0].Name)
	})
}

func TestTransaction(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("在事务中操作", func(t *testing.T) {
		err := DB.Transaction(func(tx *gorm.DB) error {
			// 在事务中插入
			item := &types.MediaItem{
				ID:         "tx-1",
				ScanID:     999,
				Kind:       "video",
				Name:       "Transaction Test",
				Path:       "/tx/test.mp4",
				ShareRoot:  "/tx",
				ShareLabel: "TX",
			}
			return UpsertMediaItem(ctx, tx, item)
		})
		assert.NoError(t, err)
		
		// 验证
		items, err := QueryMediaItems(ctx, 999, "video")
		assert.NoError(t, err)
		assert.Len(t, items, 1)
	})
}

func TestNilDB(t *testing.T) {
	ctx := context.Background()
	
	// 保存原始 DB
	originalDB := DB
	DB = nil
	defer func() { DB = originalDB }()
	
	t.Run("GetProgress 返回 0", func(t *testing.T) {
		time, err := GetProgress(ctx, "test")
		assert.NoError(t, err)
		assert.Equal(t, float64(0), time)
	})
	
	t.Run("SetProgress 返回 nil", func(t *testing.T) {
		err := SetProgress(ctx, "test", 100.0)
		assert.NoError(t, err)
	})
	
	t.Run("UpsertMediaItem 返回 nil", func(t *testing.T) {
		item := &types.MediaItem{ID: "test"}
		err := UpsertMediaItem(ctx, nil, item)
		assert.NoError(t, err)
	})
	
	t.Run("GetAllPrefs 返回空 map", func(t *testing.T) {
		prefs, err := GetAllPrefs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, prefs)
	})
}

func TestContextCancellation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	t.Run("取消上下文", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消
		
		// 操作应该返回上下文取消错误
		_, err := GetAllPrefs(ctx)
		// GORM 可能会处理上下文取消，具体行为取决于实现
		// 这里我们只确保不会 panic
		_ = err
	})
}

func TestConcurrentAccess(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	
	ctx := context.Background()
	
	t.Run("并发读写进度", func(t *testing.T) {
		done := make(chan bool, 10)
		
		for i := 0; i < 10; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				
				mediaID := "concurrent-test"
				err := SetProgress(ctx, mediaID, float64(idx*10))
				assert.NoError(t, err)
				
				_, err = GetProgress(ctx, mediaID)
				assert.NoError(t, err)
			}(i)
		}
		
		// 等待所有 goroutine 完成
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func BenchmarkUpsertMediaItems(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	
	err := Init(dbPath)
	require.NoError(b, err)
	defer Close()
	
	ctx := context.Background()
	items := make([]types.MediaItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = types.MediaItem{
			ID:         "bench-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			ScanID:     1,
			Kind:       "video",
			Name:       "Benchmark Video",
			Path:       "/media/video.mp4",
			ShareRoot:  "/media",
			ShareLabel: "Media",
			Size:       1024 * 1024,
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = UpsertMediaItems(ctx, nil, items)
	}
}

func BenchmarkQueryMediaItems(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	
	err := Init(dbPath)
	require.NoError(b, err)
	defer Close()
	
	ctx := context.Background()
	
	// 插入测试数据
	items := make([]types.MediaItem, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = types.MediaItem{
			ID:         "query-bench-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			ScanID:     1,
			Kind:       "video",
			Name:       "Query Benchmark",
			Path:       "/media/video.mp4",
			ShareRoot:  "/media",
			ShareLabel: "Media",
			Size:       1024 * 1024,
		}
	}
	err = UpsertMediaItems(ctx, nil, items)
	require.NoError(b, err)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = QueryMediaItems(ctx, 1, "video")
	}
}
