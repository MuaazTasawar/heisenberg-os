// Package server wires configuration, middleware and every feature package into one gin
// engine. Keeping this out of main makes the whole API testable.
package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/dea"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/game"
	"github.com/heisenberg-os/backend/internal/laundering"
	"github.com/heisenberg-os/backend/internal/legal"
	"github.com/heisenberg-os/backend/internal/middleware"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/pkg/response"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// New builds the HTTP handler for the whole API.
func New(cfg *config.Config) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	config.Set(cfg)
	mlclient.SetDefault(mlclient.New(cfg))

	r := gin.New()
	r.HandleMethodNotAllowed = true
	proxies := cfg.TrustedProxies
	if proxies == nil {
		proxies = []string{} // trust no proxy: client IPs come straight from the connection
	}
	_ = r.SetTrustedProxies(proxies)

	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/healthz"}}))
	r.Use(middleware.SecurityHeaders())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/healthz", healthHandler)
	r.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, "route not found") })
	r.NoMethod(func(c *gin.Context) { response.Fail(c, http.StatusMethodNotAllowed, "method not allowed") })

	api := r.Group("/api/v1")
	api.Use(
		middleware.MaxBody(maxBodyBytes),
		middleware.RequireJSON(),
		middleware.CheckOrigin(cfg.CORSOrigins),
		middleware.RateLimit(cfg.RateAPIPerMin, time.Minute),
	)

	authHandler := auth.NewHandler(cfg)
	authHandler.Public(api, middleware.RateLimit(cfg.RateAuthPerMin, time.Minute))

	protected := api.Group("", middleware.JWTAuth(cfg))
	authHandler.Protected(protected)
	game.RegisterRoutes(protected)
	empire.RegisterRoutes(protected)
	dea.RegisterRoutes(protected)
	laundering.RegisterRoutes(protected)
	legal.RegisterRoutes(protected.Group("", middleware.RateLimit(cfg.RateLegalPerMin, time.Minute)))

	return r
}

// healthHandler reports on the database (required) and the ML service (optional).
func healthHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	database := "down"
	if db.DB != nil {
		if raw, err := db.DB.DB(); err == nil && raw.PingContext(ctx) == nil {
			database = "up"
		}
	}

	ml := "disabled"
	if client := mlclient.Default(); client.Enabled() {
		if client.Health(ctx) == nil {
			ml = "up"
		} else {
			ml = "down"
		}
	}

	status, code := "ok", http.StatusOK
	switch {
	case database != "up":
		status, code = "down", http.StatusServiceUnavailable
	case ml == "down":
		status = "degraded"
	}
	c.JSON(code, gin.H{"status": status, "database": database, "ml": ml})
}
