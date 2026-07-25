package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"mikvoc/internal/crypt"
)

var DB *sql.DB

var routerCipher *crypt.Cipher

// Init opens (or creates) the SQLite database and runs migrations.
// secret digunakan untuk mendekripsi password router at-rest (AES-256-GCM).
func Init(path string, secrets ...string) error {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	DB = db
	secret := ""
	if len(secrets) > 0 {
		secret = secrets[0]
	}
	if secret != "" {
		c, err := crypt.New(secret)
		if err != nil {
			return fmt.Errorf("init crypt: %w", err)
		}
		routerCipher = c
	} else {
		routerCipher = nil
	}
	return runMigrations(db)
}

func RouterCipher() *crypt.Cipher { return routerCipher }
