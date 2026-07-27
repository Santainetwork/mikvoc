package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mikvoc/internal/database"
)

var processStart = time.Now()

type healthResponse struct {
	Status           string `json:"status"`
	DB               string `json:"db"`
	RoutersConnected int    `json:"routers_connected"`
	RoutersTotal     int    `json:"routers_total"`
	UptimeSec        int64  `json:"uptime_sec"`
}

func (a *App) routerCounts() (connected, total int) {
	if a.Routers != nil {
		if routers, err := a.Routers.List(); err == nil {
			total = len(routers)
		}
	}
	if a.Pool != nil {
		connected = a.Pool.ConnectedCount()
	}
	return connected, total
}

func (a *App) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	status := "ok"
	code := http.StatusOK
	if database.DB == nil {
		dbStatus = "error"
		status = "degraded"
		code = http.StatusServiceUnavailable
	} else if err := database.DB.Ping(); err != nil {
		dbStatus = "error"
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	connected, total := a.routerCounts()
	resp := healthResponse{
		Status:           status,
		DB:               dbStatus,
		RoutersConnected: connected,
		RoutersTotal:     total,
		UptimeSec:        int64(time.Since(processStart).Seconds()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *App) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	connected, total := a.routerCounts()
	uptime := int64(time.Since(processStart).Seconds())
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP mikvoc_up 1 if process is up\n")
	fmt.Fprintf(w, "# TYPE mikvoc_up gauge\n")
	fmt.Fprintf(w, "mikvoc_up 1\n")
	fmt.Fprintf(w, "# HELP mikvoc_routers_connected Number of connected routers\n")
	fmt.Fprintf(w, "# TYPE mikvoc_routers_connected gauge\n")
	fmt.Fprintf(w, "mikvoc_routers_connected %d\n", connected)
	fmt.Fprintf(w, "# HELP mikvoc_routers_total Total configured routers\n")
	fmt.Fprintf(w, "# TYPE mikvoc_routers_total gauge\n")
	fmt.Fprintf(w, "mikvoc_routers_total %d\n", total)
	fmt.Fprintf(w, "# HELP mikvoc_uptime_seconds Process uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE mikvoc_uptime_seconds counter\n")
	fmt.Fprintf(w, "mikvoc_uptime_seconds %d\n", uptime)
}
