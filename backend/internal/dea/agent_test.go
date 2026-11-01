package dea

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
)

var (
	testDBOnce sync.Once
	testDBErr  error
)

// openTestDB connects once per test run to TEST_DATABASE_URL in a private schema (one shared
// pool) and skips when it is not set. The ML client is switched off unless a test enables it.
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
		if err := db.EnsureSchema(boot, "dea_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "dea_test")
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
	name := unique("hank")
	user := models.User{Username: name, Email: name + "@example.com", PasswordHash: "x", Alias: "Walter"}
	if err := db.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func setEmpire(t *testing.T, userID uint, updates map[string]any) {
	t.Helper()
	if _, err := empire.Get(userID); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Model(&models.Empire{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		t.Fatal(err)
	}
}

func loadEmpire(t *testing.T, userID uint) models.Empire {
	t.Helper()
	var emp models.Empire
	if err := db.DB.Where("user_id = ?", userID).First(&emp).Error; err != nil {
		t.Fatal(err)
	}
	return emp
}

// addTransfers records laundering transfers of the given amounts; flagged ones are what the
// DEA can use against the player.
func addTransfers(t *testing.T, userID uint, flagged bool, amounts ...float64) []uint {
	t.Helper()
	emp, err := empire.Get(userID)
	if err != nil {
		t.Fatal(err)
	}
	empID := emp.ID
	ids := make([]uint, 0, len(amounts))
	for _, a := range amounts {
		tx := models.Transaction{EmpireID: empID, Amount: a, Type: models.TxLaundered, Description: "test", Flagged: flagged}
		if err := db.DB.Create(&tx).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, tx.ID)
	}
	return ids
}

func rowCount(t *testing.T, empireID uint, activeOnly bool) int64 {
	t.Helper()
	q := db.DB.Model(&models.DEAThreat{}).Where("empire_id = ?", empireID)
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func near(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s: got %v, want %v (+/- %v)", what, got, want, tol)
	}
}

// ------------------------------------------------------------------ a fake ML service

type fakeMLConfig struct {
	threatStatus   int
	threatBody     string
	evidenceStatus int
	evidenceBody   string
}

type mlRecorder struct {
	mu          sync.Mutex
	threatCalls int
	lastThreat  mlclient.ThreatInput
}

func (r *mlRecorder) snapshot() (int, mlclient.ThreatInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.threatCalls, r.lastThreat
}

func fakeML(t *testing.T, cfg fakeMLConfig) *mlRecorder {
	t.Helper()
	rec := &mlRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ml/threat/analyze" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var in mlclient.ThreatInput
			_ = json.Unmarshal(body, &in)
			rec.mu.Lock()
			rec.threatCalls++
			rec.lastThreat = in
			rec.mu.Unlock()
			status := cfg.threatStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(cfg.threatBody))
		case strings.HasPrefix(r.URL.Path, "/ml/evidence/") && r.Method == http.MethodGet:
			status := cfg.evidenceStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(cfg.evidenceBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	useML(t, srv.URL)
	return rec
}

func useML(t *testing.T, url string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.MLServiceURL = url
	cfg.MLTimeout = 2 * time.Second
	mlclient.SetDefault(mlclient.New(cfg))
}

// ---------------------------------------------------------------- pure rules

func TestBandBoundaries(t *testing.T) {
	for _, tc := range []struct {
		score float64
		level string
	}{
		{0, "low"}, {24.99, "low"},
		{25, "medium"}, {49.99, "medium"},
		{50, "high"}, {74.99, "high"},
		{75, "critical"}, {100, "critical"}, {250, "critical"},
	} {
		level, prediction, recommendation := Band(tc.score)
		if level != tc.level || prediction == "" || recommendation == "" {
			t.Errorf("Band(%v) = %q / %q / %q, want level %q with texts", tc.score, level, prediction, recommendation, tc.level)
		}
	}
	if _, p, _ := Band(10); p != "Hank suspects nothing. Stay clean." {
		t.Errorf("the low band keeps the ML service wording, got %q", p)
	}
	if _, p, r := Band(90); p != "They know. It's over \u2014 unless you talk to Saul." || r != "Full operational freeze. Destroy evidence. Run." {
		t.Errorf("the critical band keeps the ML service wording, got %q / %q", p, r)
	}
}

func TestHeuristicMatchesTheMLTrainingCurve(t *testing.T) {
	// Reference points taken from the ML service's own model (Phase 2).
	for _, tc := range []struct {
		name          string
		in            Inputs
		ml, heuristic float64
		level         string
	}{
		{"busy empire", Inputs{Heat: 70, Flagged: 12, BatchYield: 400, Territory: 4, DaysActive: 120}, 61.51, 61.37, "high"},
		{"hot but quiet", Inputs{Heat: 100, BatchYield: 110}, 26.88, 25.58, "medium"},
		{"nothing yet", Inputs{}, 1.99, 1.8, "low"},
	} {
		v := Heuristic(tc.in)
		near(t, tc.name+" vs ML", v.Score, tc.ml, 2)
		near(t, tc.name+" exact", v.Score, tc.heuristic, 0.005)
		if v.Level != tc.level || v.Source != "heuristic" || v.Prediction == "" || v.Recommendation == "" {
			t.Errorf("%s: unexpected verdict %+v", tc.name, v)
		}
	}
}

func TestHeuristicRisesWithEveryInputAndStaysInRange(t *testing.T) {
	base := Inputs{Heat: 20, Flagged: 2, BatchYield: 100, Territory: 1, DaysActive: 30}
	steps := map[string]func(Inputs) Inputs{
		"heat":      func(in Inputs) Inputs { in.Heat += 10; return in },
		"flagged":   func(in Inputs) Inputs { in.Flagged += 2; return in },
		"yield":     func(in Inputs) Inputs { in.BatchYield += 100; return in },
		"territory": func(in Inputs) Inputs { in.Territory++; return in },
		"days":      func(in Inputs) Inputs { in.DaysActive += 30; return in },
	}
	for name, step := range steps {
		in := base
		prev := Heuristic(in).Score
		for i := 0; i < 8; i++ {
			in = step(in)
			cur := Heuristic(in).Score
			if cur < prev {
				t.Errorf("%s: score fell from %v to %v when the input grew", name, prev, cur)
			}
			prev = cur
		}
	}
	for _, in := range []Inputs{
		{},
		{Heat: -50, Flagged: -3, BatchYield: -1, Territory: -2, DaysActive: -9},
		{Heat: 1e9, Flagged: 1 << 30, BatchYield: 1e12, Territory: 1 << 20, DaysActive: 1 << 30},
	} {
		v := Heuristic(in)
		if math.IsNaN(v.Score) || v.Score < 0 || v.Score > 100 {
			t.Errorf("%+v: score %v is out of range", in, v.Score)
		}
	}
	if top := Heuristic(Inputs{Heat: 100, Flagged: 20, BatchYield: 1000, Territory: 10, DaysActive: 365}); top.Level != "critical" {
		t.Errorf("a maxed-out empire is critical, got %+v", top)
	}
}

func TestFromMLRejectsNonsense(t *testing.T) {
	good := &mlclient.ThreatResult{ThreatScore: 42.567, RiskLevel: "medium", Prediction: "p", Recommendation: "r"}
	v, ok := fromML(good)
	if !ok || v.Source != "ml" || v.Level != "medium" || v.Score != 42.57 || v.Prediction != "p" || v.Recommendation != "r" {
		t.Errorf("a sane answer is accepted and rounded: %+v %v", v, ok)
	}
	for name, bad := range map[string]*mlclient.ThreatResult{
		"NaN":           {ThreatScore: math.NaN(), RiskLevel: "low"},
		"negative":      {ThreatScore: -0.1, RiskLevel: "low"},
		"over 100":      {ThreatScore: 100.5, RiskLevel: "critical"},
		"unknown level": {ThreatScore: 30, RiskLevel: "purple"},
		"empty level":   {ThreatScore: 30},
		"old label":     {ThreatScore: 30, RiskLevel: "elevated"},
	} {
		if _, ok := fromML(bad); ok {
			t.Errorf("%s must be rejected", name)
		}
	}
}

// ----------------------------------------------------------- evaluation and history

func TestNewEmpireStartsLowWithOneActiveRow(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	th, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	emp := loadEmpire(t, userID)
	if th.ID == 0 || th.EmpireID != emp.ID || !th.IsActive || th.LeadAgent != "Hank Schrader" ||
		th.RiskLevel != "low" || th.Source != "heuristic" || th.EvidenceCount != 0 || th.ThreatScore >= 25 ||
		th.Prediction == "" || th.Recommendation == "" {
		t.Errorf("unexpected first assessment: %+v", th)
	}
	if rowCount(t, emp.ID, false) != 1 || rowCount(t, emp.ID, true) != 1 {
		t.Error("exactly one active row after the first evaluation")
	}
}

func TestUnchangedReEvaluationReusesTheRow(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 50.0, "heat_updated_at": time.Now()})

	first, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		again, err := Evaluate(context.Background(), userID)
		if err != nil {
			t.Fatal(err)
		}
		if again.ID != first.ID {
			t.Fatalf("polling must not write rows: %d != %d", again.ID, first.ID)
		}
	}
	if n := rowCount(t, first.EmpireID, false); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestATinyScoreWiggleDoesNotWriteARow(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 50.0, "heat_updated_at": time.Now()})
	first, _ := Evaluate(context.Background(), userID)

	setEmpire(t, userID, map[string]any{"heat_level": 50.1, "heat_updated_at": time.Now()})
	second, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Errorf("a score move below %.1f must not create a row (%v -> %v)", minScoreChange, first.ThreatScore, second.ThreatScore)
	}
}

