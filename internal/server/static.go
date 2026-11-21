package server

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

type staticFile struct {
	body, gz []byte
	ctype    string
}

// staticHandler serves the embedded SPA. Files are loaded and gzipped once
// at startup; unknown paths fall back to index.html for client routing.
func (s *Server) staticHandler() http.Handler {
	files := map[string]*staticFile{}
	_ = fs.WalkDir(s.static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(s.static, p)
		if err != nil {
			return nil
		}
		f := &staticFile{body: b, ctype: mime.TypeByExtension(path.Ext(p))}
		if f.ctype == "" {
			f.ctype = http.DetectContentType(b)
		}
		if compressible(f.ctype) && len(b) > 1024 {
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			_, _ = zw.Write(b)
			_ = zw.Close()
			f.gz = buf.Bytes()
		}
		files["/"+p] = f
		return nil
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		f, ok := files[p]
		if !ok || p == "/index.html" {
			p, f = "/index.html", files["/index.html"]
		}
		if f == nil {
			http.Error(w, "frontend not built; run `make` first", http.StatusNotFound)
			return
		}
		h := w.Header()
		h.Set("Content-Type", f.ctype)
		if strings.HasPrefix(p, "/assets/") {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		h.Set("Vary", "Accept-Encoding")
		body := f.body
		if f.gz != nil && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.Set("Content-Encoding", "gzip")
			body = f.gz
		}
		_, _ = w.Write(body)
	})
}

func compressible(ct string) bool {
	return strings.HasPrefix(ct, "text/") || strings.Contains(ct, "javascript") ||
		strings.Contains(ct, "json") || strings.Contains(ct, "svg") || strings.Contains(ct, "xml")
}
