package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/retry"
)

func Connect(cfg config.DBConfig) (*sql.DB, error) {

	dsn := cfg.DSN()

	var db *sql.DB

	err := retry.Retry(cfg.MaxRetries, cfg.RetryDelay, func() error {

		var err error

		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return err
		}

		if err := db.Ping(); err != nil {
			db.Close()
			return err
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	ConfigurePool(db, cfg)

	logger.Log.Info("Database connected successfully.")

	return db, nil
}
