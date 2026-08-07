package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"msp/internal/config"
	"msp/internal/handler"
	"msp/internal/media"
	"msp/internal/server"
	"msp/internal/service"
	"msp/internal/storage"
	"msp/internal/util"
	"msp/internal/web"
	webassets "msp/web"
)

// Compile-time interface assertions
var (
	_ handler.ConfigProvider     = (*server.Server)(nil)
	_ handler.MediaCacheProvider = (*service.MediaService)(nil)
	_ handler.SessionProvider    = (*server.Server)(nil)
	_ handler.SessionProvider    = (*service.SessionService)(nil)
	_ handler.Logger             = (*server.Server)(nil)
	_ storage.ProgressStore      = (*storage.SQLite)(nil)
	_ storage.PrefsStore         = (*storage.SQLite)(nil)
	_ storage.FavoriteStore      = (*storage.SQLite)(nil)
)

// startTime records process start for /healthz uptime reporting.
var startTime = time.Now()

func main() {
	debug.SetGCPercent(100)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// bgCtx governs background media-cache work (rebuilds, disk writes). It is
	// derived from Background — NOT from the signal ctx — because background
	// ops must keep running while srv.Shutdown drains requests, and are only
	// cancelled afterwards, in a controlled order, via bgCancel.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	cfgPath := filepath.Join(util.MustExeDir(), "config.json")

	dbPath := filepath.Join(util.MustExeDir(), "msp.db")
	sq, err := storage.InitSQLite(dbPath)
	if err != nil {
		slog.Warn("failed to initialize database", "err", err)
		printDBUnavailableBanner()
		slog.Error("database unavailable: playback progress / favorites / preferences are disabled; all other features work normally")
	}

	idKeyPath := filepath.Join(util.MustExeDir(), "msp.key")
	idKey, err := util.LoadOrCreateKey(idKeyPath)
	if err != nil {
		slog.Warn("failed to load/create ID key", "err", err)
	}
	idCodec := util.NewIDCodec(idKey)

	processor := media.NewMediaProcessor(sq, idCodec)
	// 清理上次异常退出残留的 HLS 临时目录（msp_hls_*）
	processor.CleanupStaleHLSTempDirs()

	// Migrate old-format PlaybackProgress media_ids to deterministic IDs.
	if sq != nil && idCodec != nil {
		if err := migrateProgressMediaIDs(sq, idCodec); err != nil {
			slog.Warn("failed to migrate progress media IDs", "err", err)
		}
	}

	s := server.New(cfgPath, processor)
	s.SetBackgroundContext(bgCtx)

	if err := s.LoadOrInitConfig(); err != nil {
		log.Fatal(err)
	}

	// Migrate plaintext PIN to bcrypt hash if present.
	if s.Config().Security.PIN != "" {
		if err := s.UpdateConfig(func(cfg *config.Config) {
			config.SanitizeSecurity(cfg)
		}); err != nil {
			slog.Warn("failed to hash PIN", "err", err)
		}
	}

	s.SetupLogger()

	initHWAccel(processor, s)

	go s.WatchConfig(ctx)

	go s.MediaSvc.GetOrBuildMediaCache(context.Background(), s.Config().Shares, s.Config().Blacklist, false)

	webRoot, err := fs.Sub(webassets.FS, "dist")
	if err != nil {
		log.Fatal(err)
	}

	store := storage.NewStore(sq)

	mux := registerRoutes(s, processor, store, webRoot, idCodec)

	port := s.GetPort()
	addr := ":" + util.Itoa(port)

	printStartupBanner(cfgPath, port)

	limiter := handler.NewRateLimiter()
	finalHandler := handler.WithRecovery(s, handler.WithLog(s, handler.WithSecurity(s, s, s, handler.WithRateLimit(limiter, handler.WithAdminLockdown(handler.WithGzip(mux))))))

	srv := &http.Server{
		Addr:              addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if os.Getenv("MSP_NO_AUTO_OPEN") != "1" {
		go tryAutoOpenBrowser(port)
	}

	go func() {
		<-ctx.Done()
		shutdownGracefully(srv, s, processor, sq, bgCancel)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func shutdownGracefully(srv *http.Server, s *server.Server, processor *media.MediaProcessor, sq *storage.SQLite, bgCancel context.CancelFunc) {
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Kill transcodes first so long-lived streams don't block srv.Shutdown.
	processor.KillAllTranscodeProcesses()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Stop background media work, then give it a bounded window to finish
	// (e.g. atomic cache-file rename) so no .tmp residue is left behind.
	bgCancel()
	bgDone := make(chan struct{})
	go func() {
		s.WaitForBackgroundMediaOps()
		close(bgDone)
	}()
	select {
	case <-bgDone:
	case <-time.After(5 * time.Second):
		slog.Warn("timeout waiting for background media ops; exiting anyway")
	}

	if sq != nil {
		sq.Close()
	}

	if s.Logger() != nil {
		s.Logger().Close()
	}

	log.Println("Shutdown complete")
}

func registerRoutes(s *server.Server, processor *media.MediaProcessor, store *storage.Store, webRoot fs.FS, idCodec *util.IDCodec) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	h := handler.New(handler.Deps{
		Config:    s,
		Media:     s.MediaSvc,
		Session:   s,
		Logger:    s,
		Progress:  store,
		Prefs:     store,
		Favorites: store,
		Processor: processor,
		IDCodec:   idCodec,
	})

	mux.Handle("/api/config", http.HandlerFunc(h.HandleConfig))
	mux.Handle("/api/shares", http.HandlerFunc(h.HandleShares))
	mux.Handle("/api/media", http.HandlerFunc(h.HandleMedia))
	mux.Handle("/api/stream", http.HandlerFunc(h.HandleStream))
	mux.Handle("/api/subtitle", http.HandlerFunc(h.HandleSubtitle))
	mux.Handle("/api/probe", http.HandlerFunc(h.HandleProbe))
	mux.Handle("/api/ip", http.HandlerFunc(h.HandleIP))
	mux.Handle("/api/prefs", http.HandlerFunc(h.HandlePrefs))
	mux.Handle("/api/progress", http.HandlerFunc(h.HandleProgress))
	mux.Handle("/api/progress/recent", http.HandlerFunc(h.HandleRecentProgress))
	mux.Handle("/api/thumbnail", http.HandlerFunc(h.HandleThumbnail))
	mux.Handle("/api/favorites", http.HandlerFunc(h.HandleFavorites))
	mux.Handle("/api/log", http.HandlerFunc(h.HandleLog))
	mux.Handle("/api/pin", http.HandlerFunc(h.HandlePIN))
	mux.Handle("/api/hls/", http.HandlerFunc(h.HandleHLS))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"db":     processor.IsDBAvailable(),
			"uptime": int64(time.Since(startTime).Seconds()),
		})
	})

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.ServeEmbeddedWeb(w, r, webRoot)
	}))
	return mux
}

