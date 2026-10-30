package laundering

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
		if err := db.EnsureSchema(boot, "laundering_test"); err != nil {
			testDBErr = err
			return
		}
		if sqlDB, err := boot.DB(); err == nil {
			sqlDB.Close()
		}
		dsn, err := db.WithSchema(raw, "laundering_test")
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
	name := unique("walt")
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

func loadFront(t *testing.T, id uint) models.FrontBusiness {
	t.Helper()
	var fb models.FrontBusiness
	if err := db.DB.First(&fb, id).Error; err != nil {
		t.Fatal(err)
	}
	return fb
}

func countTx(t *testing.T, userID uint) int64 {
	t.Helper()
	var n int64
	db.DB.Model(&models.Transaction{}).Where("empire_id = ?", loadEmpire(t, userID).ID).Count(&n)
	return n
}

// setup returns a player with an empire holding `dirty` dollars and one open restaurant.
func setup(t *testing.T, dirty float64) (userID, frontID uint) {
	t.Helper()
	userID = newUser(t)
	setEmpire(t, userID, map[string]any{"dirty_money": dirty})
	fb, err := empire.AddFront(userID, "Los Pollos Hermanos", "restaurant")
	if err != nil {
		t.Fatal(err)
	}
	return userID, fb.ID
}

// ------------------------------------------------------------------ a fake ML service

type recorder struct {
	mu       sync.Mutex
	analyze  []byte
	evidence []byte
}

func (r *recorder) get() (analyze, evidence []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.analyze, r.evidence
}

func fakeML(t *testing.T, status int, analyzeBody string) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 0, 512)
		buf := make([]byte, 512)
		for {
			n, err := r.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil {
				break
			}
		}
		switch r.URL.Path {
		case "/ml/laundering/analyze":
			rec.mu.Lock()
			rec.analyze = body
			rec.mu.Unlock()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(analyzeBody))
		case "/ml/evidence":
			rec.mu.Lock()
			rec.evidence = body
			rec.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"empire_id":1,"node_count":1,"total_weight":1,"top_sources":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func useML(t *testing.T, url string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.MLServiceURL = url
	cfg.MLTimeout = 2 * time.Second
	mlclient.SetDefault(mlclient.New(cfg))
}

// ---------------------------------------------------------------- the rules in action

func TestLaunderAppliesTheRules(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 100000)

	res, err := Launder(context.Background(), userID, frontID, 20000)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "cleaned", res.CleanedAmount, 15000, 0)
	near(t, "fee", res.Fee, 5000, 0)
	near(t, "suspicion gain", res.SuspicionGain, 12, 1e-9) // 10 from size + 20 risk * 0.1
	if res.FrontClosed || res.Transaction.Flagged || res.Analysis.Source != "local" || res.Analysis.RiskScore != 20 {
		t.Errorf("unexpected result: %+v", res)
	}

	emp := loadEmpire(t, userID)
	near(t, "dirty money", emp.DirtyMoney, 80000, 1e-9)
	near(t, "clean money", emp.CleanMoney, 15000, 1e-9)
	near(t, "heat", emp.HeatLevel, 0.2, 0.01)
	near(t, "returned empire matches", res.Empire.CleanMoney, 15000, 1e-9)

	fb := loadFront(t, frontID)
	near(t, "front revenue", fb.Revenue, 15000, 1e-9)
	near(t, "front suspicion", fb.Suspicion, 12, 1e-9)
	if !fb.IsActive {
		t.Error("the front stays open")
	}

	tx := res.Transaction
	if tx.ID == 0 || tx.Type != models.TxLaundered || tx.Amount != 20000 || tx.BusinessID != frontID ||
		tx.EmpireID != emp.ID || tx.RiskScore != 20 || !strings.Contains(tx.Description, "Los Pollos Hermanos") ||
		!strings.Contains(tx.Description, "$20000.00 in") || !strings.Contains(tx.Description, "$15000.00 clean") {
		t.Errorf("unexpected ledger entry: %+v", tx)
	}
}

