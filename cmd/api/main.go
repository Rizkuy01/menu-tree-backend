package main

import (
	"context"
	"log"

	_ "menu-tree-backend/docs"
	"menu-tree-backend/internal/config"
	"menu-tree-backend/internal/database"
	"menu-tree-backend/internal/handler"
	"menu-tree-backend/internal/repository"
	"menu-tree-backend/internal/router"
	"menu-tree-backend/internal/service"
)

// @title           Menu Tree API
// @version         1.0
// @description     REST API for managing a hierarchical menu tree with unlimited depth.
// @host            localhost:8080
// @BasePath        /api

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	pool, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	// repository -> service -> handler.
	menuRepo := repository.NewMenuRepository(pool)
	menuSvc := service.NewMenuService(menuRepo)
	menuHandler := handler.NewMenuHandler(menuSvc)

	r := router.New(menuHandler)

	log.Printf("server running on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
