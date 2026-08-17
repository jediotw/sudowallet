package main

import (
	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/handler"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/service"
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

	logger.Log.Info("HTTP server listening on", cfg.HTTP.Port)

	//intialize layers
	uRepo := repository.NewMySQLUserRepository(db)
	uSvc := service.NewUserService(uRepo)
	uHandler := handler.NewUserHandler(uSvc)

	//setup gin router|
	r := gin.Default()
	//routes
	r.POST("/api/v1/users", uHandler.Register)
	r.GET("/api/v1/users/:id", uHandler.GetProfile)
	r.PUT("/api/v1/users/:id", uHandler.UpdateProfile)

	//start server
	logger.Log.Info("server running on 8080....")
	if err := r.Run(":" + cfg.HTTP.Port); err != nil {
		logger.Log.Error("server failed to run", err)
	}

}
