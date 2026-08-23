package main

import (
	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
<<<<<<< Updated upstream
=======
	ledgerHandler "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/handler"
	ledgerRepository "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	ledgerService "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/service"
>>>>>>> Stashed changes
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/middleware"
	userHandler "github.com/saurabhkr78/sudowallet/monolith/internal/user/handler"
	userRepository "github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	userService "github.com/saurabhkr78/sudowallet/monolith/internal/user/service"
	walletHandler "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/handler"
	walletRepository "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
	walletService "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/service"
)

func main() {

	logger.InitLogger()
	logger.Log.Info("starting sudowallet..")

	cfg, err := config.Load()
	if err != nil {
		logger.Log.Error("failed to load configuration", err)
	}

	db, err := database.Connect(cfg.DB)
	if err != nil {
		logger.Log.Error("failed to connect database", err)
	}
	defer db.Close()

	logger.Log.Info("Database connected.")

	logger.Log.Info("HTTP server listening on", "port", cfg.HTTP.Port)

	//intialize layers

	//repository layer
	uRepo := userRepository.NewMySQLUserRepository(db)
	wRepo := walletRepository.NewMySqlWalletRepository(db)
	//service layer
	uSvc := userService.NewUserService(db, uRepo, wRepo)
	wSvc := walletService.NewWalletService(wRepo)
<<<<<<< Updated upstream
=======
	//no handler for ledger service as it is not exposed to the user directly, it is used internally by the transaction service and wallet service but creating handler for ledger service is not a bad idea as it can be used for testing and debugging purpose
	lSvc := ledgerService.NewLedgerService(lRepo, wRepo)
	txSvc := transactionService.NewTransactionService(txRepo, wRepo, lRepo, uRepo, db)
>>>>>>> Stashed changes

	//handler layer
	uHandler := userHandler.NewUserHandler(uSvc)
	wHandler := walletHandler.NewWalletHandler(wSvc)
<<<<<<< Updated upstream
=======
	lHandler := ledgerHandler.NewLedgerHandler(lSvc)
	txHandler := transactionHandler.NewTransactionHandler(txSvc)
>>>>>>> Stashed changes

	//setup gin router
	// gin.Default() already includes Logger and Recovery middleware.
	r := gin.Default()

	api := r.Group("/api/v1")
	{
		//public routes
		api.POST("/users/register", uHandler.Register)
		api.POST("/users/login", uHandler.Login)
		//protected routes
		//apply auth middleware to all routes below
		protected := r.Group("/api/v1")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.GET("/users/me", uHandler.GetProfileMe)
			protected.GET("/users/:id", uHandler.GetProfile)
			protected.PUT("/users/:id", uHandler.UpdateProfile)
			protected.GET("/wallets/me", wHandler.GetWalletByUserID)
		}
	}

<<<<<<< Updated upstream
=======
	// Public
	api.POST("/users/register", uHandler.Register)
	api.POST("/users/login", uHandler.Login)

	// Protected
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())

	protected.GET("/users/me", uHandler.GetProfileMe)
	protected.GET("/users/:id", uHandler.GetProfile)
	protected.PUT("/users/:id", uHandler.UpdateProfile)
	protected.GET("/wallets/me", wHandler.GetWalletByUserID)
	protected.POST("/transactions/transfer", txHandler.Transfer)
	protected.GET("/transactions/history", txHandler.GetHistory)
	protected.GET("/ledger/mutations", lHandler.GetMutations)
	protected.GET("/ledger/reconcile", lHandler.Reconcile)

>>>>>>> Stashed changes
	//start server
	logger.Log.Info("server running on 8080....")
	if err := r.Run(":" + cfg.HTTP.Port); err != nil {
		logger.Log.Error("server failed to run", "error", err)
	}

}
