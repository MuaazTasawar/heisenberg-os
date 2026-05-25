package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/dea"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/game"
	"github.com/heisenberg-os/backend/internal/laundering"
	"github.com/heisenberg-os/backend/internal/legal"
	"github.com/heisenberg-os/backend/internal/middleware"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	db.Init()

	r := gin.Default()
	r.SetTrustedProxies([]string{"127.0.0.1"})
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")

	auth.RegisterRoutes(api)

	protected := api.Group("/")
	protected.Use(middleware.JWTAuth())

	game.RegisterRoutes(protected)
	empire.RegisterRoutes(protected)
	dea.RegisterRoutes(protected)
	laundering.RegisterRoutes(protected)
	legal.RegisterRoutes(protected)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(r.Run(":" + port))
}
