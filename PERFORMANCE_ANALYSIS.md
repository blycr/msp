# MSP 项目性能分析报告

> **⚠️ 历史文档说明**  
> 本文档创建于 2026-01，记录了特定时期的性能分析和优化建议。  
> 部分建议可能已被实施，部分内容可能已过时。  
> 请以最新代码和实际测试为准，本文档仅供参考。

## 执行摘要

经过对 MSP 项目的全面性能分析，发现整体架构设计良好，已实施了多项性能优化措施。本报告识别了 5 个关键优化领域，并提供了具体的改进建议。

---

## 1. 数据库性能分析

### 1.1 当前状态

**优点：**
- ✅ 使用了 SQLite WAL 模式 (`PRAGMA journal_mode=WAL`)，提高并发写入性能
- ✅ 启用了预编译语句缓存 (`PrepareStmt: true`)
- ✅ 禁用了默认事务以提高写入性能 (`SkipDefaultTransaction: true`)
- ✅ 合理设置连接池：单连接模式适合 SQLite (`SetMaxOpenConns(1)`)
- ✅ 设置了合理的缓存大小 (`PRAGMA cache_size=-2000`，约 2MB)
- ✅ 使用了复合索引优化查询

**潜在问题：**

| 问题 | 位置 | 影响 | 建议 |
|------|------|------|------|
| 缺少 `idx_scan_id` 单独索引 | `types.go:28` | 中等 | 已使用复合索引，但单独索引可能更优 |
| 频繁的单条 UPSERT 操作 | `store.go:122` | 高 | 考虑批量插入 |
| 没有使用 `PRAGMA temp_store=memory` | `db.go` | 低 | 可提高临时表性能 |
| 缺少 `PRAGMA mmap_size` 配置 | `db.go` | 中 | 内存映射可提升读取性能 |

### 1.2 索引分析

```go
// 当前索引结构 (types.go)
type MediaItem struct {
    ID         string     `gorm:"primaryKey"`
    Path       string     `gorm:"uniqueIndex;not null"`
    Kind       string     `gorm:"index:idx_kind;index:idx_scan_kind"`
    ShareLabel string     `gorm:"index:idx_share_label;index:idx_scan_share_label"`
    ScanID     int64      `gorm:"index:idx_scan_id;index:idx_scan_kind;index:idx_scan_share_label"`
}
```

**索引优化建议：**

```go
// 建议添加的索引
// 1. 用于清理过期数据的复合索引
`gorm:"index:idx_cleanup, columns:scan_id,share_root"`

// 2. 用于排序查询的复合索引  
`gorm:"index:idx_sort, columns:scan_id,kind,share_label,name"`
```

### 1.3 代码改进方案

**文件: `internal/db/db.go`**

```go
// 在 Init 函数中添加更多性能优化
func Init(dbPath string) error {
    // ... 现有代码 ...
    
    if _, err := sqlDB.Exec("PRAGMA temp_store=memory;"); err != nil {
        log.Printf("DB Warn: failed to set temp_store: %v", err)
    }
    
    // 启用内存映射 I/O (64MB)
    if _, err := sqlDB.Exec("PRAGMA mmap_size=67108864;"); err != nil {
        log.Printf("DB Warn: failed to set mmap_size: %v", err)
    }
    
    // 优化查询规划器
    if _, err := sqlDB.Exec("PRAGMA optimize;"); err != nil {
        log.Printf("DB Warn: failed to optimize: %v", err)
    }
    
    return nil
}
```

**文件: `internal/media/store.go`**

```go
// 批量插入优化 - 修改 performScan 函数
func performScan(ctx context.Context, tx *gorm.DB, scanID int64, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) (int, error) {
    seen := 0
    limit := maxItems
    if limit <= 0 {
        limit = constants.DBScanLimit
    }

    // 使用批量插入缓冲区
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
```

**文件: `internal/db/db.go`** - 添加批量插入函数

```go
// UpsertMediaItems 批量插入或更新媒体条目
func UpsertMediaItems(ctx context.Context, tx *gorm.DB, items []types.MediaItem) error {
    dbConn := DB
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
```

---

## 2. 媒体处理性能分析

### 2.1 转码效率

**当前状态：**
- ✅ 限制并发转码会话数为 2 (`transcodeLimit = make(chan struct{}, 2)`)
- ✅ 智能转码策略：H.264 视频和 AAC/MP3 音频直接复制
- ✅ 使用信号量控制并发，防止 CPU 过载
- ✅ 支持从指定偏移量开始转码