func TestRepeatedIdenticalTransfersLookLikeStructuring(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 100000)

	first, err := Launder(context.Background(), userID, frontID, 20000)
	if err != nil || first.Transaction.Flagged {
		t.Fatalf("the first transfer is fine: %+v %v", first, err)
	}
	second, err := Launder(context.Background(), userID, frontID, 20000)
	if err != nil {
		t.Fatal(err)
	}
	// structuring + high frequency + revenue ratio = 3 patterns = 60
	near(t, "risk", second.Analysis.RiskScore, 60, 0)
	if !second.Transaction.Flagged || !strings.Contains(second.Transaction.FlagReason, "analysis risk 60%") ||
		!strings.Contains(second.Transaction.FlagReason, "Structuring") {
		t.Errorf("the second identical transfer must be flagged: %+v", second.Transaction)
	}
	near(t, "suspicion builds up", loadFront(t, frontID).Suspicion, 12+16, 1e-9)
}

func TestInvalidAmountsChangeNothing(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 50000)
	setEmpire(t, userID, map[string]any{"clean_money": 40000.0, "heat_level": 30.0})

	// A negative amount used to pass the funds check and turn clean money into dirty money.
	for _, bad := range []float64{0, 99.99, -1, -1000, -500000, math.NaN(), math.Inf(1), math.Inf(-1), 1e9} {
		if _, err := Launder(context.Background(), userID, frontID, bad); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("amount %v: got %v, want ErrInvalidAmount", bad, err)
		}
	}
	emp := loadEmpire(t, userID)
	if emp.DirtyMoney != 50000 || emp.CleanMoney != 40000 || countTx(t, userID) != 0 {
		t.Errorf("rejected transfers must change nothing: %+v", emp)
	}
	near(t, "heat untouched", emp.HeatLevel, 30, 0.01)
}

func TestInsufficientDirtyMoney(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 5000)

	if _, err := Launder(context.Background(), userID, frontID, 10000); !errors.Is(err, ErrInsufficientDirty) {
		t.Fatalf("got %v", err)
	}
	emp := loadEmpire(t, userID)
	fb := loadFront(t, frontID)
	if emp.DirtyMoney != 5000 || emp.CleanMoney != 0 || countTx(t, userID) != 0 || fb.Revenue != 0 || fb.Suspicion != 0 {
		t.Errorf("a failed transfer must leave no trace: %+v %+v", emp, fb)
	}
}

