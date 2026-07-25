package service

import (
	"log"
	"time"

	"mikvoc/internal/repository"
)

type Scheduler struct {
	pool  *Pool
	store *repository.Store
	stop  chan struct{}
}

func NewScheduler(pool *Pool, store *repository.Store) *Scheduler {
	return &Scheduler{
		pool:  pool,
		store: store,
		stop:  make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.loop()
}

func (s *Scheduler) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	if s.pool == nil || s.store == nil || s.store.DB() == nil {
		return
	}
	routers, err := s.store.ListRouters()
	if err != nil {
		log.Printf("[scheduler] list routers: %v", err)
		return
	}
	for _, rt := range routers {
		cl := s.pool.Client(rt.ID)
		if cl == nil || !cl.IsConnected() {
			continue
		}
		n, err := cl.RemoveExpiredUsers(cl.ROSVersion())
		if err != nil {
			log.Printf("[scheduler] remove expired router=%d (%s): %v", rt.ID, rt.Name, err)
			continue
		}
		if n > 0 {
			s.pool.InvalidateUsers(rt.ID)
			log.Printf("[scheduler] removed %d expired users on router=%d (%s)", n, rt.ID, rt.Name)
		}
	}
}
