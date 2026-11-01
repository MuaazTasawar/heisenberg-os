package dea

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/dea")
	g.GET("/threat", getThreat)
	g.GET("/history", getHistory)
	g.GET("/evidence", getEvidence)
}

func fail(c *gin.Context, err error) {
	if errors.Is(err, ErrInvalidLimit) {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	log.Printf("dea: %v", err)
	response.Fail(c, http.StatusInternalServerError, "the DEA files are unavailable")
}

func getThreat(c *gin.Context) {
	threat, err := Evaluate(c.Request.Context(), c.GetUint(auth.UserIDKey))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Stay one step ahead of Hank", threat)
}

func getHistory(c *gin.Context) {
	limit := 30
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			fail(c, ErrInvalidLimit)
			return
		}
		limit = n
	}
	rows, err := History(c.GetUint(auth.UserIDKey), limit)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "The DEA's paper trail", rows)
}

func getEvidence(c *gin.Context) {
	file, err := Evidence(c.Request.Context(), c.GetUint(auth.UserIDKey))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "What the DEA has on you", file)
}
