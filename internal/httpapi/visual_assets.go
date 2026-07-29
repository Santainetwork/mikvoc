package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"mikvoc/internal/assets"
)

// HandleLogoUpload handles POST /template/logo-upload uploads logo images
func (a *App) HandleLogoUpload(w http.ResponseWriter, r *http.Request) {
	if a.Assets == nil {
		http.Error(w, "asset store unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoUploadBytes+1)
	
	file, header, err := r.FormFile("logo")
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal membaca upload: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal baca konten: %v", err), http.StatusBadRequest)
		return
	}
	
	// Check size AFTER full read
	if int64(len(data)) > maxLogoUploadBytes {
		http.Error(w, fmt.Sprintf("Ukuran file terlalu besar. Maksimal %d MiB", maxLogoUploadBytes/1024/1024), http.StatusForbidden)
		return
	}
	
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, fmt.Sprintf("%s bukan gambar valid", header.Filename), http.StatusBadRequest)
		return
	}
	
	if contentType == "image/svg+xml" {
		http.Error(w, "SVG tidak didukung untuk upload logo", http.StatusBadRequest)
		return
	}
	
	routerID := sessionRouterID(r)
	_, err = a.Assets.Write(routerID, assets.Logo, strings.NewReader(string(data)), maxLogoUploadBytes)
	if err != nil {
		msg := err.Error()
		var status int
		var userMsg string
		
		switch {
		case strings.Contains(msg, "dimension"):
			userMsg = "Dimensi melebihi 4096x4096 atau piksel lebih dari 16MP"
			status = http.StatusBadRequest
		case strings.Contains(msg, "invalid") || strings.Contains(msg, "decode"):
			userMsg = "Format file tidak didukung. Gunakan PNG, JPEG, atau GIF"
			status = http.StatusBadRequest
		default:
			userMsg = fmt.Sprintf("Gagal upload logo: %v", err)
			status = http.StatusInternalServerError
		}
		
		http.Error(w, userMsg, status)
		return
	}
	
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

// HandleBackgroundUpload handles POST /template/background-upload uploads background images
func (a *App) HandleBackgroundUpload(w http.ResponseWriter, r *http.Request) {
	if a.Assets == nil {
		http.Error(w, "asset store unavailable", http.StatusServiceUnavailable)
		return
	}
	
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	
	r.Body = http.MaxBytesReader(w, r.Body, maxBackgroundUploadBytes+1)
	
	file, header, err := r.FormFile("background")
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal membaca upload: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Gagal baca konten: %v", err), http.StatusBadRequest)
		return
	}
	
	if int64(len(data)) > maxBackgroundUploadBytes {
		http.Error(w, fmt.Sprintf("Ukuran file terlalu besar. Maksimal %d MiB", maxBackgroundUploadBytes/1024/1024), http.StatusForbidden)
		return
	}
	
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, fmt.Sprintf("%s bukan gambar valid", header.Filename), http.StatusBadRequest)
		return
	}
	
	if contentType == "image/svg+xml" {
		http.Error(w, "SVG tidak didukung untuk upload background", http.StatusBadRequest)
		return
	}
	
	routerID := sessionRouterID(r)
	_, err = a.Assets.Write(routerID, assets.Background, strings.NewReader(string(data)), maxBackgroundUploadBytes)
	if err != nil {
		msg := err.Error()
		var status int
		var userMsg string
		
		switch {
		case strings.Contains(msg, "dimension"):
			userMsg = "Dimensi melebihi 4096x4096 atau piksel lebih dari 16MP"
			status = http.StatusBadRequest
		case strings.Contains(msg, "invalid") || strings.Contains(msg, "decode"):
			userMsg = "Format file tidak didukung. Gunakan PNG, JPEG, atau GIF"
			status = http.StatusBadRequest
		default:
			userMsg = fmt.Sprintf("Gagal upload background: %v", err)
			status = http.StatusInternalServerError
		}
		
		http.Error(w, userMsg, status)
		return
	}
	
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}

// HandleFocalPosition handles POST /template/focal validates focal point X/Y parameters
func (a *App) HandleFocalPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	
	focalXStr := r.FormValue("focalX")
	focalYStr := r.FormValue("focalY")
	
	var focalX, focalY int
	var errX, errY error
	
	if focalXStr != "" {
		focalX, errX = strconv.Atoi(focalXStr)
	}
	if focalYStr != "" {
		focalY, errY = strconv.Atoi(focalYStr)
	}
	
	if errX != nil {
		http.Error(w, fmt.Sprintf("focalX harus bilangan bulat, dapatkan %q", focalXStr), http.StatusBadRequest)
		return
	}
	if errY != nil {
		http.Error(w, fmt.Sprintf("focalY harus bilangan bulat, dapatkan %q", focalYStr), http.StatusBadRequest)
		return
	}
	
	// Clamp values to [0, 100]
	if focalX < 0 {
		focalX = 0
	} else if focalX > 100 {
		focalX = 100
	}
	
	if focalY < 0 {
		focalY = 0
	} else if focalY > 100 {
		focalY = 100
	}
	
	http.Redirect(w, r, "/template", http.StatusSeeOther)
}
