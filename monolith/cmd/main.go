package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/handler"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/repository"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/service"
)

func main() {
	log.Println("Starting Sudowallet...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected.")

	log.Printf("HTTP server listening on :%s", cfg.HTTP.Port)

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
	log.Printf("server running on 8080....")
	if err := r.Run(":" + cfg.HTTP.Port); err != nil {
		log.Fatalf("server failed to run: %v", err)
	}

}
