package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
	"mikvoc/internal/routeros"
	"mikvoc/internal/service"
)

type userListFilters struct {
	Profile string
	Comment string
	Search  string
	Expired bool
	IDs     []string
}

type userCommentOption struct {
	Value     string
	Profile   string
	Count     int
	Generated bool
}

func userListFiltersFromRequest(r *http.Request) userListFilters {
	profile := strings.TrimSpace(r.URL.Query().Get("profile"))
	if profile == "all" {
		profile = ""
	}
	return userListFilters{
		Profile: profile,
		Comment: strings.TrimSpace(r.URL.Query().Get("comment")),
		Search:  strings.TrimSpace(r.URL.Query().Get("q")),
		Expired: r.URL.Query().Get("exp") == "1" || r.URL.Query().Get("expired") == "1",
		IDs:     queryIDs(r.URL.Query()["ids"]),
	}
}

func (f userListFilters) toCore() core.UserFilters {
	return core.UserFilters{
		Profile: f.Profile,
		Comment: f.Comment,
		Search:  f.Search,
		Expired: f.Expired,
		IDs:     f.IDs,
	}
}

func loadFilteredHotspotUsers(cl *routeros.Client, filters userListFilters) ([]routeros.HotspotUser, error) {
	users, err := cl.GetUsers(filters.Profile)
	if err != nil {
		return nil, err
	}
	return filterHotspotUsers(users, filters), nil
}

func (a *App) loadFilteredHotspotUsers(r *http.Request, cl *routeros.Client, filters userListFilters) ([]routeros.HotspotUser, error) {
	if a.Users != nil {
		users, err := a.Users.List(sessionRouterID(r), filters.toCore())
		if err != nil {
			return nil, err
		}
		return service.FromCoreUsers(users), nil
	}
	var users []routeros.HotspotUser
	var err error
	if filters.Profile == "" {
		users, err = a.getUsersCached(r, "")
	} else if cl != nil {
		users, err = cl.GetUsers(filters.Profile)
	} else {
		users, err = a.getUsersCached(r, filters.Profile)
	}
	if err != nil {
		return nil, err
	}
	return filterHotspotUsers(users, filters), nil
}

func (a *App) loadHotspotProfiles(r *http.Request, cl *routeros.Client) ([]routeros.HotspotUserProfile, error) {
	if a.Profiles != nil {
		profiles, err := a.Profiles.List(sessionRouterID(r))
		if err != nil {
			return nil, err
		}
		return service.FromCoreProfiles(profiles), nil
	}
	if cl == nil || !cl.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return cl.GetProfiles()
}

func filterHotspotUsers(users []routeros.HotspotUser, filters userListFilters) []routeros.HotspotUser {
	filtered := make([]routeros.HotspotUser, 0, len(users))
	selectedOrder := map[string]int{}
	for i, id := range filters.IDs {
		selectedOrder[id] = i
	}
	for _, u := range users {
		if filters.Profile != "" && filters.Profile != "all" && u.Profile != filters.Profile {
			continue
		}
		if filters.Comment != "" && u.Comment != filters.Comment {
			continue
		}
		if filters.Expired && !routeros.IsExpiredHotspotUser(u, time.Now()) {
			continue
		}
		if filters.Search != "" && !core.ContainsIgnoreCase(u.Name, filters.Search) &&
			!core.ContainsIgnoreCase(u.Comment, filters.Search) &&
			!core.ContainsIgnoreCase(u.Profile, filters.Search) {
			continue
		}
		if len(filters.IDs) > 0 {
			if _, ok := selectedOrder[u.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, u)
	}
	if len(filters.IDs) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			return selectedOrder[filtered[i].ID] < selectedOrder[filtered[j].ID]
		})
	}
	return filtered
}

func commentRemovalCandidates(users []routeros.HotspotUser, comment, profile string) []routeros.HotspotUser {
	comment = strings.TrimSpace(comment)
	profile = strings.TrimSpace(profile)
	if profile == "all" {
		profile = ""
	}
	if comment == "" {
		return nil
	}
	candidates := make([]routeros.HotspotUser, 0)
	for _, u := range users {
		if strings.TrimSpace(u.Comment) != comment {
			continue
		}
		if profile != "" && strings.TrimSpace(u.Profile) != profile {
			continue
		}
		if !routeros.IsUnusedHotspotUser(u) {
			continue
		}
		candidates = append(candidates, u)
	}
	return candidates
}

func commentCandidateProfiles(candidates []routeros.HotspotUser) []string {
	seen := map[string]bool{}
	profiles := make([]string, 0)
	for _, u := range candidates {
		profile := strings.TrimSpace(u.Profile)
		if seen[profile] {
			continue
		}
		seen[profile] = true
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	return profiles
}

func profileListLabel(profiles []string) string {
	labels := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if profile == "" {
			labels = append(labels, "tanpa profile")
			continue
		}
		labels = append(labels, profile)
	}
	return strings.Join(labels, ", ")
}

func profileValidityForName(profiles []routeros.HotspotUserProfile, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, p := range profiles {
		if strings.TrimSpace(p.Name) == name {
			return strings.TrimSpace(p.Validity)
		}
	}
	return ""
}

func generateOptionsFromRequest(r *http.Request, profiles []routeros.HotspotUserProfile) routeros.GenerateOptions {
	qty, _ := strconv.Atoi(r.FormValue("qty"))
	if qty < 1 {
		qty = 1
	}
	if qty > 500 {
		qty = 500
	}
	dataLimitRaw, _ := strconv.ParseInt(r.FormValue("data_limit"), 10, 64)
	dataUnit, _ := strconv.ParseInt(r.FormValue("data_unit"), 10, 64)
	if dataUnit == 0 {
		dataUnit = 1048576
	}
	profile := r.FormValue("profile")
	timeLimit := strings.TrimSpace(r.FormValue("time_limit"))
	if timeLimit == "" {
		timeLimit = profileValidityForName(profiles, profile)
	}
	return routeros.GenerateOptions{
		Qty:            qty,
		Profile:        profile,
		Server:         r.FormValue("server"),
		Mode:           r.FormValue("mode"),
		Prefix:         r.FormValue("prefix"),
		Length:         parseIntDefault(r.FormValue("length"), 6),
		CharMode:       r.FormValue("charmode"),
		TimeLimitStr:   timeLimit,
		DataLimitBytes: dataLimitRaw * dataUnit,
		Comment:        r.FormValue("comment"),
	}
}

