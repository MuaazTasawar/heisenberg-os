package empire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
)

var (
	testDBOnce sync.Once
	testDBErr  error
)

// openTestDB connects once per test run to TEST_DATABASE_URL in a private schema (one shared
// pool, so many tests cannot exhaust the server's connections) and skips when it is not set.
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
		if err := db.EnsureSchema(boot, "empire_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "empire_test")
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

func unique(prefix string) string { return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()) }

func newUser(t *testing.T) uint {
	t.Helper()
	name := unique("walt")
	user := models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", Alias: "Walter"}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func reload(t *testing.T, userID uint) models.Empire {
	t.Helper()
	var emp models.Empire
	if err := db.DB.Where("user_id = ?", userID).First(&emp).Error; err != nil {
		t.Fatal(err)
	}
	return emp
}

func set(t *testing.T, userID uint, updates map[string]any) {
	t.Helper()
	if _, err := Get(userID); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Model(&models.Empire{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------- the empire

func TestFirstAccessCreatesTheEmpireOnce(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	emp, err := Get(userID)
	if err != nil {
		t.Fatal(err)
	}
	if emp.ID == 0 || emp.Name != "Walter's Empire" || emp.PurityRating != 50 || emp.BatchYield != 10 ||
		emp.DirtyMoney != 0 || emp.CleanMoney != 0 || emp.HeatLevel != 0 || emp.TerritoryCount != 0 {
		t.Errorf("unexpected new empire: %+v", emp)
	}
	if emp.FrontBusinesses == nil || len(emp.FrontBusinesses) != 0 {
		t.Errorf("fronts must be an empty (non-nil) list, got %#v", emp.FrontBusinesses)
	}

	again, err := Get(userID)
	if err != nil || again.ID != emp.ID {
		t.Errorf("a second read must return the same empire: %v %v", again, err)
	}
	if old, _ := GetOrCreate(userID, "ignored"); old.ID != emp.ID {
		t.Error("GetOrCreate must keep working for existing callers")
	}
	if GetEmpireID(userID) != emp.ID {
		t.Error("GetEmpireID must keep working for existing callers")
	}
	if GetEmpireID(987654321) != 0 {
		t.Error("GetEmpireID of a missing user is 0")
	}
}

func TestEveryPlayerHasTheirOwnEmpire(t *testing.T) {
	openTestDB(t)
	a, b := newUser(t), newUser(t)
	if _, err := Cook(a, 5); err != nil {
		t.Fatal(err)
	}
	empB, err := Get(b)
	if err != nil {
		t.Fatal(err)
	}
	if empB.DirtyMoney != 0 || empB.BatchYield != 10 {
		t.Errorf("cooking for A must not change B: %+v", empB)
	}
}

// ------------------------------------------------------------------- cooking

func TestCookAppliesTheRulesAndPersists(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	res, err := Cook(userID, 5)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "yield gained", res.YieldGained, 8.75, 1e-9)
	near(t, "dirty gained", res.DirtyGained, 13125, 1e-9)
	near(t, "heat added", res.HeatAdded, 12.5, 1e-9)
	near(t, "purity delta", res.PurityDelta, 0.6, 1e-9)
	near(t, "empire batch yield", res.Empire.BatchYield, 18.75, 1e-9)
	near(t, "empire purity", res.Empire.PurityRating, 50.6, 1e-9)

	stored := reload(t, userID)
	near(t, "stored dirty money", stored.DirtyMoney, 13125, 1e-9)
	near(t, "stored heat", stored.HeatLevel, 12.5, 0.01)
	near(t, "stored purity", stored.PurityRating, 50.6, 1e-9)
	if stored.HeatUpdatedAt == nil || time.Since(*stored.HeatUpdatedAt) > time.Minute {
		t.Errorf("the heat clock must be stamped: %v", stored.HeatUpdatedAt)
	}

	second, err := Cook(userID, 5) // higher purity now yields a little more
	if err != nil {
		t.Fatal(err)
	}
	near(t, "second yield", second.YieldGained, 8.78, 1e-9)
}

func TestCookRejectsBadEffortWithoutChangingAnything(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	before, _ := Get(userID)

	for _, bad := range []float64{0, 0.5, 10.5, -2, math.NaN(), math.Inf(1)} {
		if _, err := Cook(userID, bad); !errors.Is(err, ErrInvalidEffort) {
			t.Errorf("effort %v: got %v, want ErrInvalidEffort", bad, err)
		}
	}
	after := reload(t, userID)
	if after.DirtyMoney != before.DirtyMoney || after.BatchYield != before.BatchYield || after.HeatLevel != before.HeatLevel {
		t.Errorf("a rejected cook must change nothing: %+v -> %+v", before, after)
	}
}

func TestCookKeepsHeatAndPurityInRange(t *testing.T) {
	openTestDB(t)

	hot := newUser(t)
	set(t, hot, map[string]any{"heat_level": 95.0})
	res, err := Cook(hot, 10)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "heat is capped at 100", res.Empire.HeatLevel, 100, 0.01)
	near(t, "only the room that was left counts as added", res.HeatAdded, 5, 0.01)

	low := newUser(t)
	set(t, low, map[string]any{"purity_rating": 20.1})
	res, _ = Cook(low, 10)
	near(t, "purity floor", res.Empire.PurityRating, MinPurity, 1e-9)

	high := newUser(t)
	set(t, high, map[string]any{"purity_rating": 99.0})
	res, _ = Cook(high, 1)
	near(t, "purity ceiling", res.Empire.PurityRating, MaxPurity, 1e-9)
}

func TestHeatCoolsDownWhileYouWait(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	set(t, userID, map[string]any{"heat_level": 50.0, "heat_updated_at": twoHoursAgo})

	emp, err := Get(userID)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "heat as shown after 2 hours", emp.HeatLevel, 38, 0.05)
	near(t, "reading must not write", reload(t, userID).HeatLevel, 50, 1e-9)

	res, err := Cook(userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "cooked on top of the cooled heat", res.Empire.HeatLevel, 38+2.5, 0.05)
	near(t, "stored", reload(t, userID).HeatLevel, 40.5, 0.05)

	frozen := newUser(t)
	set(t, frozen, map[string]any{"heat_level": 10.0, "heat_updated_at": time.Now().Add(-48 * time.Hour)})
	if emp, _ := Get(frozen); emp.HeatLevel != 0 {
		t.Errorf("heat never drops below zero, got %v", emp.HeatLevel)
	}
}