**优化建议：**

```go
// 文件: internal/media/transcoder.go

// 1. 添加转码缓存，避免重复转码相同文件
var transcodeCache = struct {
    sync.RWMutex
    items map[string]cacheEntry
}{items: make(map[string]cacheEntry)}

type cacheEntry struct {
    codecInfo CodecInfo
    timestamp time.Time
}

// 2. 优化 FFmpeg 参数
func buildFFmpegArgs(inputPath string, codec CodecInfo, opts TranscodeOptions) []string {
    args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
    
    if opts.Offset > 0 {
        args = append(args, "-ss", fmt.Sprintf("%f", opts.Offset))
    }
    
    args = append(args, "-i", inputPath)
    
    // 视频优化
    if codec.VideoCodec == "h264" {
        args = append(args, "-vcodec", "copy")
    } else {
        // 使用更快的预设
        args = append(args, "-vcodec", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p")
        if opts.Bitrate != "" {
            args = append(args, "-b:v", opts.Bitrate)
        }
        // 限制线程数以避免阻塞
        args = append(args, "-threads", "2")
    }
    
    // 音频优化
    if codec.AudioCodec == "aac" || codec.AudioCodec == "mp3" {
        args = append(args, "-acodec", "copy")
    } else {
        args = append(args, "-acodec", "aac", "-b:a", "128k")
    }
    
    // 优化输出格式
    args = append(args, 
        "-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart",
        "-f", opts.Format,
        "-map_metadata", "-1",
        "pipe:1",
    )
    
    return args
}
```

### 2.2 流媒体传输优化

**当前状态：**
- ✅ 支持 HTTP Range 请求 (`Accept-Ranges: bytes`)
- ✅ 使用 `http.ServeContent` 处理静态文件
- ✅ 转码流使用 chunked transfer encoding

**优化建议：**

```go
// 文件: internal/handler/handlers.go

// 1. 添加流缓冲区池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024) // 32KB 缓冲区
    },
}

// 2. 优化 serveDirect 函数
func (h *Handler) serveDirect(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo, ct string) {
    w.Header().Set("Content-Type", ct)
    w.Header().Set("Accept-Ranges", "bytes")
    
    // 大文件使用更长的缓存
    if st.Size() > 10*1024*1024 { // > 10MB
        w.Header().Set("Cache-Control", "private, max-age=3600") // 1小时
    } else {
        w.Header().Set("Cache-Control", "no-store")
    }
    
    w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", st.Name()))
    
    // 使用缓冲区池优化传输
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    
    http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}
```

### 2.3 缓存策略

**当前状态：**
- ✅ 媒体列表缓存 2 分钟 (`MediaCacheTTL = 2 * time.Minute`)
- ✅ 支持后台重建缓存
- ✅ 使用 ETag 实现条件请求
- ✅ 内存中缓存序列化后的 JSON 数据

**优化建议：**

```go
// 文件: internal/server/server.go

// 1. 添加分层缓存策略
const (
    MemoryCacheTTL = 2 * time.Minute
    DiskCacheTTL   = 1 * time.Hour
)

// 2. 优化缓存键生成
func mediaCacheKey(shares []config.Share, blacklist config.BlacklistConfig) string {
    // 使用更快的哈希算法
    h := xxhash.New()
    
    // 写入 shares
    for _, sh := range shares {
        h.WriteString(sh.Path)
        h.WriteString(sh.Label)
    }
    
    // 写入黑名单规则
    for _, ext := range blacklist.Extensions {
        h.WriteString(ext)
    }
    
    return strconv.FormatUint(h.Sum64(), 36)
}
```

---

## 3. 内存使用分析

### 3.1 大文件处理

**当前状态：**
- ✅ 字幕转换使用流式处理（但 SRT/ASS 仍读取整个文件）
- ✅ 媒体嗅探限制读取 2MB (`max = 2 << 20`)
- ✅ 转码使用流式输出
- ✅ 主动调用 `debug.FreeOSMemory()` 释放内存

**问题与优化：**

