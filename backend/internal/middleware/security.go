package middleware

import (
	"mime"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/pkg/response"
)

// SecurityHeaders adds conservative headers to every response. The API only returns JSON,
// so a locked-down content policy costs nothing.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Cache-Control", "no-store")
		c.Next()
	}
}

// MaxBody rejects request bodies larger than n bytes (the JSON binder then reports 413).
func MaxBody(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// RequireJSON answers 415 to POST/PUT/PATCH requests that carry a body that is not JSON.
// Together with the CORS policy this stops a foreign page from submitting a plain HTML form
// to the API with the visitor's cookie.
func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if c.Request.ContentLength != 0 {
				mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
				if err != nil || mediaType != "application/json" {
					response.Fail(c, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

// CheckOrigin rejects state-changing browser requests whose Origin header is not one of the
// allowed origins. Requests without an Origin header (mobile app, curl, server to server)
// are not browser requests and pass.
func CheckOrigin(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if origin := c.GetHeader("Origin"); origin != "" && !slices.Contains(allowed, origin) {
				response.Fail(c, http.StatusForbidden, "origin not allowed")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
