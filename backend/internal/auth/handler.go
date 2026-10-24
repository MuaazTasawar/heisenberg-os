package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/pkg/response"
)

type Handler struct {
	cfg *config.Config
}

func NewHandler(cfg *config.Config) *Handler { return &Handler{cfg: cfg} }

// Public registers the routes that need no login. limiter throttles register and login;
// logout is harmless, so a throttled client can always still sign out.
func (h *Handler) Public(rg *gin.RouterGroup, limiter gin.HandlerFunc) {
	g := rg.Group("/auth", limiter)
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	rg.POST("/auth/logout", h.logout)
}

// Protected registers the routes that need a valid token.
func (h *Handler) Protected(rg *gin.RouterGroup) {
	rg.GET("/auth/me", h.me)
}

func (h *Handler) register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Alias    string `json:"alias"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	in := RegisterInput{Username: req.Username, Email: req.Email, Password: req.Password, Alias: req.Alias}
	if problems := in.Validate(); len(problems) > 0 {
		response.Fail(c, http.StatusBadRequest, strings.Join(problems, "; "))
		return
	}
	user, err := Register(in)
	switch {
	case errors.Is(err, ErrUserExists):
		response.Fail(c, http.StatusConflict, ErrUserExists.Error())
	case err != nil:
		log.Printf("register failed: %v", err)
		response.Fail(c, http.StatusInternalServerError, "could not create the account")
	default:
		response.OK(c, http.StatusCreated, "Say my name", user)
	}
}

func (h *Handler) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	user, err := Login(req.Email, req.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		response.Fail(c, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	}
	if err != nil {
		log.Printf("login failed: %v", err)
		response.Fail(c, http.StatusInternalServerError, "login failed")
		return
	}

	now := time.Now()
	token, err := IssueToken([]byte(h.cfg.JWTSecret), user.ID, h.cfg.JWTTTL, now)
	if err != nil {
		log.Printf("could not sign token: %v", err)
		response.Fail(c, http.StatusInternalServerError, "login failed")
		return
	}

	// Browsers get an httpOnly cookie (invisible to scripts); mobile apps use the token
	// from the body as a Bearer header.
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.cfg.JWTTTL.Seconds()),
		Expires:  now.Add(h.cfg.JWTTTL),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	response.OK(c, http.StatusOK, "I am the one who knocks", gin.H{
		"token":      token,
		"expires_at": now.Add(h.cfg.JWTTTL).UTC(),
		"user":       user,
	})
}

func (h *Handler) logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	response.OK(c, http.StatusOK, "Walk away", nil)
}

func (h *Handler) me(c *gin.Context) {
	user, err := GetUser(c.GetUint(UserIDKey))
	if errors.Is(err, ErrUserNotFound) {
		response.Fail(c, http.StatusUnauthorized, "this account no longer exists")
		return
	}
	if err != nil {
		log.Printf("me failed: %v", err)
		response.Fail(c, http.StatusInternalServerError, "could not load the account")
		return
	}
	response.OK(c, http.StatusOK, "Who are you?", user)
}
