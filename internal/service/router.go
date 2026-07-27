package service

import (
	"mikvoc/internal/core"
	"mikvoc/internal/repository"
	"mikvoc/internal/routeros"
)

type RouterService struct {
	repos repository.RouterRepo
	pool  *Pool
}

func NewRouter(repos repository.RouterRepo, pool *Pool) *RouterService {
	return &RouterService{repos: repos, pool: pool}
}

func (s *RouterService) List() ([]core.Router, error) {
	return s.repos.ListRouters()
}

func (s *RouterService) Get(id int) (*core.Router, error) {
	return s.repos.GetRouter(id)
}

func (s *RouterService) Save(r *core.Router) error {
	return s.repos.SaveRouter(r)
}

func (s *RouterService) Delete(id int) error {
	s.pool.Disconnect(id)
	return s.repos.DeleteRouter(id)
}

func (s *RouterService) Connect(id int) error {
	rt, err := s.repos.GetRouter(id)
	if err != nil {
		return err
	}
	return s.pool.Connect(rt)
}

func (s *RouterService) Disconnect(id int) {
	s.pool.Disconnect(id)
}

func (s *RouterService) ConnectAll() error {
	routers, err := s.repos.ListRouters()
	if err != nil {
		return err
	}
	s.pool.ConnectAll(routers)
	return nil
}

func (s *RouterService) Status(id int) core.RouterStatus {
	st := core.RouterStatus{ID: id, Connected: s.pool.IsConnected(id)}
	if rt, err := s.repos.GetRouter(id); err == nil && rt != nil {
		st.Name = rt.Name
	}
	return st
}

func (s *RouterService) Client(id int) (*routeros.Client, error) {
	return s.pool.RequireClient(id)
}
