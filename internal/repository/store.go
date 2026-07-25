package repository

import (
	"database/sql"

	"mikvoc/internal/database"
)

type Store struct{}

func NewStore() *Store { return &Store{} }

func (s *Store) DB() *sql.DB { return database.DB }
