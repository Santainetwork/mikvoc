package repository

import (
	"mikvoc/internal/core"
	"mikvoc/internal/database"
)

func (s *Store) AddAudit(adminID int, adminName, action, target string) error {
	_, err := database.DB.Exec(
		`INSERT INTO audit_log (admin_id, admin_name, action, target) VALUES (?,?,?,?)`,
		adminID, adminName, action, target,
	)
	return err
}

func (s *Store) ListAudit(limit int) ([]core.AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := database.DB.Query(
		`SELECT id, admin_id, admin_name, action, target, created_at
		 FROM audit_log ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []core.AuditEntry
	for rows.Next() {
		var e core.AuditEntry
		if err := rows.Scan(&e.ID, &e.AdminID, &e.AdminName, &e.Action, &e.Target, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}