// printDBUnavailableBanner prints a prominent multi-line warning to stderr
// when the database failed to initialize.
func printDBUnavailableBanner() {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "========================================================")
	fmt.Fprintln(os.Stderr, "  WARNING: Database unavailable (msp.db init failed)")
	fmt.Fprintln(os.Stderr, "  播放进度 / 收藏 / 偏好功能已禁用，其余功能正常")
	fmt.Fprintln(os.Stderr, "  Playback progress / favorites / prefs are DISABLED.")
	fmt.Fprintln(os.Stderr, "  All other features (browse/stream/transcode) are OK.")
	fmt.Fprintln(os.Stderr, "========================================================")
	fmt.Fprintln(os.Stderr, "")
}

func printStartupBanner(cfgPath string, port int) {
	ips := util.GetLanIPv4s()
	urls := make([]string, 0, 2+len(ips))
	urls = append(urls, "http://127.0.0.1:"+util.Itoa(port)+"/")
	for _, ip := range ips {
		urls = append(urls, "http://"+ip+":"+util.Itoa(port)+"/")
	}

	log.Println("配置文件:", cfgPath)
	fmt.Println("配置文件:", cfgPath)
	for _, u := range urls {
		log.Println("访问:", u)
		fmt.Println("访问:", "\x1b[36m"+u+"\x1b[0m")
		// 局域网地址下附加二维码，便于手机扫码快捷访问；127.0.0.1 仅本机可
		// 用，无需扫码，不打印。
		if !strings.HasPrefix(u, "http://127.") {
			if qr, err := util.QRCodeTerminal(u); err == nil {
				fmt.Println("扫码访问（手机连同一局域网后扫描）:")
				fmt.Println(qr)
			} else {
				log.Printf("[WARN] QR code generation failed: %v", err)
			}
		}
	}
}

