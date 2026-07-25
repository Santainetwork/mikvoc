package service

import (
	"testing"
	"time"

	"mikvoc/internal/routeros"
)

func TestUserCache_HitWithinTTL(t *testing.T) {
	c := newUserCache()
	users := []routeros.HotspotUser{{ID: "*1", Name: "alice"}}
	c.set(1, users)

	got, ok := c.get(1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].Name != "alice" {
		t.Fatalf("got=%v", got)
	}
	got[0].Name = "mutated"
	again, ok := c.get(1)
	if !ok || again[0].Name != "alice" {
		t.Fatal("cache must return copy")
	}
}

func TestUserCache_MissAfterTTL(t *testing.T) {
	c := newUserCache()
	c.set(2, []routeros.HotspotUser{{ID: "*2", Name: "bob"}})
	c.mu.Lock()
	e := c.data[2]
	e.at = time.Now().Add(-userCacheTTL - time.Millisecond)
	c.data[2] = e
	c.mu.Unlock()

	if _, ok := c.get(2); ok {
		t.Fatal("expected cache miss after TTL")
	}
}

func TestUserCache_Invalidate(t *testing.T) {
	c := newUserCache()
	c.set(3, []routeros.HotspotUser{{ID: "*3", Name: "carol"}})
	c.invalidate(3)
	if _, ok := c.get(3); ok {
		t.Fatal("expected miss after invalidate")
	}
}

func TestPool_InvalidateUsers(t *testing.T) {
	p := NewPool()
	p.users.set(7, []routeros.HotspotUser{{ID: "*7", Name: "x"}})
	p.InvalidateUsers(7)
	if _, ok := p.users.get(7); ok {
		t.Fatal("expected invalidated")
	}
}

func TestUserCache_LookupByNameAndID(t *testing.T) {
	c := newUserCache()
	c.set(1, []routeros.HotspotUser{
		{ID: "*1", Name: "alice"},
		{ID: "*2", Name: "bob"},
	})
	u, ok := c.getByName(1, "bob")
	if !ok || u.ID != "*2" {
		t.Fatalf("byName: ok=%v u=%+v", ok, u)
	}
	u, ok = c.getByID(1, "*1")
	if !ok || u.Name != "alice" {
		t.Fatalf("byID: ok=%v u=%+v", ok, u)
	}
	if _, ok := c.getByName(1, "missing"); ok {
		t.Fatal("expected miss")
	}
	p := NewPool()
	p.users.set(9, []routeros.HotspotUser{{ID: "*9", Name: "z"}})
	if u, ok := p.GetUserByName(9, "z"); !ok || u.ID != "*9" {
		t.Fatalf("pool byName: ok=%v u=%+v", ok, u)
	}
	if u, ok := p.GetUserByID(9, "*9"); !ok || u.Name != "z" {
		t.Fatalf("pool byID: ok=%v u=%+v", ok, u)
	}
}
