package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/pkg/response"
	"net/http"
)

// JWTAuth requires a valid token, taken from "Authorization: Bearer ..." (mobile, scripts)
// or from the httpOnly session cookie (browser). The caller's id is stored in the context
// as auth.UserIDKey.
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	secret := []byte(cfg.JWTSecret)
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		claims, err := auth.ParseToken(secret, token)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}
		c.Set(auth.UserIDKey, claims.UserID)
		c.Next()
	}
}

func tokenFromRequest(c *gin.Context) string {
	if header := c.GetHeader("Authorization"); header != "" {
		if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
			return strings.TrimSpace(header[7:])
		}
		return ""
	}
	if cookie, err := c.Cookie(auth.CookieName); err == nil {
		return cookie
	}
	return ""
}
