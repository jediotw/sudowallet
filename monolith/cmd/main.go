package main

import (
	"log"
	"net/http"

	"github.com/saurabhkr78/sudowallet/monolith/internal/config"
	"github.com/saurabhkr78/sudowallet/monolith/internal/database"
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

	if err := http.ListenAndServe(":"+cfg.HTTP.Port, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