```go
// 文件: internal/handler/handlers.go

// 1. 大文件字幕处理优化
func (h *Handler) serveSRT(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
    // 大文件使用流式转换
    if st.Size() > 1024*1024 { // > 1MB
        h.serveSRTStreaming(w, r, f, st)
        return
    }
    
    // 小文件使用内存转换
    b, err := io.ReadAll(f)
    if err != nil {
        http.Error(w, constants.ErrMsgReadFailed, http.StatusInternalServerError)
        return
    }
    out := media.SrtToVtt(b)
    w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
    w.Header().Set("Cache-Control", "private, max-age=0")
    http.ServeContent(w, r, strings.TrimSuffix(st.Name(), filepath.Ext(st.Name()))+".vtt", st.ModTime(), bytes.NewReader(out))
}

// 2. 流式 SRT 转换
func (h *Handler) serveSRTStreaming(w http.ResponseWriter, r *http.Request, f *os.File, st os.FileInfo) {
    w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
    w.Header().Set("Cache-Control", "private, max-age=0")
    w.Write([]byte("WEBVTT\n\n"))
    
    scanner := bufio.NewScanner(f)
    scanner.Buffer(make([]byte, 1024), 1024*1024) // 最大 1MB 行
    
    var buf strings.Builder
    for scanner.Scan() {
        line := scanner.Text()
        trimmed := strings.TrimSpace(line)
        
        if trimmed == "" {
            buf.WriteString("\n")
            continue
        }
        if media.IsAllDigits(trimmed) {
            continue
        }
        if strings.Contains(line, "-->") {
            buf.WriteString(strings.ReplaceAll(line, ",", "."))
            buf.WriteString("\n")
            continue
        }
        buf.WriteString(line)
        buf.WriteString("\n")
        
        // 定期刷新缓冲区
        if buf.Len() > 4096 {
            w.Write([]byte(buf.String()))
            buf.Reset()
        }
    }
    
    if buf.Len() > 0 {
        w.Write([]byte(buf.String()))
    }
}
```

### 3.2 内存泄漏风险

**潜在风险点：**

| 位置 | 风险 | 建议 |
|------|------|------|
| `scanner.go:37` - `dirCache` | 大目录缓存可能占用大量内存 | 添加 LRU 淘汰机制 |
| `transcoder.go:17` - 信号量 | 异常情况下可能不释放 | 使用 `defer` 确保释放 |
| `server.go:48` - `seenIPs` | 无限制增长 | 添加定期清理或 LRU |

**优化代码：**

```go
// 文件: internal/server/server.go

// 添加 IP 映射清理
func (s *Server) startIPCleanup() {
    ticker := time.NewTicker(1 * time.Hour)
    go func() {
        for range ticker.C {
            // 清理过期的 IP 记录
            s.seenIPs = sync.Map{}
        }
    }()
}

// 文件: internal/media/scanner.go

// 限制目录缓存大小
type shareWalker struct {
    ctx       context.Context
    blacklist config.BlacklistConfig
    limit     int
    seen      int
    dirCache  *lru.Cache // 使用 LRU 缓存
    cb        WalkCallback
}

func NewShareWalker(ctx context.Context, blacklist config.BlacklistConfig, limit int, cb WalkCallback) *shareWalker {
    cache, _ := lru.New(1000) // 最多缓存 1000 个目录
    return &shareWalker{
        ctx:       ctx,
        blacklist: blacklist,
        limit:     limit,
        dirCache:  cache,
        cb:        cb,
    }
}
```

### 3.3 GC 优化

**当前状态：**
- ✅ 设置 `debug.SetGCPercent(50)` 积极回收内存
- ✅ 索引完成后调用 `debug.FreeOSMemory()`

**进一步优化：**

```go
// 文件: cmd/msp/main.go

func main() {
    // 现有 GC 设置
    debug.SetGCPercent(50)
    
    // 设置内存限制（如果可用）
    if limit := os.Getenv("MSP_MEMORY_LIMIT"); limit != "" {
        if bytes, err := strconv.ParseInt(limit, 10, 64); err == nil {
            debug.SetMemoryLimit(bytes)
        }
    }
    
    // 定期强制 GC
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        for range ticker.C {
            debug.FreeOSMemory()
        }
    }()
    
    // ... 其余代码
}
```

---

## 4. 并发处理分析

### 4.1 Goroutine 使用

**当前状态：**
- ✅ 后台缓存重建使用 goroutine
- ✅ 配置热重载使用 goroutine
- ✅ 转码使用独立的 goroutine 等待进程结束

**问题与优化：**

