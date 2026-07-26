package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/middleware"
	"mikvoc/internal/repository"
	"mikvoc/internal/routeros"
	"mikvoc/internal/service"
	"mikvoc/web"
)

// TemplateData is the base data passed to all SSR templates.
type TemplateData struct {
	Title      string
	ActiveMenu string
	RouterName string
	ROSVersion string
	Connected  bool
	Theme      string
	Currency   string
	Flash      string
	Routers    []database.Router
	ActiveIdx  int
	AdminRole  string
	AdminUser  string
	CSRFToken  string
	AppVersion string
	Data       any
}

// App holds shared application state.
type App struct {
	mu             sync.RWMutex
	templateEditMu sync.Mutex
	clients        map[int]*routeros.Client // key = router DB ID

	// Template cache: tmplName -> compiled *template.Template
	tmplMu    sync.RWMutex
	tmplCache map[string]*template.Template

	// Settings cache (refreshed every 30s)
	settingsMu    sync.RWMutex
	settingsCache map[string]string
	settingsAt    time.Time

	// Router list cache (refreshed every 60s)
	routersMu    sync.RWMutex
	routersCache []database.Router
	routersAt    time.Time

	Pool             *service.Pool
	Auth             *service.AuthService
	Routers          *service.RouterService
	Users            *service.UserService
	Profiles         *service.ProfileService
	Generate         *service.GenerateService
	Sales            *service.SalesService
	Stats            *service.StatsService
	Template         *service.TemplateService
	PPP              *service.PPPService
	RouterManagement *service.RouterManagementService
	Store            *repository.Store
	DBPath           string
	Secret           string

	keepAliveStop chan struct{}
	keepAliveOnce sync.Once
}

const settingsTTL = 30 * time.Second
const routersTTL = 60 * time.Second

// NewApp creates the App.
func NewApp(
	store *repository.Store,
	pool *service.Pool,
	auth *service.AuthService,
	routers *service.RouterService,
	users *service.UserService,
	profiles *service.ProfileService,
	gen *service.GenerateService,
) *App {
	return &App{
		clients:   make(map[int]*routeros.Client),
		tmplCache: make(map[string]*template.Template),
		Store:     store,
		Pool:      pool,
		Auth:      auth,
		Routers:   routers,
		Users:     users,
		Profiles:  profiles,
		Generate:  gen,
	}
}

// LoadTemplates validates embedded template tree at startup.
func (a *App) LoadTemplates() error {
	tmpls, err := fs.ReadDir(web.TemplatesFS(), "layouts")
	if err != nil || len(tmpls) == 0 {
		return fmt.Errorf("no layout templates found: %w", err)
	}
	return nil
}

// InvalidateTemplateCache clears the compiled template cache.
// Call this after pushing new templates to the router or editing template files.
func (a *App) InvalidateTemplateCache() {
	a.tmplMu.Lock()
	a.tmplCache = make(map[string]*template.Template)
	a.tmplMu.Unlock()
}

// InvalidateSettingsCache forces the next render to re-read settings from DB.
func (a *App) InvalidateSettingsCache() {
	a.settingsMu.Lock()
	a.settingsAt = time.Time{}
	a.settingsMu.Unlock()
}

// InvalidateRoutersCache forces the next render to re-read routers from DB.
func (a *App) InvalidateRoutersCache() {
	a.routersMu.Lock()
	a.routersAt = time.Time{}
	a.routersMu.Unlock()
}

// cachedSettings returns settings from cache, refreshing if stale.
func (a *App) cachedSettings() map[string]string {
	a.settingsMu.RLock()
	if time.Since(a.settingsAt) < settingsTTL && a.settingsCache != nil {
		s := a.settingsCache
		a.settingsMu.RUnlock()
		return s
	}
	a.settingsMu.RUnlock()

	s := database.GetAllSettings()
	a.settingsMu.Lock()
	a.settingsCache = s
	a.settingsAt = time.Now()
	a.settingsMu.Unlock()
	return s
}

