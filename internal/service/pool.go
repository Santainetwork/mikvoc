package service

import (
	"log"
	"sync"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

type Pool struct {
	mu      sync.RWMutex
	clients map[int]*routeros.Client
	users   *userCache

	keepAliveStop chan struct{}
	keepAliveOnce sync.Once
}

func NewPool() *Pool {
	return &Pool{
		clients: make(map[int]*routeros.Client),
		users:   newUserCache(),
	}
}

func (p *Pool) Connect(rt *core.Router) error {
	port := rt.Port
	if port == "" {
		port = "8728"
	}

	p.mu.Lock()
	if existing, ok := p.clients[rt.ID]; ok && existing != nil {
		existing.Close()
		delete(p.clients, rt.ID)
	}
	p.mu.Unlock()

	cl := routeros.NewClient(rt.IP, port)
	if err := cl.Connect(rt.Username, rt.Password); err != nil {
		return err
	}

	p.mu.Lock()
	p.clients[rt.ID] = cl
	p.mu.Unlock()
	return nil
}

func (p *Pool) Disconnect(id int) {
	if id == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cl, ok := p.clients[id]; ok && cl != nil {
		cl.Close()
		delete(p.clients, id)
	}
	if p.users != nil {
		p.users.invalidate(id)
	}
}

func (p *Pool) Client(id int) *routeros.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.clients[id]
}

func (p *Pool) IsConnected(id int) bool {
	cl := p.Client(id)
	return cl != nil && cl.IsConnected()
}

func (p *Pool) ConnectAll(routers []core.Router) {
	for i := range routers {
		go func(rt core.Router) {
			if err := p.Connect(&rt); err != nil {
				log.Printf("[warn] Router %s: %v", rt.IP, err)
			} else {
				log.Printf("[info] Connected: %s (%s)", rt.Name, rt.IP)
			}
		}(routers[i])
	}
}

func (p *Pool) RequireClient(id int) (*routeros.Client, error) {
	if id == 0 || !p.IsConnected(id) {
		return nil, core.ErrNotConnected
	}
	cl := p.Client(id)
	if cl == nil || !cl.IsConnected() {
		return nil, core.ErrNotConnected
	}
	return cl, nil
}

func (p *Pool) Put(id int, cl *routeros.Client) {
	if id == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cl == nil {
		delete(p.clients, id)
		if p.users != nil {
			p.users.invalidate(id)
		}
		return
	}
	if existing, ok := p.clients[id]; ok && existing != nil && existing != cl {
		existing.Close()
	}
	p.clients[id] = cl
}

func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, cl := range p.clients {
		if cl != nil {
			cl.Close()
		}
		delete(p.clients, id)
	}
	if p.users != nil {
		p.users.clear()
	}
}

func (p *Pool) GetUsersCached(routerID int, profile string) ([]routeros.HotspotUser, error) {
	if profile == "" && p.users != nil {
		if users, ok := p.users.get(routerID); ok {
			return users, nil
		}
	}
	cl, err := p.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	users, err := cl.GetUsers(profile)
	if err != nil {
		return nil, err
	}
	if profile == "" && p.users != nil {
		p.users.set(routerID, users)
	}
	return users, nil
}

func (p *Pool) GetUserByID(routerID int, id string) (routeros.HotspotUser, bool) {
	if p == nil || p.users == nil || routerID == 0 || id == "" {
		return routeros.HotspotUser{}, false
	}
	return p.users.getByID(routerID, id)
}

func (p *Pool) GetUserByName(routerID int, name string) (routeros.HotspotUser, bool) {
	if p == nil || p.users == nil || routerID == 0 || name == "" {
		return routeros.HotspotUser{}, false
	}
	return p.users.getByName(routerID, name)
}

func (p *Pool) InvalidateUsers(routerID int) {
	if p == nil || p.users == nil || routerID == 0 {
		return
	}
	p.users.invalidate(routerID)
}

func (p *Pool) StartKeepAlive(interval time.Duration) {
	if p == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	p.keepAliveOnce.Do(func() {
		p.keepAliveStop = make(chan struct{})
		go p.keepAliveLoop(interval)
	})
}

func (p *Pool) StopKeepAlive() {
	if p == nil || p.keepAliveStop == nil {
		return
	}
	select {
	case <-p.keepAliveStop:
	default:
		close(p.keepAliveStop)
	}
}

func (p *Pool) keepAliveLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.keepAliveStop:
			return
		case <-ticker.C:
			p.pingAll()
		}
	}
}

func (p *Pool) pingAll() {
	p.mu.RLock()
	ids := make([]int, 0, len(p.clients))
	for id := range p.clients {
		ids = append(ids, id)
	}
	p.mu.RUnlock()

	for _, id := range ids {
		p.mu.RLock()
		cl := p.clients[id]
		p.mu.RUnlock()
		if cl == nil || !cl.IsConnected() {
			p.mu.Lock()
			if cur, ok := p.clients[id]; ok && cur == cl {
				delete(p.clients, id)
			}
			p.mu.Unlock()
			if p.users != nil {
				p.users.invalidate(id)
			}
			continue
		}
		if _, err := cl.Run("/system/identity/print"); err != nil {
			log.Printf("[keepalive] router %d ping failed: %v", id, err)
			p.mu.Lock()
			if cur, ok := p.clients[id]; ok && cur == cl {
				cl.Close()
				delete(p.clients, id)
			}
			p.mu.Unlock()
			if p.users != nil {
				p.users.invalidate(id)
			}
		}
	}
}
