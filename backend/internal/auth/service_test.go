package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "unit-test-secret-unit-test-secret-1234"

func init() { bcryptCost = bcrypt.MinCost } // keep the tests fast

// ------------------------------------------------------------------ tokens

func TestTokenRoundTrip(t *testing.T) {
	token, err := IssueToken([]byte(testSecret), 7, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken([]byte(testSecret), token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 7 || claims.Issuer != Issuer || claims.Subject != "7" {
		t.Errorf("unexpected claims: %+v", claims)
	}
	if !claims.ExpiresAt.After(time.Now().Add(59 * time.Minute)) {
		t.Errorf("expiry is wrong: %v", claims.ExpiresAt)
	}
}

func TestTokenRejections(t *testing.T) {
	key := []byte(testSecret)
	good := Claims{UserID: 7, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: Issuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	sign := func(m jwt.SigningMethod, k any, c Claims) string {
		s, err := jwt.NewWithClaims(m, c).SignedString(k)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	expired := good
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))
	foreign := good
	foreign.Issuer = "someone-else"
	noExp := good
	noExp.ExpiresAt = nil
	noUser := good
	noUser.UserID = 0

	valid := sign(jwt.SigningMethodHS256, key, good)
	tampered := valid[:len(valid)-3] + "abc"
	if tampered == valid {
		tampered = valid[:len(valid)-3] + "xyz"
	}

	cases := map[string]string{
		"empty":          "",
		"garbage":        "not-a-token",
		"three dots":     "a.b.c",
		"expired":        sign(jwt.SigningMethodHS256, key, expired),
		"foreign issuer": sign(jwt.SigningMethodHS256, key, foreign),
		"no expiry":      sign(jwt.SigningMethodHS256, key, noExp),
		"no user":        sign(jwt.SigningMethodHS256, key, noUser),
		"other secret":   sign(jwt.SigningMethodHS256, []byte("other-secret-other-secret-12345678"), good),
		"alg none":       sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good),
		"alg HS512":      sign(jwt.SigningMethodHS512, key, good),
		"tampered":       tampered,
	}
	for name, token := range cases {
		if _, err := ParseToken(key, token); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
	if _, err := ParseToken(key, valid); err != nil {
		t.Errorf("the control token must be valid: %v", err)
	}
}

// -------------------------------------------------------------- validation

func TestValidateNormalisesInput(t *testing.T) {
	in := RegisterInput{Username: "  Walter_White ", Email: " Walt@Example.COM ", Password: "iamthedanger", Alias: "  "}
	if problems := in.Validate(); len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if in.Username != "walter_white" || in.Email != "walt@example.com" || in.Alias != "walter_white" {
		t.Errorf("not normalised: %+v", in)
	}

	in = RegisterInput{Username: "walt", Email: "w@x.co", Password: "12345678", Alias: " Heisenberg "}
	in.Validate()
	if in.Alias != "Heisenberg" {
		t.Errorf("alias should be trimmed, got %q", in.Alias)
	}
}

func TestValidateRules(t *testing.T) {
	good := func() RegisterInput {
		return RegisterInput{Username: "walter", Email: "walt@example.com", Password: "iamthedanger"}
	}
	cases := []struct {
		name   string
		mutate func(*RegisterInput)
		want   string // empty = valid
	}{
		{"valid", func(*RegisterInput) {}, ""},
		{"shortest username", func(in *RegisterInput) { in.Username = "abc" }, ""},
		{"longest username", func(in *RegisterInput) { in.Username = strings.Repeat("a", 32) }, ""},
		{"dots dashes underscores", func(in *RegisterInput) { in.Username = "a.b-c_d" }, ""},
		{"username too short", func(in *RegisterInput) { in.Username = "ab" }, "username must be"},
		{"username too long", func(in *RegisterInput) { in.Username = strings.Repeat("a", 33) }, "username must be"},
		{"username with space", func(in *RegisterInput) { in.Username = "bad name" }, "username must be"},
		{"username with symbol", func(in *RegisterInput) { in.Username = "walt!" }, "username must be"},
		{"username empty", func(in *RegisterInput) { in.Username = "" }, "username must be"},
		{"email empty", func(in *RegisterInput) { in.Email = "" }, "email must be"},
		{"email without @", func(in *RegisterInput) { in.Email = "walt" }, "email must be"},
		{"email without domain", func(in *RegisterInput) { in.Email = "walt@" }, "email must be"},
		{"email without dot", func(in *RegisterInput) { in.Email = "walt@example" }, "email must be"},
		{"email with display name", func(in *RegisterInput) { in.Email = "Walt <walt@example.com>" }, "email must be"},
		{"email too long", func(in *RegisterInput) { in.Email = strings.Repeat("a", 250) + "@x.co" }, "email must be"},
		{"password 7 chars", func(in *RegisterInput) { in.Password = "1234567" }, "at least 8"},
		{"password 8 chars", func(in *RegisterInput) { in.Password = "12345678" }, ""},
		{"password 72 bytes", func(in *RegisterInput) { in.Password = strings.Repeat("a", 72) }, ""},
		{"password 73 bytes", func(in *RegisterInput) { in.Password = strings.Repeat("a", 73) }, "at most 72 bytes"},
		{"password multibyte over 72 bytes", func(in *RegisterInput) { in.Password = strings.Repeat("é", 40) }, "at most 72 bytes"},
		{"password multibyte ok", func(in *RegisterInput) { in.Password = strings.Repeat("é", 8) }, ""},
		{"alias too long", func(in *RegisterInput) { in.Alias = strings.Repeat("a", 65) }, "alias must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := good()
			tc.mutate(&in)
			problems := strings.Join(in.Validate(), "; ")
			switch {
			case tc.want == "" && problems != "":
				t.Errorf("expected valid, got: %s", problems)
			case tc.want != "" && !strings.Contains(problems, tc.want):
				t.Errorf("expected %q in %q", tc.want, problems)
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	in := RegisterInput{Username: "x", Email: "nope", Password: "short"}
	if problems := in.Validate(); len(problems) != 3 {
		t.Errorf("expected 3 problems, got %v", problems)
	}
}

func TestPasswordHashIsNeverSerialised(t *testing.T) {
	b, _ := json.Marshal(models.User{Username: "walter", PasswordHash: "$2a$secret-hash"})
	if strings.Contains(string(b), "secret-hash") || strings.Contains(strings.ToLower(string(b)), "password") {
		t.Errorf("the hash leaks into JSON: %s", b)
	}
	if !strings.Contains(string(b), `"username":"walter"`) || strings.Contains(string(b), `"empire"`) {
		t.Errorf("unexpected JSON shape: %s", b)
	}
}

// ------------------------------------------------------- database-backed tests

var (
	testDBOnce sync.Once
	testDBErr  error
)

// openTestDB connects once per test run to TEST_DATABASE_URL in a private schema (one shared
// pool, so many tests cannot exhaust the server's connections). The tests are skipped when
// the variable is not set.
func openTestDB(t *testing.T) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set - skipping database tests")
	}
	testDBOnce.Do(func() {
		boot, err := db.Open(raw)
		if err != nil {
			testDBErr = err
			return
		}
		if err := db.EnsureSchema(boot, "auth_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "auth_test")
		if err != nil {
			testDBErr = err
			return
		}
		gdb, err := db.Open(dsn)
		if err != nil {
			testDBErr = err
			return
		}
		if err := db.Migrate(gdb); err != nil {
			testDBErr = err
			return
		}
		db.DB = gdb
	})
	if testDBErr != nil {
		t.Fatalf("cannot prepare the test database: %v", testDBErr)
	}
}
func unique(prefix string) string { return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()) }

func newInput(t *testing.T) RegisterInput {
	t.Helper()
	name := unique("walt")
	in := RegisterInput{Username: name, Email: name + "@example.com", Password: "iamthedanger", Alias: "Heisenberg"}
	if problems := in.Validate(); len(problems) > 0 {
		t.Fatal(problems)
	}
	return in
}

func TestRegisterThenLogin(t *testing.T) {
	openTestDB(t)
	in := newInput(t)

	user, err := Register(in)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 || user.PasswordHash == in.Password || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)) != nil {
		t.Errorf("password must be stored as a bcrypt hash: %+v", user)
	}

	logged, err := Login("  "+strings.ToUpper(in.Email)+" ", in.Password)
	if err != nil || logged.ID != user.ID {
		t.Errorf("login with different case/spacing should work: %v %v", logged, err)
	}
}

func TestDuplicatesAreRejected(t *testing.T) {
	openTestDB(t)
	first := newInput(t)
	if _, err := Register(first); err != nil {
		t.Fatal(err)
	}

	sameEmail := newInput(t)
	sameEmail.Email = first.Email
	if _, err := Register(sameEmail); err != ErrUserExists {
		t.Errorf("same email: got %v, want ErrUserExists", err)
	}

	sameName := newInput(t)
	sameName.Username = first.Username
	if _, err := Register(sameName); err != ErrUserExists {
		t.Errorf("same username: got %v, want ErrUserExists", err)
	}

	shouting := first
	shouting.Email = strings.ToUpper(first.Email)
	shouting.Username = strings.ToUpper(first.Username)
	shouting.Validate() // normalisation lower-cases both, so this is still a duplicate
	if _, err := Register(shouting); err != ErrUserExists {
		t.Errorf("different case: got %v, want ErrUserExists", err)
	}
}

func TestBadLoginsLookIdentical(t *testing.T) {
	openTestDB(t)
	in := newInput(t)
	if _, err := Register(in); err != nil {
		t.Fatal(err)
	}
	_, wrongPassword := Login(in.Email, "not-the-password")
	_, unknownEmail := Login(unique("ghost")+"@example.com", "whatever123")
	if wrongPassword != ErrInvalidCredentials || unknownEmail != ErrInvalidCredentials {
		t.Errorf("both failures must be ErrInvalidCredentials: %v / %v", wrongPassword, unknownEmail)
	}
}

func TestGetUser(t *testing.T) {
	openTestDB(t)
	user, err := Register(newInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if found, err := GetUser(user.ID); err != nil || found.Username != user.Username {
		t.Errorf("GetUser failed: %v %v", found, err)
	}
	if _, err := GetUser(999999999); err != ErrUserNotFound {
		t.Errorf("missing user: got %v, want ErrUserNotFound", err)
	}
}

// ---------------------------------------------------------- HTTP handlers

// handlerRouter builds the public + protected auth routes with a minimal token check, so
// the handlers can be tested without importing the middleware package (which imports auth).
func handlerRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(cfg)
	api := r.Group("/api/v1")
	h.Public(api, func(c *gin.Context) { c.Next() })
	protected := api.Group("", func(c *gin.Context) {
		cookie, err := c.Cookie(CookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no cookie"})
			return
		}
		claims, err := ParseToken([]byte(cfg.JWTSecret), cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bad token"})
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Next()
	})
	h.Protected(protected)
	return r
}

func post(r http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("not JSON: %v (%s)", err, w.Body.String())
	}
	return e
}

func TestHandlersEndToEnd(t *testing.T) {
	openTestDB(t)
	cfg := config.Defaults()
	cfg.JWTSecret = testSecret
	r := handlerRouter(cfg)

	name := unique("saul")
	email := name + "@example.com"
	body := fmt.Sprintf(`{"username":%q,"email":%q,"password":"better-call-saul","alias":"Slippin Jimmy"}`, name, email)

	// register
	w := post(r, "/api/v1/auth/register", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Message != "Say my name" || strings.Contains(string(e.Data), "password") || strings.Contains(string(e.Data), `"empire"`) {
		t.Errorf("register body wrong: %s", w.Body.String())
	}

	// duplicate
	if w := post(r, "/api/v1/auth/register", body); w.Code != http.StatusConflict {
		t.Errorf("duplicate register: %d", w.Code)
	}

	// validation
	w = post(r, "/api/v1/auth/register", `{"username":"x","email":"nope","password":"short"}`)
	if e := decode(t, w); w.Code != http.StatusBadRequest ||
		!strings.Contains(e.Error, "username must be") || !strings.Contains(e.Error, "email must be") || !strings.Contains(e.Error, "password must be") {
		t.Errorf("validation: %d %s", w.Code, w.Body.String())
	}
	if w := post(r, "/api/v1/auth/register", `{`); w.Code != http.StatusBadRequest {
		t.Errorf("broken JSON: %d", w.Code)
	}

	// login failures are identical and generic
	wrong := post(r, "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"wrong-password"}`, email))
	ghost := post(r, "/api/v1/auth/login", `{"email":"ghost@example.com","password":"wrong-password"}`)
	if wrong.Code != 401 || ghost.Code != 401 || decode(t, wrong).Error != decode(t, ghost).Error {
		t.Errorf("bad logins must look the same: %d %s / %d %s", wrong.Code, wrong.Body.String(), ghost.Code, ghost.Body.String())
	}
	if w := post(r, "/api/v1/auth/login", `{"email":"x@y.zz"}`); w.Code != 400 || !strings.Contains(w.Body.String(), "password is required") {
		t.Errorf("missing password: %d %s", w.Code, w.Body.String())
	}

	// login
	w = post(r, "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":"better-call-saul"}`, email))
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var data struct {
		Token     string      `json:"token"`
		ExpiresAt time.Time   `json:"expires_at"`
		User      models.User `json:"user"`
	}
	if err := json.Unmarshal(decode(t, w).Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Token == "" || data.User.Username != name || !data.ExpiresAt.After(time.Now().Add(71*time.Hour)) {
		t.Errorf("login payload wrong: %+v", data)
	}

	// session cookie
	var session *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName {
			session = c
		}
	}
	if session == nil || session.Value != data.Token || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode ||
		session.Secure || session.MaxAge != int((72*time.Hour).Seconds()) || session.Path != "/" {
		t.Fatalf("cookie wrong: %+v", session)
	}

	// me
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(session)
	mw := httptest.NewRecorder()
	r.ServeHTTP(mw, req)
	if mw.Code != http.StatusOK || !strings.Contains(mw.Body.String(), name) {
		t.Errorf("me: %d %s", mw.Code, mw.Body.String())
	}
	anon := httptest.NewRecorder()
	r.ServeHTTP(anon, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if anon.Code != http.StatusUnauthorized {
		t.Errorf("me without a session: %d", anon.Code)
	}

	// logout clears the cookie
	lw := post(r, "/api/v1/auth/logout", ``)
	cleared := false
	for _, c := range lw.Result().Cookies() {
		if c.Name == CookieName && c.MaxAge < 0 && c.Value == "" {
			cleared = true
		}
	}
	if lw.Code != http.StatusOK || !cleared {
		t.Errorf("logout: %d cleared=%v", lw.Code, cleared)
	}
}

func TestCookieIsSecureWhenConfigured(t *testing.T) {
	openTestDB(t)
	cfg := config.Defaults()
	cfg.JWTSecret = testSecret
	cfg.CookieSecure = true
	r := handlerRouter(cfg)

	name := unique("jesse")
	body := fmt.Sprintf(`{"username":%q,"email":"%s@example.com","password":"yeah-science"}`, name, name)
	if w := post(r, "/api/v1/auth/register", body); w.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	w := post(r, "/api/v1/auth/login", fmt.Sprintf(`{"email":"%s@example.com","password":"yeah-science"}`, name))
	for _, c := range w.Result().Cookies() {
		if c.Name == CookieName && !c.Secure {
			t.Error("cookie must be Secure when COOKIE_SECURE=true")
		}
	}
}