```go
// 文件: internal/server/server.go

// 1. 限制并发缓存重建数量
var cacheRebuildSemaphore = make(chan struct{}, 1)

func (s *Server) rebuildMediaCache(ctx context.Context, key string, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) {
    select {
    case cacheRebuildSemaphore <- struct{}{}:
        defer func() { <-cacheRebuildSemaphore }()
    case <-ctx.Done():
        return
    }
    
    s.buildMediaCacheAndUpdate(ctx, key, shares, blacklist, maxItems)
}

// 2. 添加 goroutine 数量监控
var (
    goroutineGauge int64
)

func trackGoroutine(name string, fn func()) {
    atomic.AddInt64(&goroutineGauge, 1)
    defer atomic.AddInt64(&goroutineGauge, -1)
    fn()
}
```

### 4.2 锁的使用

**当前状态：**
- ✅ 使用 `sync.RWMutex` 保护配置
- ✅ 使用 `sync.Mutex` + `sync.Cond` 协调缓存构建
- ✅ 使用 `sync.Map` 存储已见 IP

**优化建议：**

```go
// 文件: internal/server/server.go

// 1. 使用更细粒度的锁
// 将大锁拆分为多个小锁
type Server struct {
    cfgMu  sync.RWMutex  // 配置锁
    cfg    config.Config
    
    mediaMu      sync.Mutex
    mediaCond    *sync.Cond
    mediaCache   mediaCacheData  // 缓存数据
    
    logMu   sync.Mutex  // 日志锁
    logFile *os.File
    
    sessionMu sync.RWMutex  // 会话锁
    sessions  map[string]time.Time
}

// 2. 使用 atomic 替代锁（适用于简单计数器）
type Server struct {
    requestCount int64  // 使用 atomic.AddInt64
}

func (s *Server) IncrementRequestCount() {
    atomic.AddInt64(&s.requestCount, 1)
}
```

### 4.3 竞态条件检查

**潜在竞态条件：**

```go
// 文件: internal/server/server.go:337-348
// 问题：检查缓存过期和设置 building 标志之间可能有竞态

// 修复方案：
func (s *Server) GetOrBuildMediaCache(ctx context.Context, shares []config.Share, blacklist config.BlacklistConfig, refresh bool) (types.MediaResponse, string) {
    key := mediaCacheKey(shares, blacklist)
    
    s.mediaMu.Lock()
    
    // 双重检查锁定模式
    if s.mediaKey == key && !s.mediaBuiltAt.IsZero() && !refresh {
        if time.Since(s.mediaBuiltAt) < s.mediaTTL {
            // 缓存有效，直接返回
            var r types.MediaResponse
            _ = json.Unmarshal(s.mediaRespJSON, &r)
            etag := s.mediaETag
            s.mediaMu.Unlock()
            return r, etag
        }
        
        // 缓存过期，尝试触发重建
        if !s.mediaBuilding {
            s.mediaBuilding = true
            s.mediaMu.Unlock()
            go s.rebuildMediaCache(context.Background(), key, shares, blacklist, s.cfg.MaxItems)
            
            // 返回旧数据
            s.mediaMu.Lock()
            var r types.MediaResponse
            _ = json.Unmarshal(s.mediaRespJSON, &r)
            etag := s.mediaETag
            s.mediaMu.Unlock()
            return r, etag
        }
    }
    
    // ... 其余逻辑
}
```

---

## 5. 前端性能分析

### 5.1 资源加载优化

**当前状态：**
- ✅ 使用 Vite 构建工具
- ✅ 启用 PWA 支持
- ✅ 静态资源使用 CDN (Plyr, pinyin-pro)
- ✅ 使用 `cache: "no-store"` 避免 API 缓存问题

**问题与优化：**

```html
<!-- 文件: web/index.html -->

<!-- 1. 添加资源预加载 -->
<head>
  <link rel="preconnect" href="https://cdn.jsdelivr.net" crossorigin>
  <link rel="dns-prefetch" href="https://cdn.jsdelivr.net">
  
  <!-- 预加载关键资源 -->
  <link rel="preload" href="/assets/plyr/plyr.css" as="style">
  <link rel="preload" href="/src/app.js" as="module">
  
  <!-- 预加载首屏字体 -->
  <link rel="preload" href="/assets/fonts/main.woff2" as="font" type="font/woff2" crossorigin>
</head>
```

```javascript
// 文件: web/src/modules/api.js

// 2. 添加 API 请求去重
const pendingRequests = new Map();

export async function apiGet(url) {
  // 检查是否有相同请求正在进行
  if (pendingRequests.has(url)) {
    return pendingRequests.get(url);
  }
  
  const promise = fetch(url, { 
    cache: "no-store", 
    credentials: "include" 
  }).then(async res => {
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data?.error?.message || `${res.status} ${res.statusText}`);
    if (data?.error?.message) throw new Error(data.error.message);
    return data;
  }).finally(() => {
    pendingRequests.delete(url);
  });
  
  pendingRequests.set(url, promise);
  return promise;
}
```

