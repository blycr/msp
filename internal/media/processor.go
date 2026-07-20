package media

import (
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"msp/internal/domain"
	"msp/internal/storage"
	"msp/internal/util"
)

// MediaProcessor consolidates all media package state that was previously
// stored in package-level global variables. A single instance should be
// created at application startup and injected into handlers, cache, and
// services.
type MediaProcessor struct {
	db *storage.SQLite

	probePaths struct {
		ffmpeg  string
		ffprobe string
		once    sync.Once
	}

	probeCache sync.Map
	probeTTL   atomic.Int64 // nanoseconds

	transcode struct {
		limit  chan struct{}
		active map[*exec.Cmd]struct{}
		mu     sync.Mutex
	}

	hwAccel struct {
		once     sync.Once
		result   *HWAccelResult
		disabled atomic.Bool
	}

	// postScan 保存扫描完成后的回调：扫描成功时以本次扫描的媒体条目调用，
	// 新扫描开始或服务关闭时以 nil 调用（用于停止上一轮扫描派生的后台工作）。
	postScan struct {
		mu   sync.RWMutex
		hook func(items []domain.MediaItem)
	}

	idCodec *util.IDCodec
}

// Option configures a MediaProcessor during construction.
type Option func(*MediaProcessor)

// WithTranscodeLimit sets the maximum number of concurrent transcode sessions.
// The default is 2.
func WithTranscodeLimit(n int) Option {
	return func(mp *MediaProcessor) {
		if n > 0 {
			mp.transcode.limit = make(chan struct{}, n)
		}
	}
}

// NewMediaProcessor creates a new MediaProcessor with the given database
// connection and options.
func NewMediaProcessor(db *storage.SQLite, idCodec *util.IDCodec, opts ...Option) *MediaProcessor {
	mp := &MediaProcessor{db: db, idCodec: idCodec}
	mp.probeTTL.Store(int64(5 * time.Minute))
	mp.transcode.limit = make(chan struct{}, 2)
	mp.transcode.active = make(map[*exec.Cmd]struct{})
	for _, opt := range opts {
		opt(mp)
	}
	return mp
}

// IsDBAvailable reports whether the processor has a valid database connection.
func (mp *MediaProcessor) IsDBAvailable() bool {
	return mp != nil && mp.db != nil && mp.db.DB() != nil
}

// SetTranscodeLimit updates the concurrent transcode limit. Safe to call
// before any transcode requests (typically at startup).
func (mp *MediaProcessor) SetTranscodeLimit(n int) {
	if n <= 0 {
		n = 2
	}
	mp.transcode.mu.Lock()
	defer mp.transcode.mu.Unlock()
	// Only safe because no active transcodes exist at startup.
	mp.transcode.limit = make(chan struct{}, n)
	slog.Info("transcode concurrency limit set", "limit", n)
}

// IDCodec returns the IDCodec used by this processor.
func (mp *MediaProcessor) IDCodec() *util.IDCodec {
	if mp == nil {
		return nil
	}
	return mp.idCodec
}
