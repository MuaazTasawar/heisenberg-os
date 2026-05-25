package dea

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/dea/threat", getThreat)
}

func getThreat(c *gin.Context) {
	userID := c.GetUint("user_id")
	empID := empire.GetEmpireID(userID)
	threat, err := UpdateThreat(empID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Stay one step ahead of Hank", threat)
}
