package mlclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heisenberg-os/backend/internal/config"
)

func clientFor(url string, apiKey string, timeout time.Duration) *Client {
	cfg := config.Defaults()
	cfg.MLServiceURL = url
	cfg.MLAPIKey = apiKey
	cfg.MLTimeout = timeout
	return New(cfg)
}

func TestDisabledClient(t *testing.T) {
	cfg := config.Defaults()
	cfg.MLServiceURL = ""
	for name, c := range map[string]*Client{"from config": New(cfg), "zero value": {}, "default": Default()} {
		if c.Enabled() {
			t.Errorf("%s: should be disabled", name)
		}
		if _, err := c.AnalyzeThreat(context.Background(), ThreatInput{}); !errors.Is(err, ErrDisabled) {
			t.Errorf("%s: got %v, want ErrDisabled", name, err)
		}
		if err := c.Health(context.Background()); !errors.Is(err, ErrDisabled) {
			t.Errorf("%s: health got %v, want ErrDisabled", name, err)
		}
	}
	var nilClient *Client
	if nilClient.Enabled() {
		t.Error("a nil client must report disabled")
	}
}

func TestAnalyzeThreat(t *testing.T) {
	var seen ThreatInput
	var key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("X-API-Key")
		if r.Method != http.MethodPost || r.URL.Path != "/ml/threat/analyze" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("bad request: %s %s %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&seen)
		w.Write([]byte(`{"threat_score":42.5,"risk_level":"medium","prediction":"p","recommendation":"r"}`))
	}))
	defer srv.Close()

	c := clientFor(srv.URL+"/", "secret-key", time.Second) // trailing slash must be tolerated
	got, err := c.AnalyzeThreat(context.Background(), ThreatInput{EmpireID: 3, HeatLevel: 61.5, FlaggedTransactions: 4,
		BatchYield: 120, TerritoryCount: 2, DaysActive: 9})
	if err != nil {
		t.Fatal(err)
	}
	if got.ThreatScore != 42.5 || got.RiskLevel != "medium" || got.Prediction != "p" || got.Recommendation != "r" {
		t.Errorf("response not decoded: %+v", got)
	}
	if seen.EmpireID != 3 || seen.HeatLevel != 61.5 || seen.FlaggedTransactions != 4 || seen.BatchYield != 120 ||
		seen.TerritoryCount != 2 || seen.DaysActive != 9 {
		t.Errorf("request fields wrong: %+v", seen)
	}
	if key != "secret-key" {
		t.Errorf("API key header = %q", key)
	}
}

func TestNoKeyHeaderWhenNoneConfigured(t *testing.T) {
	var has bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, has = r.Header["X-Api-Key"]
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, err := clientFor(srv.URL, "", time.Second).OptimizeYield(context.Background(), YieldInput{}); err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("no X-API-Key header should be sent without a configured key")
	}
}

func TestOptimizeYieldAndLaundering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ml/yield/optimize":
			var in YieldInput
			json.NewDecoder(r.Body).Decode(&in)
			if in.EffortLevel != 70 || in.LabCapacity != 100 {
				t.Errorf("yield input wrong: %+v", in)
			}
			w.Write([]byte(`{"predicted_yield":30.7,"optimal_effort":83.33,"optimal_yield":33.3,"purity_projection":50.1,"efficiency_score":92}`))
		case "/ml/laundering/analyze":
			var in LaunderingInput
			json.NewDecoder(r.Body).Decode(&in)
			if len(in.Amounts) != 2 || in.BusinessType != "car_wash" {
				t.Errorf("laundering input wrong: %+v", in)
			}
			w.Write([]byte(`{"risk_score":80,"anomaly_detected":true,"suspicious_patterns":["a","b"],"recommendation":"x","revenue_ratio":0.9}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := clientFor(srv.URL, "", time.Second)

	y, err := c.OptimizeYield(context.Background(), YieldInput{CurrentPurity: 50, EffortLevel: 70, SupplyQuality: 80, LabCapacity: 100})
	if err != nil || y.OptimalEffort != 83.33 || y.PurityProjection != 50.1 || y.EfficiencyScore != 92 {
		t.Errorf("yield: %+v %v", y, err)
	}
	l, err := c.AnalyzeLaundering(context.Background(), LaunderingInput{Amounts: []float64{1, 2}, BusinessType: "car_wash"})
	if err != nil || !l.AnomalyDetected || len(l.SuspiciousPatterns) != 2 || l.RevenueRatio == nil || *l.RevenueRatio != 0.9 {
		t.Errorf("laundering: %+v %v", l, err)
	}
}

func TestNullRevenueRatioDecodesToNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"risk_score":0,"anomaly_detected":false,"suspicious_patterns":[],"recommendation":"No data","revenue_ratio":null}`))
	}))
	defer srv.Close()
	l, err := clientFor(srv.URL, "", time.Second).AnalyzeLaundering(context.Background(), LaunderingInput{})
	if err != nil || l.RevenueRatio != nil {
		t.Errorf("got %+v %v", l, err)
	}
}

func TestEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/ml/evidence":
			var in EvidenceInput
			json.NewDecoder(r.Body).Decode(&in)
			if in.EmpireID != 9 || in.Source != "tx:5" || in.Weight != 2.5 || in.Kind != "transaction" {
				t.Errorf("evidence input wrong: %+v", in)
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"empire_id":9,"node_count":1,"total_weight":2.5,"top_sources":[{"source":"tx:5","kind":"transaction","weight":2.5}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ml/evidence/9":
			w.Write([]byte(`{"empire_id":9,"node_count":1,"total_weight":2.5,"top_sources":[{"source":"tx:5","kind":"transaction","weight":2.5}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := clientFor(srv.URL, "", time.Second)

	added, err := c.AddEvidence(context.Background(), EvidenceInput{EmpireID: 9, Source: "tx:5", Weight: 2.5, Kind: "transaction"})
	if err != nil || added.NodeCount != 1 || added.TotalWeight != 2.5 || added.TopSources[0].Source != "tx:5" {
		t.Errorf("add: %+v %v", added, err)
	}
	got, err := c.GetEvidence(context.Background(), 9)
	if err != nil || got.EmpireID != 9 || len(got.TopSources) != 1 {
		t.Errorf("get: %+v %v", got, err)
	}
}

func TestClientErrorsAreNotOutages(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"detail":"invalid"}`, http.StatusUnprocessableEntity)
	}))
	defer srv.Close()
	c := clientFor(srv.URL, "", time.Second)

	for i := 0; i < 10; i++ {
		_, err := c.AnalyzeThreat(context.Background(), ThreatInput{})
		if err == nil || errors.Is(err, ErrUnavailable) {
			t.Fatalf("a 422 should be a plain error, got %v", err)
		}
	}
	if atomic.LoadInt32(&calls) != 10 {
		t.Errorf("the breaker must not open for 4xx answers (%d calls)", calls)
	}
}

func TestBreakerOpensAfterRepeatedFailuresAndRecovers(t *testing.T) {
	var calls int32
	var healthy atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if !healthy.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"threat_score":1,"risk_level":"low","prediction":"","recommendation":""}`))
	}))
	defer srv.Close()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := clientFor(srv.URL, "", time.Second)
	c.now = func() time.Time { return now }
	call := func() error { _, err := c.AnalyzeThreat(context.Background(), ThreatInput{}); return err }

	for i := 0; i < breakerThreshold; i++ {
		if err := call(); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	before := atomic.LoadInt32(&calls)
	for i := 0; i < 5; i++ {
		if err := call(); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("while open: %v", err)
		}
	}
	if atomic.LoadInt32(&calls) != before {
		t.Error("an open breaker must not call the service")
	}

	// cooldown passes, service still down: one probe, then the breaker closes the door again
	now = now.Add(breakerCooldown + time.Second)
	if err := call(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("probe: %v", err)
	}
	if atomic.LoadInt32(&calls) != before+1 {
		t.Errorf("exactly one probe expected, calls=%d (before=%d)", calls, before)
	}
	if err := call(); !errors.Is(err, ErrUnavailable) || atomic.LoadInt32(&calls) != before+1 {
		t.Error("a failed probe must re-open the breaker immediately")
	}

	// service recovers
	healthy.Store(true)
	now = now.Add(breakerCooldown + time.Second)
	if err := call(); err != nil {
		t.Fatalf("recovered call: %v", err)
	}
	if err := call(); err != nil {
		t.Errorf("breaker should be closed after a success: %v", err)
	}
}

func TestTimeoutCountsAsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := clientFor(srv.URL, "", 50*time.Millisecond).AnalyzeThreat(context.Background(), ThreatInput{})
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("got %v, want ErrUnavailable", err)
	}
}

func TestCancelledContextDoesNotTripTheBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	c := clientFor(srv.URL, "", 2*time.Second)

	for i := 0; i < breakerThreshold+2; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.AnalyzeThreat(ctx, ThreatInput{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	}
	if !c.allow() {
		t.Error("caller cancellations must not open the breaker")
	}
}

func TestUnreachableServiceAndGarbageResponses(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	if _, err := clientFor(url, "", 200*time.Millisecond).AnalyzeThreat(context.Background(), ThreatInput{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unreachable: got %v", err)
	}

	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) }))
	defer garbage.Close()
	if _, err := clientFor(garbage.URL, "", time.Second).AnalyzeThreat(context.Background(), ThreatInput{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("garbage: got %v", err)
	}
}

func TestHealthChecksTheProtectedEndpointSoAWrongKeyIsNoticed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ml/evidence/0" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-API-Key") != "right" {
			http.Error(w, `{"detail":"invalid or missing API key"}`, http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"empire_id":0,"node_count":0,"total_weight":0,"top_sources":[]}`))
	}))
	defer srv.Close()

	if err := clientFor(srv.URL, "right", time.Second).Health(context.Background()); err != nil {
		t.Errorf("right key: %v", err)
	}
	if err := clientFor(srv.URL, "wrong", time.Second).Health(context.Background()); err == nil {
		t.Error("a wrong key must make the health check fail")
	}
	if err := clientFor(srv.URL, "", time.Second).Health(context.Background()); err == nil {
		t.Error("a missing key must make the health check fail")
	}
}

func TestDefaultClientCanBeReplaced(t *testing.T) {
	t.Cleanup(func() { SetDefault(nil) })
	c := clientFor("http://example.invalid", "", time.Second)
	SetDefault(c)
	if Default() != c {
		t.Error("SetDefault/Default round trip failed")
	}
}