// cachedRouters returns routers from cache, refreshing if stale.
func (a *App) cachedRouters() []database.Router {
	a.routersMu.RLock()
	if time.Since(a.routersAt) < routersTTL && a.routersCache != nil {
		r := a.routersCache
		a.routersMu.RUnlock()
		return r
	}
	a.routersMu.RUnlock()

	routers, _ := database.GetRouters()
	a.routersMu.Lock()
	a.routersCache = routers
	a.routersAt = time.Now()
	a.routersMu.Unlock()
	return routers
}

// cachedRouter finds a specific router from the cached list (avoids extra DB call per request).
func (a *App) cachedRouter(id int) *database.Router {
	if id == 0 {
		return nil
	}
	routers := a.cachedRouters()
	for i := range routers {
		if routers[i].ID == id {
			// Copy the value to avoid data races on the shared slice
			copy := routers[i]
			return &copy
		}
	}
	return nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"now":            time.Now,
		"formatBytes":    routeros.FormatBytes,
		"formatDuration": routeros.FormatDuration,
		"add":            func(a, b int) int { return a + b },
		"parseI64": func(s string) int64 {
			var v int64
			fmt.Sscanf(s, "%d", &v)
			return v
		},
		"contains": func(s string, subs ...string) bool {
			for _, sub := range subs {
				if strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
					return true
				}
			}
			return false
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"slice": func(s string, i, j int) string {
			r := []rune(s)
			if i > len(r) {
				i = len(r)
			}
			if j > len(r) {
				j = len(r)
			}
			return string(r[i:j])
		},
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any, len(values)/2)
			for i := 0; i+1 < len(values); i += 2 {
				k, _ := values[i].(string)
				m[k] = values[i+1]
			}
			return m
		},
	}
}

// sessionRouterID reads the active router DB-ID from the session cookie.
func sessionRouterID(r *http.Request) int {
	if middleware.Store == nil {
		return 0
	}
	sess, err := middleware.Store.Get(r, middleware.SessionName)
	if err != nil {
		return 0
	}
	if id, ok := sess.Values["router_id"].(int); ok {
		return id
	}
	return 0
}

// SetSessionRouterID writes the active router DB-ID into the session.
func (a *App) SetSessionRouterID(w http.ResponseWriter, r *http.Request, id int) {
	// Keep CSRF in session when switching routers (avoid token drop on Save).
	csrfTok := middleware.EnsureCSRFToken(w, r)
	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	sess.Values["router_id"] = id
	if csrfTok != "" {
		sess.Values["csrf_token"] = csrfTok
	}
	_ = sess.Save(r, w)
}

// clientFor returns the *routeros.Client for the router chosen by this request's session.
func (a *App) clientFor(r *http.Request) *routeros.Client {
	id := sessionRouterID(r)
	if id == 0 {
		return nil
	}
	a.mu.RLock()
	cl := a.clients[id]
	a.mu.RUnlock()
	return cl
}

// routerFor returns the database.Router for the current session (uses in-memory cache).
func (a *App) routerFor(r *http.Request) *database.Router {
	return a.cachedRouter(sessionRouterID(r))
}

// ConnectRouter connects to a specific router by DB ID and stores the client.
func (a *App) ConnectRouter(rt *database.Router) error {
	port := rt.Port
	if port == "" {
		port = "8728"
	}

	a.mu.Lock()
	// Close existing connection first
	if existing, ok := a.clients[rt.ID]; ok && existing != nil {
		existing.Close()
	}
	a.mu.Unlock()

	cl := routeros.NewClient(rt.IP, port)
	if err := cl.Connect(rt.Username, rt.Password); err != nil {
		return err
	}

	a.mu.Lock()
	a.clients[rt.ID] = cl
	a.mu.Unlock()
	if a.Pool != nil {
		a.Pool.Put(rt.ID, cl)
	}
	return nil
}

// ConnectAll attempts to connect to all saved routers in the database.
func (a *App) ConnectAll() {
	routers, err := database.GetRouters()
	if err != nil {
		return
	}
	for i := range routers {
		// Connect in background to avoid blocking server startup.
		go func(rt database.Router) {
			if err := a.ConnectRouter(&rt); err != nil {
				log.Printf("[warn] Router %s: %v", rt.IP, err)
			} else {
				log.Printf("[info] Connected: %s (%s)", rt.Name, rt.IP)
			}
		}(routers[i])
	}
}

