package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/client"
	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/handler"
	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/service"
	"github.com/saurabhkr78/sudowallet/microservices/shared/config"
	"github.com/saurabhkr78/sudowallet/microservices/shared/database"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/shared/middleware"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("starting payment-service..")

	cfg, err := config.Load()
	if err != nil {
		logger.Log.Error("failed to load configuration", "error", err)
		return
	}

	rdb, err := database.ConnectRedis(cfg.Redis.Address)
	if err != nil {
		logger.Log.Error("failed to connect redis", "error", err)
		return
	}
	defer rdb.Close()

	// --------------------------------
	// Dependency Injection
	// --------------------------------

	walletClient := client.NewWalletClient(cfg.Microservices.WalletServiceURL)
	txClient := client.NewTransactionClient(cfg.Microservices.TransactionServiceURL)

	ledgerSvc := service.NewLedgerService(walletClient, txClient)
	ledgerHandler := handler.NewLedgerHandler(ledgerSvc)

	scheduler := service.NewScheduler(walletClient, txClient, "./reports")

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

		protected.GET("/ledger/mutations", ledgerHandler.GetMutations)
		protected.GET("/ledger/reconcile", ledgerHandler.Reconcile)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "payment-service",
		})
	})

	// --------------------------------
	// Background Scheduler
	// --------------------------------

	scheduler.Start()

	// --------------------------------
	// HTTP Server
	// --------------------------------

	server := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: r,
	}

	go func() {
		logger.Log.Info("payment-service listening on port " + cfg.HTTP.Port)
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

	logger.Log.Info("payment-service stopped")
}
