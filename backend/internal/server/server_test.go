package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/mlclient"
)

// These tests drive the whole backend the way a client does: real HTTP requests against
// server.New(cfg), with a cookie jar for browser-style sessions, a fake Anthropic API and a
// fake ML service. They check what the unit tests of the packages cannot: that the pieces are
// wired together and that the cross-cutting rules (login, origin, content type, size, rate
// limits, headers) hold for every route.

var (
	testDBOnce sync.Once
	testDBErr  error
	testDBRaw  string
)

func openTestDB(t *testing.T) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set - skipping database tests")
	}
	testDBOnce.Do(func() {
		testDBRaw = raw
		boot, err := db.Open(raw)
		if err != nil {
			testDBErr = err
			return
		}
		if err := db.EnsureSchema(boot, "server_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "server_test")
		if err != nil {
			testDBErr = err
			return
		}
		gdb, err := db.Open(dsn)
		if err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := gdb.DB(); err == nil {
			sqlDB.SetMaxOpenConns(30)
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

// ------------------------------------------------------------------- the test harness

const (
	allowedOrigin = "http://localhost:3000"
	foreignOrigin = "http://evil.example"
	password      = "iamthedanger"
)

type api struct {
	t   *testing.T
	srv *httptest.Server
	cfg *config.Config
}

// newAPI starts the real application with generous rate limits, ML switched off and a working
// (fake) Claude API. tweak changes the configuration before the application is built.
func newAPI(t *testing.T, tweak ...func(*config.Config)) *api {
	t.Helper()
	openTestDB(t)
	gin.SetMode(gin.TestMode)

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.JWTSecret = hex.EncodeToString(secret)
	cfg.CORSOrigins = []string{allowedOrigin}
	cfg.RateAuthPerMin, cfg.RateLegalPerMin, cfg.RateAPIPerMin = 100000, 100000, 100000
	cfg.MLServiceURL = ""
	cfg.AnthropicAPIKey = "test-key"
	cfg.AnthropicBaseURL = fakeClaude(t)
	cfg.AnthropicTimeout = 3 * time.Second
	for _, f := range tweak {
		f(cfg)
	}

	srv := httptest.NewServer(New(cfg))
	t.Cleanup(func() {
		srv.Close()
		config.Set(nil)
		mlclient.SetDefault(nil)
	})
	return &api{t: t, srv: srv, cfg: cfg}
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

type result struct {
	Status int
	Header http.Header
	Raw    string
	Env    envelope
}

// decode reads the data of a response into out.
func (r result) decode(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Env.Data, out); err != nil {
		t.Fatalf("cannot read the response data: %v\n%s", err, r.Raw)
	}
}

func (r result) data(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	r.decode(t, &m)
	return m
}

type client struct {
	t    *testing.T
	base string
	hc   *http.Client
}

// client returns a new HTTP client; withJar keeps cookies between requests like a browser.
func (a *api) client(withJar bool) *client {
	hc := &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if withJar {
		jar, err := cookiejar.New(nil)
		if err != nil {
			a.t.Fatal(err)
		}
		hc.Jar = jar
	}
	return &client{t: a.t, base: a.srv.URL, hc: hc}
}

// do sends a request. body may be nil, a string (sent as is) or anything JSON-encodable; JSON
// bodies get Content-Type application/json. hdr is key, value pairs that override the defaults;
// an empty value removes the header.
func (c *client) do(method, path string, body any, hdr ...string) result {
	c.t.Helper()
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case string:
		reader, contentType = strings.NewReader(b), "application/json"
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.t.Fatal(err)
		}
		reader, contentType = strings.NewReader(string(raw)), "application/json"
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		if hdr[i+1] == "" {
			req.Header.Del(hdr[i])
		} else {
			req.Header.Set(hdr[i], hdr[i+1])
		}
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	res := result{Status: resp.StatusCode, Header: resp.Header, Raw: string(raw)}
	_ = json.Unmarshal(raw, &res.Env)
	return res
}

func (c *client) get(path string, hdr ...string) result {
	c.t.Helper()
	return c.do("GET", path, nil, hdr...)
}
func (c *client) post(path string, body any, hdr ...string) result {
	c.t.Helper()
	return c.do("POST", path, body, hdr...)
}

// want fails the test unless the response has the given status and message.
func want(t *testing.T, r result, status int, message string) result {
	t.Helper()
	if r.Status != status || (message != "" && r.Env.Message != message) || (status < 400 && !r.Env.Success) {
		t.Fatalf("want HTTP %d %q, got HTTP %d %q (error %q)\n%s", status, message, r.Status, r.Env.Message, r.Env.Error, r.Raw)
	}
	return r
}

// wantError fails the test unless the response is the given failure.
func wantError(t *testing.T, r result, status int, message string) {
	t.Helper()
	if r.Status != status || r.Env.Success || r.Env.Error != message {
		t.Errorf("want HTTP %d error %q, got HTTP %d error %q\n%s", status, message, r.Status, r.Env.Error, r.Raw)
	}
}

type player struct {
	Username, Email, Token string
}

func uniqueName(prefix string) string { return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()) }

func (c *client) register(t *testing.T) player {
	t.Helper()
	name := uniqueName("walter")
	r := c.post("/api/v1/auth/register", map[string]string{"username": name, "email": name + "@example.com", "password": password, "alias": "Heisenberg"})
	want(t, r, 201, "Say my name")
	return player{Username: name, Email: name + "@example.com"}
}

func (c *client) login(t *testing.T, p player) player {
	t.Helper()
	r := want(t, c.post("/api/v1/auth/login", map[string]string{"email": p.Email, "password": password}), 200, "I am the one who knocks")
	p.Token, _ = r.data(t)["token"].(string)
	if p.Token == "" {
		t.Fatalf("login returned no token: %s", r.Raw)
	}
	return p
}

// newPlayer registers and logs in a player on a client with a cookie jar.
func (a *api) newPlayer() (*client, player) {
	a.t.Helper()
	c := a.client(true)
	p := c.login(a.t, c.register(a.t))
	return c, p
}

// ---------------------------------------------------------------- fake Claude and ML

const saulAnswer = `{"advice":"Don't say a word without me, kid.","risk":"high","defense":"Chain of custody, my friend."}`

func fakeClaude(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		reply, _ := json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": saulAnswer}}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// fakeML is an ML service that insists on the right API key. threat answers
// /ml/threat/analyze with the given status and body.
func fakeML(t *testing.T, key string, threatStatus int, threatBody string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-API-Key") != key {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"invalid API key"}`))
			return
		}
		switch {
		case r.URL.Path == "/ml/threat/analyze":
			w.WriteHeader(threatStatus)
			_, _ = w.Write([]byte(threatBody))
		case strings.HasPrefix(r.URL.Path, "/ml/evidence/"):
			_, _ = w.Write([]byte(`{"empire_id":0,"node_count":0,"total_weight":0,"top_sources":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// -------------------------------------------------------------------- the route table

var publicRoutes = map[string]bool{
	"POST /api/v1/auth/register": true,
	"POST /api/v1/auth/login":    true,
	"POST /api/v1/auth/logout":   true,
	"GET /api/v1/stats":          true,
	"GET /healthz":               true,
}

func routeTable(t *testing.T) []string {
	t.Helper()
	cfg := config.Defaults()
	cfg.JWTSecret = strings.Repeat("ab", 32)
	gin.SetMode(gin.TestMode)
	var routes []string
	for _, r := range New(cfg).Routes() {
		routes = append(routes, r.Method+" "+r.Path)
	}
	t.Cleanup(func() { config.Set(nil); mlclient.SetDefault(nil) })
	sort.Strings(routes)
	return routes
}

func TestTheRouteTableIsWhatWeExpect(t *testing.T) {
	expected := []string{
		"DELETE /api/v1/empire/front/:id",
		"GET /api/v1/auth/me",
		"GET /api/v1/dea/evidence",
		"GET /api/v1/dea/history",
		"GET /api/v1/dea/threat",
		"GET /api/v1/empire",
		"GET /api/v1/empire/advice",
		"GET /api/v1/empire/front-types",
		"GET /api/v1/empire/fronts",
		"GET /api/v1/game/state",
		"GET /api/v1/legal/cases",
		"GET /api/v1/legal/cases/:id",
		"GET /api/v1/stats",
		"GET /api/v1/transactions",
		"GET /healthz",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/register",
		"POST /api/v1/empire/cook",
		"POST /api/v1/empire/front",
		"POST /api/v1/empire/territory",
		"POST /api/v1/launder",
		"POST /api/v1/legal/consult",
	}
	got := routeTable(t)
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("the routes changed - update this list on purpose, and check the new route's login requirement:\n got:  %v\n want: %v", got, expected)
	}
}

func TestEveryRouteExceptThePublicOnesRequiresALogin(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)
	protected := 0
	for _, route := range routeTable(t) {
		if publicRoutes[route] {
			continue
		}
		protected++
		method, path, _ := strings.Cut(route, " ")
		path = strings.ReplaceAll(path, ":id", "1")
		var body any
		if method == "POST" || method == "DELETE" {
			body = `{}`
		}
		r := c.do(method, path, body)
		if r.Status != 401 || r.Env.Success || r.Env.Error != "authentication required" {
			t.Errorf("%s without a login: HTTP %d %s", route, r.Status, r.Raw)
		}
	}
	if protected < 15 {
		t.Errorf("only %d protected routes were checked", protected)
	}
}

func TestThePublicRoutesNeedNoLogin(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)
	if r := c.get("/api/v1/stats"); r.Status != 200 {
		t.Errorf("stats: %d", r.Status)
	}
	if r := c.get("/healthz"); r.Status != 200 {
		t.Errorf("healthz: %d", r.Status)
	}
	if r := c.post("/api/v1/auth/logout", nil); r.Status != 200 || r.Env.Message != "Walk away" {
		t.Errorf("logout while logged out: %d %s", r.Status, r.Raw)
	}
	wantError(t, c.post("/api/v1/auth/register", `{}`), 400, c.post("/api/v1/auth/register", `{}`).Env.Error)
	if r := c.post("/api/v1/auth/login", `{}`); r.Status != 400 {
		t.Errorf("login with nothing: %d %s", r.Status, r.Raw)
	}
}

// --------------------------------------------------------------------- a whole player

func TestAPlayerJourneyThroughTheWholeBackend(t *testing.T) {
	a := newAPI(t)
	c, p := a.newPlayer()

	me := want(t, c.get("/api/v1/auth/me"), 200, "Who are you?").data(t)
	if me["username"] != p.Username || me["email"] != p.Email {
		t.Errorf("me: %v", me)
	}
	if _, leaked := me["password_hash"]; leaked {
		t.Error("the password hash must never be sent")
	}

	// the empire starts empty
	emp := want(t, c.get("/api/v1/empire"), 200, "Empire retrieved").data(t)
	if emp["dirty_money"] != 0.0 || emp["clean_money"] != 0.0 || emp["territory_count"] != 0.0 {
		t.Errorf("a new empire: %v", emp)
	}
	if fronts, ok := emp["front_businesses"].([]any); !ok || len(fronts) != 0 {
		t.Errorf("front_businesses must be []: %v", emp["front_businesses"])
	}
	first := want(t, c.get("/api/v1/game/state"), 200, "Game state loaded").data(t)
	firstScore := first["dea_threat"].(map[string]any)["threat_score"].(float64)

	// cook
	for i := 1; i <= 6; i++ {
		cook := want(t, c.post("/api/v1/empire/cook", map[string]any{"effort": 8}), 200, "Batch cooked").data(t)
		if i == 1 && (cook["dirty_gained"] != 15600.0 || cook["heat_added"] != 20.0 || cook["effort"] != 8.0) {
			t.Errorf("the first cook: %v", cook)
		}
	}
	emp = want(t, c.get("/api/v1/empire"), 200, "").data(t)
	dirty := emp["dirty_money"].(float64)
	if dirty < 6*15600 || emp["heat_level"].(float64) != 100 { // purity grows, so later batches pay a little more; heat is capped
		t.Errorf("after six cooks: dirty money %v, heat %v", dirty, emp["heat_level"])
	}

	// a front business and a laundering transfer above the $50,000 line
	want(t, c.get("/api/v1/empire/front-types"), 200, "Front business types")
	front := want(t, c.post("/api/v1/empire/front", map[string]string{"name": "Los Pollos Hermanos", "type": "restaurant"}), 201, "").data(t)
	frontID := front["id"]
	launder := want(t, c.post("/api/v1/launder", map[string]any{"amount": 60000, "business_id": frontID}), 200, "").data(t)
	tx, _ := launder["transaction"].(map[string]any)
	if tx == nil || tx["flagged"] != true || tx["amount"] != 60000.0 {
		t.Fatalf("the transfer must be flagged: %v", launder)
	}
	emp = want(t, c.get("/api/v1/empire"), 200, "").data(t)
	if emp["dirty_money"].(float64) != dirty-60000 || emp["clean_money"].(float64) <= 0 {
		t.Errorf("money after laundering: %v", emp)
	}
	ledger := want(t, c.get("/api/v1/transactions"), 200, "Transaction ledger").data(t)
	if ledger["total"].(float64) < 1 {
		t.Errorf("the ledger is empty: %v", ledger)
	}

	// the DEA noticed
	threat := want(t, c.get("/api/v1/dea/threat"), 200, "Stay one step ahead of Hank").data(t)
	if threat["threat_score"].(float64) <= firstScore || threat["evidence_count"] != 1.0 || threat["lead_agent"] != "Hank Schrader" {
		t.Errorf("threat did not rise from %v: %v", firstScore, threat)
	}
	var history []map[string]any
	want(t, c.get("/api/v1/dea/history"), 200, "The DEA's paper trail").decode(t, &history)
	if len(history) < 2 || history[0]["is_active"] != true || history[0]["threat_score"] != threat["threat_score"] {
		t.Errorf("history: %v", history)
	}
	evidence := want(t, c.get("/api/v1/dea/evidence"), 200, "What the DEA has on you").data(t)
	if evidence["count"] != 1.0 || evidence["source"] != "local" {
		t.Errorf("evidence: %v", evidence)
	}

	// saul
	consult := want(t, c.post("/api/v1/legal/consult", map[string]string{"charge": "Possession with intent", "evidence": "Blue crystals in a trunk"}), 200, "Better call Saul").data(t)
	if consult["risk"] != "high" || consult["severity"] != 3.0 || consult["status"] != "open" {
		t.Errorf("consult: %v", consult)
	}
	caseID := strconv.Itoa(int(consult["id"].(float64)))
	want(t, c.get("/api/v1/legal/cases/"+caseID), 200, "The case file")
	cases := want(t, c.get("/api/v1/legal/cases"), 200, "Your case files").data(t)
	if cases["total"] != 1.0 {
		t.Errorf("cases: %v", cases)
	}

	// everything in one place
	var keys []string
	state := want(t, c.get("/api/v1/game/state"), 200, "Game state loaded").data(t)
	keys = keys[:0]
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "dea_threat,empire,legal_cases,rules" {
		t.Errorf("state keys: %v", keys)
	}
	if state["dea_threat"].(map[string]any)["threat_score"] != threat["threat_score"] {
		t.Error("the state and /dea/threat disagree")
	}
	if len(state["legal_cases"].([]any)) != 1 || len(state["empire"].(map[string]any)["front_businesses"].([]any)) != 1 {
		t.Errorf("state: %v", state)
	}
	if rules := state["rules"].(map[string]any); rules["optimal_effort"] != 8.33 || rules["next_territory_cost"] != 25000.0 {
		t.Errorf("rules: %v", rules)
	}

	// public statistics need no login
	anon := a.client(false)
	stats := want(t, anon.get("/api/v1/stats"), 200, "The empire in numbers")
	if stats.Header.Get("Cache-Control") != "public, max-age=30" {
		t.Errorf("stats Cache-Control: %q", stats.Header.Get("Cache-Control"))
	}
	// The numbers are cached for 30 seconds by the whole process, so an earlier test may have
	// filled the cache before this player existed: only the shape is checked here. The counting
	// itself is tested in the game package, where the clock can be controlled.
	s := stats.data(t)
	keys = keys[:0]
	for k, v := range s {
		keys = append(keys, k)
		if _, ok := v.(float64); !ok || v.(float64) < 0 {
			t.Errorf("stats %s: %v", k, v)
		}
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "dirty_laundered,empires,flagged_transfers,kilograms_cooked,raids" {
		t.Errorf("stats keys: %v", keys)
	}

	// logging out ends the session
	want(t, c.post("/api/v1/auth/logout", nil), 200, "Walk away")
	wantError(t, c.get("/api/v1/auth/me"), 401, "authentication required")
}

// ------------------------------------------------------------------- sessions and tokens

func TestTokensAreCheckedProperly(t *testing.T) {
	a := newAPI(t)
	c, p := a.newPlayer()
	id := uint(want(t, c.get("/api/v1/auth/me"), 200, "").data(t)["id"].(float64))
	secret := []byte(a.cfg.JWTSecret)
	anon := a.client(false)
	me := func(authorization string) result { return anon.get("/api/v1/auth/me", "Authorization", authorization) }

	// the real token works as a Bearer token, with any capitalisation of the scheme
	want(t, me("Bearer "+p.Token), 200, "Who are you?")
	want(t, me("bearer "+p.Token), 200, "Who are you?")

	good, _ := auth.IssueToken(secret, id, time.Hour, time.Now())
	otherSecret, _ := auth.IssueToken([]byte("another-secret-another-secret-123456"), id, time.Hour, time.Now())
	expired, _ := auth.IssueToken(secret, id, time.Hour, time.Now().Add(-2*time.Hour))
	parts := strings.Split(good, ".")
	segment := func(s string) []byte {
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var claims map[string]any
	if err := json.Unmarshal(segment(parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	claims["user_id"] = float64(id + 1)
	forgedClaims, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding.EncodeToString
	unsigned := enc([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + parts[1] + "."
	flipped := good[:len(good)-2] + map[bool]string{true: "AA", false: "BB"}[!strings.HasSuffix(good, "AA")]

	const invalid, absent = "invalid or expired token", "authentication required"
	for name, tc := range map[string]struct{ header, message string }{
		"signed with another secret": {"Bearer " + otherSecret, invalid},
		"expired":                    {"Bearer " + expired, invalid},
		"alg none, no signature":     {"Bearer " + unsigned, invalid},
		"payload swapped":            {"Bearer " + parts[0] + "." + enc(forgedClaims) + "." + parts[2], invalid},
		"signature damaged":          {"Bearer " + flipped, invalid},
		"garbage":                    {"Bearer not-a-token", invalid},
		"empty token":                {"Bearer ", absent},
		"wrong scheme":               {"Basic " + enc([]byte("a:b")), absent},
		"no scheme":                  {good, absent},
	} {
		r := me(tc.header)
		if r.Status != 401 || r.Env.Success || r.Env.Error != tc.message {
			t.Errorf("%s: want HTTP 401 %q, got HTTP %d %s", name, tc.message, r.Status, r.Raw)
		}
		if len(r.Header.Values("Set-Cookie")) != 0 {
			t.Errorf("%s: a rejected token must not set a cookie", name)
		}
	}

	// a token that is not in the Authorization header or cookie is not accepted
	wantError(t, anon.get("/api/v1/auth/me?token="+good), 401, "authentication required")

	// a perfectly signed token for a player that does not exist gets nothing
	ghost, _ := auth.IssueToken(secret, 999999999, time.Hour, time.Now())
	if r := me("Bearer " + ghost); r.Status == 200 || r.Status >= 500 {
		t.Errorf("a token for a missing player: HTTP %d %s", r.Status, r.Raw)
	}
}

func TestTheSessionCookie(t *testing.T) {
	for _, secure := range []bool{false, true} {
		a := newAPI(t, func(cfg *config.Config) { cfg.CookieSecure = secure })
		c := a.client(false)
		p := c.register(t)
		r := c.post("/api/v1/auth/login", map[string]string{"email": p.Email, "password": password})
		want(t, r, 200, "I am the one who knocks")

		cookies := (&http.Response{Header: r.Header}).Cookies()
		if len(cookies) != 1 {
			t.Fatalf("secure=%v: expected one cookie, got %v", secure, cookies)
		}
		ck := cookies[0]
		if ck.Name != auth.CookieName || ck.Value == "" || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode ||
			ck.Path != "/" || ck.Secure != secure || ck.MaxAge != int(a.cfg.JWTTTL.Seconds()) {
			t.Errorf("secure=%v: unexpected cookie %+v", secure, ck)
		}

		out := (&http.Response{Header: c.post("/api/v1/auth/logout", nil).Header}).Cookies()
		if len(out) != 1 || out[0].Name != auth.CookieName || out[0].Value != "" || out[0].MaxAge >= 0 || !out[0].HttpOnly || out[0].Secure != secure {
			t.Errorf("secure=%v: logout must clear the cookie the same way it was set: %+v", secure, out)
		}
	}
}

func TestLoginDoesNotRevealWhichPartWasWrong(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)
	p := c.register(t)

	wrongPassword := c.post("/api/v1/auth/login", map[string]string{"email": p.Email, "password": "not-the-password"})
	unknownEmail := c.post("/api/v1/auth/login", map[string]string{"email": "nobody@example.com", "password": password})
	if wrongPassword.Status != 401 || unknownEmail.Status != 401 || wrongPassword.Raw != unknownEmail.Raw {
		t.Errorf("the two failures must look identical:\n%d %s\n%d %s", wrongPassword.Status, wrongPassword.Raw, unknownEmail.Status, unknownEmail.Raw)
	}
	if len(wrongPassword.Header.Values("Set-Cookie")) != 0 {
		t.Error("a failed login sets no cookie")
	}
	dup := c.post("/api/v1/auth/register", map[string]string{"username": p.Username, "email": uniqueName("other") + "@example.com", "password": password})
	if dup.Status != 409 || dup.Env.Success {
		t.Errorf("a taken username: %d %s", dup.Status, dup.Raw)
	}
}

// ------------------------------------------------------- content type, origin and size

func TestOnlyJSONBodiesAreAccepted(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)
	login := `{"email":"nobody@example.com","password":"` + password + `"}`

	for name, contentType := range map[string]string{
		"plain text": "text/plain", "form": "application/x-www-form-urlencoded", "xml": "application/xml",
		"multipart": "multipart/form-data; boundary=x", "missing": "-",
	} {
		hdr := []string{"Content-Type", contentType}
		if contentType == "-" {
			hdr = []string{"Content-Type", ""}
		}
		r := c.post("/api/v1/auth/login", login, hdr...)
		if r.Status != 415 || r.Env.Error != "Content-Type must be application/json" {
			t.Errorf("%s: HTTP %d %s", name, r.Status, r.Raw)
		}
	}
	for _, contentType := range []string{"application/json", "application/json; charset=utf-8", "Application/JSON"} {
		if r := c.post("/api/v1/auth/login", login, "Content-Type", contentType); r.Status != 401 {
			t.Errorf("%q must be accepted (and then fail the login): HTTP %d %s", contentType, r.Status, r.Raw)
		}
	}
	// a request without a body needs no content type
	if r := c.do("POST", "/api/v1/auth/logout", nil); r.Status != 200 {
		t.Errorf("logout without a body: %d", r.Status)
	}
}

func TestForeignOriginsAreRefused(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)

	for _, origin := range []string{foreignOrigin, "null", "http://localhost:3000.evil.example", "https://localhost:3000"} {
		for _, route := range [][2]string{{"GET", "/api/v1/stats"}, {"GET", "/healthz"}, {"POST", "/api/v1/auth/login"}, {"DELETE", "/api/v1/empire/front/1"}} {
			var body any
			if route[0] != "GET" {
				body = `{}`
			}
			r := c.do(route[0], route[1], body, "Origin", origin)
			if r.Status != 403 || r.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Errorf("%s %s from %q: HTTP %d, ACAO %q", route[0], route[1], origin, r.Status, r.Header.Get("Access-Control-Allow-Origin"))
			}
		}
	}

	// the allowed origin may call, and is told so
	r := c.get("/api/v1/stats", "Origin", allowedOrigin)
	if r.Status != 200 || r.Header.Get("Access-Control-Allow-Origin") != allowedOrigin || r.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("allowed origin: HTTP %d, headers %v", r.Status, r.Header)
	}
	if r := c.post("/api/v1/auth/login", `{}`, "Origin", allowedOrigin); r.Status != 400 {
		t.Errorf("a POST from the allowed origin reaches the handler: %d", r.Status)
	}
	// requests without an Origin (mobile app, curl) are not browser requests
	if r := c.get("/api/v1/stats"); r.Status != 200 || r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("no origin: HTTP %d, ACAO %q", r.Status, r.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)

	r := c.do("OPTIONS", "/api/v1/auth/login", nil,
		"Origin", allowedOrigin, "Access-Control-Request-Method", "POST", "Access-Control-Request-Headers", "content-type")
	h := r.Header
	if r.Status != 204 || h.Get("Access-Control-Allow-Origin") != allowedOrigin || h.Get("Access-Control-Allow-Credentials") != "true" ||
		!strings.Contains(h.Get("Access-Control-Allow-Methods"), "POST") ||
		!strings.Contains(strings.ToLower(h.Get("Access-Control-Allow-Headers")), "content-type") {
		t.Errorf("allowed preflight: HTTP %d %v", r.Status, h)
	}
	for _, method := range []string{"POST", "DELETE", "GET"} {
		r := c.do("OPTIONS", "/api/v1/auth/login", nil, "Origin", foreignOrigin, "Access-Control-Request-Method", method)
		if r.Status != 403 || r.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("foreign preflight for %s: HTTP %d %v", method, r.Status, r.Header)
		}
	}
}

func TestOversizedBodiesAreRejected(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)

	huge := `{"username":"` + strings.Repeat("a", 2<<20) + `"}`
	r := c.post("/api/v1/auth/register", huge)
	if r.Status != 413 || r.Env.Success || !strings.Contains(r.Env.Error, "large") {
		t.Errorf("a 2 MiB body: HTTP %d %.200s", r.Status, r.Raw)
	}
	// just under the limit it is read (and then rejected for what it says)
	big := `{"username":"` + strings.Repeat("a", 900<<10) + `"}`
	if r := c.post("/api/v1/auth/register", big); r.Status != 400 {
		t.Errorf("a 900 KiB body is read and judged on its content: HTTP %d %.200s", r.Status, r.Raw)
	}
}

// ------------------------------------------------------------------------ rate limits

func TestAuthRoutesShareOneRateLimitButLogoutIsExempt(t *testing.T) {
	a := newAPI(t, func(cfg *config.Config) { cfg.RateAuthPerMin = 3 })
	c := a.client(false)
	bad := map[string]string{"email": "nobody@example.com", "password": password}

	for i, remaining := range []string{"2", "1", "0"} {
		var r result
		if i == 1 { // a registration counts against the same budget
			r = c.post("/api/v1/auth/register", `{}`)
		} else {
			r = c.post("/api/v1/auth/login", bad)
		}
		if r.Status == 429 || r.Header.Get("X-RateLimit-Limit") != "3" || r.Header.Get("X-RateLimit-Remaining") != remaining {
			t.Fatalf("request %d: HTTP %d, limit %q, remaining %q", i+1, r.Status, r.Header.Get("X-RateLimit-Limit"), r.Header.Get("X-RateLimit-Remaining"))
		}
	}
	limited := c.post("/api/v1/auth/login", bad)
	retry, err := strconv.Atoi(limited.Header.Get("Retry-After"))
	if limited.Status != 429 || limited.Env.Success || err != nil || retry < 1 || retry > 60 {
		t.Fatalf("the fourth request: HTTP %d, Retry-After %q, %s", limited.Status, limited.Header.Get("Retry-After"), limited.Raw)
	}
	// pretending to be someone else does not help: proxy headers are not trusted by default
	if r := c.post("/api/v1/auth/login", bad, "X-Forwarded-For", "203.0.113.9"); r.Status != 429 {
		t.Errorf("a forged X-Forwarded-For must not reset the limit: HTTP %d", r.Status)
	}
	// logging out is never blocked
	for i := 0; i < 5; i++ {
		if r := c.post("/api/v1/auth/logout", nil); r.Status != 200 {
			t.Fatalf("logout %d while rate limited: HTTP %d", i+1, r.Status)
		}
	}
}

func TestTheGeneralLimitCoversTheAPIButNotTheHealthCheck(t *testing.T) {
	a := newAPI(t, func(cfg *config.Config) { cfg.RateAPIPerMin = 5 })
	c := a.client(false)
	for i := 1; i <= 5; i++ {
		if r := c.get("/api/v1/stats"); r.Status != 200 {
			t.Fatalf("request %d: HTTP %d", i, r.Status)
		}
	}
	if r := c.get("/api/v1/stats"); r.Status != 429 || r.Header.Get("Retry-After") == "" {
		t.Errorf("the sixth request: HTTP %d", r.Status)
	}
	if r := c.get("/healthz"); r.Status != 200 {
		t.Errorf("the health check is not rate limited: HTTP %d", r.Status)
	}
}

func TestOnlyConsultationsAreLimitedByTheLegalLimit(t *testing.T) {
	a := newAPI(t, func(cfg *config.Config) { cfg.RateLegalPerMin = 2 })
	c, _ := a.newPlayer()
	for i := 1; i <= 2; i++ {
		want(t, c.post("/api/v1/legal/consult", map[string]string{"charge": "Fraud"}), 200, "Better call Saul")
	}
	if r := c.post("/api/v1/legal/consult", map[string]string{"charge": "Fraud"}); r.Status != 429 {
		t.Errorf("the third consultation: HTTP %d", r.Status)
	}
	for i := 0; i < 5; i++ {
		want(t, c.get("/api/v1/legal/cases"), 200, "Your case files")
	}
}

func TestZeroDisablesARateLimit(t *testing.T) {
	a := newAPI(t, func(cfg *config.Config) { cfg.RateAPIPerMin = 0 })
	c := a.client(false)
	for i := 0; i < 40; i++ {
		if r := c.get("/api/v1/stats"); r.Status != 200 {
			t.Fatalf("request %d: HTTP %d", i+1, r.Status)
		}
	}
}

// ---------------------------------------------------------------------------- health

type health struct{ Status, Database, ML string }

func getHealth(t *testing.T, c *client) (int, health, result) {
	t.Helper()
	r := c.get("/healthz")
	var h health
	if err := json.Unmarshal([]byte(r.Raw), &h); err != nil {
		t.Fatalf("healthz is not JSON: %s", r.Raw)
	}
	return r.Status, h, r
}

func TestHealthWithoutTheMLService(t *testing.T) {
	a := newAPI(t)
	status, h, r := getHealth(t, a.client(false))
	if status != 200 || h != (health{"ok", "up", "disabled"}) || r.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("HTTP %d %+v cache %q", status, h, r.Header.Get("Cache-Control"))
	}
}

func TestHealthWithTheMLServiceUpWrongKeyAndDown(t *testing.T) {
	up := newAPI(t, func(cfg *config.Config) {
		cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 200, `{}`), "ml-secret"
	})
	if status, h, _ := getHealth(t, up.client(false)); status != 200 || h != (health{"ok", "up", "up"}) {
		t.Errorf("ML up: HTTP %d %+v", status, h)
	}

	wrongKey := newAPI(t, func(cfg *config.Config) {
		cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 200, `{}`), "a-different-key"
	})
	if status, h, _ := getHealth(t, wrongKey.client(false)); status != 200 || h != (health{"degraded", "up", "down"}) {
		t.Errorf("ML rejects the key: HTTP %d %+v", status, h)
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	down := newAPI(t, func(cfg *config.Config) { cfg.MLServiceURL = deadURL })
	if status, h, _ := getHealth(t, down.client(false)); status != 200 || h != (health{"degraded", "up", "down"}) {
		t.Errorf("ML down: HTTP %d %+v", status, h)
	}
}

func TestHealthWhenTheDatabaseIsGone(t *testing.T) {
	a := newAPI(t)
	dsn, err := db.WithSchema(testDBRaw, "server_test")
	if err != nil {
		t.Fatal(err)
	}
	dead, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := dead.DB()
	_ = sqlDB.Close()
	live := db.DB
	db.DB = dead
	t.Cleanup(func() { db.DB = live })

	status, h, _ := getHealth(t, a.client(false))
	if status != 503 || h.Status != "down" || h.Database != "down" {
		t.Errorf("HTTP %d %+v", status, h)
	}
}

// ---------------------------------------------------------- unknown routes, headers

func TestUnknownRoutesAndMethods(t *testing.T) {
	a := newAPI(t)
	c := a.client(false)

	for _, path := range []string{"/nope", "/api/v1/nope", "/api/v2/stats", "/api/v1/empire/cook/extra", "/"} {
		r := c.get(path)
		if r.Status != 404 || r.Env.Success || r.Env.Error != "route not found" {
			t.Errorf("GET %s: HTTP %d %s", path, r.Status, r.Raw)
		}
	}
	for _, tc := range [][2]string{
		{"POST", "/api/v1/auth/me"}, {"PUT", "/api/v1/empire/cook"}, {"DELETE", "/api/v1/stats"},
		{"POST", "/healthz"}, {"GET", "/api/v1/auth/login"}, {"PATCH", "/api/v1/legal/cases"},
	} {
		var body any
		if tc[0] != "GET" {
			body = `{}`
		}
		r := c.do(tc[0], tc[1], body)
		if r.Status != 405 || r.Env.Success || r.Env.Error != "method not allowed" || r.Header.Get("Allow") == "" {
			t.Errorf("%s %s: HTTP %d %s (Allow %q)", tc[0], tc[1], r.Status, r.Raw, r.Header.Get("Allow"))
		}
	}
}

func TestSecurityHeadersAreOnEveryKindOfResponse(t *testing.T) {
	a := newAPI(t, func(cfg *config.Config) { cfg.RateAPIPerMin = 6 })
	c := a.client(false)
	responses := map[string]result{
		"200":  c.get("/healthz"),
		"401":  c.get("/api/v1/auth/me"),
		"404":  c.get("/nope"),
		"405":  c.do("POST", "/healthz", `{}`),
		"415":  c.post("/api/v1/auth/login", "x", "Content-Type", "text/plain"),
		"400":  c.post("/api/v1/auth/login", `{}`),
		"413":  c.post("/api/v1/auth/register", `{"username":"`+strings.Repeat("a", 2<<20)+`"}`),
		"stat": c.get("/api/v1/stats"),
	}
	for i := 0; i < 6; i++ { // the general limit is 6 a minute
		responses["429"] = c.get("/api/v1/stats")
	}
	for name, r := range responses {
		h := r.Header
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" ||
			h.Get("Referrer-Policy") != "no-referrer" || !strings.Contains(h.Get("Content-Security-Policy"), "default-src 'none'") {
			t.Errorf("%s (HTTP %d): missing security headers: %v", name, r.Status, h)
		}
		wantCache := "no-store"
		if name == "stat" {
			wantCache = "public, max-age=30"
		}
		if h.Get("Cache-Control") != wantCache {
			t.Errorf("%s (HTTP %d): Cache-Control %q, want %q", name, r.Status, h.Get("Cache-Control"), wantCache)
		}
	}
	if responses["429"].Status != 429 || responses["413"].Status != 413 || responses["415"].Status != 415 {
		t.Errorf("the scenarios did not produce what they should: %d %d %d", responses["429"].Status, responses["413"].Status, responses["415"].Status)
	}
}

// ------------------------------------------------------------------------ cross-cutting

func TestPlayersCannotSeeOrTouchEachOther(t *testing.T) {
	a := newAPI(t)
	alice, _ := a.newPlayer()
	bob, _ := a.newPlayer()

	for i := 0; i < 6; i++ {
		want(t, alice.post("/api/v1/empire/cook", map[string]any{"effort": 8}), 200, "")
	}
	front := want(t, alice.post("/api/v1/empire/front", map[string]string{"name": "Alice's Bar", "type": "bar"}), 201, "").data(t)
	want(t, alice.post("/api/v1/launder", map[string]any{"amount": 60000, "business_id": front["id"]}), 200, "")
	consult := want(t, alice.post("/api/v1/legal/consult", map[string]string{"charge": "Fraud"}), 200, "").data(t)
	frontID := strconv.Itoa(int(front["id"].(float64)))
	caseID := strconv.Itoa(int(consult["id"].(float64)))

	emp := want(t, bob.get("/api/v1/empire"), 200, "").data(t)
	if emp["dirty_money"] != 0.0 || emp["clean_money"] != 0.0 {
		t.Errorf("bob has alice's money: %v", emp)
	}
	if ledger := want(t, bob.get("/api/v1/transactions"), 200, "").data(t); ledger["total"] != 0.0 {
		t.Errorf("bob sees alice's ledger: %v", ledger)
	}
	if cases := want(t, bob.get("/api/v1/legal/cases"), 200, "").data(t); cases["total"] != 0.0 {
		t.Errorf("bob sees alice's cases: %v", cases)
	}
	wantError(t, bob.get("/api/v1/legal/cases/"+caseID), 404, "legal case not found")
	if r := bob.do("DELETE", "/api/v1/empire/front/"+frontID, nil); r.Status != 404 {
		t.Errorf("bob closing alice's front: HTTP %d %s", r.Status, r.Raw)
	}
	if r := bob.post("/api/v1/launder", map[string]any{"amount": 1000, "business_id": front["id"]}); r.Status < 400 || r.Status >= 500 {
		t.Errorf("bob laundering through alice's front: HTTP %d %s", r.Status, r.Raw)
	}
	state := want(t, bob.get("/api/v1/game/state"), 200, "").data(t)
	if len(state["legal_cases"].([]any)) != 0 || len(state["empire"].(map[string]any)["front_businesses"].([]any)) != 0 {
		t.Errorf("bob's state shows alice's things: %v", state)
	}
	// and alice's front is still open
	fronts := want(t, alice.get("/api/v1/empire/fronts"), 200, "").data
	_ = fronts
	var open []map[string]any
	want(t, alice.get("/api/v1/empire/fronts"), 200, "").decode(t, &open)
	if len(open) != 1 {
		t.Errorf("alice's fronts: %v", open)
	}
}

func TestMistakesAreAnsweredWithReadableErrors(t *testing.T) {
	a := newAPI(t)
	c, _ := a.newPlayer()

	for name, r := range map[string]result{
		"effort too low":      c.post("/api/v1/empire/cook", map[string]any{"effort": 0}),
		"effort too high":     c.post("/api/v1/empire/cook", map[string]any{"effort": 11}),
		"effort not a number": c.post("/api/v1/empire/cook", map[string]any{"effort": "max"}),
		"no effort":           c.post("/api/v1/empire/cook", `{}`),
		"empty body":          c.post("/api/v1/empire/cook", ""),
		"broken JSON":         c.post("/api/v1/empire/cook", `{"effort":`),
		"unknown front type":  c.post("/api/v1/empire/front", map[string]string{"name": "X", "type": "spaceship"}),
		"front without name":  c.post("/api/v1/empire/front", map[string]string{"type": "bar"}),
		"launder nothing":     c.post("/api/v1/launder", map[string]any{"amount": 0, "business_id": 1}),
		"launder negative":    c.post("/api/v1/launder", map[string]any{"amount": -5, "business_id": 1}),
		"close a bad id":      c.do("DELETE", "/api/v1/empire/front/abc", nil),
		"saul without charge": c.post("/api/v1/legal/consult", `{}`),
		"ledger bad limit":    c.get("/api/v1/transactions?limit=0"),
		"dea bad limit":       c.get("/api/v1/dea/history?limit=999"),
	} {
		if r.Status != 400 || r.Env.Success || r.Env.Error == "" {
			t.Errorf("%s: HTTP %d %s", name, r.Status, r.Raw)
		}
	}
	wantError(t, c.post("/api/v1/empire/territory", `{}`), 400, "insufficient clean money: territory 1 costs $25000")
	if r := c.do("DELETE", "/api/v1/empire/front/999999", nil); r.Status != 404 {
		t.Errorf("closing a front that does not exist: HTTP %d %s", r.Status, r.Raw)
	}
	if r := c.post("/api/v1/launder", map[string]any{"amount": 1000, "business_id": 999999}); r.Status < 400 || r.Status >= 500 {
		t.Errorf("laundering through a front that does not exist: HTTP %d %s", r.Status, r.Raw)
	}
	for _, r := range []result{c.get("/api/v1/empire"), c.get("/api/v1/game/state")} {
		if r.Status != 200 {
			t.Errorf("the mistakes above must leave the account intact: HTTP %d", r.Status)
		}
	}
}

// ----------------------------------------------------------------------- the ML wiring

func TestTheMLServiceIsUsedWhenConfiguredAndBypassedWhenBroken(t *testing.T) {
	threat := `{"threat_score":42.5,"risk_level":"medium","prediction":"The ML service says so","recommendation":"Listen to it"}`

	working := newAPI(t, func(cfg *config.Config) {
		cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 200, threat), "ml-secret"
	})
	c, _ := working.newPlayer()
	got := want(t, c.get("/api/v1/dea/threat"), 200, "").data(t)
	if got["source"] != "ml" || got["threat_score"] != 42.5 || got["prediction"] != "The ML service says so" {
		t.Errorf("the ML verdict must reach the client: %v", got)
	}

	for name, tweak := range map[string]func(*config.Config){
		"ML fails": func(cfg *config.Config) {
			cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 500, `{}`), "ml-secret"
		},
		"ML sends junk": func(cfg *config.Config) {
			cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 200, `{"threat_score":900}`), "ml-secret"
		},
		"wrong ML key": func(cfg *config.Config) { cfg.MLServiceURL, cfg.MLAPIKey = fakeML(t, "ml-secret", 200, threat), "oops" },
	} {
		broken := newAPI(t, tweak)
		c, _ := broken.newPlayer()
		got := want(t, c.get("/api/v1/dea/threat"), 200, "").data(t)
		if got["source"] != "heuristic" || got["threat_score"].(float64) <= 0 || got["threat_score"].(float64) > 100 {
			t.Errorf("%s: the player must still get a verdict: %v", name, got)
		}
		want(t, c.get("/api/v1/game/state"), 200, "Game state loaded")
		want(t, c.get("/api/v1/empire/advice"), 200, "Advice from the lab")
	}
}