// DisconnectRouter closes the API connection for a router and removes it from the client map.
func (a *App) DisconnectRouter(id int) {
	if id == 0 {
		return
	}
	a.mu.Lock()
	if cl, ok := a.clients[id]; ok && cl != nil {
		cl.Close()
		delete(a.clients, id)
	}
	a.mu.Unlock()
	if a.Pool != nil {
		a.Pool.Put(id, nil)
		a.Pool.InvalidateUsers(id)
	}
}

func (a *App) getUsersCached(r *http.Request, profile string) ([]routeros.HotspotUser, error) {
	id := sessionRouterID(r)
	if a.Pool != nil && id != 0 && a.Pool.IsConnected(id) {
		return a.Pool.GetUsersCached(id, profile)
	}
	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return cl.GetUsers(profile)
}

func (a *App) getUserByIDCached(r *http.Request, userID string) (routeros.HotspotUser, bool) {
	id := sessionRouterID(r)
	if a.Pool == nil || id == 0 {
		return routeros.HotspotUser{}, false
	}
	return a.Pool.GetUserByID(id, userID)
}

func (a *App) getUserByNameCached(r *http.Request, name string) (routeros.HotspotUser, bool) {
	id := sessionRouterID(r)
	if a.Pool == nil || id == 0 {
		return routeros.HotspotUser{}, false
	}
	return a.Pool.GetUserByName(id, name)
}

func (a *App) invalidateUsers(r *http.Request) {
	if a.Pool == nil {
		return
	}
	a.Pool.InvalidateUsers(sessionRouterID(r))
}

func (a *App) StartKeepAlive(interval time.Duration) {
	if a == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	a.keepAliveOnce.Do(func() {
		a.keepAliveStop = make(chan struct{})
		go a.keepAliveLoop(interval)
	})
}

func (a *App) keepAliveLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-a.keepAliveStop:
			return
		case <-ticker.C:
			a.pingClients()
		}
	}
}

func (a *App) pingClients() {
	a.mu.RLock()
	ids := make([]int, 0, len(a.clients))
	for id := range a.clients {
		ids = append(ids, id)
	}
	a.mu.RUnlock()

	for _, id := range ids {
		a.mu.RLock()
		cl := a.clients[id]
		a.mu.RUnlock()
		if cl == nil || !cl.IsConnected() {
			a.mu.Lock()
			if cur, ok := a.clients[id]; ok && cur == cl {
				delete(a.clients, id)
			}
			a.mu.Unlock()
			if a.Pool != nil {
				a.Pool.Put(id, nil)
				a.Pool.InvalidateUsers(id)
			}
			continue
		}
		if _, err := cl.Run("/system/identity/print"); err != nil {
			log.Printf("[keepalive] router %d ping failed: %v", id, err)
			a.mu.Lock()
			if cur, ok := a.clients[id]; ok && cur == cl {
				cl.Close()
				delete(a.clients, id)
			}
			a.mu.Unlock()
			if a.Pool != nil {
				a.Pool.Put(id, nil)
				a.Pool.InvalidateUsers(id)
			}
		}
	}
}

// getTemplate returns a compiled template from cache, or parses and caches it.
// Uses "layout" as the execution entry-point for reliable block override behaviour.
func (a *App) getTemplate(tmplName string) (*template.Template, error) {
	a.tmplMu.RLock()
	t, ok := a.tmplCache[tmplName]
	a.tmplMu.RUnlock()
	if ok {
		return t, nil
	}

	tfs := web.TemplatesFS()

	// Collect layout + partial files.
	var files []string
	for _, dir := range []string{"layouts", "partials"} {
		entries, err := fs.ReadDir(tfs, dir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
				continue
			}
			files = append(files, path.Join(dir, e.Name()))
		}
	}
	// Add the requested page (clean name without "pages/" prefix).
	files = append(files, path.Join("pages", tmplName))

	// Create a fresh template set each time a new page is first loaded.
	// Naming the root "layout" ensures ExecuteTemplate("layout") works correctly
	// and each page's {{define "content"}} properly overrides the block.
	t, err := template.New("layout").Funcs(templateFuncs()).ParseFS(tfs, files...)
	if err != nil {
		return nil, err
	}

	a.tmplMu.Lock()
	a.tmplCache[tmplName] = t
	a.tmplMu.Unlock()
	return t, nil
}

