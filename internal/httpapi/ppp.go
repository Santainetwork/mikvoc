package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mikvoc/internal/core"
	"mikvoc/internal/service"
)

func (a *App) finishPPPAction(w http.ResponseWriter, r *http.Request, redirect, message string, status int) {
	if wantsUserActionJSON(r) {
		if status == 0 {
			status = http.StatusOK
		}
		payload := map[string]any{
			"ok":      status < http.StatusBadRequest,
			"message": message,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	if message != "" {
		a.setFlash(w, r, message)
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func pppFiltersFromRequest(r *http.Request) core.PPPFilters {
	f := core.PPPFilters{
		Profile: strings.TrimSpace(r.URL.Query().Get("profile")),
		Search:  strings.TrimSpace(r.URL.Query().Get("q")),
		Comment: strings.TrimSpace(r.URL.Query().Get("comment")),
	}
	if d := strings.TrimSpace(r.URL.Query().Get("disabled")); d == "1" || d == "true" {
		v := true
		f.Disabled = &v
	} else if d == "0" || d == "false" {
		v := false
		f.Disabled = &v
	}
	return f
}

func (a *App) HandlePPPSecrets(w http.ResponseWriter, r *http.Request) {
	rid := sessionRouterID(r)
	filters := pppFiltersFromRequest(r)
	type commentOpt struct {
		Value   string
		Profile string
		Count   int
	}
	type pageData struct {
		Secrets      []core.PPPSecret
		Profiles     []core.PPPProfile
		Comments     []commentOpt
		Profile      string
		Search       string
		Comment      string
		Disabled     string
		ExportCSVURL string
	}
	d := pageData{
		Profile: filters.Profile,
		Search:  filters.Search,
		Comment: filters.Comment,
	}
	if filters.Disabled != nil {
		if *filters.Disabled {
			d.Disabled = "1"
		} else {
			d.Disabled = "0"
		}
	}
	q := r.URL.Query()
	d.ExportCSVURL = "/ppp/secrets/export-csv"
	if enc := q.Encode(); enc != "" {
		d.ExportCSVURL += "?" + enc
	}

	if a.PPP != nil {
		if list, err := a.PPP.ListSecrets(rid, filters); err == nil {
			d.Secrets = list
		}
		if profiles, err := a.PPP.ListProfiles(rid); err == nil {
			d.Profiles = profiles
		}
		if all, err := a.PPP.ListSecrets(rid, core.PPPFilters{}); err == nil {
			counts := map[string]commentOpt{}
			for _, s := range all {
				c := strings.TrimSpace(s.Comment)
				if c == "" {
					continue
				}
				key := c + "\x00" + s.Profile
				opt := counts[key]
				opt.Value = c
				opt.Profile = s.Profile
				opt.Count++
				counts[key] = opt
			}
			for _, opt := range counts {
				d.Comments = append(d.Comments, opt)
			}
		}
	}

	a.render(w, r, "ppp_secrets.html", TemplateData{
		Title:      "PPP Secrets — MikVoc",
		ActiveMenu: "ppp-secrets",
		Data:       d,
	})
}

func (a *App) HandlePPPSecretCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/secrets", http.StatusSeeOther)
		return
	}
	sec := core.PPPSecret{
		Name:          strings.TrimSpace(r.FormValue("name")),
		Password:      r.FormValue("password"),
		Service:       strings.TrimSpace(r.FormValue("service")),
		Profile:       strings.TrimSpace(r.FormValue("profile")),
		LocalAddress:  strings.TrimSpace(r.FormValue("local_address")),
		RemoteAddress: strings.TrimSpace(r.FormValue("remote_address")),
		Comment:       strings.TrimSpace(r.FormValue("comment")),
		CallerID:      strings.TrimSpace(r.FormValue("caller_id")),
	}
	if sec.Password == "" {
		sec.Password = sec.Name
	}
	if sec.Service == "" {
		sec.Service = "pppoe"
	}
	var err error
	if a.PPP != nil {
		err = a.PPP.AddSecret(sessionRouterID(r), sec)
	} else {
		err = core.ErrNotConnected
	}
	msg := "Secret ditambahkan."
	status := http.StatusOK
	if err != nil {
		msg = "Gagal tambah secret: " + err.Error()
		status = core.StatusOf(err)
	}
	a.finishPPPAction(w, r, "/ppp/secrets", msg, status)
}

func (a *App) HandlePPPSecretEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/secrets", http.StatusSeeOther)
		return
	}
	sec := core.PPPSecret{
		ID:            strings.TrimSpace(r.FormValue("id")),
		Name:          strings.TrimSpace(r.FormValue("name")),
		Password:      r.FormValue("password"),
		Service:       strings.TrimSpace(r.FormValue("service")),
		Profile:       strings.TrimSpace(r.FormValue("profile")),
		LocalAddress:  strings.TrimSpace(r.FormValue("local_address")),
		RemoteAddress: strings.TrimSpace(r.FormValue("remote_address")),
		Comment:       strings.TrimSpace(r.FormValue("comment")),
		CallerID:      strings.TrimSpace(r.FormValue("caller_id")),
		Disabled:      r.FormValue("disabled") == "1" || r.FormValue("disabled") == "true" || r.FormValue("disabled") == "on",
	}
	var err error
	if a.PPP != nil {
		err = a.PPP.UpdateSecret(sessionRouterID(r), sec)
	} else {
		err = core.ErrNotConnected
	}
	msg := "Secret diupdate."
	status := http.StatusOK
	if err != nil {
		msg = "Gagal update: " + err.Error()
		status = core.StatusOf(err)
	}
	a.finishPPPAction(w, r, "/ppp/secrets", msg, status)
}