// If cooking did not lock the empire row, concurrent requests would read the same balance and
// overwrite each other, and money and product would go missing.
func TestConcurrentCooksDoNotLoseUpdates(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	if _, err := Get(userID); err != nil {
		t.Fatal(err)
	}

	const cooks = 20
	var wg sync.WaitGroup
	errs := make(chan error, cooks)
	for i := 0; i < cooks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Cook(userID, 1); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a cook failed: %v", err)
	}

	// The purity after each of the 20 serialised cooks is the same whatever the order was.
	purity, yield, dirty := 50.0, 10.0, 0.0
	for i := 0; i < cooks; i++ {
		o := CookOutcome(1, purity, 0)
		yield = round2(yield + o.Yield)
		dirty = round2(dirty + o.Dirty)
		purity = round2(PurityAfter(purity, 1))
	}
	got := reload(t, userID)
	near(t, "purity", got.PurityRating, purity, 1e-9)
	near(t, "batch yield", got.BatchYield, yield, 1e-6)
	near(t, "dirty money", got.DirtyMoney, dirty, 1e-6)
	near(t, "heat", got.HeatLevel, float64(cooks)*HeatPerEffort, 0.05)
}

func TestConcurrentFirstAccessCreatesExactlyOneEmpire(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = Get(userID) }()
	}
	wg.Wait()

	var count int64
	db.DB.Model(&models.Empire{}).Where("user_id = ?", userID).Count(&count)
	if count != 1 {
		t.Errorf("expected exactly one empire, found %d", count)
	}
}

// ---------------------------------------------------------------- territories

func TestTerritoriesCostCleanMoneyAndAddHeat(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	if _, err := Get(userID); err != nil { // a failed first-ever action would roll the empire back too
		t.Fatal(err)
	}

	if _, err := ExpandTerritory(userID); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("no money: got %v", err)
	}
	near(t, "nothing changes on failure", float64(reload(t, userID).TerritoryCount), 0, 0)

	set(t, userID, map[string]any{"clean_money": 30000.0})
	emp, err := ExpandTerritory(userID)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "territories", float64(emp.TerritoryCount), 1, 0)
	near(t, "clean money left", emp.CleanMoney, 5000, 1e-9)
	near(t, "heat from expanding", emp.HeatLevel, TerritoryHeat, 0.01)

	if _, err := ExpandTerritory(userID); !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("the second territory costs $50,000: got %v", err)
	}

	cook, _ := Cook(userID, 5)
	near(t, "one territory is +5% yield", cook.YieldGained, round2(8.75*1.05), 0.01)

	set(t, userID, map[string]any{"territory_count": MaxTerritories, "clean_money": 1e9})
	if _, err := ExpandTerritory(userID); !errors.Is(err, ErrMaxTerritories) {
		t.Errorf("at the maximum: got %v", err)
	}
}