// render executes a named template within the full layout using cached templates and settings.
func (a *App) render(w http.ResponseWriter, r *http.Request, tmplName string, data TemplateData) {
	cl := a.clientFor(r)
	rt := a.routerFor(r)

	// Use cached lookups instead of DB hits per request
	settings := a.cachedSettings()
	data.Theme = settings["theme"]
	data.Currency = settings["currency"]
	data.Routers = a.cachedRouters()
	data.ActiveIdx = sessionRouterID(r)

	if cl != nil && cl.IsConnected() {
		data.Connected = true
		data.ROSVersion = cl.ROSVersion()
	}
	if rt != nil {
		data.RouterName = rt.Name
		if data.RouterName == "" {
			data.RouterName = rt.IP
		}
	}

	// Flash from session
	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	if flash, ok := sess.Values["flash"].(string); ok && flash != "" {
		data.Flash = flash
		delete(sess.Values, "flash")
		_ = sess.Save(r, w)
	}
	if role, ok := sess.Values["admin_role"].(string); ok {
		data.AdminRole = role
	}
	if user, ok := sess.Values["admin_user"].(string); ok {
		data.AdminUser = user
	}
	if data.AdminRole == "" && sess.Values["authenticated"] == true {
		data.AdminRole = "owner"
	}
	data.CSRFToken = middleware.EnsureCSRFToken(w, r)
	data.AppVersion = core.Version

	t, err := a.getTemplate(tmplName)
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	// Execute via "layout" so {{block "content"}} picks up the correct {{define "content"}} from the page file.
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// renderStandalone renders a standalone HTML page (not wrapped in the main layout).
// Partials (head, toasts) are parsed so the page can reference them.
func (a *App) renderStandalone(w http.ResponseWriter, r *http.Request, tmplName string, data TemplateData) {
	settings := a.cachedSettings()
	data.Theme = settings["theme"]
	data.Currency = settings["currency"]

	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	if flash, ok := sess.Values["flash"].(string); ok && flash != "" {
		data.Flash = flash
		delete(sess.Values, "flash")
		_ = sess.Save(r, w)
	}
	data.CSRFToken = middleware.EnsureCSRFToken(w, r)
	data.AppVersion = core.Version

	tfs := web.TemplatesFS()
	var files []string
	entries, err := fs.ReadDir(tfs, "partials")
	if err != nil {
		http.Error(w, "Template read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		files = append(files, path.Join("partials", e.Name()))
	}
	files = append(files, path.Join("pages", tmplName))

	t, err := template.New(tmplName).Funcs(templateFuncs()).ParseFS(tfs, files...)
	if err != nil {
		http.Error(w, "Template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// setFlash stores a one-time flash message in the session.
func (a *App) setFlash(w http.ResponseWriter, r *http.Request, msg string) {
	csrfTok := middleware.EnsureCSRFToken(w, r)
	sess, _ := middleware.Store.Get(r, middleware.SessionName)
	sess.Values["flash"] = msg
	if csrfTok != "" {
		sess.Values["csrf_token"] = csrfTok
	}
	_ = sess.Save(r, w)
}

func (a *App) writeErr(w http.ResponseWriter, r *http.Request, err error, redirect string) {
	if err == nil {
		return
	}
	status := core.StatusOf(err)
	msg := err.Error()
	if wantsUserActionJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      false,
			"message": msg,
		})
		return
	}
	if redirect == "" {
		http.Error(w, msg, status)
		return
	}
	a.setFlash(w, r, msg)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (a *App) audit(r *http.Request, action, target string) {
	if a.Store == nil {
		return
	}
	adminID := middleware.AdminIDFromRequest(r)
	adminName := middleware.AdminNameFromRequest(r)
	_ = a.Store.AddAudit(adminID, adminName, action, target)
}

// Sentinel to avoid unused import warnings.
var _ = (*App)(nil)
