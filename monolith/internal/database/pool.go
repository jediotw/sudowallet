package database

import (
	"database/sql"

	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
)

func ConfigurePool(db *sql.DB, cfg config.DBConfig) {

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}
