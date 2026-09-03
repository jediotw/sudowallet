package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
	"github.com/saurabhkr78/sudowallet/monolith/internal/email"
	ledgerHandler "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/handler"
	ledgerRepository "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	ledgerService "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/service"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/middleware"
	otpRepository "github.com/saurabhkr78/sudowallet/monolith/internal/otp/repository"
	transactionHandler "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/handler"
	transactionRepository "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/repository"
	transactionService "github.com/saurabhkr78/sudowallet/monolith/internal/transaction/service"
	userHandler "github.com/saurabhkr78/sudowallet/monolith/internal/user/handler"
	userRepository "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	userService "github.com/saurabhkr78/sudowallet/monolith/internal/user/service"
	walletHandler "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/handler"
	walletRepository "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	walletService "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/service"

	//swagger
	_ "github.com/saurabhkr78/sudowallet/monolith/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	//scheduler/cron job
	"github.com/saurabhkr78/sudowallet/monolith/internal/scheduler"
)

// @title            wallet API
// @version         1.0
// @description     This is a server for my Go application.
// @host      localhost:8080
// @BasePath  /api/v1
func main() {
	//initalize logger first so that we can use it in the rest of the application
	logger.InitLogger()
	logger.Log.Info("starting sudowallet..")

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

	logger.Log.Info("Database connected.")

	rdb, err := database.ConnectRedis(cfg.Redis.Address)
	if err != nil {
		logger.Log.Error("failed to connect redis", "error", err)
		return
	}
	defer rdb.Close()

	//initalize the email sender
	emailSender := email.NewSMTPEmailSender(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From)
	logger.Log.Info("SMTP Email sender initialized with host: %s, port: %s, from: %s", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port, "from", cfg.SMTP.From)
	logger.Log.Info("Successfully connected to SMTP server at %s", "address", cfg.SMTP.Host)

	//http server
	logger.Log.Info("HTTP server listening on", "port", cfg.HTTP.Port)
	logger.Log.Info("Successfully connected to Redis")

	// --------------------------------
	// Dependency Injection
	// --------------------------------

	uRepo := userRepository.NewMySQLUserRepository(db)
	wRepo := walletRepository.NewMySQLWalletRepository(db)
	lRepo := ledgerRepository.NewMySQLLedgerRepository(db)
	txRepo := transactionRepository.NewMySQLTransactionRepository(db)
	otpRepo := otpRepository.NewMySQLOTPRepository(db)
	//service layer
	uSvc := userService.NewUserService(db, uRepo, wRepo, rdb, emailSender, otpRepo)

	uSvc := userService.NewUserService(db, uRepo, wRepo, rdb)
	wSvc := walletService.NewWalletService(wRepo, rdb)

	lSvc := ledgerService.NewLedgerService(lRepo, wRepo)

	txSvc := transactionService.NewTransactionService(
		txRepo,
		wRepo,
		lRepo,
		uRepo,
		db,
		rdb,
	)

	uHandler := userHandler.NewUserHandler(uSvc)
	wHandler := walletHandler.NewWalletHandler(wSvc)
	lHandler := ledgerHandler.NewLedgerHandler(lSvc)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)

	// --------------------------------
	// Scheduler
	// --------------------------------

	scheduler := scheduler.NewScheduler(db, wRepo, lRepo)
	scheduler.Start()
	defer scheduler.Stop()

	// --------------------------------
	// Gin
	// --------------------------------

	r := gin.Default()

	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, time.Minute)) // 60 requests per minute
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Use(middleware.RateLimit(rdb, 60, 1))

	r.GET(
		"/swagger/*any",
		ginSwagger.WrapHandler(swaggerFiles.Handler),
	)

	api := r.Group("/api/v1")

	api.POST("/users/register", uHandler.Register)
	api.POST("/users/login", uHandler.Login)

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(rdb))

	protected.GET("/users/me", uHandler.GetProfileMe)
	protected.POST("/users/verify-email", uHandler.VerifyEmail)
	protected.POST("/users/avatar", uHandler.UpdateAvatar)
	protected.GET("/users/:id", uHandler.GetProfile)
	protected.PUT("/users/:id", uHandler.UpdateProfile)

	protected.GET("/wallets/me", wHandler.GetWalletByUserID)

	protected.POST(
		"/transactions/transfer",
		txHandler.Transfer,
	)

	protected.GET(
		"/transactions/history",
		txHandler.GetHistory,
	)

	protected.GET(
		"/ledger/mutations",
		lHandler.GetMutations,
	)

	protected.GET(
		"/ledger/reconcile",
		lHandler.Reconcile,
	)

	protected.DELETE(
		"/users/me",
		uHandler.DeleteAccount,
	)

	protected.POST(
		"/users/logout",
		uHandler.Logout,
	)

	// --------------------------------
	// HTTP Server
	// --------------------------------

	server := &http.Server{
		Addr:    ":" + cfg.HTTP.Port,
		Handler: r,
	}

	go func() {
		logger.Log.Info(
			"server running on " + cfg.HTTP.Port,
		)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			logger.Log.Error(
				"server failed",
				"error",
				err,
			)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(
		quit,

		os.Interrupt,
		syscall.SIGTERM,
	)

	<-quit

	logger.Log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error(
			"server forced to shutdown",
			"error",
			err,
		)
	}
	//stop the scheduler before exiting the application
	scheduler.Stop()

	logger.Log.Info("server stopped")
}
