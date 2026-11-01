package legal

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/pkg/response"
)

// RegisterRoutes adds the legal routes. consultLimit (for example a rate limiter) applies only
// to the consultation, which costs a call to the Claude API; reading old cases is not limited
// by it.
func RegisterRoutes(rg *gin.RouterGroup, consultLimit ...gin.HandlerFunc) {
	consult := append(append([]gin.HandlerFunc{}, consultLimit...), consultHandler)
	rg.POST("/legal/consult", consult...)
	rg.GET("/legal/cases", listCases)
	rg.GET("/legal/cases/:id", getCase)
}

func fail(c *gin.Context, err error) {
	var input *InputError
	switch {
	case errors.As(err, &input):
		response.Fail(c, http.StatusBadRequest, input.Msg)
	case errors.Is(err, ErrInvalidPage):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrCaseNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotConfigured):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrUpstream):
		response.Fail(c, http.StatusBadGateway, err.Error())
	default:
		log.Printf("legal: %v", err)
		response.Fail(c, http.StatusInternalServerError, "the legal files are unavailable")
	}
}

func consultHandler(c *gin.Context) {
	var req struct {
		Charge        string `json:"charge"`
		Evidence      string `json:"evidence"`
		PlayerContext string `json:"player_context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	legalCase, err := Consult(c.Request.Context(), c.GetUint(auth.UserIDKey), ConsultInput{
		Charge: req.Charge, Evidence: req.Evidence, PlayerContext: req.PlayerContext,
	})
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Better call Saul", legalCase)
}

func listCases(c *gin.Context) {
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
	page, err := ListCases(c.GetUint(auth.UserIDKey), limit, offset)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Your case files", page)
}

func getCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "case id must be a positive number")
		return
	}
	legalCase, err := GetCase(c.GetUint(auth.UserIDKey), uint(id))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, "The case file", legalCase)
}
