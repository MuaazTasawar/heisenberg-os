package laundering

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/launder", launderHandler)
	rg.GET("/transactions", transactionsHandler)
}

func launderHandler(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		Amount     float64 `json:"amount" binding:"required"`
		BusinessID uint    `json:"business_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	empID := empire.GetEmpireID(userID)
	tx, err := LaunderMoney(empID, req.Amount, req.BusinessID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Money's clean, Mr. White", tx)
}

func transactionsHandler(c *gin.Context) {
	userID := c.GetUint("user_id")
	empID := empire.GetEmpireID(userID)
	txs, _ := GetTransactions(empID)
	response.OK(c, http.StatusOK, "Transaction ledger", txs)
}
