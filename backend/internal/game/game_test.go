package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heisenberg-os/backend/internal/auth"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
)

var (
	testDBOnce sync.Once
	testDBErr  error
)

// openTestDB connects once per test run to TEST_DATABASE_URL in a private schema (one shared
// pool) and skips when it is not set. The ML client is switched off.
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
		if err := db.EnsureSchema(boot, "game_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "game_test")
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
	mlclient.SetDefault(nil)
	t.Cleanup(func() { mlclient.SetDefault(nil) })
}

func unique(prefix string) string { return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()) }

func newUser(t *testing.T) uint {
	t.Helper()
	name := unique("jesse")
	user := models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", Alias: "Pinkman"}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func setEmpire(t *testing.T, userID uint, updates map[string]any) models.Empire {
	t.Helper()
	if _, err := empire.Get(userID); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Model(&models.Empire{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}
	var emp models.Empire
	if err := db.DB.Where("user_id = ?", userID).First(&emp).Error; err != nil {
		t.Fatal(err)
	}
	return emp
}

func addTx(t *testing.T, empireID uint, typ string, amount float64, flagged bool) {
	t.Helper()
	tx := models.Transaction{EmpireID: empireID, Amount: amount, Type: typ, Description: "test", Flagged: flagged}
	if err := db.DB.Create(&tx).Error; err != nil {
		t.Fatal(err)
	}
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if d := got - want; d > 0.005 || d < -0.005 {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

// ------------------------------------------------------------------------- rules

func TestRulesExposeTheEngineConstants(t *testing.T) {
	r := CurrentRules(0)
	if r.MinEffort != 1 || r.MaxEffort != 10 || r.OptimalEffort != 8.33 || r.DirtyPerKg != 1500 ||
		r.HeatPerEffort != 2.5 || r.HeatDecayPerHour != 6 || r.LaunderRate != 0.75 || r.LaunderMin != 100 ||
		r.LargeTransfer != 50000 || r.TerritoryBonus != 0.05 || r.MaxTerritories != 10 || r.MaxActiveFronts != 5 {
		t.Errorf("unexpected rules: %+v", r)
	}
	for territories, want := range map[int]float64{0: 25000, 1: 50000, 3: 100000, 9: 250000} {
		got := CurrentRules(territories).NextTerritoryCost
		if got == nil || *got != want {
			t.Errorf("territories %d: next cost %v, want %v", territories, got, want)
		}
	}
	if CurrentRules(10).NextTerritoryCost != nil || CurrentRules(11).NextTerritoryCost != nil {
		t.Error("no next territory at the maximum")
	}
	raw, _ := json.Marshal(CurrentRules(10))
	if !strings.Contains(string(raw), `"next_territory_cost":null`) {
		t.Errorf("the maximum is reported as null, not omitted or zero: %s", raw)
	}
}

// ------------------------------------------------------------------ the game state

func TestStateForANewPlayer(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	st, err := GetState(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Empire == nil || st.Empire.UserID != userID || st.Empire.FrontBusinesses == nil || len(st.Empire.FrontBusinesses) != 0 {
		t.Errorf("empire with an empty (not null) front list expected: %+v", st.Empire)
	}
	if st.DEAThreat == nil || st.DEAThreat.ID == 0 || !st.DEAThreat.IsActive || st.DEAThreat.EmpireID != st.Empire.ID ||
		st.DEAThreat.LeadAgent != "Hank Schrader" || st.DEAThreat.RiskLevel != "low" {
		t.Errorf("a new player already has a DEA assessment: %+v", st.DEAThreat)
	}
	if st.LegalCases == nil || len(st.LegalCases) != 0 {
		t.Errorf("legal cases are an empty list, not null: %#v", st.LegalCases)
	}
	if st.Rules.NextTerritoryCost == nil || *st.Rules.NextTerritoryCost != 25000 {
		t.Errorf("rules: %+v", st.Rules)
	}
}

func TestStateEvaluatesTheThreatWithoutVisitingTheDEAPage(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	emp := setEmpire(t, userID, map[string]any{"heat_level": 85.0, "heat_updated_at": time.Now(), "batch_yield": 500.0, "territory_count": 3})
	for i := 0; i < 8; i++ {
		addTx(t, emp.ID, models.TxLaundered, 80000, true)
	}

	st, err := GetState(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if st.DEAThreat.ThreatScore < 50 || st.DEAThreat.EvidenceCount != 8 || st.DEAThreat.RiskLevel == "low" {
		t.Errorf("the threat reflects the empire right away, it is not zero until /dea/threat is called: %+v", st.DEAThreat)
	}
	var active int64
	db.DB.Model(&models.DEAThreat{}).Where("empire_id = ? AND is_active = ?", emp.ID, true).Count(&active)
	if active != 1 {
		t.Errorf("active rows = %d, want 1", active)
	}
}

func TestStateShowsTheLatestTenCasesNewestFirstAndOnlyMine(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	emp := setEmpire(t, userID, map[string]any{})
	other := setEmpire(t, newUser(t), map[string]any{})

	base := time.Now().Add(-time.Hour)
	for i := 0; i < 12; i++ {
		c := models.LegalCase{EmpireID: emp.ID, Charge: fmt.Sprintf("charge %02d", i), Status: models.CaseOpen, Severity: 1}
		c.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := db.DB.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
	}
	foreign := models.LegalCase{EmpireID: other.ID, Charge: "someone else's problem", Status: models.CaseOpen}
	if err := db.DB.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}

	st, err := GetState(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.LegalCases) != 10 {
		t.Fatalf("cases = %d, want 10", len(st.LegalCases))
	}
	if st.LegalCases[0].Charge != "charge 11" || st.LegalCases[9].Charge != "charge 02" {
		t.Errorf("newest first: %q ... %q", st.LegalCases[0].Charge, st.LegalCases[9].Charge)
	}
	for _, c := range st.LegalCases {
		if c.EmpireID != emp.ID {
			t.Errorf("a case of another player leaked: %+v", c)
		}
	}
}

func TestStateIncludesFrontsAndCooledHeat(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 60.0, "heat_updated_at": time.Now().Add(-5 * time.Hour)})
	if _, err := empire.AddFront(userID, "Los Pollos Hermanos", "restaurant"); err != nil {
		t.Fatal(err)
	}

	st, err := GetState(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Empire.FrontBusinesses) != 1 || st.Empire.FrontBusinesses[0].Name != "Los Pollos Hermanos" {
		t.Errorf("fronts: %+v", st.Empire.FrontBusinesses)
	}
	near(t, "heat cooled by 5 hours x 6", st.Empire.HeatLevel, 30)
}

func TestStateHandler(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api/v1", func(c *gin.Context) { c.Set(auth.UserIDKey, userID); c.Next() }))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/game/state", nil))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var e struct {
		Success bool                       `json:"success"`
		Message string                     `json:"message"`
		Data    map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || !e.Success || e.Message != "Game state loaded" {
		t.Fatalf("%v %+v", err, e)
	}
	keys := make([]string, 0, len(e.Data))
	for k := range e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "dea_threat,empire,legal_cases,rules" {
		t.Errorf("keys: %v", keys)
	}
	if string(e.Data["legal_cases"]) != "[]" {
		t.Errorf("legal_cases must be [], got %s", e.Data["legal_cases"])
	}
	var rules map[string]any
	if err := json.Unmarshal(e.Data["rules"], &rules); err != nil || rules["optimal_effort"] != 8.33 || rules["next_territory_cost"] != 25000.0 {
		t.Errorf("rules: %v %v", rules, err)
	}
	var emp map[string]any
	_ = json.Unmarshal(e.Data["empire"], &emp)
	if fronts, ok := emp["front_businesses"].([]any); !ok || len(fronts) != 0 {
		t.Errorf("front_businesses must be []: %v", emp["front_businesses"])
	}
}

// ---------------------------------------------------------------------- statistics

func TestComputeStatsCountsTheRightThings(t *testing.T) {
	openTestDB(t)
	before, err := computeStats()
	if err != nil {
		t.Fatal(err)
	}

	userID := newUser(t)
	emp := setEmpire(t, userID, map[string]any{"batch_yield": 10.0 + 40.5})
	lowYield := setEmpire(t, newUser(t), map[string]any{"batch_yield": 4.0}) // below the starting 10 kg: counts as 0
	_ = lowYield
	addTx(t, emp.ID, models.TxLaundered, 1000, false)
	addTx(t, emp.ID, models.TxLaundered, 2000, true)
	addTx(t, emp.ID, models.TxDirty, 99999, false) // cooking income is not laundering
	addTx(t, emp.ID, models.TxClean, 88888, true)  // flagged but not a laundered transfer still counts as flagged
	fronts := []models.FrontBusiness{
		{EmpireID: emp.ID, Name: "Raided One", Type: "bar", IsActive: false, ShutdownReason: "raided by the DEA: suspicion reached 100"},
		{EmpireID: emp.ID, Name: "Closed By Owner", Type: "bar", IsActive: false, ShutdownReason: "closed by the owner"},
		{EmpireID: emp.ID, Name: "Still Open", Type: "bar", IsActive: true},
	}
	if err := db.DB.Create(&fronts).Error; err != nil {
		t.Fatal(err)
	}

	after, err := computeStats()
	if err != nil {
		t.Fatal(err)
	}
	if after.Empires-before.Empires != 2 {
		t.Errorf("empires +%d, want +2", after.Empires-before.Empires)
	}
	near(t, "kilograms cooked", after.KilogramsCooked-before.KilogramsCooked, 40.5)
	near(t, "dirty laundered", after.DirtyLaundered-before.DirtyLaundered, 3000)
	if after.FlaggedTransfers-before.FlaggedTransfers != 2 {
		t.Errorf("flagged +%d, want +2", after.FlaggedTransfers-before.FlaggedTransfers)
	}
	if after.Raids-before.Raids != 1 {
		t.Errorf("raids +%d, want +1", after.Raids-before.Raids)
	}
}

func TestStatsCacheReusesTheResultUntilItExpires(t *testing.T) {
	clock := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	loads := 0
	c := newStatsCache(func() time.Time { return clock }, 30*time.Second, func() (*Stats, error) {
		loads++
		return &Stats{Empires: int64(loads)}, nil
	})

	for i := 0; i < 5; i++ {
		s, err := c.get()
		if err != nil || s.Empires != 1 {
			t.Fatalf("call %d: %+v %v", i, s, err)
		}
	}
	clock = clock.Add(29 * time.Second)
	if s, _ := c.get(); s.Empires != 1 || loads != 1 {
		t.Errorf("still fresh after 29 s: %+v, loads %d", s, loads)
	}
	clock = clock.Add(time.Second) // exactly 30 s old
	if s, _ := c.get(); s.Empires != 2 || loads != 2 {
		t.Errorf("expired after 30 s: %+v, loads %d", s, loads)
	}
	clock = clock.Add(-time.Hour) // the clock jumped backwards
	if s, _ := c.get(); s.Empires != 3 || loads != 3 {
		t.Errorf("a clock that went backwards must not freeze the cache: %+v, loads %d", s, loads)
	}
}

func TestStatsCacheDoesNotCacheFailures(t *testing.T) {
	clock := time.Now()
	fail := true
	loads := 0
	c := newStatsCache(func() time.Time { return clock }, time.Minute, func() (*Stats, error) {
		loads++
		if fail {
			return nil, errors.New("database is down")
		}
		return &Stats{Empires: 7}, nil
	})
	if _, err := c.get(); err == nil {
		t.Fatal("the error is reported")
	}
	if _, err := c.get(); err == nil || loads != 2 {
		t.Errorf("a failure is retried at once, not cached (loads %d)", loads)
	}
	fail = false
	if s, err := c.get(); err != nil || s.Empires != 7 {
		t.Errorf("recovers: %+v %v", s, err)
	}
	fail = true // an earlier success is cached, so a later outage inside the window is invisible
	if s, err := c.get(); err != nil || s.Empires != 7 || loads != 3 {
		t.Errorf("fresh cache survives: %+v %v loads %d", s, err, loads)
	}
}

func TestStatsCacheIsSafeUnderConcurrency(t *testing.T) {
	var mu sync.Mutex
	loads := 0
	c := newStatsCache(time.Now, time.Minute, func() (*Stats, error) {
		mu.Lock()
		loads++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return &Stats{Empires: 1}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s, err := c.get(); err != nil || s.Empires != 1 {
				t.Errorf("%+v %v", s, err)
			}
		}()
	}
	wg.Wait()
	if loads != 1 {
		t.Errorf("30 simultaneous requests must cause one query, got %d", loads)
	}
}

func statsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterPublicRoutes(r.Group("/api/v1")) // no login middleware: the endpoint is public
	return r
}