func voucherSpecFromGenerateOpts(opts routeros.GenerateOptions) core.VoucherSpec {
	return core.VoucherSpec{
		Qty:            opts.Qty,
		Profile:        opts.Profile,
		Server:         opts.Server,
		Mode:           opts.Mode,
		Prefix:         opts.Prefix,
		CharMode:       opts.CharMode,
		TimeLimitStr:   opts.TimeLimitStr,
		Comment:        opts.Comment,
		Length:         opts.Length,
		DataLimitBytes: opts.DataLimitBytes,
	}
}

func buildUserCommentOptions(users []routeros.HotspotUser) []userCommentOption {
	type commentKey struct {
		value   string
		profile string
	}
	counts := make(map[commentKey]int)
	for _, u := range users {
		comment := strings.TrimSpace(u.Comment)
		if comment == "" {
			continue
		}
		if !looksLikeBatchComment(comment) {
			continue
		}
		key := commentKey{value: comment, profile: strings.TrimSpace(u.Profile)}
		counts[key]++
	}

	options := make([]userCommentOption, 0, len(counts))
	for key, count := range counts {
		options = append(options, userCommentOption{
			Value:     key.value,
			Profile:   key.profile,
			Count:     count,
			Generated: true,
		})
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].Value == options[j].Value {
			return options[i].Profile < options[j].Profile
		}
		return options[i].Value < options[j].Value
	})
	return options
}

func looksLikeBatchComment(comment string) bool {
	if comment == "" {
		return false
	}
	if strings.HasPrefix(comment, "vc-") || strings.HasPrefix(comment, "up-") {
		return true
	}
	parts := strings.SplitN(comment, "-", 2)
	if len(parts) != 2 {
		return false
	}
	return len(parts[1]) >= 8 && strings.Count(parts[1], ".") >= 2
}

func userListQuery(filters userListFilters, extra map[string]string) url.Values {
	q := url.Values{}
	if filters.Profile != "" {
		q.Set("profile", filters.Profile)
	}
	if filters.Comment != "" {
		q.Set("comment", filters.Comment)
	}
	if filters.Search != "" {
		q.Set("q", filters.Search)
	}
	if filters.Expired {
		q.Set("exp", "1")
	}
	for _, id := range filters.IDs {
		q.Add("ids", id)
	}
	for k, v := range extra {
		if v == "" {
			continue
		}
		q.Set(k, v)
	}
	return q
}

func userListURL(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

func userListModeLabel(u routeros.HotspotUser) string {
	if u.Password == u.Name {
		return "voucher"
	}
	return "member"
}

func userListRowLabel(u routeros.HotspotUser) string {
	if routeros.IsExpiredHotspotUser(u, time.Now()) {
		return "expired"
	}
	return u.Comment
}

func queryIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	ids := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func userListPrintURLs(filters userListFilters) (string, string, string) {
	defaultURL := userListURL("/hotspot/users/print", userListQuery(filters, map[string]string{"qr": "no"}))
	qrURL := userListURL("/hotspot/users/print", userListQuery(filters, map[string]string{"qr": "yes"}))
	smallURL := userListURL("/hotspot/users/print", userListQuery(filters, map[string]string{"template": "compact", "qr": "no"}))
	return defaultURL, qrURL, smallURL
}

func routerOSQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func wantsUserActionJSON(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("X-Requested-With"), "fetch") ||
		strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}

func normalizeProfileExpiredForm(mode string) string {
	switch strings.TrimSpace(mode) {
	case "rem", "remove":
		return "remove"
	case "ntf", "notice":
		return "notice"
	case "remc", "remove_record":
		return "remove_record"
	case "ntfc", "notice_record":
		return "notice_record"
	case "disable":
		return "disable"
	case "0", "", "none":
		return "none"
	default:
		return mode
	}
}