func TestChangeWritesANewRowAndKeepsExactlyOneActive(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	first, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}

	setEmpire(t, userID, map[string]any{"heat_level": 90.0, "heat_updated_at": time.Now(), "batch_yield": 600.0, "territory_count": 5})
	second, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID || second.ThreatScore <= first.ThreatScore {
		t.Fatalf("the heat spike must produce a higher score in a new row: %+v -> %+v", first, second)
	}
	empID := first.EmpireID
	if rowCount(t, empID, false) != 2 || rowCount(t, empID, true) != 1 {
		t.Error("two rows in total, exactly one of them active")
	}
	var active models.DEAThreat
	if err := db.DB.Where("empire_id = ? AND is_active = ?", empID, true).First(&active).Error; err != nil || active.ID != second.ID {
		t.Errorf("the newest row is the active one: %+v %v", active, err)
	}
}

func TestAnOldSnapshotIsRefreshed(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	first, _ := Evaluate(context.Background(), userID)

	old := time.Now().Add(-(snapshotEvery + time.Minute))
	if err := db.DB.Model(&models.DEAThreat{}).Where("id = ?", first.ID).Update("created_at", old).Error; err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Error("after five minutes the picture is recorded again even though nothing changed")
	}
	if rowCount(t, first.EmpireID, true) != 1 {
		t.Error("still exactly one active row")
	}
}

