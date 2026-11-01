package game

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/pkg/response"
)

// RegisterRoutes adds the routes that need a logged-in player.
func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/game/state", stateHandler)
}

// RegisterPublicRoutes adds the routes anyone may call.
func RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/stats", statsHandler)
}

func stateHandler(c *gin.Context) {
	state, err := GetState(c.Request.Context(), c.GetUint(auth.UserIDKey))
	if err != nil {
		log.Printf("game: state: %v", err)
		response.Fail(c, http.StatusInternalServerError, "the game state is unavailable")
		return
	}
	response.OK(c, http.StatusOK, "Game state loaded", state)
}

func statsHandler(c *gin.Context) {
	stats, err := GetStats()
	if err != nil {
		log.Printf("game: stats: %v", err)
		response.Fail(c, http.StatusInternalServerError, "statistics are unavailable")
		return
	}
	// Everyone gets the same numbers, so shared caches may keep them for as long as we do.
	c.Header("Cache-Control", "public, max-age=30")
	response.OK(c, http.StatusOK, "The empire in numbers", stats)
}