func (a *App) HandlePPPSecretRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/secrets", http.StatusSeeOther)
		return
	}
	ids := formIDs(r, "id")
	if len(ids) == 0 {
		if id := strings.TrimSpace(r.FormValue("id")); id != "" {
			ids = []string{id}
		}
	}
	n, err := 0, error(nil)
	if a.PPP != nil {
		n, err = a.PPP.RemoveSecrets(sessionRouterID(r), ids)
	} else {
		err = core.ErrNotConnected
	}
	msg := fmt.Sprintf("Dihapus %d secret.", n)
	status := http.StatusOK
	if err != nil && n == 0 {
		msg = "Gagal hapus: " + err.Error()
		status = core.StatusOf(err)
	} else if err != nil {
		msg = fmt.Sprintf("Dihapus %d secret (sebagian gagal: %v).", n, err)
	}
	a.finishPPPAction(w, r, "/ppp/secrets", msg, status)
}

func (a *App) HandlePPPSecretDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/secrets", http.StatusSeeOther)
		return
	}
	ids := formIDs(r, "id")
	if len(ids) == 0 {
		if id := strings.TrimSpace(r.FormValue("id")); id != "" {
			ids = []string{id}
		}
	}
	disabled := r.FormValue("disabled") == "1" || r.FormValue("action") == "disable"
	if r.FormValue("action") == "enable" {
		disabled = false
	}
	n, err := 0, error(nil)
	if a.PPP != nil {
		n, err = a.PPP.SetDisabled(sessionRouterID(r), ids, disabled)
	} else {
		err = core.ErrNotConnected
	}
	msg := fmt.Sprintf("Enable %d secret.", n)
	if disabled {
		msg = fmt.Sprintf("Disable %d secret.", n)
	}
	status := http.StatusOK
	if err != nil && n == 0 {
		msg = "Gagal: " + err.Error()
		status = core.StatusOf(err)
	}
	a.finishPPPAction(w, r, "/ppp/secrets", msg, status)
}