func TestStatsEndpointIsPublicAndCacheable(t *testing.T) {
	openTestDB(t)
	defaultStats = newStatsCache(time.Now, statsTTL, computeStats)
	t.Cleanup(func() { defaultStats = newStatsCache(time.Now, statsTTL, computeStats) })
	setEmpire(t, newUser(t), map[string]any{"batch_yield": 30.0})

	w := httptest.NewRecorder()
	statsRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=30" {
		t.Errorf("Cache-Control = %q", cc)
	}
	var e struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || !e.Success {
		t.Fatalf("%v %s", err, w.Body.String())
	}
	keys := make([]string, 0, len(e.Data))
	for k := range e.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "dirty_laundered,empires,flagged_transfers,kilograms_cooked,raids" {
		t.Errorf("aggregates only, nothing personal: %v", keys)
	}
	if e.Data["empires"].(float64) < 1 || e.Data["kilograms_cooked"].(float64) < 20 {
		t.Errorf("unexpected numbers: %v", e.Data)
	}
}

func TestStatsEndpointFailureIsNotCacheable(t *testing.T) {
	old := defaultStats
	defaultStats = newStatsCache(time.Now, statsTTL, func() (*Stats, error) { return nil, errors.New("secret database detail") })
	t.Cleanup(func() { defaultStats = old })

	w := httptest.NewRecorder()
	statsRouter().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/stats", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), "statistics are unavailable") {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Header().Get("Cache-Control"), "public") {
		t.Errorf("an error must not be publicly cacheable: %q", w.Header().Get("Cache-Control"))
	}
}
