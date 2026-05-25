package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Alias    string `json:"alias"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/register", handleRegister)
	rg.POST("/auth/login", handleLogin)
}

func handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	user, err := Register(req.Username, req.Email, req.Password, req.Alias)
	if err != nil {
		response.Fail(c, http.StatusConflict, err.Error())
		return
	}
	response.OK(c, http.StatusCreated, "Say my name", user)
}

func handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	token, err := Login(req.Email, req.Password)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}
	response.OK(c, http.StatusOK, "I am the one who knocks", gin.H{"token": token})
}