func (a *App) HandlePPPExportCSV(w http.ResponseWriter, r *http.Request) {
	if a.PPP == nil {
		http.Error(w, "not connected", http.StatusServiceUnavailable)
		return
	}
	filters := pppFiltersFromRequest(r)
	list, err := a.PPP.ListSecrets(sessionRouterID(r), filters)
	if err != nil {
		http.Error(w, err.Error(), core.StatusOf(err))
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ppp-secrets.csv"`)
	_, _ = w.Write([]byte("\ufeff"))
	_ = service.WritePPPSecretsCSV(w, list)
}

func (a *App) HandlePPPImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/secrets", http.StatusSeeOther)
		return
	}
	if a.PPP == nil {
		a.finishPPPAction(w, r, "/ppp/secrets", "Router tidak terhubung.", http.StatusServiceUnavailable)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		file, _, err = r.FormFile("csv")
	}
	if err != nil {
		a.finishPPPAction(w, r, "/ppp/secrets", "File CSV wajib.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	n, err := a.PPP.ImportCSV(sessionRouterID(r), file)
	msg := fmt.Sprintf("Import %d secret.", n)
	status := http.StatusOK
	if err != nil && n == 0 {
		msg = "Import gagal: " + err.Error()
		status = core.StatusOf(err)
	} else if err != nil {
		msg = fmt.Sprintf("Import %d secret (sebagian gagal: %v).", n, err)
	}
	a.finishPPPAction(w, r, "/ppp/secrets", msg, status)
}

func (a *App) HandlePPPGenerate(w http.ResponseWriter, r *http.Request) {
	rid := sessionRouterID(r)
	type lastOpts struct {
		Qty     int
		Prefix  string
		Start   int
		Pad     int
		Profile string
		Service string
		Comment string
	}
	type pageData struct {
		Profiles []core.PPPProfile
		LastOpts lastOpts
		Preview  []string
		Result   *core.PPPGenerateBatch
	}
	d := pageData{
		LastOpts: lastOpts{Qty: 10, Prefix: "user", Start: 1, Pad: 3, Service: "pppoe"},
	}
	if a.PPP != nil {
		if profiles, err := a.PPP.ListProfiles(rid); err == nil {
			d.Profiles = profiles
		}
	}

	if r.Method == http.MethodPost {
		qty, _ := strconv.Atoi(r.FormValue("qty"))
		start, _ := strconv.Atoi(r.FormValue("start"))
		pad, _ := strconv.Atoi(r.FormValue("pad"))
		spec := core.PPPGenerateSpec{
			Qty:     qty,
			Prefix:  strings.TrimSpace(r.FormValue("prefix")),
			Start:   start,
			Pad:     pad,
			Profile: strings.TrimSpace(r.FormValue("profile")),
			Service: strings.TrimSpace(r.FormValue("service")),
			Comment: strings.TrimSpace(r.FormValue("comment")),
		}
		d.LastOpts = lastOpts{
			Qty: spec.Qty, Prefix: spec.Prefix, Start: spec.Start, Pad: spec.Pad,
			Profile: spec.Profile, Service: spec.Service, Comment: spec.Comment,
		}
		prevN := spec.Qty
		if prevN > 5 {
			prevN = 5
		}
		d.Preview = service.PPPGeneratePreview(spec.Prefix, spec.Start, spec.Pad, prevN)
		if a.PPP == nil {
			a.setFlash(w, r, "Router tidak terhubung.")
		} else {
			batch, err := a.PPP.Generate(rid, spec)
			if batch != nil {
				d.Result = batch
			}
			if err != nil && (batch == nil || len(batch.Items) == 0) {
				a.setFlash(w, r, "Generate gagal: "+err.Error())
			} else {
				msg := service.FormatPPPBatchSummary(batch)
				if err != nil {
					msg += " (sebagian error: " + err.Error() + ")"
				}
				a.setFlash(w, r, msg)
			}
		}
	} else {
		d.Preview = service.PPPGeneratePreview(d.LastOpts.Prefix, d.LastOpts.Start, d.LastOpts.Pad, 5)
	}

	a.render(w, r, "ppp_generate.html", TemplateData{
		Title:      "Generate PPP — MikVoc",
		ActiveMenu: "ppp-generate",
		Data:       d,
	})
}

func (a *App) HandlePPPActive(w http.ResponseWriter, r *http.Request) {
	type pageData struct {
		Active []core.PPPActive
	}
	d := pageData{}
	if a.PPP != nil {
		if list, err := a.PPP.Active(sessionRouterID(r)); err == nil {
			d.Active = list
		}
	}
	a.render(w, r, "ppp_active.html", TemplateData{
		Title:      "PPP Aktif — MikVoc",
		ActiveMenu: "ppp-active",
		Data:       d,
	})
}

func (a *App) HandlePPPKick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/active", http.StatusSeeOther)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	var err error
	if a.PPP != nil {
		err = a.PPP.Kick(sessionRouterID(r), id)
	} else {
		err = core.ErrNotConnected
	}
	if err == nil {
		a.setFlash(w, r, "Sesi PPP di-kick.")
	} else {
		a.setFlash(w, r, "Kick gagal: "+err.Error())
	}
	http.Redirect(w, r, "/ppp/active", http.StatusSeeOther)
}

