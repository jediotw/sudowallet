package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/handler"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/repository"
	"github.com/saurabhkr78/sudowallet/microservices/auth-service/internal/auth/service"
	"github.com/saurabhkr78/sudowallet/microservices/shared/config"
	"github.com/saurabhkr78/sudowallet/microservices/shared/database"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
	"github.com/saurabhkr78/sudowallet/microservices/shared/middleware"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("starting auth-service..")

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

	db, err := database.Connect(cfg.DB)
	if err != nil {
		logger.Log.Error("failed to connect database", "error", err)
		return
	}
	defer db.Close()

	// --------------------------------
	// Dependency Injection
	// --------------------------------

	rtRepo := repository.NewMySQLRefreshTokenRepository(db)
	userRepo := repository.NewMySQLUserRepository(db)

	authSvc := service.NewAuthService(rdb, rtRepo, userRepo)
	authHandler := handler.NewAuthHandler(authSvc)

	// --------------------------------
	// Gin
	// --------------------------------

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 requests per minute

	api := r.Group("/api/v1")
	{
		// Public
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/refresh", authHandler.RefreshToken)

		// Authenticated: logout needs the raw token so it can be blacklisted.
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))
		protected.POST("/auth/logout", authHandler.Logout)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "auth-service",
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
		logger.Log.Info("auth-service listening on port " + cfg.HTTP.Port)
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

	logger.Log.Info("auth-service stopped")
}
