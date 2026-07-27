package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/env"
	"mikvoc/internal/httpapi"
	"mikvoc/internal/middleware"
	"mikvoc/internal/repository"
	"mikvoc/internal/service"
	"mikvoc/web"
)

func main() {
	// Load .env file if present. Existing env vars take precedence.
	if err := env.Load(".env"); err != nil {
		log.Printf("[WARN] gagal load .env: %v", err)
	}

	port := flag.Int("port", envInt("MIKVOC_PORT", 8080), "Port untuk menjalankan aplikasi MikVoc (env: MIKVOC_PORT)")
	dbPath := flag.String("db", envStr("MIKVOC_DB", "mikvoc.db"), "Path ke file database SQLite (env: MIKVOC_DB)")
	secret := flag.String("secret", "", "Secret key untuk session + enkripsi password router (env: MIKVOC_SECRET)")
	showVersion := flag.Bool("version", false, "Tampilkan versi MikVoc")
	flag.Parse()
	if *showVersion {
		writeVersion(os.Stdout)
		return
	}

	appSecret := *secret
	if appSecret == "" {
		appSecret = os.Getenv("MIKVOC_SECRET")
	}
	if appSecret == "" {
		appSecret = generateSecret()
		log.Printf("[WARN] MIKVOC_SECRET tidak diset. Generate secret acak untuk sesi ini: %s", appSecret)
		log.Printf("[WARN] Simpan secret ini ke file .env (MIKVOC_SECRET=...) agar session & password router tetap bisa di-decrypt setelah restart.")
	}

	// Init SQLite database dengan secret untuk enkripsi password router at-rest.
	if err := database.Init(*dbPath, appSecret); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	log.Printf("Database ready: %s", *dbPath)

	// Init session store dengan secret dari config/env.
	middleware.InitSession(appSecret)

	store := repository.NewStore()
	pool := service.NewPool()
	authSvc := service.NewAuth(store)
	routerSvc := service.NewRouter(store, pool)
	userSvc := service.NewUser(pool)
	profileSvc := service.NewProfile(pool)
	genSvc := service.NewGenerate(pool, store)
	salesSvc := service.NewSales(pool, store)
	statsSvc := service.NewStats(pool)
	templateSvc := service.NewTemplate(pool, store)
	pppSvc := service.NewPPP(pool)
	routerManagementSvc := service.NewRouterManagement(pool)

	app := httpapi.NewApp(store, pool, authSvc, routerSvc, userSvc, profileSvc, genSvc)
	app.Sales = salesSvc
	app.Stats = statsSvc
	app.Template = templateSvc
	app.PPP = pppSvc
	app.RouterManagement = routerManagementSvc
	app.SetDBPath(*dbPath)
	app.SetSecret(appSecret)

	// Load HTML templates
	if err := app.LoadTemplates(); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Connect to all saved routers at startup
	if err := routerSvc.ConnectAll(); err != nil {
		log.Printf("[warn] connect routers: %v", err)
	}
	pool.StartKeepAlive(30 * time.Second)

	sched := service.NewScheduler(pool, store)
	sched.Start()

	// Setup HTTP router
	r := mux.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.Logging,
		middleware.SecurityHeaders,
		middleware.Gzip,
		middleware.TemplateUploadLimit,
		middleware.CSRF,
	)

	// Static files — long-lived immutable cache so fonts/CSS/JS survive reloads.
	staticFS, _ := fs.Sub(web.StaticFS(), ".")
	staticHandler := http.FileServer(http.FS(staticFS))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JS needs revalidation after CSRF/main.js fixes; fonts/css stay long-lived.
		p := r.URL.Path
		if strings.HasSuffix(p, ".js") {
			w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		staticHandler.ServeHTTP(w, r)
	})))

	app.RegisterRoutes(r)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("MikVoc starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func writeVersion(w io.Writer) {
	fmt.Fprintf(w, "MikVoc v%s\n", core.Version)
}

func generateSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("gagal generate secret: %v", err)
	}
	return hex.EncodeToString(b)
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}
