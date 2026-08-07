package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type HTTPConfig struct {
	Port string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string

	MaxRetries int
	RetryDelay time.Duration

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DSN returns a MySQL connection string.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true",
		d.User,
		d.Password,
		d.Host,
		d.Port,
		d.Name,
	)
}

type JWTConfig struct {
	Secret string
}

type Config struct {
	HTTP HTTPConfig
	DB   DBConfig
	JWT  JWTConfig
}

func Load() (*Config, error) {
	// Load .env in development.
	// In production this usually does nothing because
	// environment variables are provided by Docker/Kubernetes.
	// _ = loadDotEnv()
	_ = godotenv.Load()

	maxRetries, err := strconv.Atoi(os.Getenv("DB_MAX_RETRIES"))
	if err != nil {
		maxRetries = 5
	}

	retryDelay, err := time.ParseDuration(os.Getenv("DB_RETRY_DELAY"))
	if err != nil {
		retryDelay = 2 * time.Second
	}

	maxOpenConns, err := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
	if err != nil {
		maxOpenConns = 25
	}

	maxIdleConns, err := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
	if err != nil {
		maxIdleConns = 25
	}

	connMaxLifetime, err := time.ParseDuration(os.Getenv("DB_CONN_MAX_LIFETIME"))
	if err != nil {
		connMaxLifetime = 5 * time.Minute
	}

	connMaxIdleTime, err := time.ParseDuration(os.Getenv("DB_CONN_MAX_IDLE_TIME"))
	if err != nil {
		connMaxIdleTime = 2 * time.Minute
	}

	cfg := &Config{
		HTTP: HTTPConfig{
			Port: os.Getenv("PORT"),
		},
		DB: DBConfig{
			Host:            os.Getenv("DB_HOST"),
			Port:            os.Getenv("DB_PORT"),
			User:            os.Getenv("DB_USER"),
			Password:        os.Getenv("DB_PASSWORD"),
			Name:            os.Getenv("DB_NAME"),
			MaxRetries:      maxRetries,
			RetryDelay:      retryDelay,
			MaxOpenConns:    maxOpenConns,
			MaxIdleConns:    maxIdleConns,
			ConnMaxLifetime: connMaxLifetime,
			ConnMaxIdleTime: connMaxIdleTime,
		},
		JWT: JWTConfig{
			Secret: os.Getenv("JWT_SECRET"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadDotEnv() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return godotenv.Load(envPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