// --------------------------------------------------------------------- fronts

func TestFrontBusinessRules(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	fb, err := AddFront(userID, "  Los Pollos   Hermanos ", " Restaurant ")
	if err != nil {
		t.Fatal(err)
	}
	if fb.ID == 0 || fb.Name != "Los Pollos Hermanos" || fb.Type != "restaurant" || !fb.IsActive || fb.Suspicion != 0 || fb.Revenue != 0 {
		t.Errorf("unexpected front: %+v", fb)
	}

	if _, err := AddFront(userID, "los pollos hermanos", "bar"); !errors.Is(err, ErrDuplicateFront) {
		t.Errorf("same name (any case): got %v", err)
	}
	if _, err := AddFront(userID, "Casino Royale", "casino"); !errors.Is(err, ErrInvalidFront) {
		t.Errorf("unknown type: got %v", err)
	}
	for _, name := range []string{"", "x", strings.Repeat("y", 41), "bad\x01name"} {
		if _, err := AddFront(userID, name, "bar"); !errors.Is(err, ErrInvalidFront) {
			t.Errorf("name %q: got %v", name, err)
		}
	}

	for i := 2; i <= MaxActiveFronts; i++ {
		if _, err := AddFront(userID, fmt.Sprintf("Front number %d", i), "laundromat"); err != nil {
			t.Fatalf("front %d: %v", i, err)
		}
	}
	if _, err := AddFront(userID, "One too many", "bar"); !errors.Is(err, ErrFrontLimit) {
		t.Errorf("limit: got %v", err)
	}

	closed, err := CloseFront(userID, fb.ID)
	if err != nil || closed.IsActive || closed.ShutdownReason != "closed by owner" {
		t.Errorf("close: %+v %v", closed, err)
	}
	if _, err := CloseFront(userID, fb.ID); !errors.Is(err, ErrFrontClosed) {
		t.Errorf("closing twice: got %v", err)
	}
	if _, err := AddFront(userID, "Los Pollos Hermanos", "bar"); err != nil {
		t.Errorf("closing a front frees a slot, and its name can be reused: %v", err)
	}
	if _, err := AddFront(userID, "Fresh start", "bar"); !errors.Is(err, ErrFrontLimit) {
		t.Errorf("the slot is taken again: got %v", err)
	}

	fronts, err := ListFronts(userID)
	if err != nil || len(fronts) != MaxActiveFronts+1 {
		t.Errorf("the list should include the closed front: %d %v", len(fronts), err)
	}
}

func TestPlayersCannotTouchEachOthersFronts(t *testing.T) {
	openTestDB(t)
	owner, intruder := newUser(t), newUser(t)
	fb, err := AddFront(owner, "Private Business", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CloseFront(intruder, fb.ID); !errors.Is(err, ErrFrontNotFound) {
		t.Errorf("closing someone else's front: got %v", err)
	}
	if _, err := CloseFront(owner, 999999999); !errors.Is(err, ErrFrontNotFound) {
		t.Errorf("unknown id: got %v", err)
	}
	if list, _ := ListFronts(intruder); len(list) != 0 {
		t.Errorf("the intruder must not see the owner's fronts: %v", list)
	}
}

func TestConcurrentFrontsRespectTheLimit(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = AddFront(userID, fmt.Sprintf("Racer %d", i), "bar")
		}(i)
	}
	wg.Wait()
	var active int64
	emp := reload(t, userID)
	db.DB.Model(&models.FrontBusiness{}).Where("empire_id = ? AND is_active = ?", emp.ID, true).Count(&active)
	if active != MaxActiveFronts {
		t.Errorf("the limit of %d must hold under concurrency, found %d", MaxActiveFronts, active)
	}
}

// --------------------------------------------------------------------- advice

func mlServer(t *testing.T, status int, body string, seen *mlclient.YieldInput) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			_ = json.NewDecoder(r.Body).Decode(seen)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func useML(t *testing.T, url string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.MLServiceURL = url
	cfg.MLTimeout = time.Second
	mlclient.SetDefault(mlclient.New(cfg))
	t.Cleanup(func() { mlclient.SetDefault(nil) })
}