### 5.2 代码分割

**当前状态：**
- ❌ 没有使用代码分割，所有 JS 打包为单个文件

**优化方案：**

```javascript
// 文件: web/vite.config.js

import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'logo.svg'],
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            urlPattern: /^\/api\//,
            handler: 'NetworkOnly',
          },
          // 缓存静态资源
          {
            urlPattern: /\.(js|css|png|jpg|jpeg|gif|webp|svg|woff2)$/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'static-resources',
              expiration: {
                maxEntries: 100,
                maxAgeSeconds: 7 * 24 * 60 * 60, // 7天
              },
            },
          },
        ],
      },
      manifest: {
        // ... 现有配置
      }
    })
  ],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // 启用代码分割
    rollupOptions: {
      output: {
        manualChunks: {
          // 将第三方库分离
          'plyr': ['plyr'],
          'pinyin': ['/assets/pinyin-pro/pinyin-pro.js'],
          // 按功能分割
          'player': ['./src/modules/player.js'],
          'playlist': ['./src/modules/playlist.js'],
          'lyrics': ['./src/modules/lyrics.js'],
        },
        // 优化 chunk 文件名
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        assetFileNames: (assetInfo) => {
          const info = assetInfo.name.split('.');
          const ext = info[info.length - 1];
          if (/\.(css)$/i.test(assetInfo.name)) {
            return 'assets/css/[name]-[hash][extname]';
          }
          return 'assets/[name]-[hash][extname]';
        },
      },
    },
    // 启用压缩
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,
        drop_debugger: true,
      },
    },
  },
})
```

### 5.3 PWA 配置优化

```javascript
// 文件: web/vite.config.js

VitePWA({
  registerType: 'autoUpdate',
  includeAssets: ['favicon.ico', 'logo.svg', 'assets/**/*'],
  
  // 添加离线页面
  workbox: {
    navigateFallback: '/index.html',
    navigateFallbackDenylist: [/^\/api\//],
    
    // 预缓存清单
    globPatterns: ['**/*.{js,css,html,svg,png,ico,woff2}'],
    
    runtimeCaching: [
      {
        urlPattern: /^\/api\//,
        handler: 'NetworkOnly',
      },
      {
        urlPattern: /\.(?:png|jpg|jpeg|svg|gif|webp)$/,
        handler: 'CacheFirst',
        options: {
          cacheName: 'images',
          expiration: {
            maxEntries: 200,
            maxAgeSeconds: 30 * 24 * 60 * 60, // 30天
          },
        },
      },
      {
        urlPattern: /\.(?:js|css)$/,
        handler: 'StaleWhileRevalidate',
        options: {
          cacheName: 'assets',
          expiration: {
            maxEntries: 50,
            maxAgeSeconds: 7 * 24 * 60 * 60, // 7天
          },
        },
      },
    ],
  },
  
  manifest: {
    name: 'MSP Media Share',
    short_name: 'MSP',
    description: 'Local LAN Media Share & Preview',
    theme_color: '#ffffff',
    background_color: '#ffffff',
    display: 'standalone',
    scope: '/',
    start_url: '/',
    icons: [
      {
        src: 'logo.svg',
        sizes: 'any',
        type: 'image/svg+xml',
        purpose: 'any maskable',
      },
      {
        src: 'logo-192.png',
        sizes: '192x192',
        type: 'image/png',
      },
      {
        src: 'logo-512.png',
        sizes: '512x512',
        type: 'image/png',
      },
    ],
  },
})
```

### 5.4 虚拟列表优化

