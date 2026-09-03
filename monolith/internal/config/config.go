package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
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

// RedisConfig holds the configuration for Redis.
type RedisConfig struct {
	Host    string
	Port    string
	Address string
}

// smtp config
type SMTPConfig struct {
	Host string
	Port string
	From string
}
type OTPConfig struct {
	Secret string
}

type Config struct {
	HTTP  HTTPConfig
	DB    DBConfig
	JWT   JWTConfig
	Redis RedisConfig
	SMTP  SMTPConfig
	OTP   OTPConfig
}

func Load() (*Config, error) {
	// Load .env in development.
	// In production this usually does nothing because
	// environment variables are provided by Docker/Kubernetes.
	// _ = loadDotEnv()
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}
	logger.Log.Info("Environment variables loaded.")
	//laod redis
	redisAddr := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
	logger.Log.Info("Redis address built for env")
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
		//SET REDIS CONFIG
		Redis: RedisConfig{
			Host:    os.Getenv("REDIS_HOST"),
			Port:    os.Getenv("REDIS_PORT"),
			Address: redisAddr,
		},
		SMTP: SMTPConfig{
			Host: os.Getenv("SMTP_HOST"),
			Port: os.Getenv("SMTP_PORT"),
			From: os.Getenv("SMTP_FROM"),
		},
		OTP: OTPConfig{
			Secret: os.Getenv("OTP_SECRET"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
