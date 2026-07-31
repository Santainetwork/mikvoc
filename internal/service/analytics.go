package service

import (
	"sync"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

// AnalyticsService provides comprehensive metrics and analytics
type AnalyticsService struct {
	pool      *Pool
	cacheMu   sync.RWMutex
	cacheData *DashboardStats
	cacheExp  time.Time
	cacheTTL  time.Duration
}

// DashboardStats contains all dashboard metrics
type DashboardStats struct {
	TotalUsers         int               `json:"total_users"`
	ActiveUsers        int               `json:"active_users"`
	UserGrowth         map[string]int    `json:"user_growth,omitempty"`
	TotalRouters       int               `json:"total_routers"`
	HealthyRouters     int               `json:"healthy_routers"`
	RouterHealth       []RouterHealthStat `json:"router_health,omitempty"`
	OnlineRouters      int               `json:"online_routers"`
	OfflineRouters     int               `json:"offline_routers"`
	StorageUsedMB      float64           `json:"storage_used_mb"`
	AssetsByType       map[string]int    `json:"assets_by_type"`
	AssetsByRouter     map[int]int       `json:"assets_by_router,omitempty"`
	UploadsLast24H     int               `json:"uploads_last_24h"`
	TemplateUpdates    int               `json:"template_updates"`
	AvgTemplateRenderTimeMS int          `json:"avg_template_render_time_ms"`
	PoolConnectedCount int               `json:"pool_connected_count"`
	GeneratedAt        time.Time         `json:"generated_at"`
	CacheExpiresAt     time.Time         `json:"cache_expires_at"`
}

// RouterHealthStat holds health data for a single router
type RouterHealthStat struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	ConnectivityStatus string  `json:"connectivity_status"`
	UptimeSeconds      int64   `json:"uptime_seconds"`
	CPUUtilizationPct  float64 `json:"cpu_utilization_pct"`
	MemoryUsagePct     float64 `json:"memory_usage_pct"`
	LastCheck          time.Time `json:"last_check"`
}

func NewAnalyticsService(pool *Pool) *AnalyticsService {
	return &AnalyticsService{pool: pool, cacheTTL: 1 * time.Minute}
}

func (s *AnalyticsService) DashboardStats() *DashboardStats {
	s.cacheMu.RLock()
	if s.cacheExp.After(time.Now()) && s.cacheData != nil {
		data := s.cacheData
		s.cacheMu.RUnlock()
		return data
	}
	s.cacheMu.RUnlock()

	stats := &DashboardStats{
		GeneratedAt:        time.Now(),
		CacheExpiresAt:     time.Now().Add(s.cacheTTL),
		AssetsByType:       map[string]int{"logos": 0, "backgrounds": 0},
		AssetsByRouter:     make(map[int]int),
		UserGrowth:         map[string]int{"today": 0},
		AvgTemplateRenderTimeMS: 150,
	}

	routers, _ := database.GetRouters()
	stats.TotalRouters = len(routers)
	stats.PoolConnectedCount = s.pool.ConnectedCount()

	for _, router := range routers {
		cl := s.pool.Client(router.ID)
		isOnline := cl != nil && cl.IsConnected()
		
		if isOnline {
			stats.OnlineRouters++
			stats.HealthyRouters++
			
			stats.RouterHealth = append(stats.RouterHealth, RouterHealthStat{
				ID: router.ID, Name: router.Name,
				ConnectivityStatus: "online", LastCheck: time.Now(),
			})
		} else {
			stats.OfflineRouters++
			stats.RouterHealth = append(stats.RouterHealth, RouterHealthStat{
				ID: router.ID, Name: router.Name,
				ConnectivityStatus: "offline", LastCheck: time.Now(),
			})
		}
		
		settings := database.GetRouterSettings(router.ID)
		count := 0
		if settings["tpl_logo_url"] != "" { count++ }
		if settings["tpl_bg_image"] != "" { count++ }
		stats.AssetsByRouter[router.ID] = count
		
		activeCount := 0
		if isOnline { activeCount = cl.CountActiveUsers() }
		stats.ActiveUsers += activeCount
	}

	today := time.Now().Format("2006-01-02")
	salesToday, _ := database.GetSales(0, today, "")
	stats.UploadsLast24H = len(salesToday)
	stats.TemplateUpdates = len(routers)
	
	s.cacheMu.Lock()
	s.cacheData = stats
	s.cacheExp = time.Now().Add(s.cacheTTL)
	s.cacheMu.Unlock()

	return stats
}

func (s *AnalyticsService) SetCacheTTL(d time.Duration) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cacheTTL = d
}

func (s *AnalyticsService) GetRouterHealthDetail(routerID int) (*RouterHealthStat, error) {
	router, err := database.GetRouter(routerID)
	if err != nil || router == nil {
		return nil, core.ErrNotFound
	}
	
	return &RouterHealthStat{
		ID: router.ID, Name: router.Name,
		LastCheck: time.Now(), ConnectivityStatus: "unknown",
	}, nil
}

func (s *AnalyticsService) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cacheData = nil
	s.cacheExp = time.Time{}
}
