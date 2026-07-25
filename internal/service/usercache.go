package service

import (
	"sync"
	"time"

	"mikvoc/internal/routeros"
)

const userCacheTTL = 5 * time.Second

type userCacheEntry struct {
	users  []routeros.HotspotUser
	byName map[string]int
	byID   map[string]int
	at     time.Time
}

type userCache struct {
	mu   sync.RWMutex
	data map[int]userCacheEntry
}

func newUserCache() *userCache {
	return &userCache{data: make(map[int]userCacheEntry)}
}

func copyHotspotUsers(in []routeros.HotspotUser) []routeros.HotspotUser {
	if in == nil {
		return nil
	}
	out := make([]routeros.HotspotUser, len(in))
	copy(out, in)
	return out
}

func buildUserIndexes(users []routeros.HotspotUser) (byName, byID map[string]int) {
	byName = make(map[string]int, len(users))
	byID = make(map[string]int, len(users))
	for i, u := range users {
		if u.Name != "" {
			byName[u.Name] = i
		}
		if u.ID != "" {
			byID[u.ID] = i
		}
	}
	return byName, byID
}

func (c *userCache) get(routerID int) ([]routeros.HotspotUser, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.data[routerID]
	if !ok || time.Since(e.at) >= userCacheTTL {
		return nil, false
	}
	return copyHotspotUsers(e.users), true
}

func (c *userCache) getByName(routerID int, name string) (routeros.HotspotUser, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.data[routerID]
	if !ok || time.Since(e.at) >= userCacheTTL {
		return routeros.HotspotUser{}, false
	}
	idx, ok := e.byName[name]
	if !ok || idx < 0 || idx >= len(e.users) {
		return routeros.HotspotUser{}, false
	}
	return e.users[idx], true
}

func (c *userCache) getByID(routerID int, id string) (routeros.HotspotUser, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.data[routerID]
	if !ok || time.Since(e.at) >= userCacheTTL {
		return routeros.HotspotUser{}, false
	}
	idx, ok := e.byID[id]
	if !ok || idx < 0 || idx >= len(e.users) {
		return routeros.HotspotUser{}, false
	}
	return e.users[idx], true
}

func (c *userCache) set(routerID int, users []routeros.HotspotUser) {
	c.mu.Lock()
	defer c.mu.Unlock()
	copied := copyHotspotUsers(users)
	byName, byID := buildUserIndexes(copied)
	c.data[routerID] = userCacheEntry{
		users:  copied,
		byName: byName,
		byID:   byID,
		at:     time.Now(),
	}
}

func (c *userCache) invalidate(routerID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, routerID)
}

func (c *userCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[int]userCacheEntry)
}