func TestTheFrontMustBeYoursAndOpen(t *testing.T) {
	openTestDB(t)
	userID, _ := setup(t, 100000)
	other, otherFront := setup(t, 100000)
	_ = other

	if _, err := Launder(context.Background(), userID, 999999999, 1000); !errors.Is(err, empire.ErrFrontNotFound) {
		t.Errorf("unknown front: got %v", err)
	}
	if _, err := Launder(context.Background(), userID, otherFront, 1000); !errors.Is(err, empire.ErrFrontNotFound) {
		t.Errorf("someone else's front: got %v", err)
	}

	closed, err := empire.AddFront(userID, "Closed Down", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := empire.CloseFront(userID, closed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Launder(context.Background(), userID, closed.ID, 1000); !errors.Is(err, empire.ErrFrontClosed) {
		t.Errorf("closed front: got %v", err)
	}
	if emp := loadEmpire(t, userID); emp.DirtyMoney != 100000 || emp.CleanMoney != 0 || countTx(t, userID) != 0 {
		t.Errorf("nothing may move: %+v", emp)
	}
	if emp := loadEmpire(t, other); emp.DirtyMoney != 100000 {
		t.Errorf("the other player's money must be untouched: %+v", emp)
	}
}

func TestHeatNeverExceedsOneHundred(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 200000)
	setEmpire(t, userID, map[string]any{"heat_level": 99.9})

	res, err := Launder(context.Background(), userID, frontID, 100000)
	if err != nil {
		t.Fatal(err)
	}
	near(t, "heat is capped", res.Empire.HeatLevel, 100, 1e-9)
	near(t, "stored", loadEmpire(t, userID).HeatLevel, 100, 1e-9)
}

func TestOverusedFrontsGetRaided(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 100000)
	if err := db.DB.Model(&models.FrontBusiness{}).Where("id = ?", frontID).Update("suspicion", 95.0).Error; err != nil {
		t.Fatal(err)
	}

	res, err := Launder(context.Background(), userID, frontID, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FrontClosed || res.Front.IsActive || !strings.Contains(res.Front.ShutdownReason, "raided") {
		t.Errorf("the front must be raided: %+v", res.Front)
	}
	near(t, "suspicion is capped", res.Front.Suspicion, 100, 0)
	near(t, "the raid adds heat", res.Empire.HeatLevel, 0.2+RaidHeat, 0.01)
	near(t, "the money still moved", res.Empire.CleanMoney, 15000, 1e-9)

	stored := loadFront(t, frontID)
	if stored.IsActive || stored.Suspicion != 100 {
		t.Errorf("stored front: %+v", stored)
	}
	if _, err := Launder(context.Background(), userID, frontID, 1000); !errors.Is(err, empire.ErrFrontClosed) {
		t.Errorf("a raided front cannot be used again: got %v", err)
	}
}

// ------------------------------------------------------------------ concurrency

func TestConcurrentTransfersAddUp(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 1_000_000)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Launder(context.Background(), userID, frontID, 10000); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a transfer failed: %v", err)
	}

	emp := loadEmpire(t, userID)
	near(t, "dirty money", emp.DirtyMoney, 900000, 1e-6)
	near(t, "clean money", emp.CleanMoney, 75000, 1e-6)
	near(t, "heat", emp.HeatLevel, 1.0, 0.02)
	fb := loadFront(t, frontID)
	near(t, "front revenue", fb.Revenue, 75000, 1e-6)
	if fb.Suspicion <= 0 || fb.Suspicion > 100 {
		t.Errorf("suspicion out of range: %v", fb.Suspicion)
	}
	if countTx(t, userID) != 10 {
		t.Errorf("expected 10 ledger entries, found %d", countTx(t, userID))
	}
}

// Without the row lock two transfers could both see the same balance and spend it twice.
func TestConcurrentTransfersCannotOverdraw(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 25000)

	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Launder(context.Background(), userID, frontID, 10000)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	ok, refused := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrInsufficientDirty):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 2 || refused != 8 {
		t.Errorf("exactly two $10,000 transfers fit in $25,000: %d succeeded, %d refused", ok, refused)
	}
	emp := loadEmpire(t, userID)
	near(t, "dirty money left", emp.DirtyMoney, 5000, 1e-6)
	near(t, "clean money", emp.CleanMoney, 15000, 1e-6)
	if countTx(t, userID) != 2 {
		t.Errorf("only the two real transfers are recorded: %d", countTx(t, userID))
	}
}

// ------------------------------------------------------------------- the ML service

func TestLargeTransfersAreFlaggedAndReportedToTheDEA(t *testing.T) {
	openTestDB(t)
	srv, rec := fakeML(t, 200, `{"risk_score":10,"anomaly_detected":false,"suspicious_patterns":[],"recommendation":"Patterns look normal","revenue_ratio":3.0}`)
	useML(t, srv.URL)
	userID, frontID := setup(t, 100000)

	res, err := Launder(context.Background(), userID, frontID, 60000)
	if err != nil {
		t.Fatal(err)
	}
	if res.Analysis.Source != "ml" || res.Analysis.RiskScore != 10 {
		t.Errorf("the ML verdict should be used: %+v", res.Analysis)
	}
	if !res.Transaction.Flagged || !strings.Contains(res.Transaction.FlagReason, "large transfer") {
		t.Errorf("a $60,000 transfer must be flagged: %+v", res.Transaction)
	}

	_, evidence := rec.get()
	var sent mlclient.EvidenceInput
	if err := json.Unmarshal(evidence, &sent); err != nil {
		t.Fatalf("no evidence was sent to the ML service: %v (%q)", err, evidence)
	}
	if sent.Source != fmt.Sprintf("tx:%d", res.Transaction.ID) || sent.EmpireID != res.Empire.ID ||
		sent.Weight != 1.2 || sent.Kind != "transaction" {
		t.Errorf("wrong evidence: %+v", sent)
	}
}