func TestAdviceUsesTheMLServiceWhenItAnswers(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	set(t, userID, map[string]any{"territory_count": 4, "purity_rating": 61.0})

	var sent mlclient.YieldInput
	srv := mlServer(t, 200, `{"predicted_yield":9.9,"optimal_effort":83.33,"optimal_yield":12.5,"purity_projection":61.4,"efficiency_score":79.2}`, &sent)
	defer srv.Close()
	useML(t, srv.URL)

	advice, err := GetAdvice(context.Background(), userID, 6)
	if err != nil {
		t.Fatal(err)
	}
	if advice.Source != "ml" || advice.PredictedYield != 9.9 || advice.OptimalEffort != 8.33 ||
		advice.OptimalYield != 12.5 || advice.PurityProjection != 61.4 || advice.EfficiencyScore != 79.2 || advice.Effort != 6 {
		t.Errorf("ML answer not mapped: %+v", advice)
	}
	if sent.EffortLevel != 60 || sent.CurrentPurity != 61 || sent.SupplyQuality != 100 || sent.LabCapacity != Capacity(4) {
		t.Errorf("wrong request sent to the ML service: %+v", sent)
	}
}

func TestAdviceFallsBackToLocalRules(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	cases := map[string]*httptest.Server{
		"ML is down":        mlServer(t, 500, `boom`, nil),
		"ML rejects us":     mlServer(t, 401, `{"detail":"nope"}`, nil),
		"ML sends nonsense": mlServer(t, 200, `{"predicted_yield":1,"optimal_effort":9000,"optimal_yield":1,"purity_projection":50,"efficiency_score":50}`, nil),
		"ML sends garbage":  mlServer(t, 200, `<html>`, nil),
	}
	for name, srv := range cases {
		useML(t, srv.URL)
		advice, err := GetAdvice(context.Background(), userID, 5)
		srv.Close()
		if err != nil || advice.Source != "local" || advice.OptimalEffort != 8.33 {
			t.Errorf("%s: %+v %v", name, advice, err)
		}
	}

	mlclient.SetDefault(nil) // disabled
	if advice, err := GetAdvice(context.Background(), userID, 5); err != nil || advice.Source != "local" {
		t.Errorf("ML disabled: %+v %v", advice, err)
	}
	if _, err := GetAdvice(context.Background(), userID, 11); !errors.Is(err, ErrInvalidEffort) {
		t.Errorf("bad effort: %v", err)
	}
}

// ------------------------------------------------------------ HTTP handlers

func router(userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/api/v1", func(c *gin.Context) { c.Set(auth.UserIDKey, userID); c.Next() })
	RegisterRoutes(protected)
	return r
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Data    json.RawMessage `json:"data"`
}

func call(t *testing.T, r http.Handler, method, path, body string) (int, envelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("%s %s: not JSON: %v (%s)", method, path, err, w.Body.String())
	}
	return w.Code, e
}

func TestCookEndpointValidation(t *testing.T) {
	openTestDB(t)
	r := router(newUser(t))
	cases := []struct {
		body, want string
		status     int
	}{
		{`{}`, "effort is required", 400},
		{`{"effort":0}`, "effort is required", 400},
		{`{"effort":0.5}`, "effort must be at least 1", 400},
		{`{"effort":11}`, "effort must be at most 10", 400},
		{`{"effort":-4}`, "effort must be at least 1", 400},
		{`{"effort":"max"}`, "effort must be a number", 400},
		{`{"effort":`, "request body is not valid JSON", 400},
	}
	for _, tc := range cases {
		status, e := call(t, r, "POST", "/api/v1/empire/cook", tc.body)
		if status != tc.status || e.Error != tc.want || e.Success {
			t.Errorf("%s -> %d %q, want %d %q", tc.body, status, e.Error, tc.status, tc.want)
		}
	}
}

