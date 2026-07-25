package repository

import (
	"database/sql"
	"errors"

	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

func (s *Store) GetAdmin() (username, passwordHash string) {
	return database.GetAdmin()
}

func (s *Store) SetAdmin(username, passwordHash string) error {
	return database.SetAdmin(username, passwordHash)
}

func (s *Store) ListAdmins() ([]core.Admin, error) {
	rows, err := database.DB.Query(`SELECT id, username, password, COALESCE(role,'owner') FROM admins ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []core.Admin
	for rows.Next() {
		var a core.Admin
		var role string
		if err := rows.Scan(&a.ID, &a.Username, &a.PasswordHash, &role); err != nil {
			return nil, err
		}
		a.Role = core.AuditRole(role)
		if a.Role == "" {
			a.Role = core.RoleOwner
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *Store) GetAdminByUsername(username string) (*core.Admin, error) {
	var a core.Admin
	var role string
	err := database.DB.QueryRow(
		`SELECT id, username, password, COALESCE(role,'owner') FROM admins WHERE username=?`,
		username,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Role = core.AuditRole(role)
	if a.Role == "" {
		a.Role = core.RoleOwner
	}
	return &a, nil
}

func (s *Store) GetAdminByID(id int) (*core.Admin, error) {
	var a core.Admin
	var role string
	err := database.DB.QueryRow(
		`SELECT id, username, password, COALESCE(role,'owner') FROM admins WHERE id=?`,
		id,
	).Scan(&a.ID, &a.Username, &a.PasswordHash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Role = core.AuditRole(role)
	if a.Role == "" {
		a.Role = core.RoleOwner
	}
	return &a, nil
}

func (s *Store) CreateAdmin(username, passwordHash string, role core.AuditRole) error {
	if role == "" {
		role = core.RoleViewer
	}
	_, err := database.DB.Exec(
		`INSERT INTO admins (username, password, role) VALUES (?,?,?)`,
		username, passwordHash, string(role),
	)
	return err
}

func (s *Store) UpdateAdmin(id int, username, passwordHash string, role core.AuditRole) error {
	if passwordHash == "" {
		_, err := database.DB.Exec(
			`UPDATE admins SET username=?, role=? WHERE id=?`,
			username, string(role), id,
		)
		return err
	}
	_, err := database.DB.Exec(
		`UPDATE admins SET username=?, password=?, role=? WHERE id=?`,
		username, passwordHash, string(role), id,
	)
	return err
}

func (s *Store) DeleteAdmin(id int) error {
	_, err := database.DB.Exec(`DELETE FROM admins WHERE id=?`, id)
	return err
}

func (s *Store) CountOwners() (int, error) {
	var n int
	err := database.DB.QueryRow(
		`SELECT COUNT(*) FROM admins WHERE COALESCE(role,'owner')='owner'`,
	).Scan(&n)
	return n, err
}
