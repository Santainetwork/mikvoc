package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

type gzipWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	skip        bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	if !g.skip {
		ct := g.Header().Get("Content-Type")
		if !compressibleType(ct) {
			g.skip = true
		} else {
			g.Header().Del("Content-Length")
			g.Header().Set("Content-Encoding", "gzip")
			g.Header().Add("Vary", "Accept-Encoding")
		}
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.skip {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

func (g *gzipWriter) Flush() {
	if !g.skip && g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		gw := &gzipWriter{ResponseWriter: w, gz: gz}
		defer func() {
			if gw.wroteHeader && !gw.skip {
				_ = gz.Close()
			}
			gz.Reset(nil)
			gzipPool.Put(gz)
		}()
		next.ServeHTTP(gw, r)
	})
}

func compressibleType(ct string) bool {
	if ct == "" {
		return false
	}
	ct = strings.ToLower(ct)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "text/html", "application/json", "text/css", "application/javascript",
		"text/javascript", "text/plain", "application/xml", "text/xml",
		"image/svg+xml", "application/xhtml+xml":
		return true
	default:
		return strings.HasPrefix(ct, "text/")
	}
}
