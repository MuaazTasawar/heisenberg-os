package empire

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
	g := rg.Group("/empire")
	g.GET("", getEmpire)
	g.POST("/cook", cookBatch)
	g.POST("/territory", claimTerritory)
	g.GET("/advice", getAdvice)
	g.GET("/front-types", listFrontTypes)
	g.GET("/fronts", listFronts)
	g.POST("/front", addFront)
	g.DELETE("/front/:id", closeFront)
}

// fail maps a service error to an HTTP status.
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidEffort), errors.Is(err, ErrInvalidFront), errors.Is(err, ErrInsufficientFunds):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrMaxTerritories), errors.Is(err, ErrFrontLimit),
		errors.Is(err, ErrDuplicateFront), errors.Is(err, ErrFrontClosed):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrFrontNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		log.Printf("empire: %v", err)
		response.Fail(c, http.StatusInternalServerError, "something went wrong in the lab")
	}
}

func getEmpire(c *gin.Context) {
	emp, err := Get(c.GetUint(auth.UserIDKey))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Empire retrieved", emp)
}

func cookBatch(c *gin.Context) {
	var req struct {
		Effort float64 `json:"effort" binding:"required,gte=1,lte=10"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	result, err := Cook(c.GetUint(auth.UserIDKey), req.Effort)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Batch cooked", result)
}

func claimTerritory(c *gin.Context) {
	emp, err := ExpandTerritory(c.GetUint(auth.UserIDKey))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Territory claimed", emp)
}

func getAdvice(c *gin.Context) {
	effort := 5.0
	if raw := c.Query("effort"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			fail(c, ErrInvalidEffort)
			return
		}
		effort = parsed
	}
	advice, err := GetAdvice(c.Request.Context(), c.GetUint(auth.UserIDKey), effort)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Advice from the lab", advice)
}

func listFrontTypes(c *gin.Context) {
	response.OK(c, http.StatusOK, "Front business types", FrontTypes)
}

func listFronts(c *gin.Context) {
	fronts, err := ListFronts(c.GetUint(auth.UserIDKey))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Your front businesses", fronts)
}

func addFront(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	fb, err := AddFront(c.GetUint(auth.UserIDKey), req.Name, req.Type)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "Front business added — keep it clean", fb)
}

func closeFront(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "front business id must be a positive number")
		return
	}
	fb, err := CloseFront(c.GetUint(auth.UserIDKey), uint(id))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Front business closed", fb)
}
