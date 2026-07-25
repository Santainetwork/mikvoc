package repository

import (
	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

func (s *Store) ListRouters() ([]core.Router, error) {
	rs, err := database.GetRouters()
	if err != nil {
		return nil, err
	}
	out := make([]core.Router, len(rs))
	for i := range rs {
		out[i] = toCoreRouter(rs[i])
	}
	return out, nil
}

func (s *Store) GetRouter(id int) (*core.Router, error) {
	r, err := database.GetRouter(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, core.ErrNotFound
	}
	cr := toCoreRouter(*r)
	return &cr, nil
}

func (s *Store) SaveRouter(r *core.Router) error {
	dr := fromCoreRouter(r)
	if err := database.SaveRouter(dr); err != nil {
		return err
	}
	r.ID = dr.ID
	return nil
}

func (s *Store) DeleteRouter(id int) error {
	return database.DeleteRouter(id)
}

func toCoreRouter(r database.Router) core.Router {
	return core.Router{
		ID:              r.ID,
		Name:            r.Name,
		IP:              r.IP,
		Port:            r.Port,
		Username:        r.Username,
		Password:        r.Password,
		SortOrder:       r.SortOrder,
		VoucherTemplate: r.VoucherTemplate,
	}
}

func fromCoreRouter(r *core.Router) *database.Router {
	return &database.Router{
		ID:              r.ID,
		Name:            r.Name,
		IP:              r.IP,
		Port:            r.Port,
		Username:        r.Username,
		Password:        r.Password,
		SortOrder:       r.SortOrder,
		VoucherTemplate: r.VoucherTemplate,
	}
}
