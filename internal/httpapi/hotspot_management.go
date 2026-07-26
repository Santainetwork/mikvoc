package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mikvoc/internal/core"
)

type hostsPageData struct {
	Hosts          []core.HotspotHost
	Server, Search string
	Servers        []string
	Available      bool
	Error          string
}

type bindingsPageData struct {
	Bindings  []core.IPBinding
	Available bool
	Error     string
}

type cookiesPageData struct {
	Cookies   []core.HotspotCookie
	Search    string
	Available bool
	Error     string
}

type logsPageData struct {
	Logs          []core.SystemLog
	Topic, Search string
	Limit         int
	Available     bool
	Error         string
}

type serversPageData struct {
	Servers    []core.HotspotServer
	Interfaces []core.RouterInterface
	Pools      []string
	Profiles   []core.HotspotServerProfile
	Available  bool
	Error      string
}

type serverProfilesPageData struct {
	Profiles  []core.HotspotServerProfile
	Available bool
	Error     string
}

func filterHosts(hosts []core.HotspotHost, server, search string) []core.HotspotHost {
	server, search = strings.TrimSpace(server), strings.ToLower(strings.TrimSpace(search))
	out := make([]core.HotspotHost, 0, len(hosts))
	for _, host := range hosts {
		if server != "" && host.Server != server {
			continue
		}
		text := strings.ToLower(strings.Join([]string{host.ID, host.MACAddress, host.Address, host.ToAddress, host.Server}, " "))
		if search == "" || strings.Contains(text, search) {
			out = append(out, host)
		}
	}
	return out
}

func (a *App) HandleHotspotHosts(w http.ResponseWriter, r *http.Request) {
	data := hostsPageData{Server: strings.TrimSpace(r.URL.Query().Get("server")), Search: strings.TrimSpace(r.URL.Query().Get("q")), Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		hosts, err := a.RouterManagement.ListHosts(sessionRouterID(r))
		if err != nil {
			data.Error = managementError(err)
		} else {
			seen := map[string]bool{}
			for _, host := range hosts {
				if host.Server != "" && !seen[host.Server] {
					seen[host.Server] = true
					data.Servers = append(data.Servers, host.Server)
				}
			}
			data.Hosts = filterHosts(hosts, data.Server, data.Search)
		}
	}
	a.render(w, r, "hotspot_hosts.html", TemplateData{Title: "Hotspot Hosts", ActiveMenu: "hosts", Data: data})
}

func (a *App) HandleHostMakeBinding(w http.ResponseWriter, r *http.Request) {
	if a.RouterManagement == nil {
		a.finishManagementAction(w, r, "/hotspot/hosts", "host_binding", r.FormValue("id"), fmt.Errorf("layanan manajemen router tidak tersedia"))
		return
	}
	err := a.RouterManagement.MakeHostBinding(sessionRouterID(r), r.FormValue("id"), r.FormValue("type"))
	a.finishManagementAction(w, r, "/hotspot/hosts", "host_binding", r.FormValue("id"), err)
}

func (a *App) HandleIPBindings(w http.ResponseWriter, r *http.Request) {
	data := bindingsPageData{Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		var err error
		if data.Bindings, err = a.RouterManagement.ListIPBindings(sessionRouterID(r)); err != nil {
			data.Error = managementError(err)
		}
	}
	a.render(w, r, "hotspot_ip_bindings.html", TemplateData{Title: "IP Bindings", ActiveMenu: "ip-bindings", Data: data})
}

func ipBindingFromRequest(r *http.Request) core.IPBinding {
	return core.IPBinding{ID: r.FormValue("id"), MACAddress: r.FormValue("mac_address"), Address: r.FormValue("address"), ToAddress: r.FormValue("to_address"), Server: r.FormValue("server"), Type: r.FormValue("type"), Comment: r.FormValue("comment"), Disabled: r.FormValue("disabled") != ""}
}

func (a *App) HandleIPBindingCreate(w http.ResponseWriter, r *http.Request) {
	a.mutateBinding(w, r, "ip_binding_create", func() error { return a.RouterManagement.AddIPBinding(sessionRouterID(r), ipBindingFromRequest(r)) })
}

