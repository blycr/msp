package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

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
)

func main() {
	debug.SetGCPercent(50)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfgPath := filepath.Join(util.MustExeDir(), "config.json")

	dbPath := filepath.Join(util.MustExeDir(), "msp.db")
	sq, err := storage.InitSQLite(dbPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
	}

	processor := media.NewMediaProcessor(sq)

	s := server.New(cfgPath, processor)

	if err := s.LoadOrInitConfig(); err != nil {
		log.Fatal(err)
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

	mux := registerRoutes(s, processor, store, webRoot)

	port := s.GetPort()
	addr := ":" + util.Itoa(port)

	printStartupBanner(cfgPath, port)

	finalHandler := handler.WithRecovery(handler.WithLog(s, handler.WithSecurity(s, s, s, handler.WithGzip(mux))))

	srv := &http.Server{
		Addr:              addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if os.Getenv("MSP_NO_AUTO_OPEN") != "1" {
		go tryAutoOpenBrowser(port)
	}

	go func() {
		<-ctx.Done()
		shutdownGracefully(srv, s, processor, sq)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func shutdownGracefully(srv *http.Server, s *server.Server, processor *media.MediaProcessor, sq *storage.SQLite) {
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	processor.KillAllTranscodeProcesses()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	if sq != nil {
		sq.Close()
	}

	if s.Logger() != nil {
		s.Logger().Close()
	}

	log.Println("Shutdown complete")
}

func registerRoutes(s *server.Server, processor *media.MediaProcessor, store *storage.Store, webRoot fs.FS) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	h := handler.New(handler.Deps{
		Config:    s,
		Media:     s.MediaSvc,
		Session:   s,
		Logger:    s,
		Progress:  store,
		Prefs:     store,
		Processor: processor,
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
	mux.Handle("/api/log", http.HandlerFunc(h.HandleLog))
	mux.Handle("/api/pin", http.HandlerFunc(h.HandlePIN))

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.ServeEmbeddedWeb(w, r, webRoot)
	}))
	return mux
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