func TestEmpireEndpointsEndToEnd(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	r := router(userID)

	status, e := call(t, r, "GET", "/api/v1/empire", "")
	if status != 200 || e.Message != "Empire retrieved" || !strings.Contains(string(e.Data), `"front_businesses":[]`) ||
		!strings.Contains(string(e.Data), `"purity_rating":50`) || strings.Contains(string(e.Data), `"ID"`) {
		t.Errorf("GET /empire: %d %s", status, e.Data)
	}

	status, e = call(t, r, "POST", "/api/v1/empire/cook", `{"effort":5}`)
	var cook CookResult
	_ = json.Unmarshal(e.Data, &cook)
	if status != 200 || e.Message != "Batch cooked" || cook.YieldGained != 8.75 || cook.DirtyGained != 13125 ||
		cook.HeatAdded != 12.5 || cook.Empire == nil || cook.Empire.DirtyMoney != 13125 {
		t.Errorf("cook: %d %s", status, e.Data)
	}

	status, e = call(t, r, "POST", "/api/v1/empire/territory", "")
	if status != 400 || !strings.Contains(e.Error, "insufficient clean money") {
		t.Errorf("territory without money: %d %q", status, e.Error)
	}

	status, e = call(t, r, "GET", "/api/v1/empire/advice?effort=7", "")
	var advice Advice
	_ = json.Unmarshal(e.Data, &advice)
	if status != 200 || advice.Source != "local" || advice.Effort != 7 || advice.OptimalEffort != 8.33 {
		t.Errorf("advice: %d %s", status, e.Data)
	}
	if status, _ = call(t, r, "GET", "/api/v1/empire/advice", ""); status != 200 {
		t.Errorf("advice without an effort defaults to 5: %d", status)
	}
	for _, q := range []string{"?effort=abc", "?effort=0", "?effort=11"} {
		if status, e = call(t, r, "GET", "/api/v1/empire/advice"+q, ""); status != 400 || e.Error != ErrInvalidEffort.Error() {
			t.Errorf("advice%s: %d %q", q, status, e.Error)
		}
	}
}

func TestFrontEndpoints(t *testing.T) {
	openTestDB(t)
	r := router(newUser(t))

	status, e := call(t, r, "GET", "/api/v1/empire/front-types", "")
	var types []FrontType
	_ = json.Unmarshal(e.Data, &types)
	if status != 200 || len(types) != 6 || types[0].Type == "" {
		t.Errorf("front types: %d %s", status, e.Data)
	}

	for _, tc := range []struct {
		body, want string
		status     int
	}{
		{`{}`, "name is required; type is required", 400},
		{`{"name":"Pollos"}`, "type is required", 400},
		{`{"name":"Pollos","type":"casino"}`, "unknown type", 400},
		{`{"name":"x","type":"bar"}`, "the name must be", 400},
	} {
		status, e := call(t, r, "POST", "/api/v1/empire/front", tc.body)
		if status != tc.status || !strings.Contains(e.Error, tc.want) {
			t.Errorf("%s -> %d %q, want %d containing %q", tc.body, status, e.Error, tc.status, tc.want)
		}
	}

	status, e = call(t, r, "POST", "/api/v1/empire/front", `{"name":"Los Pollos","type":"restaurant"}`)
	var created struct {
		ID       uint   `json:"id"`
		EmpireID uint   `json:"empire_id"`
		IsActive bool   `json:"is_active"`
		Type     string `json:"type"`
	}
	_ = json.Unmarshal(e.Data, &created)
	if status != 201 || created.ID == 0 || !created.IsActive || created.Type != "restaurant" || !strings.Contains(e.Message, "keep it clean") {
		t.Fatalf("create: %d %s", status, e.Data)
	}
	if status, _ = call(t, r, "POST", "/api/v1/empire/front", `{"name":"LOS POLLOS","type":"bar"}`); status != 409 {
		t.Errorf("duplicate name: %d", status)
	}

	_, e = call(t, r, "GET", "/api/v1/empire/fronts", "")
	if !strings.Contains(string(e.Data), "Los Pollos") {
		t.Errorf("list: %s", e.Data)
	}

	for path, want := range map[string]int{
		"/api/v1/empire/front/abc":       400,
		"/api/v1/empire/front/0":         400,
		"/api/v1/empire/front/999999999": 404,
	} {
		if status, _ = call(t, r, "DELETE", path, ""); status != want {
			t.Errorf("DELETE %s: %d, want %d", path, status, want)
		}
	}
	if status, e = call(t, r, "DELETE", fmt.Sprintf("/api/v1/empire/front/%d", created.ID), ""); status != 200 || e.Message != "Front business closed" {
		t.Errorf("close: %d %s", status, e.Data)
	}
	if status, _ = call(t, r, "DELETE", fmt.Sprintf("/api/v1/empire/front/%d", created.ID), ""); status != 409 {
		t.Errorf("closing twice: %d", status)
	}
}
