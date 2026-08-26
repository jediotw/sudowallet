package main

import (
	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
	ledgerHandler "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/handler"
	ledgerRepository "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	ledgerService "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/service"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/middleware"
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
)

// @title            wallet API
// @version         1.0
// @description     This is a server for my Go application.
// @host      localhost:8080
// @BasePath  /api/v1
func main() {

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

	//Redis connection
	//get the redis connection string from the config folder

	logger.Log.Info("Connecting to Redis at %s", "@ address", cfg.Redis.Address)
	rdb, err := database.ConnectRedis(cfg.Redis.Address)
	if err != nil {
		logger.Log.Error("failed to connect redis", "error", err)
		return
	}
	defer rdb.Close()
	logger.Log.Info("Successfully connected to Redis at %s", "address", cfg.Redis.Address)

	//http server
	logger.Log.Info("HTTP server listening on", "port", cfg.HTTP.Port)

	//intialize layers

	//repository layer : each service layer need db connection and repository layer to perform the db operations so we need to inject the db connection and repository layer into the service layer
	uRepo := userRepository.NewMySQLUserRepository(db)
	wRepo := walletRepository.NewMySQLWalletRepository(db)
	lRepo := ledgerRepository.NewMySQLLedgerRepository(db)
	txRepo := transactionRepository.NewMySQLTransactionRepository(db)
	//service layer
	uSvc := userService.NewUserService(db, uRepo, wRepo, rdb)
	wSvc := walletService.NewWalletService(wRepo, rdb)
	//no handler for ledger service as it is not exposed to the user directly, it is used internally by the transaction service and wallet service but creating handler for ledger service is not a bad idea as it can be used for testing and debugging purpose
	lSvc := ledgerService.NewLedgerService(lRepo, wRepo)
	txSvc := transactionService.NewTransactionService(txRepo, wRepo, lRepo, uRepo, db, rdb)

	//handler layer
	uHandler := userHandler.NewUserHandler(uSvc)
	wHandler := walletHandler.NewWalletHandler(wSvc)
	lHandler := ledgerHandler.NewLedgerHandler(lSvc)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)

	//setup gin router
	// gin.Default() already includes Logger and Recovery middleware.
	r := gin.Default()

	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RateLimit(rdb, 60, 1)) // 60 requests per minute
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	api := r.Group("/api/v1")

	// Public
	api.POST("/users/register", uHandler.Register)
	api.POST("/users/login", uHandler.Login)
	// Protected
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(rdb))

	protected.GET("/users/me", uHandler.GetProfileMe)
	protected.POST("/users/avatar", uHandler.UpdateAvatar)
	protected.GET("/users/:id", uHandler.GetProfile)
	protected.PUT("/users/:id", uHandler.UpdateProfile)
	protected.GET("/wallets/me", wHandler.GetWalletByUserID)
	protected.POST("/transactions/transfer", txHandler.Transfer)
	protected.GET("/transactions/history", txHandler.GetHistory)
	protected.GET("/ledger/mutations", lHandler.GetMutations)
	protected.GET("/ledger/reconcile", lHandler.Reconcile)
	protected.DELETE("/users/me", uHandler.DeleteAccount)
	protected.POST("/users/logout", uHandler.Logout)

	//start server
	logger.Log.Info("server running on 8080....")
	if err := r.Run(":" + cfg.HTTP.Port); err != nil {
		logger.Log.Error("server failed to run", "error", err)
	}

}
