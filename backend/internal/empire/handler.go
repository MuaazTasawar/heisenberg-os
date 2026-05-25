package empire

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/empire", getEmpire)
	rg.POST("/empire/cook", cookBatch)
	rg.POST("/empire/front", addFront)
}

func getEmpire(c *gin.Context) {
	userID := c.GetUint("user_id")
	emp, err := GetOrCreate(userID, "New Empire")
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Empire retrieved", emp)
}

func cookBatch(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		Effort float64 `json:"effort" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	var emp interface{ GetID() uint }
	_ = emp
	var empModel struct{ ID uint }
	db_result := getEmpireIDForUser(userID)
	result, err := Cook(db_result, req.Effort)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	_ = empModel
	response.OK(c, http.StatusOK, "Batch cooked", result)
}

func addFront(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req struct {
		Name string `json:"name" binding:"required"`
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	empID := getEmpireIDForUser(userID)
	fb, err := AddFrontBusiness(empID, req.Name, req.Type)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, http.StatusCreated, "Front business added — keep it clean", fb)
}

func getEmpireIDForUser(userID uint) uint {
	emp, _ := GetOrCreate(userID, "Empire")
	return emp.ID
}
