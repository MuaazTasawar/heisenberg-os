package legal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/legal/consult", consultHandler)
}

func consultHandler(c *gin.Context) {
	var req struct {
		Charge        string `json:"charge" binding:"required"`
		Evidence      string `json:"evidence"`
		PlayerContext string `json:"player_context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	advice, err := ConsultSaul(req.Charge, req.Evidence, req.PlayerContext)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Saul is unavailable right now")
		return
	}
	response.OK(c, http.StatusOK, "Better call Saul", advice)
}
