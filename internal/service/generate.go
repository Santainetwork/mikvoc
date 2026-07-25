package service

import (
	"mikvoc/internal/core"
	"mikvoc/internal/repository"
)

type GenerateService struct {
	pool  *Pool
	sales repository.SaleRepo
}

func NewGenerate(pool *Pool, sales repository.SaleRepo) *GenerateService {
	return &GenerateService{pool: pool, sales: sales}
}

func (s *GenerateService) Generate(routerID int, spec core.VoucherSpec) (*core.VoucherBatch, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	generated, comment, err := cl.GenerateUsers(toROSGenerateOpts(spec))
	if err != nil {
		return nil, err
	}
	s.pool.InvalidateUsers(routerID)
	out := make([]core.VoucherResult, len(generated))
	for i, pair := range generated {
		out[i] = core.VoucherResult{Username: pair[0], Password: pair[1]}
	}
	return &core.VoucherBatch{Comment: comment, Items: out}, nil
}
