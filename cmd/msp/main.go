package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"msp/internal/handler"
	"msp/internal/server"
	"msp/internal/util"
	"msp/internal/web"
	webassets "msp/web"
)

func main() {
	debug.SetGCPercent(50) // Aggressive GC to keep memory low
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfgPath := filepath.Join(util.MustExeDir(), "config.json")
	s := server.New(cfgPath)

	if err := s.LoadOrInitConfig(); err != nil {
		log.Fatal(err)
	}

	s.SetupLogger()

	webRoot, err := fs.Sub(webassets.FS, "dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	h := handler.New(s)

	mux.Handle("/api/config", http.HandlerFunc(h.HandleConfig))
	mux.Handle("/api/shares", http.HandlerFunc(h.HandleShares))
	mux.Handle("/api/media", http.HandlerFunc(h.HandleMedia))
	mux.Handle("/api/stream", http.HandlerFunc(h.HandleStream))
	mux.Handle("/api/subtitle", http.HandlerFunc(h.HandleSubtitle))
	mux.Handle("/api/probe", http.HandlerFunc(h.HandleProbe))
	mux.Handle("/api/ip", http.HandlerFunc(h.HandleIP))

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.ServeEmbeddedWeb(w, r, webRoot)
	}))

	port := s.GetPort()
	addr := ":" + util.Itoa(port)

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

	finalHandler := handler.WithLog(s, handler.WithGzip(mux))

	server := &http.Server{
		Addr:              addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if os.Getenv("MSP_NO_AUTO_OPEN") != "1" {
		go func() {
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
		}()
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
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
