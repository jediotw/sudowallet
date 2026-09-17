package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/microservices/shared/config"
	"github.com/saurabhkr78/sudowallet/microservices/shared/database"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/shared/middleware"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/client"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/handler"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/repository"
	"github.com/saurabhkr78/sudowallet/microservices/transaction-service/internal/transaction/service"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("starting transaction-service..")

	cfg, err := config.Load()
	if err != nil {
		logger.Log.Error("failed to load configuration", "error", err)
		return
	}

	db, err := database.Connect(cfg.DB)
	if err != nil {
		logger.Log.Error("failed to connect database", "error", err)
		return
	}
	defer db.Close()

	rdb, err := database.ConnectRedis(cfg.Redis.Address)
	if err != nil {
		logger.Log.Error("failed to connect redis", "error", err)
		return
	}
	defer rdb.Close()

	// --------------------------------
	// Dependency Injection
	// --------------------------------

	txRepo := repository.NewMySQLTransactionRepository(db)
	ledgerRepo := repository.NewMySQLLedgerRepository(db)

	userClient := client.NewUserClient(cfg.Microservices.UserServiceURL)
	walletClient := client.NewWalletClient(cfg.Microservices.WalletServiceURL)

	txSvc := service.NewTransactionService(txRepo, ledgerRepo, userClient, walletClient, db)
	txHandler := handler.NewTransactionHandler(txSvc)
	internalHandler := handler.NewInternalHandler(txRepo, ledgerRepo)

	// --------------------------------
	// Gin
	// --------------------------------

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 requests per minute

	api := r.Group("/api/v1")
	{
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))

		protected.POST("/transactions/transfer", txHandler.Transfer)
		protected.GET("/transactions/history", txHandler.GetHistory)
	}

	// Internal service-to-service API consumed by payment-service (not exposed
	// via the API gateway).
	internal := r.Group("/internal")
	{
		internal.GET("/transactions", internalHandler.GetTransactionsByDate)
		internal.GET("/ledger/wallets/:walletID/balance", internalHandler.GetLedgerBalance)
		internal.GET("/ledger/wallets/:walletID/entries", internalHandler.GetLedgerEntries)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "transaction-service",
		})
	})

	// --------------------------------
	// HTTP Server
	// --------------------------------

	server := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: r,
	}

	go func() {
		logger.Log.Info("transaction-service listening on port " + cfg.HTTP.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error("server forced to shutdown", "error", err)
	}

	logger.Log.Info("transaction-service stopped")
}
