package laundering

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/launder", launder)
	rg.GET("/transactions", listTransactions)
}

func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrInsufficientDirty), errors.Is(err, ErrInvalidPage):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, empire.ErrFrontNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, empire.ErrFrontClosed):
		response.Fail(c, http.StatusConflict, err.Error())
	default:
		log.Printf("laundering: %v", err)
		response.Fail(c, http.StatusInternalServerError, "the money got lost on the way")
	}
}

func launder(c *gin.Context) {
	var req struct {
		Amount     float64 `json:"amount" binding:"required,gte=100,lte=100000000"`
		BusinessID uint    `json:"business_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	result, err := Launder(c.Request.Context(), c.GetUint(auth.UserIDKey), req.BusinessID, req.Amount)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Money's clean, Mr. White", result)
}

func listTransactions(c *gin.Context) {
	limit, offset := 50, 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			response.Fail(c, http.StatusBadRequest, "limit must be a number from 1 to 200")
			return
		}
		limit = n
	}
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			response.Fail(c, http.StatusBadRequest, "offset must be 0 or more")
			return
		}
		offset = n
	}
	var flagged *bool
	if raw := c.Query("flagged"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, "flagged must be true or false")
			return
		}
		flagged = &b
	}

	page, err := ListTransactions(c.GetUint(auth.UserIDKey), limit, offset, flagged)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Transaction ledger", page)
}
