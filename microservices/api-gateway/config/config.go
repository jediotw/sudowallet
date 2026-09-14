package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	AuthServiceURL        string
	UserServiceURL        string
	WalletServiceURL      string
	TransactionServiceURL string
	PaymentServiceURL     string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:                  getEnv("PORT", "8080"),
		AuthServiceURL:        getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
		UserServiceURL:        getEnv("USER_SERVICE_URL", "http://localhost:8084"),
		WalletServiceURL:      getEnv("WALLET_SERVICE_URL", "http://localhost:8082"),
		TransactionServiceURL: getEnv("TRANSACTION_SERVICE_URL", "http://localhost:8086"),
		PaymentServiceURL:     getEnv("PAYMENT_SERVICE_URL", "http://localhost:8083"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
