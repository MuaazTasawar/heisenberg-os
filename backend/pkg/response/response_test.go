package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type sample struct {
	Amount     float64 `json:"amount" binding:"required,gt=0"`
	BusinessID uint    `json:"business_id" binding:"required"`
	Email      string  `json:"email" binding:"omitempty,email"`
	Kind       string  `json:"kind" binding:"omitempty,oneof=a b c"`
	Name       string  `json:"name" binding:"omitempty,min=3,max=5"`
	Effort     float64 `json:"effort" binding:"omitempty,gte=1,lte=10"`
}

func bind(t *testing.T, body string) (int, APIResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	var s sample
	if err := c.ShouldBindJSON(&s); err != nil {
		BindError(c, err)
	} else {
		OK(c, http.StatusOK, "fine", s)
	}
	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	return w.Code, resp
}

func TestBindErrorMessages(t *testing.T) {
	cases := []struct {
		name, body, want string
		status           int
	}{
		{"valid", `{"amount":5,"business_id":1}`, "", 200},
		{"missing field", `{"business_id":1}`, "amount is required", 400},
		{"snake case name", `{"amount":5}`, "business_id is required", 400},
		{"too small", `{"amount":-1,"business_id":1}`, "amount must be greater than 0", 400},
		{"several problems", `{}`, "amount is required; business_id is required", 400},
		{"wrong type", `{"amount":"lots","business_id":1}`, "amount must be a number", 400},
		{"wrong type text", `{"amount":1,"business_id":1,"name":5}`, "name must be text", 400},
		{"wrong type bool", `{"amount":true,"business_id":1}`, "amount must be a number", 400},
		{"broken json", `{"amount":`, "request body is not valid JSON", 400},
		{"not json", `hello`, "request body is not valid JSON", 400},
		{"empty body", ``, "request body is empty", 400},
		{"bad email", `{"amount":1,"business_id":1,"email":"nope"}`, "email must be a valid email address", 400},
		{"one of", `{"amount":1,"business_id":1,"kind":"z"}`, "kind must be one of: a, b, c", 400},
		{"text too short", `{"amount":1,"business_id":1,"name":"ab"}`, "name must be at least 3 characters", 400},
		{"text too long", `{"amount":1,"business_id":1,"name":"abcdefg"}`, "name must be at most 5 characters", 400},
		{"number too small", `{"amount":1,"business_id":1,"effort":0.5}`, "effort must be at least 1", 400},
		{"number too big", `{"amount":1,"business_id":1,"effort":11}`, "effort must be at most 10", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := bind(t, tc.body)
			if status != tc.status {
				t.Fatalf("status = %d, want %d (%+v)", status, tc.status, resp)
			}
			if tc.want == "" {
				if !resp.Success {
					t.Errorf("expected success, got %+v", resp)
				}
				return
			}
			if resp.Success || resp.Error != tc.want {
				t.Errorf("error = %q, want %q", resp.Error, tc.want)
			}
		})
	}
}

func TestOversizedBodyIs413(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"amount":5,"business_id":1,"name":"padding padding"}`))
	c.Request.Body = http.MaxBytesReader(w, c.Request.Body, 10)
	var s sample
	if err := c.ShouldBindJSON(&s); err == nil {
		t.Fatal("expected an error for an oversized body")
	} else {
		BindError(c, err)
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", w.Code)
	}
}

func TestEnvelopeShapes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, http.StatusCreated, "made", gin.H{"id": 1})
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"success":true`) || !strings.Contains(w.Body.String(), `"data":{"id":1}`) {
		t.Errorf("unexpected OK body: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	Fail(c, http.StatusConflict, "nope")
	body := w.Body.String()
	if w.Code != 409 || !strings.Contains(body, `"success":false`) || !strings.Contains(body, `"error":"nope"`) || strings.Contains(body, `"data"`) {
		t.Errorf("unexpected Fail body: %d %s", w.Code, body)
	}
}

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"Amount": "amount", "BusinessID": "business_id", "PlayerContext": "player_context",
		"ID": "id", "URLPath": "url_path", "A": "a", "already_snake": "already_snake", "": "",
	}
	for in, want := range cases {
		if got := snake(in); got != want {
			t.Errorf("snake(%q) = %q, want %q", in, got, want)
		}
	}
}
