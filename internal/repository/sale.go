package repository

import (
	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

func (s *Store) AddSale(routerID int, username, profile, comment string, price int) error {
	return database.AddSale(routerID, username, profile, comment, price)
}

func (s *Store) AddSaleWithTime(routerID int, username, profile, comment string, price int, timestamp string) error {
	return database.AddSaleWithTime(routerID, username, profile, comment, price, timestamp)
}

func (s *Store) AddSaleWithTimeIdempotent(routerID int, username, profile, comment string, price int, timestamp, sourceKey string) (bool, error) {
	return database.AddSaleWithTimeIdempotent(routerID, username, profile, comment, price, timestamp, sourceKey)
}

func (s *Store) GetSales(routerID int, from, to string) ([]core.Sale, error) {
	recs, err := database.GetSales(routerID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]core.Sale, len(recs))
	for i, rec := range recs {
		out[i] = core.Sale{
			ID:        rec.ID,
			RouterID:  rec.RouterID,
			Username:  rec.Username,
			Profile:   rec.Profile,
			Comment:   rec.Comment,
			Price:     rec.Price,
			CreatedAt: rec.CreatedAt,
		}
	}
	return out, nil
}

func (s *Store) GetSalesTotalByDay(routerID int, from, to string) ([]map[string]interface{}, error) {
	return database.GetSalesTotalByDay(routerID, from, to)
}
