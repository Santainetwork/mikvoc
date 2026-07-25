package service

import (
	"strconv"

	"mikvoc/internal/routeros"
)

type StatsService struct {
	pool *Pool
}

func NewStats(pool *Pool) *StatsService {
	return &StatsService{pool: pool}
}

type Summary struct {
	Connected   bool
	Resource    *routeros.SystemResource
	UserCount   int
	ActiveCount int
	Servers     []map[string]string
	Interfaces  []map[string]string
}

type Traffic struct {
	RX int64
	TX int64
}

func (s *StatsService) Summary(routerID int) (Summary, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return Summary{Connected: false}, nil
	}
	sum := Summary{
		Connected:  true,
		Servers:    []map[string]string{},
		Interfaces: []map[string]string{},
	}
	res, _ := cl.GetSystemResource()
	if res != nil {
		sum.Resource = res
	}
	sum.ActiveCount = cl.CountActiveUsers()
	sum.UserCount = cl.CountUsers()
	servers, _ := cl.GetServers()
	if servers != nil {
		sum.Servers = servers
	}
	interfaces, _ := cl.GetInterfaces()
	if interfaces != nil {
		sum.Interfaces = interfaces
	}
	return sum, nil
}

func (s *StatsService) Traffic(routerID int, iface string) (Traffic, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return Traffic{}, err
	}
	if iface == "" {
		iface = "ether1"
	}
	rows, err := cl.Run("/interface/monitor-traffic", map[string]string{
		"interface": iface,
		"once":      "",
	})
	if err != nil || len(rows) == 0 {
		return Traffic{}, nil
	}
	rx, _ := strconv.ParseInt(rows[0]["rx-bits-per-second"], 10, 64)
	tx, _ := strconv.ParseInt(rows[0]["tx-bits-per-second"], 10, 64)
	return Traffic{RX: rx, TX: tx}, nil
}
