package httpapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func buildTemplateZip(set templateFileSet) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	seen := make(map[string]bool, len(set.Files))
	for _, file := range set.Files {
		name, err := normalizeAssetName(file.Name)
		if err != nil || name != file.Name {
			_ = zw.Close()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("non-canonical asset path %q", file.Name)
		}
		key := strings.ToLower(name)
		if key == "hotspot" || strings.HasPrefix(key, "hotspot/") {
			_ = zw.Close()
			return nil, fmt.Errorf("asset path %q must be relative to the hotspot directory", name)
		}
		if seen[key] {
			_ = zw.Close()
			return nil, fmt.Errorf("duplicate asset path %q", name)
		}
		seen[key] = true
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create %q: %w", name, err)
		}
		if _, err := w.Write(file.Content); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write %q: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close ZIP: %w", err)
	}
	return buf.Bytes(), nil
}

func (a *App) HandleTemplateDownload(w http.ResponseWriter, r *http.Request) {
	set, err := assembleTemplateFiles(currentHotspotSettings(sessionRouterID(r)))
	if err != nil {
		a.setFlash(w, r, "Gagal membuat paket template: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	raw, err := buildTemplateZip(set)
	if err != nil {
		a.setFlash(w, r, "Gagal membuat paket template: "+err.Error())
		http.Redirect(w, r, "/template", http.StatusSeeOther)
		return
	}
	filename := "mikvoc-hotspot-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}