func TestFlaggedTransfersRaiseEvidenceAndScore(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 40.0, "heat_updated_at": time.Now()})
	before, _ := Evaluate(context.Background(), userID)

	addTransfers(t, userID, false, 5000, 6000, 7000) // unflagged transfers prove nothing
	same, _ := Evaluate(context.Background(), userID)
	if same.ID != before.ID || same.EvidenceCount != 0 {
		t.Errorf("unflagged transfers are not evidence: %+v", same)
	}

	addTransfers(t, userID, true, 60000, 70000, 80000, 90000, 100000, 120000)
	after, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EvidenceCount != 6 || after.ThreatScore <= before.ThreatScore || after.ID == before.ID {
		t.Errorf("six flagged transfers: evidence 6 and a higher score expected: before %+v after %+v", before, after)
	}
}

func TestHistoryIsPrunedToTheNewestRows(t *testing.T) {
	openTestDB(t)
	old := maxHistory
	maxHistory = 3
	t.Cleanup(func() { maxHistory = old })

	userID := newUser(t)
	var last *models.DEAThreat
	for i := 0; i < 7; i++ {
		setEmpire(t, userID, map[string]any{"heat_level": float64(10 + i*12), "heat_updated_at": time.Now()})
		th, err := Evaluate(context.Background(), userID)
		if err != nil {
			t.Fatal(err)
		}
		last = th
	}
	empID := last.EmpireID
	if n := rowCount(t, empID, false); n != 3 {
		t.Fatalf("rows = %d, want 3", n)
	}
	rows, err := History(userID, 10)
	if err != nil || len(rows) != 3 {
		t.Fatalf("history: %v %v", rows, err)
	}
	if rows[0].ID != last.ID || !rows[0].IsActive || rows[0].ThreatScore < rows[1].ThreatScore {
		t.Errorf("newest first, and the newest is the active one: %+v", rows)
	}
	if rowCount(t, empID, true) != 1 {
		t.Error("still exactly one active row after pruning")
	}
}

