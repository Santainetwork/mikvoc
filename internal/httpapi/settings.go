package httpapi

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"mikvoc/internal/authn"
	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/routeros"
)

// HandleSettings renders the settings page (GET) and saves (POST).
func (a *App) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		action := r.FormValue("action")
		switch action {
		case "router_add":
			if a.Routers != nil {
				rt := &core.Router{Name: "Router Baru", IP: "192.168.88.1", Port: "8728", Username: "admin", Password: ""}
				if err := a.Routers.Save(rt); err != nil {
					a.setFlash(w, r, "Error: "+err.Error())
				} else {
					a.SetSessionRouterID(w, r, rt.ID)
					a.InvalidateRoutersCache()
					a.setFlash(w, r, "Router baru ditambahkan. Silakan ubah konfigurasi di bawah ini.")
				}
			} else {
				rt := &database.Router{Name: "Router Baru", IP: "192.168.88.1", Port: "8728", Username: "admin", Password: ""}
				if err := database.SaveRouter(rt); err != nil {
					a.setFlash(w, r, "Error: "+err.Error())
				} else {
					a.SetSessionRouterID(w, r, rt.ID)
					a.InvalidateRoutersCache()
					a.setFlash(w, r, "Router baru ditambahkan. Silakan ubah konfigurasi di bawah ini.")
				}
			}

		case "router_add_full":
			name := r.FormValue("router_name")
			ip := r.FormValue("router_ip")
			port := r.FormValue("router_port")
			if port == "" {
				port = "8728"
			}
			username := r.FormValue("router_username")
			if username == "" {
				username = "admin"
			}
			password := r.FormValue("router_password")
			if name == "" {
				name = ip
			}
			var newID int
			var err error
			if a.Routers != nil {
				rt := &core.Router{Name: name, IP: ip, Port: port, Username: username, Password: password}
				err = a.Routers.Save(rt)
				newID = rt.ID
			} else {
				rt := &database.Router{Name: name, IP: ip, Port: port, Username: username, Password: password}
				err = database.SaveRouter(rt)
				newID = rt.ID
			}
			if err != nil {
				a.setFlash(w, r, "Gagal tambah router: "+err.Error())
				http.Redirect(w, r, "/routers", http.StatusSeeOther)
				return
			}
			a.InvalidateRoutersCache()
			a.SetSessionRouterID(w, r, newID)
			a.audit(r, "router_add", name+" "+ip)
			a.setFlash(w, r, "Router "+name+" ditambahkan. Pilih sesi untuk menghubungkan.")
			http.Redirect(w, r, "/routers", http.StatusSeeOther)
			return

		case "router":
			if a.Routers == nil {
				a.setFlash(w, r, "Service router tidak tersedia.")
				break
			}
			var id int
			fmt.Sscanf(r.FormValue("router_id"), "%d", &id)
			ip := r.FormValue("router_ip")
			port := r.FormValue("router_port")
			if port == "" {
				port = "8728"
			}
			username := r.FormValue("router_username")
			password := r.FormValue("router_password")
			name := r.FormValue("router_name")
			voucherTmpl := r.FormValue("router_voucher_template")

			rt, _ := a.Routers.Get(id)
			if rt == nil {
				rt = &core.Router{}
			}
			rt.ID = id
			rt.Name = name
			rt.IP = ip
			rt.Port = port
			rt.Username = username
			if voucherTmpl != "" {
				rt.VoucherTemplate = voucherTmpl
			}
			if password != "" {
				rt.Password = password
			}
			err := a.Routers.Save(rt)
			a.InvalidateRoutersCache()

			if err == nil {
				err = a.Routers.Connect(rt.ID)
			}
			if err == nil {
				// Set session to this router
				a.SetSessionRouterID(w, r, rt.ID)
				a.setFlash(w, r, "Router berhasil disambungkan.")
			} else {
				a.setFlash(w, r, "Pengaturan disimpan, tapi gagal konek: "+err.Error())
			}

		case "router_delete":
			var id int
			fmt.Sscanf(r.FormValue("router_id"), "%d", &id)
			if id == 0 {
				a.setFlash(w, r, "Router tidak valid.")
				break
			}
			var err error
			if a.Routers != nil {
				err = a.Routers.Delete(id)
			} else {
				err = database.DeleteRouter(id)
			}
			if err != nil {
				a.setFlash(w, r, "Gagal hapus router: "+err.Error())
				break
			}
			// Clean router assets directory AFTER successful DB delete
			if a.Assets != nil && a.Assets.Root() != "" {
				routerAssetDir := filepath.Join(a.Assets.Root(), "routers", fmt.Sprintf("%d", id))
				if _, statErr := os.Stat(routerAssetDir); statErr == nil {
					if cleanupErr := os.RemoveAll(routerAssetDir); cleanupErr != nil {
						log.Printf("[settings] failed to remove router assets for ID %d: %v", id, cleanupErr)
					}
				}
			}
			a.InvalidateRoutersCache()
			if sessionRouterID(r) == id {
				a.SetSessionRouterID(w, r, 0)
			}
			a.audit(r, "router_delete", fmt.Sprintf("id=%d", id))
			a.setFlash(w, r, "Router dihapus.")

		case "appearance":
			_ = database.SetSetting("currency", r.FormValue("currency"))
			_ = database.SetSetting("timezone", r.FormValue("timezone"))
			_ = database.SetSetting("language", r.FormValue("language"))
			a.InvalidateSettingsCache()
			a.setFlash(w, r, "Tampilan berhasil diperbarui.")

		case "admin":
			if middleware.RoleLevel(middleware.RoleFromRequest(r)) < middleware.RoleLevel(core.RoleOwner) {
				a.setFlash(w, r, "Akses ditolak: ubah akun admin hanya untuk owner.")
				http.Redirect(w, r, "/settings", http.StatusSeeOther)
				return
			}
			dbUser, _ := database.GetAdmin()
			newUser := r.FormValue("admin_username")
			if newUser == "" {
				newUser = dbUser
			}
			newPass := r.FormValue("admin_password")
			if newPass == "" {
				_ = database.SetAdmin(newUser, "")
			} else {
				hashed, err := authn.HashPassword(newPass)
				if err != nil {
					a.setFlash(w, r, "Error: gagal hash password: "+err.Error())
					http.Redirect(w, r, "/settings", http.StatusSeeOther)
					return
				}
				_ = database.SetAdminPassword(newUser, hashed)
			}
			a.audit(r, "admin_self_update", newUser)
			a.setFlash(w, r, "Akun admin berhasil diperbarui.")
		}

		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	// GET — gather data for form
	routers, _ := database.GetRouters()
	activeID := sessionRouterID(r)
	settings := database.GetAllSettings()
	adminUser, _ := database.GetAdmin()

	type SettingsData struct {
		Routers        []database.Router
		ActiveRouter   database.Router
		ActiveRouterID int
		Settings       map[string]string
		AdminUsername  string
	}

	var activeRouter database.Router
	for _, r := range routers {
		if r.ID == activeID {
			activeRouter = r
			break
		}
	}
	if activeID == 0 && len(routers) > 0 {
		activeRouter = routers[0]
		activeID = activeRouter.ID
	}

	a.render(w, r, "settings.html", TemplateData{
		Title:      "Pengaturan — MikVoc",
		ActiveMenu: "settings",
		Data: SettingsData{
			Routers:        routers,
			ActiveRouter:   activeRouter,
			ActiveRouterID: activeID,
			Settings:       settings,
			AdminUsername:  adminUser,
		},
	})
}