func tryAutoOpenBrowser(port int) {
	localURL := "http://localhost:" + util.Itoa(port) + "/"
	addr := "127.0.0.1:" + util.Itoa(port)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	_ = openBrowser(localURL)
}

//nolint:gosec
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func initHWAccel(processor *media.MediaProcessor, s *server.Server) {
	cfg := s.Config()

	mode := media.HWAccelAuto
	maxJobs := 0
	if enc := cfg.Playback.Video.Encoding; enc != nil {
		if enc.HWAccel != "" {
			mode = media.HWAccelMode(enc.HWAccel)
		}
		maxJobs = enc.MaxJobs
	}

	if !processor.CheckFFmpeg() {
		log.Printf("转码引擎: %s (并发上限: 0)", processor.FormatHWAccelStatus())
		fmt.Printf("转码引擎: %s (并发上限: 0)\n", processor.FormatHWAccelStatus())
		return
	}

	result := processor.DetectHWAccel(mode)

	if maxJobs <= 0 {
		if result != nil && result.Available {
			maxJobs = 4
		} else {
			maxJobs = 2
		}
	}
	processor.SetTranscodeLimit(maxJobs)

	log.Printf("转码引擎: %s (并发上限: %d)", processor.FormatHWAccelStatus(), maxJobs)
	fmt.Printf("转码引擎: %s (并发上限: %d)\n", processor.FormatHWAccelStatus(), maxJobs)
}

// migrateProgressMediaIDs migrates old PlaybackProgress records whose
// media_id was generated with a random nonce to the new deterministic
// format. The old IDs remain decryptable because the AES-GCM algorithm
// itself has not changed — only the nonce derivation is now deterministic.
func migrateProgressMediaIDs(sq *storage.SQLite, codec *util.IDCodec) error {
	if sq == nil || codec == nil {
		return nil
	}
	progressList, err := sq.ListAllProgress(context.Background())
	if err != nil {
		return fmt.Errorf("list progress: %w", err)
	}
	var migrated int
	for _, p := range progressList {
		path, err := codec.DecodeID(p.MediaID)
		if err != nil {
			// Old ID may be corrupted or plain base64; skip.
			continue
		}
		newID := codec.EncodeID(path)
		if newID == p.MediaID {
			continue // already deterministic
		}
		if err := sq.DeleteProgress(context.Background(), p.MediaID); err != nil {
			log.Printf("[WARN] migrate progress: failed to delete old ID %q: %v", p.MediaID, err)
			continue
		}
		if err := sq.SetProgress(context.Background(), newID, p.Time); err != nil {
			log.Printf("[WARN] migrate progress: failed to set new ID %q: %v", newID, err)
			continue
		}
		migrated++
	}
	if migrated > 0 {
		log.Printf("[INFO] Migrated %d playback progress records to deterministic IDs", migrated)
	}
	return nil
}
