package web

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

func ServeEmbeddedWeb(w http.ResponseWriter, r *http.Request, webFS fs.FS) {
	p := path.Clean("/" + r.URL.Path)
	if p == "/" || p == "/index.html" {
		ServeEmbeddedFSFile(w, r, webFS, "index.html", "text/html; charset=utf-8", "no-store")
		return
	}
	if strings.HasPrefix(p, "/api/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(p, "/")
	ext := strings.ToLower(filepath.Ext(name))
	cache := "no-store"
	switch ext {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff2", ".ttf":
		cache = "no-cache"
	}
	ServeEmbeddedFSFile(w, r, webFS, name, "", cache)
}

func ServeEmbeddedFSFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string, contentType string, cacheControl string) {
	f, err := fsys.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}

	ext := strings.ToLower(filepath.Ext(st.Name()))
	ct := contentType
	if ct == "" {
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			switch ext {
			case ".vtt":
				ct = "text/vtt; charset=utf-8"
			case ".lrc", ".srt":
				ct = "text/plain; charset=utf-8"
			default:
				ct = "application/octet-stream"
			}
		}
	}
	w.Header().Set("Content-Type", ct)
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	http.ServeContent(w, r, st.Name(), st.ModTime(), readSeeker{f})
}

type readSeeker struct {
	f fs.File
}

func (r readSeeker) Read(p []byte) (int, error) {
	return r.f.Read(p)
}

func (r readSeeker) Seek(offset int64, whence int) (int64, error) {
	if s, ok := r.f.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, errors.New("seek not supported")
}
