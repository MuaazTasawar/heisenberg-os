package laundering

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/heisenberg-os/backend/internal/models"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (±%v)", name, got, want, tol)
	}
}

func TestValidateAmount(t *testing.T) {
	for _, ok := range []float64{100, 100.01, 12345.67, 50000, MaxAmount} {
		if err := ValidateAmount(ok); err != nil {
			t.Errorf("%v should be valid: %v", ok, err)
		}
	}
	// A negative amount used to create dirty money out of clean money; it must never pass.
	for _, bad := range []float64{0, 99.99, -1, -100000, MaxAmount + 1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := ValidateAmount(bad); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("%v should be rejected, got %v", bad, err)
		}
	}
}

func TestCleanedAmountAndHeat(t *testing.T) {
	near(t, "cleaned", CleanedAmount(100000), 75000, 0)
	near(t, "cleaned odd amount", CleanedAmount(1234.57), 925.93, 0)
	near(t, "heat for $100k", HeatFor(100000), 1, 0)
	near(t, "heat for $20k", HeatFor(20000), 0.2, 0)
}

func TestSuspicionGain(t *testing.T) {
	// a first $20,000 through a restaurant: 20000/20000 * 10 = 10, plus 20 risk points * 0.1
	near(t, "first transfer", SuspicionGain(20000, 0, 1.0, 20), 12, 1e-9)
	// the gain from size alone is capped
	near(t, "capped", SuspicionGain(5_000_000, 0, 1.0, 0), MaxSuspicionGain, 1e-9)
	// riskier front types draw more attention, safer ones less
	if !(SuspicionGain(20000, 0, 0.8, 0) < SuspicionGain(20000, 0, 1.0, 0) &&
		SuspicionGain(20000, 0, 1.0, 0) < SuspicionGain(20000, 0, 1.3, 0)) {
		t.Error("the type factor must scale the gain")
	}
	// an established front can take bigger transfers without raising eyebrows
	if SuspicionGain(20000, 400000, 1.0, 0) >= SuspicionGain(20000, 0, 1.0, 0) {
		t.Error("a front with more revenue behind it should draw less suspicion")
	}
	// the analysis score always adds on top
	near(t, "risk bump", SuspicionGain(20000, 0, 1.0, 100)-SuspicionGain(20000, 0, 1.0, 0), 10, 1e-9)
	if SuspicionGain(1, 0, 1.0, 0) < 0 {
		t.Error("suspicion can never go down from a transfer")
	}
}

func has(a Analysis, text string) bool {
	for _, p := range a.Patterns {
		if strings.Contains(p, text) {
			return true
		}
	}
	return false
}

func TestLocalAnalysisPatterns(t *testing.T) {
	t.Run("no data", func(t *testing.T) {
		a := LocalAnalysis(nil, nil, 20000, "restaurant")
		if a.RiskScore != 0 || a.Recommendation != "No data" || a.Source != "local" || a.Patterns == nil {
			t.Errorf("%+v", a)
		}
	})
	t.Run("a normal transfer through an established front", func(t *testing.T) {
		a := LocalAnalysis([]float64{4000}, []float64{}, 80000, "restaurant")
		if a.RiskScore != 0 || len(a.Patterns) != 0 || a.Recommendation != "Patterns look normal" {
			t.Errorf("%+v", a)
		}
	})
	t.Run("large single transfer, strictly above the limit", func(t *testing.T) {
		if has(LocalAnalysis([]float64{50000}, nil, 1e9, "bar"), "Large single") {
			t.Error("exactly $50,000 is not large")
		}
		if !has(LocalAnalysis([]float64{50000.01}, nil, 1e9, "bar"), "Large single") {
			t.Error("just over $50,000 is large")
		}
	})
	t.Run("structuring needs two transfers and a spread under 100", func(t *testing.T) {
		if has(LocalAnalysis([]float64{9500}, nil, 1e9, "bar"), "Structuring") {
			t.Error("one transfer cannot be structuring")
		}
		if !has(LocalAnalysis([]float64{9500, 9500, 9500}, nil, 1e9, "bar"), "Structuring") {
			t.Error("identical amounts are structuring")
		}
		if !has(LocalAnalysis([]float64{9500, 9560}, nil, 1e9, "bar"), "Structuring") {
			t.Error("a spread of 30 is still structuring")
		}
		if has(LocalAnalysis([]float64{9000, 9300}, nil, 1e9, "bar"), "Structuring") {
			t.Error("a spread of 150 is not structuring")
		}
	})
	t.Run("high frequency", func(t *testing.T) {
		if !has(LocalAnalysis([]float64{1000, 2000}, []float64{0.2, 0.5}, 1e9, "bar"), "High-frequency") {
			t.Error("gaps under an hour are high frequency")
		}
		if has(LocalAnalysis([]float64{1000, 2000}, []float64{3, 5}, 1e9, "bar"), "High-frequency") {
			t.Error("gaps of hours are fine")
		}
		if has(LocalAnalysis([]float64{1000}, []float64{}, 1e9, "bar"), "High-frequency") {
			t.Error("no gaps, no pattern")
		}
	})
	t.Run("small businesses cannot move big money", func(t *testing.T) {
		for _, kind := range []string{"car_wash", "Nail_Salon"} {
			if !has(LocalAnalysis([]float64{25000}, nil, 1e9, kind), "inconsistent with business type") {
				t.Errorf("%s moving $25,000 should stand out", kind)
			}
		}
		if has(LocalAnalysis([]float64{25000}, nil, 1e9, "restaurant"), "inconsistent with business type") {
			t.Error("a restaurant can take $25,000")
		}
	})
	t.Run("revenue ratio", func(t *testing.T) {
		a := LocalAnalysis([]float64{9000}, nil, 10000, "bar")
		if !has(a, "90% of declared revenue") {
			t.Errorf("%+v", a)
		}
		if has(LocalAnalysis([]float64{5000}, nil, 10000, "bar"), "declared revenue") {
			t.Error("exactly half of revenue is not over the limit")
		}
		if has(LocalAnalysis([]float64{9000}, nil, 0, "bar"), "declared revenue") {
			t.Error("without revenue data there is no ratio")
		}
	})
}

