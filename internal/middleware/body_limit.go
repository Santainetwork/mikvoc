package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const TemplateUploadMaxBytes int64 = 7 * 1024 * 1024 // 7 MiB + overhead

func TemplateUploadLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to /template route and subpaths
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/template") {
			next.ServeHTTP(w, r)
			return
		}
		
		r.Body = http.MaxBytesReader(w, r.Body, TemplateUploadMaxBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "request body terlalu besar (maks "+fmt.Sprintf("%.2f", float64(TemplateUploadMaxBytes)/1024/1024)+" MiB)", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}
