package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"mikvoc/internal/routeros"
)

type routerHealthSummary struct {
	Label  string
	Tone   string
	Icon   string
	Detail string
}

type dashboardResponse struct {
	Connected    bool   `json:"connected"`
	Active       int    `json:"active"`
	Users        int    `json:"users"`
	CPU          string `json:"cpu,omitempty"`
	Uptime       string `json:"uptime,omitempty"`
	Board        string `json:"board,omitempty"`
	Version      string `json:"version,omitempty"`
	FreeMemory   string `json:"free_mem,omitempty"`
	MemoryPct    int    `json:"mem_pct"`
	HealthLabel  string `json:"health_label,omitempty"`
	HealthTone   string `json:"health_tone,omitempty"`
	HealthIcon   string `json:"health_icon,omitempty"`
	HealthDetail string `json:"health_detail,omitempty"`
}

func dashboardHealthSummary(connected bool, cpuLoad, memUsedPct int) routerHealthSummary {
	if !connected {
		return routerHealthSummary{Label: "Tidak terhubung", Tone: "amber", Icon: "wifi_off", Detail: "Router belum aktif pada sesi ini"}
	}
	if cpuLoad >= 85 || memUsedPct >= 90 {
		return routerHealthSummary{Label: "Kritis", Tone: "red", Icon: "emergency_home", Detail: "CPU atau memory mendekati batas tinggi"}
	}
	if cpuLoad >= 70 || memUsedPct >= 80 {
		return routerHealthSummary{Label: "Perlu dicek", Tone: "amber", Icon: "warning", Detail: "Beban router mulai tinggi"}
	}
	return routerHealthSummary{Label: "Sehat", Tone: "emerald", Icon: "verified", Detail: "Beban router normal"}
}

func (a *App) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if a.Stats == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	type DashData struct {
		Resource    interface{}
		ActiveCount int
		UserCount   int
		MemUsedPct  int
		Health      routerHealthSummary
		Servers     []map[string]string
		Interfaces  []map[string]string
	}

	d := DashData{
		Resource: &routeros.SystemResource{
			CPULoad:     "0",
			FreeMemory:  "0",
			TotalMemory: "0",
			BoardName:   "-",
			Version:     "-",
			Uptime:      "0s",
		},
		ActiveCount: 0,
		UserCount:   0,
		MemUsedPct:  0,
		Health:      dashboardHealthSummary(false, 0, 0),
		Servers:     []map[string]string{},
		Interfaces:  []map[string]string{},
	}

	connected := false
	routerID := sessionRouterID(r)

	sum, _ := a.Stats.Summary(routerID)
	connected = sum.Connected
	if connected {
		if sum.Resource != nil {
			d.Resource = sum.Resource
			total := mustParseI64(sum.Resource.TotalMemory)
			free := mustParseI64(sum.Resource.FreeMemory)
			if total > 0 {
				d.MemUsedPct = int((total - free) * 100 / total)
			}
		}
		d.ActiveCount = sum.ActiveCount
		d.UserCount = sum.UserCount
		if sum.Servers != nil {
			d.Servers = sum.Servers
		}
		if sum.Interfaces != nil {
			d.Interfaces = sum.Interfaces
		}
	}
	d.Health = dashboardHealthSummary(connected, mustParseInt(d.Resource.(*routeros.SystemResource).CPULoad), d.MemUsedPct)

	a.render(w, r, "dashboard.html", TemplateData{
		Title:      "Dashboard — MikVoc",
		ActiveMenu: "dashboard",
		Data:       d,
	})
}

func (a *App) HandleTrafficAPI(w http.ResponseWriter, r *http.Request) {
	if a.Stats == nil {
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	iface := r.URL.Query().Get("interface")
	if iface == "" {
		iface = "ether1"
	}
	routerID := sessionRouterID(r)

	tr, err := a.Stats.Traffic(routerID, iface)
	if err != nil {
		http.Error(w, `{"error":"not connected"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{"rx": tr.RX, "tx": tr.TX})
}

func (a *App) HandleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if a.Stats == nil {
		http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	routerID := sessionRouterID(r)

	var (
		connected   bool
		activeCount int
		userCount   int
		cpu         string
		uptime      string
		board       string
		version     string
		freeMem     string
		totalMem    string
	)

	sum, _ := a.Stats.Summary(routerID)
	connected = sum.Connected
	if connected {
		activeCount = sum.ActiveCount
		userCount = sum.UserCount
		if sum.Resource != nil {
			cpu = sum.Resource.CPULoad
			uptime = sum.Resource.Uptime
			board = sum.Resource.BoardName
			version = sum.Resource.Version
			freeMem = sum.Resource.FreeMemory
			totalMem = sum.Resource.TotalMemory
		}
	}

	if !connected {
		_ = json.NewEncoder(w).Encode(dashboardResponse{Connected: false})
		return
	}

	memUsedPct := 0
	if totalMem != "" && freeMem != "" {
		total, _ := strconv.ParseInt(totalMem, 10, 64)
		free, _ := strconv.ParseInt(freeMem, 10, 64)
		if total > 0 {
			memUsedPct = int((total - free) * 100 / total)
		}
	}

	freeMemFmt := routeros.FormatBytes(mustParseI64(freeMem))
	health := dashboardHealthSummary(true, mustParseInt(cpu), memUsedPct)

	_ = json.NewEncoder(w).Encode(dashboardResponse{
		Connected: true, Active: activeCount, Users: userCount, CPU: cpu, Uptime: uptime,
		Board: board, Version: version, FreeMemory: freeMemFmt, MemoryPct: memUsedPct,
		HealthLabel: health.Label, HealthTone: health.Tone, HealthIcon: health.Icon, HealthDetail: health.Detail,
	})
}

func mustParseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func mustParseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