func (a *App) HandleIPBindingUpdate(w http.ResponseWriter, r *http.Request) {
	a.mutateBinding(w, r, "ip_binding_update", func() error { return a.RouterManagement.SetIPBinding(sessionRouterID(r), ipBindingFromRequest(r)) })
}

func (a *App) HandleIPBindingRemove(w http.ResponseWriter, r *http.Request) {
	a.mutateBinding(w, r, "ip_binding_remove", func() error { return a.RouterManagement.RemoveIPBinding(sessionRouterID(r), r.FormValue("id")) })
}

func (a *App) mutateBinding(w http.ResponseWriter, r *http.Request, action string, mutate func() error) {
	var err error
	if a.RouterManagement == nil {
		err = fmt.Errorf("layanan manajemen router tidak tersedia")
	} else {
		err = mutate()
	}
	a.finishManagementAction(w, r, "/hotspot/ip-bindings", action, r.FormValue("id"), err)
}

func (a *App) HandleHotspotCookies(w http.ResponseWriter, r *http.Request) {
	data := cookiesPageData{Search: strings.TrimSpace(r.URL.Query().Get("q")), Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		cookies, err := a.RouterManagement.ListCookies(sessionRouterID(r))
		if err != nil {
			data.Error = managementError(err)
		} else {
			q := strings.ToLower(data.Search)
			for _, cookie := range cookies {
				if q == "" || strings.Contains(strings.ToLower(cookie.User+" "+cookie.MACAddress), q) {
					data.Cookies = append(data.Cookies, cookie)
				}
			}
		}
	}
	a.render(w, r, "hotspot_cookies.html", TemplateData{Title: "Hotspot Cookies", ActiveMenu: "cookies", Data: data})
}

func (a *App) HandleCookieRemove(w http.ResponseWriter, r *http.Request) {
	var err error
	if a.RouterManagement == nil {
		err = fmt.Errorf("layanan manajemen router tidak tersedia")
	} else {
		err = a.RouterManagement.RemoveCookie(sessionRouterID(r), r.FormValue("id"))
	}
	a.finishManagementAction(w, r, "/hotspot/cookies", "cookie_remove", r.FormValue("id"), err)
}

func (a *App) HandleSystemLog(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit != 50 && limit != 100 && limit != 200 {
		if limit > 200 {
			limit = 200
		} else {
			limit = 100
		}
	}
	data := logsPageData{Topic: strings.TrimSpace(r.URL.Query().Get("topic")), Search: strings.TrimSpace(r.URL.Query().Get("q")), Limit: limit, Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		logs, err := a.RouterManagement.ListSystemLogs(sessionRouterID(r), data.Topic, data.Search, limit)
		if err != nil {
			data.Error = managementError(err)
		} else {
			data.Logs = logs
		}
	}
	a.render(w, r, "system_log.html", TemplateData{Title: "System Log", ActiveMenu: "system-log", Data: data})
}

func hotspotServerFromRequest(r *http.Request) core.HotspotServer {
	return core.HotspotServer{ID: r.FormValue("id"), Name: r.FormValue("name"), Interface: r.FormValue("interface"), AddressPool: r.FormValue("address_pool"), Profile: r.FormValue("profile"), IdleTimeout: r.FormValue("idle_timeout"), KeepaliveTimeout: r.FormValue("keepalive_timeout"), Disabled: r.FormValue("disabled") != ""}
}

func (a *App) HandleHotspotServers(w http.ResponseWriter, r *http.Request) {
	data := serversPageData{Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		var err error
		if data.Servers, err = a.RouterManagement.ListHotspotServers(sessionRouterID(r)); err != nil {
			data.Error = managementError(err)
		}
		data.Interfaces, _ = a.RouterManagement.ListInterfaces(sessionRouterID(r))
		data.Pools, _ = a.RouterManagement.ListIPPools(sessionRouterID(r))
		data.Profiles, _ = a.RouterManagement.ListHotspotServerProfiles(sessionRouterID(r))
	}
	a.render(w, r, "hotspot_servers.html", TemplateData{Title: "Hotspot Servers", ActiveMenu: "servers", Data: data})
}