func TestConcurrentEvaluationsCreateOneRow(t *testing.T) {
	openTestDB(t)
	// Several rounds, each on a fresh empire, with every goroutine released at the same moment
	// so that without the row lock they would all see "no active row" and each insert one.
	for round := 0; round < 4; round++ {
		userID := newUser(t)
		if _, err := empire.Get(userID); err != nil {
			t.Fatal(err)
		}
		const workers = 20
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		ids := make(chan uint, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				th, err := Evaluate(context.Background(), userID)
				if err != nil {
					errs <- err
					return
				}
				ids <- th.ID
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		close(ids)
		for err := range errs {
			t.Errorf("round %d: evaluation failed: %v", round, err)
		}
		seen := map[uint]bool{}
		for id := range ids {
			seen[id] = true
		}
		empID := loadEmpire(t, userID).ID
		if len(seen) != 1 || rowCount(t, empID, false) != 1 || rowCount(t, empID, true) != 1 {
			t.Errorf("round %d: simultaneous evaluations must share one row: ids %v rows %d active %d",
				round, seen, rowCount(t, empID, false), rowCount(t, empID, true))
		}
	}
}

func TestDaysActiveComesFromTheEmpireAge(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"created_at": time.Now().Add(-45 * 24 * time.Hour)})
	rec := fakeML(t, fakeMLConfig{threatBody: `{"threat_score":12,"risk_level":"low","prediction":"p","recommendation":"r"}`})

	if _, err := Evaluate(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	calls, in := rec.snapshot()
	if calls != 1 || in.DaysActive != 45 || in.EmpireID != loadEmpire(t, userID).ID {
		t.Errorf("unexpected ML request (%d calls): %+v", calls, in)
	}
}

// ------------------------------------------------------------------ ML versus fallback

func TestTheMLVerdictIsUsedWhenItIsSane(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 33.0, "heat_updated_at": time.Now(), "batch_yield": 250.0, "territory_count": 2})
	addTransfers(t, userID, true, 70000, 80000)
	rec := fakeML(t, fakeMLConfig{threatBody: `{"threat_score":42.5,"risk_level":"medium","prediction":"ML says watch out","recommendation":"ML says lawyer up"}`})

	th, err := Evaluate(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if th.Source != "ml" || th.ThreatScore != 42.5 || th.RiskLevel != "medium" ||
		th.Prediction != "ML says watch out" || th.Recommendation != "ML says lawyer up" || th.EvidenceCount != 2 {
		t.Errorf("unexpected row: %+v", th)
	}
	calls, in := rec.snapshot()
	if calls != 1 || in.FlaggedTransactions != 2 || in.TerritoryCount != 2 || in.BatchYield != 250 || math.Abs(in.HeatLevel-33) > 0.1 {
		t.Errorf("the ML request carries the empire's facts: %+v", in)
	}
}

func TestEveryKindOfMLFailureFallsBackToTheHeuristic(t *testing.T) {
	openTestDB(t)
	for name, cfg := range map[string]fakeMLConfig{
		"server error": {threatStatus: 500, threatBody: `{"detail":"boom"}`},
		"wrong key":    {threatStatus: 401, threatBody: `{"detail":"invalid API key"}`},
		"rejected":     {threatStatus: 422, threatBody: `{"detail":"bad input"}`},
		"garbage":      {threatBody: `this is not json`},
		"empty":        {threatBody: ``},
		"out of range": {threatBody: `{"threat_score":250,"risk_level":"critical","prediction":"p","recommendation":"r"}`},
		"negative":     {threatBody: `{"threat_score":-5,"risk_level":"low","prediction":"p","recommendation":"r"}`},
		"unknown":      {threatBody: `{"threat_score":10,"risk_level":"purple","prediction":"p","recommendation":"r"}`},
		"wrong shape":  {threatBody: `{"score":10}`},
	} {
		t.Run(name, func(t *testing.T) {
			userID := newUser(t)
			setEmpire(t, userID, map[string]any{"heat_level": 70.0, "heat_updated_at": time.Now(), "batch_yield": 400.0, "territory_count": 4})
			fakeML(t, cfg)

			th, err := Evaluate(context.Background(), userID)
			if err != nil {
				t.Fatalf("a broken ML service must never break the DEA: %v", err)
			}
			if th.Source != "heuristic" || th.ThreatScore <= 0 || th.ThreatScore > 100 || th.RiskLevel == "" || th.Prediction == "" {
				t.Errorf("expected the heuristic verdict, got %+v", th)
			}
		})
	}
}

