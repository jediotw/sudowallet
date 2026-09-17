package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/microservices/api-gateway/config"
	"github.com/saurabhkr78/sudowallet/microservices/api-gateway/internal/proxy"
	corsmw "github.com/saurabhkr78/sudowallet/microservices/api-gateway/internal/proxy/middleware"
	"github.com/saurabhkr78/sudowallet/microservices/shared/middleware"
)

func main() {
	log.Println("Starting API Gateway on port 8080...")

	// Load configuration
	cfg := config.LoadConfig()

	// Create reverse proxy for each target microservice
	authProxy, err := proxy.NewReverseProxy(cfg.AuthServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize auth proxy: %v", err)
	}

	userProxy, err := proxy.NewReverseProxy(cfg.UserServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize user proxy: %v", err)
	}

	walletProxy, err := proxy.NewReverseProxy(cfg.WalletServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize wallet proxy: %v", err)
	}

	transactionProxy, err := proxy.NewReverseProxy(cfg.TransactionServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize transaction proxy: %v", err)
	}

	paymentProxy, err := proxy.NewReverseProxy(cfg.PaymentServiceURL)
	if err != nil {
		log.Fatalf("Failed to initialize payment proxy: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(corsmw.CORSMiddleware())

	// Pure reverse-proxy routing: the gateway holds no auth/business logic.
	// Each upstream microservice is responsible for protecting its own routes.
	//
	// /api/v1/auth/*          -> Auth Service (8081)
	r.Any("/api/v1/auth/*path", func(c *gin.Context) {
		authProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/users/*         -> User Service (8084)
	r.Any("/api/v1/users/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/wallets/*       -> Wallet Service (8082)
	r.Any("/api/v1/wallets/*path", func(c *gin.Context) {
		walletProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/transactions/*  -> Transaction Service (8086)
	r.Any("/api/v1/transactions/*path", func(c *gin.Context) {
		transactionProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /api/v1/ledger/*        -> Payment Service (8083)
	r.Any("/api/v1/ledger/*path", func(c *gin.Context) {
		paymentProxy.ServeHTTP(c.Writer, c.Request)
	})

	// /uploads/* (avatars)    -> User Service (8084)
	r.Any("/uploads/*path", func(c *gin.Context) {
		userProxy.ServeHTTP(c.Writer, c.Request)
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	log.Printf("API Gateway listening on port %s...", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Gateway failed: %v", err)
	}
}