func TestUnflaggedTransfersSendNoEvidence(t *testing.T) {
	openTestDB(t)
	srv, rec := fakeML(t, 200, `{"risk_score":0,"anomaly_detected":false,"suspicious_patterns":[],"recommendation":"Patterns look normal","revenue_ratio":0.5}`)
	useML(t, srv.URL)
	userID, frontID := setup(t, 100000)

	res, err := Launder(context.Background(), userID, frontID, 10000)
	if err != nil || res.Transaction.Flagged {
		t.Fatalf("%+v %v", res, err)
	}
	if _, evidence := rec.get(); evidence != nil {
		t.Errorf("a clean transfer must not create evidence: %s", evidence)
	}
}

func TestTheMLVerdictAndRequest(t *testing.T) {
	openTestDB(t)
	srv, rec := fakeML(t, 200, `{"risk_score":80,"anomaly_detected":true,"suspicious_patterns":["Structuring pattern — identical amounts"],"recommendation":"Reduce transaction sizes","revenue_ratio":0.5}`)
	useML(t, srv.URL)
	userID, frontID := setup(t, 100000)

	res, err := Launder(context.Background(), userID, frontID, 10000)
	if err != nil {
		t.Fatal(err)
	}
	a := res.Analysis
	if a.Source != "ml" || a.RiskScore != 80 || !a.Anomaly || len(a.Patterns) != 1 {
		t.Errorf("verdict not mapped: %+v", a)
	}
	if !res.Transaction.Flagged || !strings.Contains(res.Transaction.FlagReason, "analysis risk 80%") ||
		!strings.Contains(res.Transaction.FlagReason, "unusual") {
		t.Errorf("a risky ML verdict flags the transfer: %+v", res.Transaction)
	}
	near(t, "ML risk raises suspicion", res.SuspicionGain, 5+8, 1e-9) // 10000/20000*10 + 80*0.1

	raw, _ := rec.get()
	var sent mlclient.LaunderingInput
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Amounts) != 1 || sent.Amounts[0] != 10000 || len(sent.BusinessRevenues) != 1 ||
		sent.BusinessRevenues[0] != 20000 || sent.BusinessType != "restaurant" {
		t.Errorf("wrong request: %+v", sent)
	}
	if !strings.Contains(string(raw), `"time_gaps":[]`) {
		t.Errorf("time_gaps must be an empty list, not null (the ML service rejects null): %s", raw)
	}
}

func TestTheMLRequestIncludesRecentHistory(t *testing.T) {
	openTestDB(t)
	srv, rec := fakeML(t, 200, `{"risk_score":0,"anomaly_detected":false,"suspicious_patterns":[],"recommendation":"ok","revenue_ratio":null}`)
	useML(t, srv.URL)
	userID, frontID := setup(t, 100000)

	for _, amount := range []float64{3000, 4000} {
		if _, err := Launder(context.Background(), userID, frontID, amount); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := rec.get()
	var sent mlclient.LaunderingInput
	_ = json.Unmarshal(raw, &sent)
	if len(sent.Amounts) != 2 || sent.Amounts[0] != 3000 || sent.Amounts[1] != 4000 || len(sent.TimeGaps) != 1 {
		t.Errorf("the second request must contain the first transfer: %+v", sent)
	}
}

func TestFallsBackToLocalAnalysis(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 1_000_000)

	broken := map[string]struct {
		status int
		body   string
	}{
		"ML is down":        {500, `boom`},
		"ML rejects us":     {401, `{"detail":"nope"}`},
		"ML sends nonsense": {200, `{"risk_score":500,"anomaly_detected":false,"suspicious_patterns":[],"recommendation":"","revenue_ratio":null}`},
		"ML sends garbage":  {200, `<html>`},
	}
	for name, c := range broken {
		srv, _ := fakeML(t, c.status, c.body)
		useML(t, srv.URL)
		res, err := Launder(context.Background(), userID, frontID, 1000)
		if err != nil || res.Analysis.Source != "local" {
			t.Errorf("%s: %+v %v", name, res.Analysis, err)
		}
	}
	mlclient.SetDefault(nil)
	if res, err := Launder(context.Background(), userID, frontID, 1000); err != nil || res.Analysis.Source != "local" {
		t.Errorf("ML disabled: %+v %v", res, err)
	}
}