func TestLocalAnalysisRiskScore(t *testing.T) {
	// structuring + high frequency + revenue ratio = three patterns = 60
	a := LocalAnalysis([]float64{20000, 20000}, []float64{0.1}, 20000, "bar")
	near(t, "three patterns", a.RiskScore, 60, 0)
	if a.Recommendation != "Reduce transaction sizes" {
		t.Errorf("recommendation: %q", a.Recommendation)
	}
	// every pattern at once is capped at 100
	b := LocalAnalysis([]float64{90000, 90000}, []float64{0.1}, 20000, "car_wash")
	near(t, "capped", b.RiskScore, 100, 0)
	// two patterns (40) still reads as normal
	c := LocalAnalysis([]float64{60000}, nil, 20000, "bar")
	near(t, "two patterns", c.RiskScore, 40, 0)
	if c.Recommendation != "Patterns look normal" {
		t.Errorf("recommendation: %q", c.Recommendation)
	}
}

func TestBuildSeries(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(minutesAgo int, amount float64) models.Transaction {
		return models.Transaction{Amount: amount, Base: models.Base{CreatedAt: now.Add(-time.Duration(minutesAgo) * time.Minute)}}
	}

	amounts, gaps := BuildSeries(nil, 5000, now)
	if len(amounts) != 1 || amounts[0] != 5000 || gaps == nil || len(gaps) != 0 {
		t.Errorf("no history: %v %#v", amounts, gaps)
	}

	history := []models.Transaction{at(180, 1000), at(120, 2000), at(30, 3000)}
	amounts, gaps = BuildSeries(history, 4000, now)
	if len(amounts) != 4 || amounts[0] != 1000 || amounts[3] != 4000 {
		t.Errorf("amounts: %v", amounts)
	}
	want := []float64{1, 1.5, 0.5} // 60 min, 90 min between, then 30 min until now
	if len(gaps) != 3 {
		t.Fatalf("gaps: %v", gaps)
	}
	for i := range want {
		near(t, "gap", gaps[i], want[i], 1e-9)
	}

	// a clock that disagrees must not produce negative gaps
	_, gaps = BuildSeries([]models.Transaction{at(-60, 100)}, 100, now)
	if gaps[0] != 0 {
		t.Errorf("negative gap must be clamped to 0: %v", gaps)
	}
}

func TestFlagReasons(t *testing.T) {
	calm := Analysis{RiskScore: 20, Patterns: []string{"x"}}
	if r := FlagReasons(20000, calm); len(r) != 0 {
		t.Errorf("a calm transfer is not flagged: %v", r)
	}
	if r := FlagReasons(50000, calm); len(r) != 0 {
		t.Errorf("exactly $50,000 is not flagged: %v", r)
	}
	if r := FlagReasons(60000, calm); len(r) != 1 || !strings.Contains(r[0], "large transfer") {
		t.Errorf("a large transfer is flagged: %v", r)
	}
	risky := Analysis{RiskScore: 60, Patterns: []string{"Structuring pattern", "High-frequency transactions"}}
	if r := FlagReasons(1000, risky); len(r) != 1 || !strings.Contains(r[0], "analysis risk 60%") || !strings.Contains(r[0], "Structuring pattern, High-frequency") {
		t.Errorf("a risky analysis is flagged with its patterns: %v", r)
	}
	if r := FlagReasons(1000, Analysis{RiskScore: 59.9}); len(r) != 0 {
		t.Errorf("just under the risk limit is not flagged: %v", r)
	}
	if r := FlagReasons(1000, Analysis{Anomaly: true}); len(r) != 1 || !strings.Contains(r[0], "unusual") {
		t.Errorf("an anomaly is flagged: %v", r)
	}
	both := FlagReasons(80000, Analysis{RiskScore: 80, Anomaly: true, Recommendation: "Reduce transaction sizes"})
	if len(both) != 3 {
		t.Errorf("all reasons are listed: %v", both)
	}
}
