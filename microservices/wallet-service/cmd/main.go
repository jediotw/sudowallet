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
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/handler"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/repository"
	"github.com/saurabhkr78/sudowallet/microservices/wallet-service/internal/wallet/service"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("starting wallet-service..")

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

	walletRepo := repository.NewMySQLWalletRepository(db)
	cache := repository.NewWalletCache(rdb)

	walletSvc := service.NewWalletService(walletRepo, cache)
	mutationSvc := service.NewMutationService(walletRepo, cache)

	walletHandler := handler.NewWalletHandler(walletSvc)
	internalHandler := handler.NewInternalHandler(mutationSvc, walletSvc)

	// --------------------------------
	// Gin
	// --------------------------------

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 requests per minute

	// External, authenticated API (identity from JWT set by shared AuthMiddleware).
	api := r.Group("/api/v1")
	{
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		protected.GET("/wallets/me", walletHandler.GetWalletByUserID)
	}

	// Internal service-to-service API (not exposed via the API gateway).
	internal := r.Group("/internal")
	{
		internal.POST("/wallets", internalHandler.CreateWallet)
		internal.POST("/wallets/adjust", internalHandler.AdjustBalance)
		internal.GET("/wallets/user/:userID", internalHandler.ResolveByUserID)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "wallet-service",
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
		logger.Log.Info("wallet-service listening on port " + cfg.HTTP.Port)
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

	logger.Log.Info("wallet-service stopped")
}
