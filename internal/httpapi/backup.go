package httpapi

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
)

func (a *App) SetDBPath(p string) {
	a.DBPath = p
}

func (a *App) SetSecret(s string) {
	a.Secret = s
}

func (a *App) HandleBackup(w http.ResponseWriter, r *http.Request) {
	if a.DBPath == "" {
		http.Error(w, "db path not configured", http.StatusInternalServerError)
		return
	}
	if database.DB != nil {
		_, _ = database.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	f, err := os.Open(a.DBPath)
	if err != nil {
		http.Error(w, "gagal buka database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	name := fmt.Sprintf("mikvoc-backup-%s.db", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("[backup] copy error: %v", err)
	}
	a.audit(r, "backup", name)
}

func (a *App) HandleRestore(w http.ResponseWriter, r *http.Request) {
	if middleware.RoleLevel(middleware.RoleFromRequest(r)) < middleware.RoleLevel(core.RoleOwner) {
		a.setFlash(w, r, "Akses ditolak: restore hanya untuk owner.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if a.DBPath == "" {
		a.setFlash(w, r, "Error: db path tidak dikonfigurasi.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		a.setFlash(w, r, "Error: gagal parse upload: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("db_file")
	if err != nil {
		a.setFlash(w, r, "Error: pilih file database (.db).")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	defer file.Close()

	header := make([]byte, 16)
	n, err := io.ReadFull(file, header)
	if err != nil || n < 16 || string(header) != "SQLite format 3\x00" {
		a.setFlash(w, r, "Error: file bukan SQLite database yang valid.")
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	tmp := a.DBPath + ".incoming"
	out, err := os.Create(tmp)
	if err != nil {
		a.setFlash(w, r, "Error: gagal buat file sementara: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if _, err := out.Write(header); err != nil {
		out.Close()
		os.Remove(tmp)
		a.setFlash(w, r, "Error: gagal tulis file: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tmp)
		a.setFlash(w, r, "Error: gagal simpan upload: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		a.setFlash(w, r, "Error: gagal tutup file: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	a.mu.Lock()
	for id, cl := range a.clients {
		if cl != nil {
			cl.Close()
		}
		delete(a.clients, id)
	}
	a.mu.Unlock()
	if a.Pool != nil {
		a.Pool.Clear()
	}

	if database.DB != nil {
		_ = database.DB.Close()
		database.DB = nil
	}

	bak := a.DBPath + ".bak." + time.Now().Format("20060102-150405")
	if err := os.Rename(a.DBPath, bak); err != nil && !os.IsNotExist(err) {
		os.Remove(tmp)
		a.setFlash(w, r, "Error: gagal backup DB lama: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if err := os.Rename(tmp, a.DBPath); err != nil {
		_ = os.Rename(bak, a.DBPath)
		a.setFlash(w, r, "Error: gagal apply restore: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	if err := database.Init(a.DBPath, a.Secret); err != nil {
		log.Printf("[restore] re-init failed: %v", err)
		_ = os.Rename(a.DBPath, a.DBPath+".failed")
		_ = os.Rename(bak, a.DBPath)
		_ = database.Init(a.DBPath, a.Secret)
		a.setFlash(w, r, "Error: restore gagal, DB lama dikembalikan: "+err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	a.InvalidateSettingsCache()
	a.InvalidateRoutersCache()
	a.InvalidateTemplateCache()
	go a.ConnectAll()

	a.audit(r, "restore", filepath.Base(bak))
	a.setFlash(w, r, "Restore berhasil. Database diganti (backup lama: "+filepath.Base(bak)+").")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
