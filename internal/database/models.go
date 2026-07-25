package database

import (
	"database/sql"
	"fmt"
	"sort"

	"mikvoc/internal/crypt"
)

// Router represents a saved router configuration.
type Router struct {
	ID              int
	Name            string
	IP              string
	Port            string
	Username        string
	Password        string
	SortOrder       int
	VoucherTemplate string
}

// GetRouters returns all routers ordered by sort_order.
// Routers whose password fails to decrypt are skipped (not fatal) to avoid
// hiding all routers when one has a corrupt/legacy password.
func GetRouters() ([]Router, error) {
	rows, err := DB.Query(`SELECT id, name, ip, port, username, password, sort_order, voucher_template FROM routers ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	type rawRouter struct {
		Router
		encrypted string
		sortOrder sql.NullInt64
	}
	var raws []rawRouter
	for rows.Next() {
		var rr rawRouter
		if err := rows.Scan(&rr.ID, &rr.Name, &rr.IP, &rr.Port, &rr.Username, &rr.encrypted, &rr.sortOrder, &rr.VoucherTemplate); err != nil {
			rows.Close()
			return nil, err
		}
		if rr.sortOrder.Valid {
			rr.SortOrder = int(rr.sortOrder.Int64)
		}
		raws = append(raws, rr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rs := make([]Router, 0, len(raws))
	for _, rr := range raws {
		r := rr.Router
		r.Password = rr.encrypted
		if err := decryptRouterPassword(&r); err != nil {
			// Can't decrypt (e.g. secret changed). Keep router visible with empty
			// password so admin can re-enter it via Settings. Log for visibility.
			fmt.Printf("[warn] GetRouters: router %d (%s) password undecryptable: %v — showing with empty password\n", r.ID, r.IP, err)
			r.Password = ""
		}
		rs = append(rs, r)
	}
	return rs, nil
}

// GetRouter returns a single router by ID.
// If the password can't be decrypted, the router is still returned with an empty password.
func GetRouter(id int) (*Router, error) {
	r := &Router{}
	var sortOrder sql.NullInt64
	err := DB.QueryRow(`SELECT id, name, ip, port, username, password, sort_order, voucher_template FROM routers WHERE id=?`, id).
		Scan(&r.ID, &r.Name, &r.IP, &r.Port, &r.Username, &r.Password, &sortOrder, &r.VoucherTemplate)
	if sortOrder.Valid {
		r.SortOrder = int(sortOrder.Int64)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decryptRouterPassword(r); err != nil {
		fmt.Printf("[warn] GetRouter(%d): password undecryptable: %v — returning empty password\n", r.ID, err)
		r.Password = ""
	}
	return r, nil
}

// SaveRouter inserts or updates a router.
func SaveRouter(r *Router) error {
	password := r.Password
	if routerCipher != nil && password != "" {
		var err error
		password, err = routerCipher.Encrypt(password)
		if err != nil {
			return fmt.Errorf("encrypt router password: %w", err)
		}
	}
	voucherTmpl := r.VoucherTemplate
	if voucherTmpl == "" {
		voucherTmpl = "classic"
	}
	if r.ID == 0 {
		res, err := DB.Exec(`INSERT INTO routers (name,ip,port,username,password,sort_order,voucher_template) VALUES (?,?,?,?,?,?,?)`,
			r.Name, r.IP, r.Port, r.Username, password, r.SortOrder, voucherTmpl)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		r.ID = int(id)
		return nil
	}
	_, err := DB.Exec(`UPDATE routers SET name=?,ip=?,port=?,username=?,password=?,sort_order=?,voucher_template=? WHERE id=?`,
		r.Name, r.IP, r.Port, r.Username, password, r.SortOrder, voucherTmpl, r.ID)
	return err
}

func decryptRouterPassword(r *Router) error {
	if routerCipher == nil || r.Password == "" {
		return nil
	}
	wasEncrypted := crypt.IsEncrypted(r.Password)
	plaintext, err := routerCipher.Decrypt(r.Password)
	if err != nil {
		return err
	}
	r.Password = plaintext
	if !wasEncrypted {
		encrypted, err := routerCipher.Encrypt(plaintext)
		if err != nil {
			return err
		}
		tx, err := DB.Begin()
		if err != nil {
			return fmt.Errorf("migrate router password (begin): %w", err)
		}
		if _, err := tx.Exec(`UPDATE routers SET password=? WHERE id=?`, encrypted, r.ID); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate router password: %w", err)
		}
		return tx.Commit()
	}
	return nil
}

// DeleteRouter removes a router by ID.
func DeleteRouter(id int) error {
	_, err := DB.Exec(`DELETE FROM routers WHERE id=?`, id)
	return err
}

// --- Settings ---

func GetSetting(key string) string {
	var val string
	_ = DB.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&val)
	return val
}

func SetSetting(key, value string) error {
	_, err := DB.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func SetTemplateSettings(routerID int, settings map[string]string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := settings[key]
		if routerID > 0 {
			var global string
			if scanErr := tx.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&global); scanErr != nil && scanErr != sql.ErrNoRows {
				_ = tx.Rollback()
				return scanErr
			}
			if value == global && !templateSettingKeepsRouterOverride(key) {
				_, err = tx.Exec(`DELETE FROM router_settings WHERE router_id=? AND key=?`, routerID, key)
			} else {
				_, err = tx.Exec(`INSERT INTO router_settings(router_id,key,value) VALUES(?,?,?) ON CONFLICT(router_id,key) DO UPDATE SET value=excluded.value`, routerID, key, value)
			}
		} else {
			_, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func templateSettingKeepsRouterOverride(key string) bool {
	return key == "tpl_custom_assets_zip" || key == "tpl_custom_assets_manifest"
}

func GetAllSettings() map[string]string {
	rows, _ := DB.Query(`SELECT key, value FROM settings`)
	m := map[string]string{}
	if rows == nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		_ = rows.Scan(&k, &v)
		m[k] = v
	}
	return m
}

// --- Per-router settings (template hotspot, etc) ---

// GetRouterSetting returns the per-router setting value, or empty string if not set.
func GetRouterSetting(routerID int, key string) string {
	if routerID == 0 {
		return ""
	}
	var val string
	_ = DB.QueryRow(`SELECT value FROM router_settings WHERE router_id=? AND key=?`, routerID, key).Scan(&val)
	return val
}

// SetRouterSetting upserts a per-router setting.
func SetRouterSetting(routerID int, key, value string) error {
	if routerID == 0 {
		return fmt.Errorf("router_id required for router setting")
	}
	_, err := DB.Exec(`INSERT INTO router_settings(router_id,key,value) VALUES(?,?,?) ON CONFLICT(router_id,key) DO UPDATE SET value=excluded.value`, routerID, key, value)
	return err
}

// GetRouterSettings returns all per-router settings merged with global fallback.
// Router-specific values override global ones. Includes voucher_template from routers table.
func GetRouterSettings(routerID int) map[string]string {
	global := GetAllSettings()
	if routerID == 0 {
		return global
	}
	rows, err := DB.Query(`SELECT key, value FROM router_settings WHERE router_id=?`, routerID)
	if err != nil {
		return global
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		_ = rows.Scan(&k, &v)
		global[k] = v
	}
	if vt := GetRouterVoucherTemplate(routerID); vt != "" {
		global["voucher_template"] = vt
	}
	return global
}

// GetRouterVoucherTemplate returns the voucher print template for a router, defaulting to 'classic'.
func GetRouterVoucherTemplate(routerID int) string {
	if routerID == 0 {
		return GetSetting("voucher_template")
	}
	var val string
	err := DB.QueryRow(`SELECT voucher_template FROM routers WHERE id=?`, routerID).Scan(&val)
	if err != nil || val == "" {
		return "classic"
	}
	return val
}

// SetRouterVoucherTemplate updates the voucher print template on the routers row.
func SetRouterVoucherTemplate(routerID int, tmplID string) error {
	if routerID == 0 {
		return SetSetting("voucher_template", tmplID)
	}
	_, err := DB.Exec(`UPDATE routers SET voucher_template=? WHERE id=?`, tmplID, routerID)
	return err
}

// --- Admin ---

// GetAdmin returns the stored admin credentials (password_hash column).
func GetAdmin() (username, passwordHash string) {
	_ = DB.QueryRow(`SELECT username, password FROM admins LIMIT 1`).Scan(&username, &passwordHash)
	return
}

// SetAdmin updates admin credentials. passwordHash must already be hashed (bcrypt) or empty to keep current.
func SetAdmin(username, passwordHash string) error {
	if passwordHash == "" {
		_, err := DB.Exec(`UPDATE admins SET username=? WHERE id=1`, username)
		return err
	}
	_, err := DB.Exec(`UPDATE admins SET username=?, password=? WHERE id=1`, username, passwordHash)
	return err
}

// SetAdminPassword sets a new admin password hash.
func SetAdminPassword(username, passwordHash string) error {
	return SetAdmin(username, passwordHash)
}

// --- Sales ---

type SaleRecord struct {
	ID        int
	RouterID  int
	Username  string
	Profile   string
	Comment   string
	Price     int
	CreatedAt string
}

func AddSale(routerID int, username, profile, comment string, price int) error {
	_, err := DB.Exec(`INSERT INTO sales (router_id,username,profile,comment,price) VALUES (?,?,?,?,?)`,
		routerID, username, profile, comment, price)
	return err
}

func AddSaleWithTime(routerID int, username, profile, comment string, price int, timestamp string) error {
	_, err := DB.Exec(`INSERT INTO sales (router_id,username,profile,comment,price,created_at) VALUES (?,?,?,?,?,?)`,
		routerID, username, profile, comment, price, timestamp)
	return err
}

func AddSaleWithTimeIdempotent(routerID int, username, profile, comment string, price int, timestamp, sourceKey string) (bool, error) {
	if sourceKey == "" {
		if err := AddSaleWithTime(routerID, username, profile, comment, price, timestamp); err != nil {
			return false, err
		}
		return true, nil
	}
	res, err := DB.Exec(
		`INSERT OR IGNORE INTO sales (router_id,username,profile,comment,price,created_at,source_key) VALUES (?,?,?,?,?,?,?)`,
		routerID, username, profile, comment, price, timestamp, sourceKey,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetSales returns sales filtered by date range (YYYY-MM-DD). Pass empty strings for no filter.
func GetSales(routerID int, from, to string) ([]SaleRecord, error) {
	query := `SELECT id, router_id, username, profile, comment, price, created_at FROM sales WHERE 1=1`
	args := []interface{}{}
	if routerID > 0 {
		query += ` AND router_id=?`
		args = append(args, routerID)
	}
	if from != "" {
		query += ` AND date(created_at) >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND date(created_at) <= ?`
		args = append(args, to)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []SaleRecord
	for rows.Next() {
		var s SaleRecord
		_ = rows.Scan(&s.ID, &s.RouterID, &s.Username, &s.Profile, &s.Comment, &s.Price, &s.CreatedAt)
		records = append(records, s)
	}
	return records, rows.Err()
}

// GetSalesTotalByDay returns daily aggregated sales for a router.
func GetSalesTotalByDay(routerID int, from, to string) ([]map[string]interface{}, error) {
	query := `SELECT date(created_at) as day, COUNT(*) as count, SUM(price) as total FROM sales WHERE router_id=?`
	args := []interface{}{routerID}
	if from != "" {
		query += ` AND date(created_at) >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND date(created_at) <= ?`
		args = append(args, to)
	}
	query += ` GROUP BY day ORDER BY day DESC`

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var day string
		var count, total int
		_ = rows.Scan(&day, &count, &total)
		result = append(result, map[string]interface{}{
			"day": day, "count": count, "total": total,
		})
	}
	return result, rows.Err()
}
