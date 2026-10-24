package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/pkg/response"
)

const testSecret = "unit-test-secret-unit-test-secret-1234"

func testConfig() *config.Config {
	cfg := config.Defaults()
	cfg.JWTSecret = testSecret
	return cfg
}

func newRouter(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies([]string{})
	r.Use(mw...)
	handler := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"user": c.GetUint(auth.UserIDKey)}) }
	r.Any("/x", handler)
	return r
}

func send(r http.Handler, method string, body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/x", reader)
	if mutate != nil {
		mutate(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bearer(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func validToken(t *testing.T) string {
	t.Helper()
	token, err := auth.IssueToken([]byte(testSecret), 42, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func signed(t *testing.T, method jwt.SigningMethod, key any, claims auth.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func goodClaims() auth.Claims {
	return auth.Claims{UserID: 42, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: auth.Issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
}

// ------------------------------------------------------------------- JWT

func TestJWTAcceptsBearerAndCookie(t *testing.T) {
	r := newRouter(JWTAuth(testConfig()))
	token := validToken(t)

	for name, mutate := range map[string]func(*http.Request){
		"bearer":            bearer(token),
		"lower-case scheme": func(r *http.Request) { r.Header.Set("Authorization", "bearer "+token) },
		"cookie": func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		},
	} {
		w := send(r, http.MethodGet, "", mutate)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"user":42`) {
			t.Errorf("%s: status %d body %s", name, w.Code, w.Body.String())
		}
	}
}

func TestJWTRejectsMissingAndMalformedCredentials(t *testing.T) {
	r := newRouter(JWTAuth(testConfig()))
	token := validToken(t)

	cases := map[string]func(*http.Request){
		"nothing":      nil,
		"wrong scheme": func(r *http.Request) { r.Header.Set("Authorization", "Token "+token) },
		"empty bearer": func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") },
		"garbage":      bearer("not-a-token"),
		"three dots":   bearer("a.b.c"),
		"header wins over a valid cookie": func(r *http.Request) {
			r.Header.Set("Authorization", "Basic abc")
			r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		},
	}
	for name, mutate := range cases {
		if w := send(r, http.MethodGet, "", mutate); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, w.Code)
		}
	}
}

func TestJWTRejectsBadTokens(t *testing.T) {
	r := newRouter(JWTAuth(testConfig()))
	key := []byte(testSecret)

	expired := goodClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	wrongIssuer := goodClaims()
	wrongIssuer.Issuer = "someone-else"
	noExpiry := goodClaims()
	noExpiry.ExpiresAt = nil
	noUser := goodClaims()
	noUser.UserID = 0

	cases := map[string]string{
		"expired":         signed(t, jwt.SigningMethodHS256, key, expired),
		"wrong issuer":    signed(t, jwt.SigningMethodHS256, key, wrongIssuer),
		"no expiry":       signed(t, jwt.SigningMethodHS256, key, noExpiry),
		"no user":         signed(t, jwt.SigningMethodHS256, key, noUser),
		"wrong secret":    signed(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-123456"), goodClaims()),
		"algorithm none":  signed(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, goodClaims()),
		"algorithm HS512": signed(t, jwt.SigningMethodHS512, key, goodClaims()),
	}
	for name, token := range cases {
		w := send(r, http.MethodGet, "", bearer(token))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, w.Code)
		}
	}
}

func TestJWTErrorsUseTheStandardEnvelope(t *testing.T) {
	r := newRouter(JWTAuth(testConfig()))
	w := send(r, http.MethodGet, "", nil)
	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Success || resp.Error != "authentication required" {
		t.Errorf("unexpected body: %+v", resp)
	}
}

// ------------------------------------------------------------ rate limit

type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func TestRateLimiterAllowsThenBlocksThenResets(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(3, time.Minute)
	rl.now = clock.now

	for i, wantLeft := range []int{2, 1, 0} {
		ok, left, _ := rl.Allow("a")
		if !ok || left != wantLeft {
			t.Fatalf("request %d: allowed=%v left=%d", i+1, ok, left)
		}
	}
	ok, left, retry := rl.Allow("a")
	if ok || left != 0 || retry != time.Minute {
		t.Fatalf("4th request: allowed=%v left=%d retry=%v", ok, left, retry)
	}

	clock.advance(20 * time.Second)
	if _, _, retry := rl.Allow("a"); retry != 40*time.Second {
		t.Errorf("retry-after should shrink to 40s, got %v", retry)
	}

	clock.advance(41 * time.Second)
	if ok, _, _ := rl.Allow("a"); !ok {
		t.Error("the window should have reset")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	if ok, _, _ := rl.Allow("a"); !ok {
		t.Fatal("a should pass")
	}
	if ok, _, _ := rl.Allow("b"); !ok {
		t.Error("b must not be affected by a")
	}
	if ok, _, _ := rl.Allow("a"); ok {
		t.Error("a should be blocked")
	}
}

func TestRateLimiterForgetsExpiredClients(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(5, time.Minute)
	rl.now = clock.now
	for _, k := range []string{"a", "b", "c"} {
		rl.Allow(k)
	}
	clock.advance(2 * time.Minute)
	rl.Allow("d")
	if len(rl.hits) != 1 {
		t.Errorf("expired entries should be swept, %d remain", len(rl.hits))
	}
}

func TestRateLimitMiddlewareResponses(t *testing.T) {
	r := newRouter(RateLimit(2, time.Minute))
	from := func(addr string) func(*http.Request) {
		return func(req *http.Request) { req.RemoteAddr = addr }
	}

	for i := 0; i < 2; i++ {
		w := send(r, http.MethodGet, "", from("203.0.113.5:1000"))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
		if w.Header().Get("X-RateLimit-Limit") != "2" {
			t.Errorf("missing limit header: %v", w.Header())
		}
	}
	w := send(r, http.MethodGet, "", from("203.0.113.5:2000"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") != "60" || w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("unexpected headers: %v", w.Header())
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Errorf("429 should use the envelope: %s", w.Body.String())
	}

	if w := send(r, http.MethodGet, "", from("198.51.100.9:1")); w.Code != http.StatusOK {
		t.Errorf("another client must not be limited: %d", w.Code)
	}
}

func TestRateLimitOfZeroDisablesTheLimiter(t *testing.T) {
	r := newRouter(RateLimit(0, time.Minute))
	for i := 0; i < 50; i++ {
		if w := send(r, http.MethodGet, "", nil); w.Code != http.StatusOK {
			t.Fatalf("request %d blocked: %d", i, w.Code)
		}
	}
}

// -------------------------------------------------------------- security

func TestSecurityHeaders(t *testing.T) {
	w := send(newRouter(SecurityHeaders()), http.MethodGet, "", nil)
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("missing CSP: %v", w.Header())
	}
}

func TestRequireJSON(t *testing.T) {
	r := newRouter(RequireJSON())
	asType := func(ct string) func(*http.Request) {
		return func(req *http.Request) { req.Header.Set("Content-Type", ct) }
	}

	cases := []struct {
		name, method, body string
		mutate             func(*http.Request)
		want               int
	}{
		{"json", http.MethodPost, `{}`, asType("application/json"), 200},
		{"json with charset", http.MethodPost, `{}`, asType("application/json; charset=utf-8"), 200},
		{"plain text", http.MethodPost, `{}`, asType("text/plain"), 415},
		{"form post", http.MethodPost, `a=1`, asType("application/x-www-form-urlencoded"), 415},
		{"no content type", http.MethodPost, `{}`, nil, 415},
		{"PUT plain text", http.MethodPut, `{}`, asType("text/plain"), 415},
		{"PATCH plain text", http.MethodPatch, `{}`, asType("text/plain"), 415},
		{"no body", http.MethodPost, ``, nil, 200},
		{"GET is exempt", http.MethodGet, ``, asType("text/plain"), 200},
		{"DELETE is exempt", http.MethodDelete, ``, asType("text/plain"), 200},
		{"unknown length", http.MethodPost, `{}`, func(req *http.Request) {
			req.ContentLength = -1
			req.Header.Set("Content-Type", "text/plain")
		}, 415},
	}
	for _, tc := range cases {
		if w := send(r, tc.method, tc.body, tc.mutate); w.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestMaxBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MaxBody(16))
	r.POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			response.BindError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})

	small := httptest.NewRecorder()
	r.ServeHTTP(small, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("tiny")))
	if small.Code != http.StatusNoContent {
		t.Errorf("small body: %d", small.Code)
	}
	big := httptest.NewRecorder()
	r.ServeHTTP(big, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("x", 100))))
	if big.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("big body: %d, want 413", big.Code)
	}
}

func TestCheckOrigin(t *testing.T) {
	r := newRouter(CheckOrigin([]string{"http://localhost:3000"}))
	from := func(origin string) func(*http.Request) {
		return func(req *http.Request) { req.Header.Set("Origin", origin) }
	}

	cases := []struct {
		name, method string
		mutate       func(*http.Request)
		want         int
	}{
		{"allowed origin", http.MethodPost, from("http://localhost:3000"), 200},
		{"foreign origin POST", http.MethodPost, from("https://evil.example"), 403},
		{"foreign origin PUT", http.MethodPut, from("https://evil.example"), 403},
		{"foreign origin DELETE", http.MethodDelete, from("https://evil.example"), 403},
		{"look-alike origin", http.MethodPost, from("http://localhost:3000.evil.example"), 403},
		{"no origin (mobile, curl)", http.MethodPost, nil, 200},
		{"foreign origin GET is harmless", http.MethodGet, from("https://evil.example"), 200},
		{"preflight is not blocked here", http.MethodOptions, from("https://evil.example"), 200},
	}
	for _, tc := range cases {
		if w := send(r, tc.method, "", tc.mutate); w.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}