// HandleSwitchRouter sets the active router for this browser session.
func (a *App) HandleSwitchRouter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
		return
	}
	var id int
	fmt.Sscanf(r.FormValue("id"), "%d", &id)

	if a.Routers == nil {
		a.setFlash(w, r, "Service router tidak tersedia.")
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
		return
	}
	rt, err := a.Routers.Get(id)
	if err != nil || rt == nil {
		a.setFlash(w, r, "Router tidak ditemukan.")
		http.Redirect(w, r, "/routers", http.StatusSeeOther)
		return
	}

	// Set session ID first so the user can edit it even if it fails to connect
	a.SetSessionRouterID(w, r, id)

	// Connect in background — avoid blocking the request (10s dial timeout per attempt).
	// Dashboard will show "Offline" until the connection succeeds.
	go func() {
		if err := a.Routers.Connect(id); err != nil {
			log.Printf("[warn] switch-router: connect %s: %v", rt.IP, err)
		}
	}()

	a.setFlash(w, r, "Sesi dipilih: "+rt.Name+". Menghubungkan ke router di latar belakang...")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// HandleTestRouterAPI attempts to connect to a router and returns JSON success/failure.
func (a *App) HandleTestRouterAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.FormValue("ip")
	port := r.FormValue("port")
	username := r.FormValue("username")
	password := r.FormValue("password")

	if port == "" {
		port = "8728"
	}

	rt := &database.Router{
		IP:       ip,
		Port:     port,
		Username: username,
		Password: password,
	}

	cl := routeros.NewClient(rt.IP, rt.Port)
	w.Header().Set("Content-Type", "application/json")

	if err := cl.Connect(rt.Username, rt.Password); err != nil {
		w.Write([]byte(fmt.Sprintf(`{"success":false,"message":%q}`, err.Error())))
		return
	}
	defer cl.Close()

	w.Write([]byte(fmt.Sprintf(`{"success":true,"message":"%s (ROS %s)"}`, rt.IP, cl.ROSVersion())))
}