// ---------------------------------------------------------------------- history

func TestRecentHistoryIsPerFrontLaunderedAndWindowed(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 0)
	other, err := empire.AddFront(userID, "Second Front", "bar")
	if err != nil {
		t.Fatal(err)
	}
	empireID := loadEmpire(t, userID).ID

	start := time.Now().Add(-48 * time.Hour)
	for i := 0; i < 25; i++ {
		row := models.Transaction{EmpireID: empireID, Amount: float64(1000 + i), Type: models.TxLaundered,
			BusinessID: frontID, Base: models.Base{CreatedAt: start.Add(time.Duration(i) * time.Minute)}}
		if err := db.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.DB.Create(&models.Transaction{EmpireID: empireID, Amount: 5, Type: models.TxLaundered, BusinessID: other.ID})
	db.DB.Create(&models.Transaction{EmpireID: empireID, Amount: 6, Type: models.TxDirty, BusinessID: frontID})

	history, err := recentHistory(db.DB, empireID, frontID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != HistoryWindow {
		t.Fatalf("expected %d rows, got %d", HistoryWindow, len(history))
	}
	if history[0].Amount != 1006 || history[len(history)-1].Amount != 1024 {
		t.Errorf("the newest %d, oldest first: first=%v last=%v", HistoryWindow, history[0].Amount, history[len(history)-1].Amount)
	}
	for _, h := range history {
		if h.BusinessID != frontID || h.Type != models.TxLaundered {
			t.Errorf("foreign row in the history: %+v", h)
		}
	}
}

// ----------------------------------------------------------------------- ledger

func TestLedgerPagingFilteringAndIsolation(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 1_000_000)
	stranger, strangerFront := setup(t, 100000)

	for _, amount := range []float64{1000, 2000, 3000, 4000, 60000} {
		if _, err := Launder(context.Background(), userID, frontID, amount); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Launder(context.Background(), stranger, strangerFront, 7777); err != nil {
		t.Fatal(err)
	}

	all, err := ListTransactions(userID, 50, 0, nil)
	if err != nil || all.Total != 5 || len(all.Items) != 5 {
		t.Fatalf("all: %+v %v", all, err)
	}
	if all.Items[0].Amount != 60000 || all.Items[4].Amount != 1000 {
		t.Errorf("newest first expected: %v ... %v", all.Items[0].Amount, all.Items[4].Amount)
	}
	for _, item := range all.Items {
		if item.Amount == 7777 {
			t.Error("another player's transfer leaked into the ledger")
		}
	}

	page, _ := ListTransactions(userID, 2, 1, nil)
	if page.Total != 5 || len(page.Items) != 2 || page.Items[0].Amount != 4000 || page.Limit != 2 || page.Offset != 1 {
		t.Errorf("page: %+v", page)
	}
	yes, no := true, false
	flagged, _ := ListTransactions(userID, 50, 0, &yes)
	if flagged.Total != 1 || flagged.Items[0].Amount != 60000 {
		t.Errorf("flagged: %+v", flagged)
	}
	clean, _ := ListTransactions(userID, 50, 0, &no)
	if clean.Total != 4 {
		t.Errorf("not flagged: %+v", clean)
	}

	empty := newUser(t)
	if page, err := ListTransactions(empty, 50, 0, nil); err != nil || page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
		t.Errorf("an empty ledger is an empty list, not null: %+v %v", page, err)
	}
	for _, bad := range [][2]int{{0, 0}, {201, 0}, {-5, 0}, {50, -1}} {
		if _, err := ListTransactions(userID, bad[0], bad[1], nil); !errors.Is(err, ErrInvalidPage) {
			t.Errorf("limit %d offset %d: got %v", bad[0], bad[1], err)
		}
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

func TestLaunderEndpoint(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 100000)
	r := router(userID)

	for _, tc := range []struct {
		body, want string
		status     int
	}{
		{`{}`, "amount is required; business_id is required", 400},
		{fmt.Sprintf(`{"amount":99,"business_id":%d}`, frontID), "amount must be at least 100", 400},
		{fmt.Sprintf(`{"amount":-5000,"business_id":%d}`, frontID), "amount must be at least 100", 400},
		{fmt.Sprintf(`{"amount":1e9,"business_id":%d}`, frontID), "amount must be at most 100000000", 400},
		{fmt.Sprintf(`{"amount":"lots","business_id":%d}`, frontID), "amount must be a number", 400},
		{`{"amount":5000}`, "business_id is required", 400},
		{`{"amount":5000,"business_id":0}`, "business_id is required", 400},
		{`{"amount":5000,"business_id":-3}`, "business_id must be a number", 400},
		{`{"amount":5000,"business_id":999999999}`, "front business not found", 404},
		{fmt.Sprintf(`{"amount":500000,"business_id":%d}`, frontID), "insufficient dirty money", 400},
	} {
		status, e := call(t, r, "POST", "/api/v1/launder", tc.body)
		if status != tc.status || e.Error != tc.want || e.Success {
			t.Errorf("%s -> %d %q, want %d %q", tc.body, status, e.Error, tc.status, tc.want)
		}
	}

	status, e := call(t, r, "POST", "/api/v1/launder", fmt.Sprintf(`{"amount":20000,"business_id":%d}`, frontID))
	var res Result
	_ = json.Unmarshal(e.Data, &res)
	if status != 200 || e.Message != "Money's clean, Mr. White" || res.CleanedAmount != 15000 || res.Fee != 5000 ||
		res.Transaction.Type != "laundered" || res.Empire == nil || res.Empire.CleanMoney != 15000 ||
		res.Analysis.Source != "local" || res.Front.Revenue != 15000 || !strings.Contains(string(e.Data), `"cleaned_amount":15000`) {
		t.Errorf("success: %d %s", status, e.Data)
	}
}

func TestTransactionsEndpoint(t *testing.T) {
	openTestDB(t)
	userID, frontID := setup(t, 200000)
	r := router(userID)
	for _, amount := range []float64{1000, 2000, 60000} {
		if status, e := call(t, r, "POST", "/api/v1/launder", fmt.Sprintf(`{"amount":%v,"business_id":%d}`, amount, frontID)); status != 200 {
			t.Fatalf("setup transfer: %d %s", status, e.Error)
		}
	}

	status, e := call(t, r, "GET", "/api/v1/transactions", "")
	var page Page
	_ = json.Unmarshal(e.Data, &page)
	if status != 200 || e.Message != "Transaction ledger" || page.Total != 3 || len(page.Items) != 3 || page.Limit != 50 {
		t.Errorf("list: %d %s", status, e.Data)
	}
	_, e = call(t, r, "GET", "/api/v1/transactions?limit=1&offset=1", "")
	_ = json.Unmarshal(e.Data, &page)
	if page.Total != 3 || len(page.Items) != 1 || page.Items[0].Amount != 2000 {
		t.Errorf("paging: %s", e.Data)
	}
	_, e = call(t, r, "GET", "/api/v1/transactions?flagged=true", "")
	_ = json.Unmarshal(e.Data, &page)
	if page.Total != 1 || page.Items[0].Amount != 60000 {
		t.Errorf("flagged filter: %s", e.Data)
	}

	for q, want := range map[string]string{
		"?limit=0":       "limit must be a number from 1 to 200",
		"?limit=201":     "limit must be a number from 1 to 200",
		"?limit=abc":     "limit must be a number from 1 to 200",
		"?offset=-1":     "offset must be 0 or more",
		"?offset=x":      "offset must be 0 or more",
		"?flagged=maybe": "flagged must be true or false",
	} {
		if status, e := call(t, r, "GET", "/api/v1/transactions"+q, ""); status != 400 || e.Error != want {
			t.Errorf("%s -> %d %q, want 400 %q", q, status, e.Error, want)
		}
	}
}