func TestADeadMLServiceFallsBackToo(t *testing.T) {
	openTestDB(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	useML(t, url)

	userID := newUser(t)
	th, err := Evaluate(context.Background(), userID)
	if err != nil || th.Source != "heuristic" {
		t.Errorf("unexpected: %+v %v", th, err)
	}
}

// ------------------------------------------------------------------------- evidence

func TestEvidenceFromTheLedger(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)

	empty, err := Evidence(context.Background(), userID)
	if err != nil || empty.Source != "local" || empty.Count != 0 || empty.TotalWeight != 0 || empty.TopSources == nil || len(empty.TopSources) != 0 {
		t.Fatalf("an empty file is empty, with an empty list rather than null: %+v %v", empty, err)
	}

	addTransfers(t, userID, false, 900000) // not flagged: not evidence
	ids := addTransfers(t, userID, true, 20000, 60000, 500000)
	file, err := Evidence(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if file.Source != "local" || file.Count != 3 {
		t.Fatalf("unexpected file: %+v", file)
	}
	near(t, "total weight", file.TotalWeight, 0.4+1.2+10, 0.005) // clamp(amount/50000, 0.1, 10)
	if len(file.TopSources) != 3 {
		t.Fatalf("top sources: %+v", file.TopSources)
	}
	wantOrder := []uint{ids[2], ids[1], ids[0]} // the heaviest first
	wantWeight := []float64{10, 1.2, 0.4}
	for i, s := range file.TopSources {
		if s.Source != fmt.Sprintf("tx:%d", wantOrder[i]) || s.Kind != "transaction" || math.Abs(s.Weight-wantWeight[i]) > 0.005 {
			t.Errorf("source %d: %+v", i, s)
		}
	}
}

func TestEvidenceShowsOnlyTheFiveHeaviestSources(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	addTransfers(t, userID, true, 51000, 52000, 53000, 54000, 55000, 56000, 57000)
	file, err := Evidence(context.Background(), userID)
	if err != nil || file.Count != 7 || len(file.TopSources) != 5 {
		t.Fatalf("%+v %v", file, err)
	}
}

func TestEvidenceUsesTheMLGraphOnlyWhenItKnowsAtLeastAsMuch(t *testing.T) {
	openTestDB(t)
	mlFile := `{"empire_id":1,"node_count":%d,"total_weight":%v,"top_sources":[{"source":"tx:9","kind":"transaction","weight":4.5}]}`
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantSource string
		wantCount  int
	}{
		{"graph knows more", 200, fmt.Sprintf(mlFile, 5, 7.5), "ml", 5},
		{"graph knows the same", 200, fmt.Sprintf(mlFile, 3, 6.5), "ml", 3},
		{"graph restarted and knows less", 200, fmt.Sprintf(mlFile, 2, 1.5), "local", 3},
		{"graph is empty", 200, fmt.Sprintf(mlFile, 0, 0), "local", 3},
		{"graph is down", 500, `{"detail":"boom"}`, "local", 3},
		{"graph sends garbage", 200, `nope`, "local", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID := newUser(t)
			addTransfers(t, userID, true, 20000, 60000, 500000)
			fakeML(t, fakeMLConfig{evidenceStatus: tc.status, evidenceBody: tc.body})

			file, err := Evidence(context.Background(), userID)
			if err != nil {
				t.Fatal(err)
			}
			if file.Source != tc.wantSource || file.Count != tc.wantCount || file.TopSources == nil {
				t.Errorf("got %+v, want source %s count %d", file, tc.wantSource, tc.wantCount)
			}
			if tc.wantSource == "ml" && (len(file.TopSources) != 1 || file.TopSources[0].Source != "tx:9") {
				t.Errorf("the graph's own sources are returned: %+v", file.TopSources)
			}
		})
	}
}