func (a *App) HandleHotspotServerCreate(w http.ResponseWriter, r *http.Request) {
	a.mutateServer(w, r, "hotspot_server_create", func() error {
		return a.RouterManagement.AddHotspotServer(sessionRouterID(r), hotspotServerFromRequest(r))
	})
}
func (a *App) HandleHotspotServerUpdate(w http.ResponseWriter, r *http.Request) {
	a.mutateServer(w, r, "hotspot_server_update", func() error {
		return a.RouterManagement.SetHotspotServer(sessionRouterID(r), hotspotServerFromRequest(r))
	})
}
func (a *App) HandleHotspotServerRemove(w http.ResponseWriter, r *http.Request) {
	a.mutateServer(w, r, "hotspot_server_remove", func() error { return a.RouterManagement.RemoveHotspotServer(sessionRouterID(r), r.FormValue("id")) })
}

func (a *App) mutateServer(w http.ResponseWriter, r *http.Request, action string, mutate func() error) {
	var err error
	if a.RouterManagement == nil {
		err = fmt.Errorf("layanan manajemen router tidak tersedia")
	} else {
		err = mutate()
	}
	a.finishManagementAction(w, r, "/hotspot/servers", action, r.FormValue("id")+r.FormValue("name"), err)
}

func hotspotServerProfileFromRequest(r *http.Request) core.HotspotServerProfile {
	_ = r.ParseForm()
	loginBy := r.Form["login_by"]
	return core.HotspotServerProfile{ID: r.FormValue("id"), Name: r.FormValue("name"), HotspotAddress: r.FormValue("hotspot_address"), DNSName: r.FormValue("dns_name"), HTMLDirectory: r.FormValue("html_directory"), LoginBy: strings.Join(loginBy, ","), CookieLifetime: r.FormValue("cookie_lifetime"), RateLimit: r.FormValue("rate_limit")}
}

func (a *App) HandleHotspotServerProfiles(w http.ResponseWriter, r *http.Request) {
	data := serverProfilesPageData{Available: a.RouterManagement != nil}
	if a.RouterManagement != nil {
		var err error
		if data.Profiles, err = a.RouterManagement.ListHotspotServerProfiles(sessionRouterID(r)); err != nil {
			data.Error = managementError(err)
		}
	}
	a.render(w, r, "hotspot_server_profiles.html", TemplateData{Title: "Server Profiles", ActiveMenu: "server-profiles", Data: data})
}

func (a *App) HandleHotspotServerProfileCreate(w http.ResponseWriter, r *http.Request) {
	a.mutateServerProfile(w, r, "hotspot_server_profile_create", func() error {
		return a.RouterManagement.AddHotspotServerProfile(sessionRouterID(r), hotspotServerProfileFromRequest(r))
	})
}
func (a *App) HandleHotspotServerProfileUpdate(w http.ResponseWriter, r *http.Request) {
	a.mutateServerProfile(w, r, "hotspot_server_profile_update", func() error {
		return a.RouterManagement.SetHotspotServerProfile(sessionRouterID(r), hotspotServerProfileFromRequest(r))
	})
}
func (a *App) HandleHotspotServerProfileRemove(w http.ResponseWriter, r *http.Request) {
	a.mutateServerProfile(w, r, "hotspot_server_profile_remove", func() error {
		return a.RouterManagement.RemoveHotspotServerProfile(sessionRouterID(r), r.FormValue("id"))
	})
}

func (a *App) mutateServerProfile(w http.ResponseWriter, r *http.Request, action string, mutate func() error) {
	var err error
	if a.RouterManagement == nil {
		err = fmt.Errorf("layanan manajemen router tidak tersedia")
	} else {
		err = mutate()
	}
	a.finishManagementAction(w, r, "/hotspot/server-profiles", action, r.FormValue("id")+r.FormValue("name"), err)
}

func managementError(err error) string {
	if err == nil {
		return ""
	}
	switch core.StatusOf(err) {
	case http.StatusBadRequest:
		return "Data tidak valid."
	case http.StatusServiceUnavailable:
		return "Router belum terhubung."
	default:
		return "Gagal memuat data router."
	}
}

func (a *App) finishManagementAction(w http.ResponseWriter, r *http.Request, redirect, action, target string, err error) {
	if err != nil {
		a.setFlash(w, r, "Error: "+managementError(err))
	} else {
		a.audit(r, action, strings.TrimSpace(target))
		a.setFlash(w, r, "Success: Perubahan berhasil disimpan.")
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