func (a *App) finishUserAction(w http.ResponseWriter, r *http.Request, status int, message string, warning bool, extra map[string]any) {
	if wantsUserActionJSON(r) {
		if status == 0 {
			status = http.StatusOK
		}
		payload := map[string]any{
			"ok":      status < http.StatusBadRequest,
			"message": message,
		}
		if warning {
			payload["warning"] = true
		}
		for k, v := range extra {
			payload[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	if message != "" {
		a.setFlash(w, r, message)
	}
	http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
}

func formIDs(r *http.Request, key string) []string {
	values := r.Form[key]
	ids := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

const (
	usersJSONDefaultLimit = 500
	usersJSONMaxLimit     = 500
)

func parseUsersJSONPagination(r *http.Request) (limit, offset int) {
	limit = usersJSONDefaultLimit
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	limit = service.ClampLimit(limit, usersJSONDefaultLimit, usersJSONMaxLimit)
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = service.ClampOffset(n)
		}
	}
	return limit, offset
}

func (a *App) HandleUsersJSON(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	filters := userListFiltersFromRequest(r)
	limit, offset := parseUsersJSONPagination(r)

	var page []routeros.HotspotUser
	var total int
	if a.Users != nil {
		users, n, err := a.Users.ListPage(sessionRouterID(r), filters.toCore(), limit, offset)
		if err != nil {
			if core.StatusOf(err) == http.StatusServiceUnavailable {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		total = n
		page = service.FromCoreUsers(users)
	} else {
		users, err := a.loadFilteredHotspotUsers(r, cl, filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		total = len(users)
		page = service.PageSlice(users, limit, offset)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"users":  page,
	})
}

// HandleUsers renders the user list.
func (a *App) HandleUsers(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	filters := userListFiltersFromRequest(r)
	printDefaultURL, printQRURL, printCompactURL := userListPrintURLs(filters)

	type UsersData struct {
		Profiles        []routeros.HotspotUserProfile
		Comments        []userCommentOption
		Profile         string
		Comment         string
		Search          string
		Expired         bool
		ExpiredCount    int
		ExportCSVURL    string
		ExportScriptURL string
		PrintDefaultURL string
		PrintThermalURL string
		PrintCompactURL string
	}

	d := UsersData{
		Profile:         filters.Profile,
		Comment:         filters.Comment,
		Search:          filters.Search,
		Expired:         filters.Expired,
		ExportCSVURL:    userListURL("/hotspot/users/export-csv", userListQuery(filters, nil)),
		ExportScriptURL: userListURL("/hotspot/users/export-script", userListQuery(filters, nil)),
		PrintDefaultURL: printDefaultURL,
		PrintThermalURL: printQRURL,
		PrintCompactURL: printCompactURL,
	}
	connected := (cl != nil && cl.IsConnected()) || a.Users != nil || a.Profiles != nil
	if connected {
		if profiles, err := a.loadHotspotProfiles(r, cl); err == nil {
			d.Profiles = profiles
		}

		var users []routeros.HotspotUser
		if a.Users != nil {
			if coreUsers, err := a.Users.List(sessionRouterID(r), core.UserFilters{}); err == nil {
				users = service.FromCoreUsers(coreUsers)
			}
		} else {
			users, _ = a.getUsersCached(r, "")
		}
		commentSource := filterHotspotUsers(users, userListFilters{Profile: filters.Profile})
		d.Comments = buildUserCommentOptions(commentSource)
		now := time.Now()
		for _, u := range users {
			if routeros.IsExpiredHotspotUser(u, now) {
				d.ExpiredCount++
			}
		}
	}

	a.render(w, r, "users.html", TemplateData{
		Title:      "User List — MikVoc",
		ActiveMenu: "users",
		Data:       d,
	})
}

// HandlePrint renders the printable voucher page based on filters (standalone, no layout).
// Mikhmon-compatible: filter by comment batch prints unused vouchers (uptime=0) only,
// unless ids= is set (selected users) or all=1 is forced.
func (a *App) HandlePrint(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	filters := userListFiltersFromRequest(r)
	users, err := a.loadFilteredHotspotUsers(r, cl, filters)
	if err != nil {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}

	// Mikhmon: print by comment only prints unused vouchers (uptime 0 / never used).
	// Skip this when specific ids selected or all=1.
	forceAll := r.URL.Query().Get("all") == "1"
	if !forceAll && len(filters.IDs) == 0 && filters.Comment != "" {
		unused := make([]routeros.HotspotUser, 0, len(users))
		for _, u := range users {
			if routeros.IsUnusedHotspotUser(u) {
				unused = append(unused, u)
			}
		}
		users = unused
	}

	routerName := ""
	if rt := a.routerFor(r); rt != nil {
		routerName = rt.Name
	}

	profileMeta := make(map[string]voucherProfileMeta)
	profiles, _ := a.loadHotspotProfiles(r, cl)
	for _, p := range profiles {
		profileMeta[p.Name] = voucherProfileMeta{Price: profileVoucherPrice(p), Validity: p.Validity}
	}

	tmplID := strings.TrimSpace(r.URL.Query().Get("template"))
	if tmplID == "" {
		if r.URL.Query().Get("small") == "yes" {
			tmplID = "compact"
		} else {
			tmplID = database.GetRouterVoucherTemplate(sessionRouterID(r))
		}
	}
	if tmplID == "" {
		tmplID = "classic"
	}

	withQR := r.URL.Query().Get("qr") == "yes"
	html := generateMultiPrintHTML(tmplID, sessionRouterID(r), routerName, users, profileMeta, withQR)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(html))
}

// HandleUserRemove deletes a user by ID (POST).
func (a *App) HandleUserRemove(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal membaca pilihan user: "+err.Error(), false, nil)
		return
	}
	ids := formIDs(r, "id")
	if len(ids) == 0 {
		a.finishUserAction(w, r, http.StatusBadRequest, "Pilih user terlebih dulu.", false, nil)
		return
	}

	removed := 0
	var firstErr error
	if a.Users != nil {
		routerID := sessionRouterID(r)
		n, err := a.Users.Remove(routerID, ids)
		removed = n
		firstErr = err
	} else {
		if cl == nil {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		for _, id := range ids {
			if err := cl.RemoveUser(id); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			removed++
		}
		if removed > 0 {
			a.invalidateUsers(r)
		}
	}
	if removed > 0 {
		a.audit(r, "user_remove", fmt.Sprintf("count=%d", removed))
	}

	if firstErr != nil && removed == 0 {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal hapus user: "+firstErr.Error(), false, map[string]any{"removed": removed})
		return
	}
	if firstErr != nil {
		a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user berhasil dihapus, sebagian gagal: %v", removed, firstErr), true, map[string]any{"removed": removed})
		return
	}
	if removed == 1 {
		a.finishUserAction(w, r, http.StatusOK, "User berhasil dihapus.", false, map[string]any{"removed": removed})
		return
	}
	a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user berhasil dihapus.", removed), false, map[string]any{"removed": removed})
}

// HandleUserDisable enables or disables a user (POST).
func (a *App) HandleUserDisable(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	id := r.FormValue("id")
	disabled := r.FormValue("disabled") == "true"
	if id == "" {
		a.finishUserAction(w, r, http.StatusBadRequest, "ID user tidak valid.", false, nil)
		return
	}
	var err error
	if a.Users != nil {
		_, err = a.Users.SetDisabled(sessionRouterID(r), []string{id}, disabled)
	} else {
		if cl == nil {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		err = cl.SetUserDisabled(id, disabled)
		if err == nil {
			a.invalidateUsers(r)
		}
	}
	if err != nil {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal update user: "+err.Error(), false, nil)
		return
	}
	a.finishUserAction(w, r, http.StatusOK, "Status user berhasil diperbarui.", false, nil)
}

// HandleUserReset resets a user's counters (POST).
func (a *App) HandleUserReset(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	id := r.FormValue("id")
	if id == "" {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	var err error
	if a.Users != nil {
		err = a.Users.Reset(sessionRouterID(r), id)
	} else if cl != nil {
		err = cl.ResetUser(id)
		if err == nil {
			a.invalidateUsers(r)
		}
	}
	if err != nil {
		a.setFlash(w, r, "Gagal reset: "+err.Error())
	} else {
		a.setFlash(w, r, "Counter user berhasil direset.")
	}
	http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
}

// HandleUserGet returns a single hotspot user as JSON (for edit modal).
func (a *App) HandleUserGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"message":"ID user tidak valid"}`))
		return
	}
	writeUserJSON := func(u routeros.HotspotUser) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":           true,
			"id":           u.ID,
			"name":         u.Name,
			"password":     u.Password,
			"profile":      u.Profile,
			"server":       u.Server,
			"comment":      u.Comment,
			"limit_uptime": u.LimitUptime,
			"limit_bytes":  u.LimitBytesTotal,
			"mac_address":  u.MacAddress,
			"disabled":     u.Disabled,
			"uptime":       u.Uptime,
			"bytes_in":     u.BytesIn,
			"bytes_out":    u.BytesOut,
		})
	}
	if a.Users != nil {
		u, err := a.Users.Get(sessionRouterID(r), id)
		if err != nil {
			status := core.StatusOf(err)
			msg := "User tidak ditemukan"
			if status != http.StatusNotFound {
				msg = err.Error()
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "message": msg})
			return
		}
		writeUserJSON(routeros.HotspotUser{
			ID: u.ID, Name: u.Name, Password: u.Password, Profile: u.Profile,
			Server: u.Server, Comment: u.Comment, LimitUptime: u.LimitUptime,
			LimitBytesTotal: u.LimitBytesTotal, MacAddress: u.MacAddress,
			Disabled: u.Disabled, Uptime: u.Uptime, BytesIn: u.BytesIn, BytesOut: u.BytesOut,
		})
		return
	}
	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"ok":false,"message":"Router tidak terhubung"}`))
		return
	}
	if u, ok := a.getUserByIDCached(r, id); ok {
		writeUserJSON(u)
		return
	}
	users, err := a.getUsersCached(r, "")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok":false,"message":"` + err.Error() + `"}`))
		return
	}
	for _, u := range users {
		if u.ID == id {
			writeUserJSON(u)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"ok":false,"message":"User tidak ditemukan"}`))
}

// HandleUserEdit updates a hotspot user (POST, JSON response).
func (a *App) HandleUserEdit(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal membaca form: "+err.Error(), false, nil)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		a.finishUserAction(w, r, http.StatusBadRequest, "ID user tidak valid.", false, nil)
		return
	}
	u := routeros.HotspotUser{
		Password:        strings.TrimSpace(r.FormValue("password")),
		Profile:         strings.TrimSpace(r.FormValue("profile")),
		Server:          strings.TrimSpace(r.FormValue("server")),
		Comment:         strings.TrimSpace(r.FormValue("comment")),
		LimitUptime:     strings.TrimSpace(r.FormValue("limit_uptime")),
		LimitBytesTotal: strings.TrimSpace(r.FormValue("limit_bytes")),
		MacAddress:      strings.TrimSpace(r.FormValue("mac_address")),
	}
	var err error
	if a.Users != nil {
		err = a.Users.Update(sessionRouterID(r), id, core.User{
			Password:        u.Password,
			Profile:         u.Profile,
			Server:          u.Server,
			Comment:         u.Comment,
			LimitUptime:     u.LimitUptime,
			LimitBytesTotal: u.LimitBytesTotal,
			MacAddress:      u.MacAddress,
		})
	} else {
		if cl == nil || !cl.IsConnected() {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		err = cl.UpdateUser(id, u)
		if err == nil {
			a.invalidateUsers(r)
		}
	}
	if err != nil {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal update user: "+err.Error(), false, nil)
		return
	}
	a.audit(r, "user_edit", id)
	a.finishUserAction(w, r, http.StatusOK, "User berhasil diperbarui.", false, nil)
}

// HandleUserBulkDisable enables/disables multiple users at once (POST, JSON).
func (a *App) HandleUserBulkDisable(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal membaca form: "+err.Error(), false, nil)
		return
	}
	ids := formIDs(r, "id")
	disabled := r.FormValue("disabled") == "true"
	if len(ids) == 0 {
		a.finishUserAction(w, r, http.StatusBadRequest, "Pilih user terlebih dulu.", false, nil)
		return
	}
	done := 0
	var firstErr error
	if a.Users != nil {
		n, err := a.Users.SetDisabled(sessionRouterID(r), ids, disabled)
		done = n
		firstErr = err
	} else {
		if cl == nil || !cl.IsConnected() {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		for _, id := range ids {
			if err := cl.SetUserDisabled(id, disabled); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			done++
		}
		if done > 0 {
			a.invalidateUsers(r)
		}
	}
	action := "diaktifkan"
	if disabled {
		action = "dinonaktifkan"
	}
	if firstErr != nil && done == 0 {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal update user: "+firstErr.Error(), false, map[string]any{"done": done})
		return
	}
	if firstErr != nil {
		a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user %s, sebagian gagal: %v", done, action, firstErr), true, map[string]any{"done": done})
		return
	}
	a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user berhasil %s.", done, action), false, map[string]any{"done": done})
}

// HandleUserBulkProfile moves multiple users to a new profile (POST, JSON).
func (a *App) HandleUserBulkProfile(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal membaca form: "+err.Error(), false, nil)
		return
	}
	ids := formIDs(r, "id")
	profile := strings.TrimSpace(r.FormValue("profile"))
	if len(ids) == 0 {
		a.finishUserAction(w, r, http.StatusBadRequest, "Pilih user terlebih dulu.", false, nil)
		return
	}
	if profile == "" {
		a.finishUserAction(w, r, http.StatusBadRequest, "Profile tujuan tidak valid.", false, nil)
		return
	}
	done := 0
	var firstErr error
	if a.Users != nil {
		n, err := a.Users.SetProfile(sessionRouterID(r), ids, profile)
		done = n
		firstErr = err
	} else {
		if cl == nil || !cl.IsConnected() {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		for _, id := range ids {
			if err := cl.UpdateUser(id, routeros.HotspotUser{Profile: profile}); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			done++
		}
		if done > 0 {
			a.invalidateUsers(r)
		}
	}
	if firstErr != nil && done == 0 {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal pindah profile: "+firstErr.Error(), false, map[string]any{"done": done})
		return
	}
	if firstErr != nil {
		a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user dipindah ke %s, sebagian gagal: %v", done, profile, firstErr), true, map[string]any{"done": done})
		return
	}
	a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user berhasil dipindah ke profile %s.", done, profile), false, map[string]any{"done": done})
}

// HandleRemoveExpired removes all expired users (POST).
func (a *App) HandleRemoveExpired(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	var n int
	var err error
	if a.Users != nil {
		n, err = a.Users.RemoveExpired(sessionRouterID(r))
	} else {
		if cl == nil {
			a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
			return
		}
		n, err = cl.RemoveExpiredUsers(cl.ROSVersion())
		if err == nil && n > 0 {
			a.invalidateUsers(r)
		}
	}
	if err != nil {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Error: "+err.Error(), false, nil)
		return
	}
	a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user expired berhasil dihapus.", n), false, map[string]any{"removed": n})
}

// HandleRemoveComment removes users by their comment (POST).
func (a *App) HandleRemoveComment(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	comment := strings.TrimSpace(r.FormValue("comment"))
	profile := strings.TrimSpace(r.FormValue("profile"))
	if profile == "all" {
		profile = ""
	}
	if comment == "" {
		a.finishUserAction(w, r, http.StatusBadRequest, "Comment wajib dipilih sebelum delete by comment.", false, nil)
		return
	}

	profileText := ""
	if profile != "" {
		profileText = " pada profile '" + profile + "'"
	}

	if a.Users != nil {
		removed, err := a.Users.RemoveByComment(sessionRouterID(r), comment, profile)
		if err != nil {
			var multi *service.CommentMultiProfileError
			if errors.As(err, &multi) {
				a.finishUserAction(w, r, http.StatusConflict, fmt.Sprintf("Comment '%s' ada di beberapa profile (%s). Pilih profile dulu supaya delete tidak lintas profile.", comment, profileListLabel(multi.Profiles)), false, map[string]any{"candidate_profiles": multi.Profiles})
				return
			}
			if errors.Is(err, core.ErrNotFound) {
				a.finishUserAction(w, r, http.StatusNotFound, fmt.Sprintf("Tidak ada user belum terpakai dengan comment '%s' yang bisa dihapus.", comment), false, nil)
				return
			}
			if errors.Is(err, core.ErrNotConnected) || core.StatusOf(err) == http.StatusServiceUnavailable {
				a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
				return
			}
			if removed > 0 {
				a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user belum terpakai dengan comment '%s'%s berhasil dihapus, sebagian gagal: %v", removed, comment, profileText, err), true, map[string]any{"removed": removed})
				return
			}
			a.finishUserAction(w, r, http.StatusInternalServerError, fmt.Sprintf("%d user belum terpakai dengan comment '%s'%s berhasil dihapus, sebagian gagal: %v", removed, comment, profileText, err), false, map[string]any{"removed": removed})
			return
		}
		a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user belum terpakai dengan comment '%s'%s berhasil dihapus.", removed, comment, profileText), false, map[string]any{"removed": removed})
		return
	}

	if cl == nil {
		a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
		return
	}

	users, err := a.getUsersCached(r, profile)
	if err != nil {
		a.finishUserAction(w, r, http.StatusInternalServerError, "Gagal ambil user: "+err.Error(), false, nil)
		return
	}
	candidates := commentRemovalCandidates(users, comment, profile)
	if profile == "" {
		profiles := commentCandidateProfiles(candidates)
		if len(profiles) > 1 {
			a.finishUserAction(w, r, http.StatusConflict, fmt.Sprintf("Comment '%s' ada di beberapa profile (%s). Pilih profile dulu supaya delete tidak lintas profile.", comment, profileListLabel(profiles)), false, map[string]any{"candidate_profiles": profiles})
			return
		}
	}
	if len(candidates) == 0 {
		a.finishUserAction(w, r, http.StatusNotFound, fmt.Sprintf("Tidak ada user belum terpakai dengan comment '%s' yang bisa dihapus.", comment), false, nil)
		return
	}

	removed := 0
	var firstErr error
	for _, u := range candidates {
		if err := cl.RemoveUser(u.ID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	if removed > 0 {
		a.invalidateUsers(r)
	}

	if firstErr != nil {
		status := http.StatusInternalServerError
		if removed > 0 {
			status = http.StatusOK
		}
		a.finishUserAction(w, r, status, fmt.Sprintf("%d user belum terpakai dengan comment '%s'%s berhasil dihapus, sebagian gagal: %v", removed, comment, profileText, firstErr), removed > 0, map[string]any{"removed": removed})
		return
	}
	a.finishUserAction(w, r, http.StatusOK, fmt.Sprintf("%d user belum terpakai dengan comment '%s'%s berhasil dihapus.", removed, comment, profileText), false, map[string]any{"removed": removed})
}

// HandleGenerate renders the voucher generation form (GET) and generates (POST).
// After success, Mikhmon-style: redirect to print page filtered by batch comment.
func (a *App) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	type GenData struct {
		Profiles      []routeros.HotspotUserProfile
		Servers       []map[string]string
		Generated     [][2]string
		LastOpts      routeros.GenerateOptions
		LoginMode     string
		BatchComment  string
		PrintURL      string
		PrintQRURL    string
		PrintSmallURL string
	}

	d := GenData{
		LoginMode: database.GetSetting("tpl_login_mode"),
	}
	if cl != nil && cl.IsConnected() {
		d.Profiles, _ = cl.GetProfiles()
		d.Servers, _ = cl.GetServers()
	}

	if r.Method == http.MethodPost {
		opts := generateOptionsFromRequest(r, d.Profiles)
		d.LastOpts = opts
		if opts.Mode == "" {
			opts.Mode = "vc"
		}

		var generated [][2]string
		var batch string
		var err error
		if a.Generate != nil {
			results, e := a.Generate.Generate(sessionRouterID(r), voucherSpecFromGenerateOpts(opts))
			err = e
			if err == nil && results != nil {
				batch = results.Comment
				generated = make([][2]string, len(results.Items))
				for i, v := range results.Items {
					generated[i] = [2]string{v.Username, v.Password}
				}
			}
		} else if cl != nil {
			generated, batch, err = cl.GenerateUsers(opts)
			if err == nil {
				a.invalidateUsers(r)
			}
		}
		if err != nil {
			a.setFlash(w, r, "Error generate: "+err.Error())
		} else if generated != nil {
			d.Generated = generated
			d.BatchComment = batch
			q := url.Values{}
			q.Set("comment", batch)
			if opts.Profile != "" {
				q.Set("profile", opts.Profile)
			}
			d.PrintURL = "/hotspot/users/print?" + q.Encode()
			q.Set("qr", "yes")
			d.PrintQRURL = "/hotspot/users/print?" + q.Encode()
			q.Del("qr")
			q.Set("template", "compact")
			d.PrintSmallURL = "/hotspot/users/print?" + q.Encode()

			a.audit(r, "user_generate", fmt.Sprintf("count=%d profile=%s comment=%s", len(generated), opts.Profile, batch))
			a.setFlash(w, r, fmt.Sprintf("%d voucher berhasil digenerate. Comment: %s", len(generated), batch))

			if r.FormValue("auto_print") == "1" || r.FormValue("auto_print") == "yes" {
				http.Redirect(w, r, d.PrintURL, http.StatusSeeOther)
				return
			}
		}
	}

	a.render(w, r, "generate.html", TemplateData{
		Title:      "Generate Voucher — MikVoc",
		ActiveMenu: "generate",
		Data:       d,
	})
}

// HandleActiveUsers renders the active hotspot sessions page.
func (a *App) HandleActiveUsers(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	server := r.URL.Query().Get("server")
	type ActiveData struct {
		Active  []routeros.HotspotActive
		Servers []map[string]string
		Server  string
	}
	d := ActiveData{Server: server}
	if a.Users != nil {
		actives, err := a.Users.Active(sessionRouterID(r), server)
		if err == nil {
			d.Active = make([]routeros.HotspotActive, len(actives))
			for i, s := range actives {
				d.Active[i] = routeros.HotspotActive{
					ID: s.ID, User: s.User, Server: s.Server, IP: s.IP,
					MacAddr: s.MacAddr, Uptime: s.Uptime, IdleTime: s.IdleTime,
					BytesIn: s.BytesIn, BytesOut: s.BytesOut, Comment: s.Comment,
				}
			}
		}
		if cl != nil && cl.IsConnected() {
			d.Servers, _ = cl.GetServers()
		}
	} else if cl != nil && cl.IsConnected() {
		d.Active, _ = cl.GetActiveUsers(server)
		d.Servers, _ = cl.GetServers()
	}
	a.render(w, r, "active.html", TemplateData{
		Title:      "Hotspot Aktif — MikVoc",
		ActiveMenu: "active",
		Data:       d,
	})
}

// HandleKickUser kicks an active session (POST).
func (a *App) HandleKickUser(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/active", http.StatusSeeOther)
		return
	}
	id := r.FormValue("id")
	if id == "" {
		http.Redirect(w, r, "/hotspot/active", http.StatusSeeOther)
		return
	}
	var err error
	if a.Users != nil {
		err = a.Users.Kick(sessionRouterID(r), id)
	} else if cl != nil {
		err = cl.KickActiveUser(id)
	}
	if err == nil {
		a.setFlash(w, r, "User berhasil di-kick.")
	}
	http.Redirect(w, r, "/hotspot/active", http.StatusSeeOther)
}

// --- Profiles ---

// HandleProfiles renders the hotspot profiles list.
func (a *App) HandleProfiles(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	type ProfilesData struct {
		Profiles []routeros.HotspotUserProfile
		Pools    []string
		Queues   []string
	}

	d := ProfilesData{}
	if profiles, err := a.loadHotspotProfiles(r, cl); err == nil {
		d.Profiles = profiles
	}
	rid := sessionRouterID(r)
	if a.Profiles != nil {
		if pools, err := a.Profiles.ListPools(rid); err == nil {
			d.Pools = pools
		}
		if queues, err := a.Profiles.ListQueues(rid); err == nil {
			d.Queues = queues
		}
	} else if cl != nil && cl.IsConnected() {
		d.Pools, _ = cl.GetIPPools()
		d.Queues, _ = cl.GetSimpleQueues()
	}

	a.render(w, r, "profiles.html", TemplateData{
		Title:      "Hotspot Profiles — MikVoc",
		ActiveMenu: "profiles",
		Data:       d,
	})
}

// HandleProfileRemove deletes a profile.
func (a *App) HandleProfileRemove(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	var err error
	if a.Profiles != nil {
		err = a.Profiles.Remove(sessionRouterID(r), id)
	} else {
		cl := a.clientFor(r)
		if cl == nil || !cl.IsConnected() {
			a.setFlash(w, r, "Error: Tidak terhubung ke router.")
			http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
			return
		}
		err = cl.RemoveProfile(id)
	}
	if err != nil {
		a.setFlash(w, r, "Error: Gagal menghapus profil ("+err.Error()+")")
	} else {
		a.audit(r, "profile_remove", id)
		a.setFlash(w, r, "Success: Profil berhasil dihapus.")
	}
	http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
}

// HandleProfileCreate creates a new profile.
func (a *App) HandleProfileCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	name := strings.ReplaceAll(strings.TrimSpace(r.FormValue("name")), " ", "-")
	shared := r.FormValue("shared_users")
	rate := r.FormValue("rate_limit")
	price := r.FormValue("price")
	sprice := r.FormValue("sprice")
	addressPool := r.FormValue("address_pool")
	parentQueue := r.FormValue("parent_queue")
	expiredMode := r.FormValue("expired_mode")
	validity := r.FormValue("validity")
	gracePeriod := r.FormValue("grace_period")
	lockMac := r.FormValue("lock_mac")
	if lockMac == "" {
		lockMac = r.FormValue("lockunlock")
	}

	if lockMac == "on" || strings.EqualFold(lockMac, "Enable") || lockMac == "1" {
		lockMac = "1"
	} else {
		lockMac = "0"
	}
	expiredMode = normalizeProfileExpiredForm(expiredMode)
	if gracePeriod == "" {
		gracePeriod = "5m"
	}

	if name == "" {
		a.setFlash(w, r, "Error: Nama profil wajib diisi.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	var err error
	if a.Profiles != nil {
		err = a.Profiles.Create(sessionRouterID(r), name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sprice, gracePeriod)
	} else {
		cl := a.clientFor(r)
		if cl == nil || !cl.IsConnected() {
			a.setFlash(w, r, "Error: Tidak terhubung ke router.")
			http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
			return
		}
		err = cl.CreateProfile(name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sprice, gracePeriod)
	}
	if err != nil {
		a.setFlash(w, r, "Error: Gagal membuat profil ("+err.Error()+")")
	} else {
		a.audit(r, "profile_create", name)
		a.setFlash(w, r, "Success: Profil "+name+" berhasil dibuat.")
	}
	http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
}

// HandleProfileUpdate updates an existing profile.
func (a *App) HandleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	id := r.FormValue("id")
	name := strings.ReplaceAll(strings.TrimSpace(r.FormValue("name")), " ", "-")
	shared := r.FormValue("shared_users")
	rate := r.FormValue("rate_limit")
	price := r.FormValue("price")
	sprice := r.FormValue("sprice")
	addressPool := r.FormValue("address_pool")
	parentQueue := r.FormValue("parent_queue")
	expiredMode := r.FormValue("expired_mode")
	validity := r.FormValue("validity")
	gracePeriod := r.FormValue("grace_period")
	lockMac := r.FormValue("lock_mac")
	if lockMac == "" {
		lockMac = r.FormValue("lockunlock")
	}

	if lockMac == "on" || strings.EqualFold(lockMac, "Enable") || lockMac == "1" {
		lockMac = "1"
	} else {
		lockMac = "0"
	}
	expiredMode = normalizeProfileExpiredForm(expiredMode)
	if gracePeriod == "" {
		gracePeriod = "5m"
	}

	if id == "" {
		a.setFlash(w, r, "Error: ID profil tidak valid.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	var err error
	if a.Profiles != nil {
		err = a.Profiles.Update(sessionRouterID(r), id, name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sprice, gracePeriod)
	} else {
		cl := a.clientFor(r)
		if cl == nil || !cl.IsConnected() {
			a.setFlash(w, r, "Error: Tidak terhubung ke router.")
			http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
			return
		}
		err = cl.UpdateProfile(id, name, shared, rate, addressPool, parentQueue, expiredMode, validity, lockMac, price, sprice, gracePeriod)
	}
	if err != nil {
		a.setFlash(w, r, "Error: Gagal update profil "+name+" ("+err.Error()+")")
	} else {
		a.setFlash(w, r, "Success: Profil "+name+" berhasil diperbarui.")
	}
	http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
}

// --- helpers ---

// HandleExportCSV exports all hotspot users (with optional filter) as a CSV file download.
func (a *App) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		http.Error(w, "Tidak terhubung ke router", http.StatusServiceUnavailable)
		return
	}

	filters := userListFiltersFromRequest(r)
	users, err := a.loadFilteredHotspotUsers(r, cl, filters)
	if err != nil {
		http.Error(w, "Gagal mengambil data user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "mikvoc-users-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Cache-Control", "no-cache")

	// Write UTF-8 BOM so Excel opens correctly
	w.Write([]byte("\xEF\xBB\xBF"))

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"No", "Server", "Username", "Password", "Mode", "Profile", "MAC Address", "Limit Uptime", "Limit Bytes Total", "Uptime", "Bytes In", "Bytes Out", "Komentar", "Status"})
	for i, u := range users {
		status := "Aktif"
		if u.Disabled {
			status = "Nonaktif"
		}
		_ = cw.Write([]string{
			strconv.Itoa(i + 1),
			u.Server,
			u.Name,
			u.Password,
			userListModeLabel(u),
			u.Profile,
			u.MacAddress,
			u.LimitUptime,
			u.LimitBytesTotal,
			u.Uptime,
			u.BytesIn,
			u.BytesOut,
			u.Comment,
			status,
		})
	}
	cw.Flush()
}

// HandleImportCSV imports users from a CSV file (POST multipart, JSON response).
// Expected columns: username, password, profile (others optional: server, comment, limit_uptime, limit_bytes, mac_address).
func (a *App) HandleImportCSV(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		a.finishUserAction(w, r, http.StatusServiceUnavailable, "Router tidak terhubung.", false, nil)
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal membaca upload: "+err.Error(), false, nil)
		return
	}
	file, _, err := r.FormFile("csv_file")
	if err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "File CSV tidak ditemukan.", false, nil)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		a.finishUserAction(w, r, http.StatusBadRequest, "Gagal parse CSV: "+err.Error(), false, nil)
		return
	}
	if len(rows) < 2 {
		a.finishUserAction(w, r, http.StatusBadRequest, "CSV kosong atau tidak ada data.", false, nil)
		return
	}

	header := rows[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"username", "password", "profile"}
	for _, k := range required {
		if _, ok := colIdx[k]; !ok {
			a.finishUserAction(w, r, http.StatusBadRequest, "Kolom wajib tidak ada: "+k, false, nil)
			return
		}
	}
	getCell := func(row []string, key string) string {
		if i, ok := colIdx[key]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	created, skipped := 0, 0
	routerID := sessionRouterID(r)
	for _, row := range rows[1:] {
		name := getCell(row, "username")
		pass := getCell(row, "password")
		profile := getCell(row, "profile")
		if name == "" {
			skipped++
			continue
		}
		if pass == "" {
			pass = name
		}
		u := routeros.HotspotUser{
			Name:            name,
			Password:        pass,
			Profile:         profile,
			Server:          getCell(row, "server"),
			Comment:         getCell(row, "comment"),
			LimitUptime:     getCell(row, "limit_uptime"),
			LimitBytesTotal: getCell(row, "limit_bytes"),
			MacAddress:      getCell(row, "mac_address"),
		}
		var addErr error
		if a.Users != nil {
			addErr = a.Users.Add(routerID, core.User{
				Name: u.Name, Password: u.Password, Profile: u.Profile,
				Server: u.Server, Comment: u.Comment, LimitUptime: u.LimitUptime,
				LimitBytesTotal: u.LimitBytesTotal, MacAddress: u.MacAddress,
			})
		} else {
			addErr = cl.AddUser(u)
		}
		if addErr != nil {
			skipped++
			continue
		}
		created++
	}
	if created > 0 && a.Users == nil {
		a.invalidateUsers(r)
	}
	msg := fmt.Sprintf("%d user diimport, %d dilewati.", created, skipped)
	warning := skipped > 0
	a.finishUserAction(w, r, http.StatusOK, msg, warning, map[string]any{"created": created, "skipped": skipped})
}

// HandleExportScript exports users as a RouterOS import script, like Mikhmon's script export.
func (a *App) HandleExportScript(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		http.Error(w, "Tidak terhubung ke router", http.StatusServiceUnavailable)
		return
	}

	filters := userListFiltersFromRequest(r)
	users, err := a.loadFilteredHotspotUsers(r, cl, filters)
	if err != nil {
		http.Error(w, "Gagal mengambil data user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "mikvoc-users-" + time.Now().Format("20060102-150405") + ".rsc"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Cache-Control", "no-cache")

	fmt.Fprintln(w, "/ip hotspot user")
	for _, u := range users {
		parts := []string{
			"add",
			"name=" + routerOSQuote(u.Name),
			"password=" + routerOSQuote(u.Password),
			"profile=" + routerOSQuote(u.Profile),
		}
		if u.Server != "" && u.Server != "all" {
			parts = append(parts, "server="+routerOSQuote(u.Server))
		}
		if u.Comment != "" {
			parts = append(parts, "comment="+routerOSQuote(u.Comment))
		}
		if u.LimitUptime != "" && u.LimitUptime != "0s" {
			parts = append(parts, "limit-uptime="+routerOSQuote(u.LimitUptime))
		}
		if u.LimitBytesTotal != "" && u.LimitBytesTotal != "0" {
			parts = append(parts, "limit-bytes-total="+routerOSQuote(u.LimitBytesTotal))
		}
		if u.MacAddress != "" {
			parts = append(parts, "mac-address="+routerOSQuote(u.MacAddress))
		}
		if u.Disabled {
			parts = append(parts, "disabled=yes")
		}
		fmt.Fprintln(w, strings.Join(parts, " "))
	}
}

// HandleQuickPrint renders a printable single-voucher slip for one user (GET).
// Uses the voucher_template setting to choose the print style.
func (a *App) HandleQuickPrint(w http.ResponseWriter, r *http.Request) {
	cl := a.clientFor(r)
	if a.Users == nil && (cl == nil || !cl.IsConnected()) {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}

	var found *routeros.HotspotUser
	if a.Users != nil {
		if u, err := a.Users.GetByName(sessionRouterID(r), username); err == nil {
			uc := u
			found = &uc
		}
	} else if u, ok := a.getUserByNameCached(r, username); ok {
		uc := u
		found = &uc
	} else {
		users, err := a.getUsersCached(r, "")
		if err != nil {
			http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
			return
		}
		for _, u := range users {
			if u.Name == username {
				uc := u
				found = &uc
				break
			}
		}
	}
	if found == nil {
		http.Redirect(w, r, "/hotspot/users", http.StatusSeeOther)
		return
	}

	routerName := ""
	if rt := a.routerFor(r); rt != nil {
		routerName = rt.Name
	}

	// Look up the user's profile to get price/validity (like Mikhmon)
	price := ""
	validity := ""
	profiles, _ := a.loadHotspotProfiles(r, cl)
	for _, p := range profiles {
		if p.Name == found.Profile {
			price = profileVoucherPrice(p)
			validity = p.Validity
			break
		}
	}

	// Read voucher template preference from active router (default: classic)
	tmplID := strings.TrimSpace(r.URL.Query().Get("template"))
	if tmplID == "" {
		tmplID = database.GetRouterVoucherTemplate(sessionRouterID(r))
	}
	if tmplID == "" {
		tmplID = "classic"
	}
	withQR := r.URL.Query().Get("qr") == "yes"

	html := generateQuickPrintHTML(
		tmplID, sessionRouterID(r), routerName,
		found.Name, found.Password, found.Profile,
		found.LimitUptime, found.LimitBytesTotal,
		found.Comment, price, validity,
		withQR,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func profileVoucherPrice(p routeros.HotspotUserProfile) string {
	if price := strings.TrimSpace(p.SellingPrice); price != "" && price != "0" {
		return price
	}
	return p.Price
}

// HandleSetMonitorProfile creates or removes the Mikrotik scheduler that auto-deletes expired hotspot users.
// POST body: profile_name=xxx&action=enable|disable
func (a *App) HandleSetMonitorProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	profileName := r.FormValue("profile_name")
	action := r.FormValue("action")

	if profileName == "" {
		a.setFlash(w, r, "Error: Nama profil tidak valid.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	enable := action != "disable"
	if a.Profiles != nil {
		if err := a.Profiles.SetMonitor(sessionRouterID(r), profileName, enable); err != nil {
			if enable {
				a.setFlash(w, r, "Gagal aktifkan monitor: "+err.Error())
			} else {
				a.setFlash(w, r, "Gagal nonaktifkan monitor: "+err.Error())
			}
		} else if enable {
			a.setFlash(w, r, "Monitor Profile '"+profileName+"' aktif — expired users akan dihapus tiap 1 jam.")
		} else {
			a.setFlash(w, r, "Monitor Profile '"+profileName+"' berhasil dinonaktifkan.")
		}
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		a.setFlash(w, r, "Error: Tidak terhubung ke router.")
		http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
		return
	}

	schedulerName := "mikvoc-monitor-" + strings.ReplaceAll(profileName, " ", "_")

	if action == "disable" {
		rows, err := cl.Run("/system/scheduler/print", map[string]string{"?name": schedulerName})
		if err == nil && len(rows) > 0 {
			id := rows[0][".id"]
			_, err = cl.Run("/system/scheduler/remove", map[string]string{".id": id})
		}
		if err != nil {
			a.setFlash(w, r, "Gagal nonaktifkan monitor: "+err.Error())
		} else {
			a.setFlash(w, r, "Monitor Profile '"+profileName+"' berhasil dinonaktifkan.")
		}
	} else {
		script := `:local profileName "` + profileName + `"
:foreach u in=[/ip hotspot user find profile=$profileName] do={
  :local uptime [/ip hotspot user get $u uptime]
  :local limitUptime [/ip hotspot user get $u limit-uptime]
  :if ($limitUptime != "0s" and $uptime >= $limitUptime) do={
    /ip hotspot user remove $u
  }
}`
		rows, err := cl.Run("/system/scheduler/print", map[string]string{"?name": schedulerName})
		if err == nil && len(rows) > 0 {
			id := rows[0][".id"]
			_, err = cl.Run("/system/scheduler/set", map[string]string{
				".id":      id,
				"on-event": script,
			})
		} else {
			_, err = cl.Run("/system/scheduler/add", map[string]string{
				"name":     schedulerName,
				"interval": "01:00:00",
				"on-event": script,
				"comment":  "MikVoc Monitor: " + profileName,
			})
		}
		if err != nil {
			a.setFlash(w, r, "Gagal aktifkan monitor: "+err.Error())
		} else {
			a.setFlash(w, r, "Monitor Profile '"+profileName+"' aktif — expired users akan dihapus tiap 1 jam.")
		}
	}
	http.Redirect(w, r, "/hotspot/profiles", http.StatusSeeOther)
}

func parseIntDefault(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}
