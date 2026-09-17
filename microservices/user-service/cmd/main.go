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
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/client"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/email"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/handler"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/repository"
	"github.com/saurabhkr78/sudowallet/microservices/user-service/internal/user/service"
)

func main() {
	logger.InitLogger()
	logger.Log.Info("starting user-service..")

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

	userRepo := repository.NewMySQLUserRepository(db)
	otpStore := repository.NewOTPStore(rdb)

	emailSender := email.NewSMTPEmailSender(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From)

	walletClient := client.NewWalletClient(cfg.Microservices.WalletServiceURL)

	userSvc := service.NewUserService(
		userRepo,
		otpStore,
		emailSender,
		walletClient,
		[]byte(cfg.OTP.Secret),
		"./uploads",
	)
	userHandler := handler.NewUserHandler(userSvc)

	// --------------------------------
	// Gin
	// --------------------------------

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 requests per minute

	r.Static("/uploads", "./uploads")

	api := r.Group("/api/v1")
	{
		// Public
		api.POST("/users/register", userHandler.Register)
		api.POST("/users/forgot-password", userHandler.RequestPasswordReset)
		api.POST("/users/verify-password-reset", userHandler.VerifyPasswordReset)
		api.POST("/users/reset-password", userHandler.ResetPassword)

		// Authenticated
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(rdb))

		protected.GET("/users/me", userHandler.GetProfile)
		protected.POST("/users/verify-email", userHandler.VerifyEmail)
		protected.POST("/users/avatar", userHandler.UpdateAvatar)
		protected.GET("/users/:id", userHandler.GetProfileByID)
		protected.PUT("/users/:id", userHandler.UpdateProfile)
		protected.DELETE("/users/me", userHandler.SoftDelete)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "user-service",
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
		logger.Log.Info("user-service listening on port " + cfg.HTTP.Port)
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

	logger.Log.Info("user-service stopped")
}
