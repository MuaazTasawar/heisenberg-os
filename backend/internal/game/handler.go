package game

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/game/state", stateHandler)
}

func stateHandler(c *gin.Context) {
	userID := c.GetUint("user_id")
	state, err := GetGameState(userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Game state loaded", state)
}