func (a *App) HandlePPPProfiles(w http.ResponseWriter, r *http.Request) {
	type pageData struct {
		Profiles []core.PPPProfile
		Pools    []string
		Bridges  []string
	}
	d := pageData{}
	if a.PPP != nil {
		if list, err := a.PPP.ListProfiles(sessionRouterID(r)); err == nil {
			d.Profiles = list
		}
		if pools, err := a.PPP.ListPools(sessionRouterID(r)); err == nil {
			d.Pools = pools
		}
		if bridges, err := a.PPP.ListBridges(sessionRouterID(r)); err == nil {
			d.Bridges = bridges
		}
	}
	a.render(w, r, "ppp_profiles.html", TemplateData{
		Title:      "PPP Profiles — MikVoc",
		ActiveMenu: "ppp-profiles",
		Data:       d,
	})
}

func (a *App) HandlePPPProfileCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
		return
	}
	p := pppProfileFromForm(r)
	var err error
	if a.PPP != nil {
		err = a.PPP.CreateProfile(sessionRouterID(r), p)
	} else {
		err = core.ErrNotConnected
	}
	if err != nil {
		a.setFlash(w, r, "Gagal buat profile: "+err.Error())
	} else {
		a.setFlash(w, r, "Profile dibuat.")
	}
	http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
}

func (a *App) HandlePPPProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
		return
	}
	p := pppProfileFromForm(r)
	p.ID = strings.TrimSpace(r.FormValue("id"))
	var err error
	if a.PPP != nil {
		err = a.PPP.UpdateProfile(sessionRouterID(r), p)
	} else {
		err = core.ErrNotConnected
	}
	if err != nil {
		a.setFlash(w, r, "Gagal update: "+err.Error())
	} else {
		a.setFlash(w, r, "Profile diupdate.")
	}
	http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
}

func pppProfileFromForm(r *http.Request) core.PPPProfile {
	return core.PPPProfile{
		Name:           strings.ReplaceAll(strings.TrimSpace(r.FormValue("name")), " ", "-"),
		LocalAddress:   strings.TrimSpace(r.FormValue("local_address")),
		RemoteAddress:  strings.TrimSpace(r.FormValue("remote_address")),
		Bridge:         strings.TrimSpace(r.FormValue("bridge")),
		IncomingFilter: strings.TrimSpace(r.FormValue("incoming_filter")),
		OutgoingFilter: strings.TrimSpace(r.FormValue("outgoing_filter")),
		AddressList:    strings.TrimSpace(r.FormValue("address_list")),
		DNSServer:      strings.TrimSpace(r.FormValue("dns_server")),
		WINSServer:     strings.TrimSpace(r.FormValue("wins_server")),
		ChangeTCPMSS:   strings.TrimSpace(r.FormValue("change_tcp_mss")),
		UseUPnP:        strings.TrimSpace(r.FormValue("use_upnp")),
		RateLimit:      strings.TrimSpace(r.FormValue("rate_limit")),
		OnlyOne:        strings.TrimSpace(r.FormValue("only_one")),
	}
}

func (a *App) HandlePPPProfileRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	var err error
	if a.PPP != nil {
		err = a.PPP.RemoveProfile(sessionRouterID(r), id)
	} else {
		err = core.ErrNotConnected
	}
	if err != nil {
		a.setFlash(w, r, "Gagal hapus: "+err.Error())
	} else {
		a.setFlash(w, r, "Profile dihapus.")
	}
	http.Redirect(w, r, "/ppp/profiles", http.StatusSeeOther)
}