```javascript
// 文件: web/src/modules/ui.js (新增)

// 大列表使用虚拟滚动
export class VirtualList {
  constructor(container, itemHeight, renderItem) {
    this.container = container;
    this.itemHeight = itemHeight;
    this.renderItem = renderItem;
    this.items = [];
    this.visibleCount = 0;
    this.startIndex = 0;
    
    this.init();
  }
  
  init() {
    this.container.style.overflow = 'auto';
    this.container.style.position = 'relative';
    
    this.content = document.createElement('div');
    this.content.style.position = 'relative';
    this.container.appendChild(this.content);
    
    this.container.addEventListener('scroll', this.onScroll.bind(this));
    this.updateVisibleCount();
  }
  
  setItems(items) {
    this.items = items;
    this.content.style.height = `${items.length * this.itemHeight}px`;
    this.render();
  }
  
  updateVisibleCount() {
    this.visibleCount = Math.ceil(this.container.clientHeight / this.itemHeight) + 2;
    this.render();
  }
  
  onScroll() {
    const scrollTop = this.container.scrollTop;
    this.startIndex = Math.floor(scrollTop / this.itemHeight);
    this.render();
  }
  
  render() {
    const endIndex = Math.min(this.startIndex + this.visibleCount, this.items.length);
    
    this.content.innerHTML = '';
    
    for (let i = this.startIndex; i < endIndex; i++) {
      const item = this.renderItem(this.items[i], i);
      item.style.position = 'absolute';
      item.style.top = `${i * this.itemHeight}px`;
      item.style.height = `${this.itemHeight}px`;
      this.content.appendChild(item);
    }
  }
}
```

---

## 6. 性能监控建议

### 6.1 添加性能指标收集

```go
// 文件: internal/server/metrics.go (新增)

package server

import (
    "expvar"
    "runtime"
    "time"
)

var (
    // 请求统计
    requestCount    = expvar.NewInt("requests.total")
    requestDuration = expvar.NewFloat("requests.duration_ms.avg")
    
    // 缓存统计
    cacheHits   = expvar.NewInt("cache.hits")
    cacheMisses = expvar.NewInt("cache.misses")
    
    // 转码统计
    transcodeActive = expvar.NewInt("transcode.active")
    transcodeTotal  = expvar.NewInt("transcode.total")
    
    // 内存统计
    memStats = expvar.NewMap("mem")
)

func (s *Server) StartMetricsCollection() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            memStats.Set("alloc", int64(m.Alloc))
            memStats.Set("sys", int64(m.Sys))
            memStats.Set("heap_alloc", int64(m.HeapAlloc))
            memStats.Set("heap_sys", int64(m.HeapSys))
            memStats.Set("num_gc", int64(m.NumGC))
            memStats.Set("goroutines", int64(runtime.NumGoroutine()))
        }
    }()
}

// 添加 /debug/vars 端点
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    expvar.Handler().ServeHTTP(w, r)
}
```

### 6.2 前端性能监控

```javascript
// 文件: web/src/modules/perf.js (新增)

export function initPerfMonitoring() {
  // 收集 Web Vitals
  if ('web-vitals' in window) {
    // LCP, FID, CLS 等
  }
  
  // 监控 API 响应时间
  const originalFetch = window.fetch;
  window.fetch = async function(...args) {
    const start = performance.now();
    try {
      const response = await originalFetch.apply(this, args);
      const duration = performance.now() - start;
      
      // 上报慢请求
      if (duration > 1000) {
        logRemote('warn', `Slow API: ${args[0]} took ${duration.toFixed(2)}ms`);
      }
      
      return response;
    } catch (err) {
      const duration = performance.now() - start;
      logRemote('error', `API Error: ${args[0]} failed after ${duration.toFixed(2)}ms`);
      throw err;
    }
  };
  
  // 监控资源加载
  if (PerformanceObserver) {
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.duration > 3000) {
          logRemote('warn', `Slow resource: ${entry.name} took ${entry.duration.toFixed(2)}ms`);
        }
      }
    });
    observer.observe({ entryTypes: ['resource'] });
  }
}
```

---

## 7. 总结与优先级

### 高优先级（立即实施）

1. **数据库批量插入** - 显著提升大量媒体文件扫描性能
2. **转码参数优化** - 添加 `-preset fast` 和线程限制
3. **API 请求去重** - 避免重复请求造成的服务器压力
4. **竞态条件修复** - 确保并发安全

### 中优先级（短期实施）

5. **内存映射 I/O** - 提升数据库读取性能
6. **代码分割** - 减少首屏加载时间
7. **虚拟列表** - 支持大量媒体文件流畅滚动
8. **目录缓存 LRU** - 防止内存无限增长

### 低优先级（长期考虑）

9. **PWA 离线支持增强**
10. **性能监控仪表板**
11. **HTTP/2 服务器推送**
12. **WebSocket 实时更新**

---

## 附录：快速修复清单

```bash
# 1. 数据库优化
# 修改 internal/db/db.go，添加 PRAGMA 设置

# 2. 转码优化
# 修改 internal/media/transcoder.go，更新 FFmpeg 参数

# 3. 批量插入
# 修改 internal/media/store.go 和 internal/db/db.go

# 4. 前端优化
# 修改 web/vite.config.js，启用代码分割
```