// ------------------------------------------------------------------ HTTP handlers

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

func call(t *testing.T, r http.Handler, method, path string) (int, envelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("%s %s: not JSON: %v (%s)", method, path, err, w.Body.String())
	}
	return w.Code, e
}

func TestThreatEndpoint(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	setEmpire(t, userID, map[string]any{"heat_level": 80.0, "heat_updated_at": time.Now()})
	r := router(userID)

	status, e := call(t, r, "GET", "/api/v1/dea/threat")
	if status != 200 || !e.Success || e.Message != "Stay one step ahead of Hank" {
		t.Fatalf("%d %+v", status, e)
	}
	var raw map[string]any
	if err := json.Unmarshal(e.Data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "empire_id", "threat_score", "risk_level", "prediction", "recommendation", "source", "evidence_count", "lead_agent", "is_active", "created_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response is missing %q: %v", key, raw)
		}
	}
	if raw["lead_agent"] != "Hank Schrader" || raw["source"] != "heuristic" || raw["is_active"] != true {
		t.Errorf("unexpected values: %v", raw)
	}
}

func TestHistoryEndpoint(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	r := router(userID)

	status, e := call(t, r, "GET", "/api/v1/dea/history")
	if status != 200 || e.Message != "The DEA's paper trail" || strings.TrimSpace(string(e.Data)) != "[]" {
		t.Fatalf("an empty history is an empty list, not null: %d %+v %s", status, e, e.Data)
	}

	for i := 0; i < 4; i++ {
		setEmpire(t, userID, map[string]any{"heat_level": float64(10 + i*20), "heat_updated_at": time.Now()})
		if _, err := Evaluate(context.Background(), userID); err != nil {
			t.Fatal(err)
		}
	}
	var all []models.DEAThreat
	_, e = call(t, r, "GET", "/api/v1/dea/history")
	if err := json.Unmarshal(e.Data, &all); err != nil || len(all) != 4 {
		t.Fatalf("history: %v %s", err, e.Data)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ID <= all[i].ID {
			t.Errorf("newest first: %d then %d", all[i-1].ID, all[i].ID)
		}
	}
	var one []models.DEAThreat
	_, e = call(t, r, "GET", "/api/v1/dea/history?limit=1")
	if err := json.Unmarshal(e.Data, &one); err != nil || len(one) != 1 || one[0].ID != all[0].ID {
		t.Errorf("limit=1 returns the newest row only: %v %s", err, e.Data)
	}
}

func TestHistoryRejectsBadLimits(t *testing.T) {
	openTestDB(t)
	r := router(newUser(t))
	for _, q := range []string{"limit=0", "limit=201", "limit=-1", "limit=abc", "limit=1.5", "limit=99999999999999999999"} {
		status, e := call(t, r, "GET", "/api/v1/dea/history?"+q)
		if status != 400 || e.Success || e.Error != "limit must be a number from 1 to 200" {
			t.Errorf("%s: %d %+v", q, status, e)
		}
	}
	for _, q := range []string{"limit=1", "limit=200"} {
		if status, _ := call(t, r, "GET", "/api/v1/dea/history?"+q); status != 200 {
			t.Errorf("%s should be accepted, got %d", q, status)
		}
	}
}

func TestEvidenceEndpoint(t *testing.T) {
	openTestDB(t)
	userID := newUser(t)
	addTransfers(t, userID, true, 75000)
	r := router(userID)

	status, e := call(t, r, "GET", "/api/v1/dea/evidence")
	if status != 200 || e.Message != "What the DEA has on you" {
		t.Fatalf("%d %+v", status, e)
	}
	var file EvidenceFile
	if err := json.Unmarshal(e.Data, &file); err != nil || file.Source != "local" || file.Count != 1 || len(file.TopSources) != 1 {
		t.Errorf("unexpected file: %+v %v", file, err)
	}
	if !strings.Contains(string(e.Data), `"top_sources"`) || !strings.Contains(string(e.Data), `"total_weight"`) {
		t.Errorf("snake_case keys expected: %s", e.Data)
	}
}
