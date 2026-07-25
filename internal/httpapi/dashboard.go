package httpapi

import (
	"fmt"
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
	type DashData struct {
		Resource    interface{}
		ActiveCount int
		UserCount   int
		MemUsedPct  int
		Health      routerHealthSummary
		Servers     []map[string]string
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
	}

	connected := false
	routerID := sessionRouterID(r)

	if a.Stats != nil {
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
		}
	} else {
		cl := a.clientFor(r)
		connected = cl != nil && cl.IsConnected()
		if connected {
			res, _ := cl.GetSystemResource()
			if res != nil {
				d.Resource = res
				total := mustParseI64(res.TotalMemory)
				free := mustParseI64(res.FreeMemory)
				if total > 0 {
					d.MemUsedPct = int((total - free) * 100 / total)
				}
			}
			d.ActiveCount = cl.CountActiveUsers()
			d.UserCount = cl.CountUsers()
			servers, _ := cl.GetServers()
			if servers != nil {
				d.Servers = servers
			}
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
	iface := r.URL.Query().Get("interface")
	if iface == "" {
		iface = "ether1"
	}
	routerID := sessionRouterID(r)

	if a.Stats != nil {
		tr, err := a.Stats.Traffic(routerID, iface)
		if err != nil {
			http.Error(w, `{"error":"not connected"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rx":` + strconv.FormatInt(tr.RX, 10) + `,"tx":` + strconv.FormatInt(tr.TX, 10) + `}`))
		return
	}

	cl := a.clientFor(r)
	if cl == nil || !cl.IsConnected() {
		http.Error(w, `{"error":"not connected"}`, http.StatusServiceUnavailable)
		return
	}
	rows, err := cl.Run("/interface/monitor-traffic", map[string]string{
		"interface": iface,
		"once":      "",
	})
	if err != nil || len(rows) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rx":0,"tx":0}`))
		return
	}
	rx, _ := strconv.ParseInt(rows[0]["rx-bits-per-second"], 10, 64)
	tx, _ := strconv.ParseInt(rows[0]["tx-bits-per-second"], 10, 64)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"rx":` + strconv.FormatInt(rx, 10) + `,"tx":` + strconv.FormatInt(tx, 10) + `}`))
}

func (a *App) HandleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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

	if a.Stats != nil {
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
	} else {
		cl := a.clientFor(r)
		if cl != nil && cl.IsConnected() {
			connected = true
			res, _ := cl.GetSystemResource()
			activeCount = cl.CountActiveUsers()
			userCount = cl.CountUsers()
			if res != nil {
				cpu = res.CPULoad
				uptime = res.Uptime
				board = res.BoardName
				version = res.Version
				freeMem = res.FreeMemory
				totalMem = res.TotalMemory
			}
		}
	}

	if !connected {
		w.Write([]byte(`{"connected":false}`))
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

	w.Write([]byte(fmt.Sprintf(
		`{"connected":true,"active":%d,"users":%d,"cpu":"%s","uptime":"%s","board":"%s","version":"%s","free_mem":"%s","mem_pct":%d,"health_label":"%s","health_tone":"%s","health_icon":"%s","health_detail":"%s"}`,
		activeCount, userCount, cpu, uptime, board, version, freeMemFmt, memUsedPct, health.Label, health.Tone, health.Icon, health.Detail,
	)))
}

func mustParseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func mustParseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
