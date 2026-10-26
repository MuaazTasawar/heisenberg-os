package empire

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (±%v)", name, got, want, tol)
	}
}

func TestValidateEffort(t *testing.T) {
	for _, ok := range []float64{1, 5.5, 8.33, 10} {
		if err := ValidateEffort(ok); err != nil {
			t.Errorf("%v should be valid: %v", ok, err)
		}
	}
	for _, bad := range []float64{0, 0.99, 10.01, -3, 1000, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := ValidateEffort(bad); !errors.Is(err, ErrInvalidEffort) {
			t.Errorf("%v should be rejected, got %v", bad, err)
		}
	}
}

func TestCookOutcomeKnownValues(t *testing.T) {
	// effort 5 at purity 50, no territories: x = 0.5, yield = 25 * 1 * 0.5 * (1 - 0.3) = 8.75
	o := CookOutcome(5, 50, 0)
	near(t, "yield", o.Yield, 8.75, 1e-9)
	near(t, "dirty money", o.Dirty, 13125, 1e-9)
	near(t, "heat", o.Heat, 12.5, 1e-9)
	near(t, "purity delta", o.PurityDelta, 0.6, 1e-9)

	// full effort is wasteful: x = 1 -> 25 * 0.4 = 10 kg for 25 heat
	o = CookOutcome(10, 50, 0)
	near(t, "yield at 10", o.Yield, 10, 1e-9)
	near(t, "heat at 10", o.Heat, 25, 1e-9)
	near(t, "purity delta at 10", o.PurityDelta, -0.8, 1e-9)
}

func TestTheBestEffortIsInsideTheRange(t *testing.T) {
	near(t, "OptimalEffort", OptimalEffort(), 8.3333, 1e-3)

	bestEffort, bestYield := 0.0, 0.0
	for e := 1.0; e <= 10.0; e += 0.01 {
		if y := YieldFor(e, 50, 0); y > bestYield {
			bestEffort, bestYield = e, y
		}
	}
	near(t, "brute-force best effort", bestEffort, 8.33, 0.02)
	if YieldFor(10, 50, 0) >= YieldFor(8.33, 50, 0) {
		t.Error("rushing at full effort must yield less than the best effort")
	}
	if YieldFor(5, 50, 0) >= YieldFor(8.33, 50, 0) {
		t.Error("the best effort must beat a relaxed one")
	}
}

func TestYieldGrowsWithPurityAndTerritories(t *testing.T) {
	base := YieldFor(5, 50, 0)
	if YieldFor(5, 90, 0) <= base {
		t.Error("purer product should yield more")
	}
	near(t, "4 territories = +20%", YieldFor(5, 50, 4), base*1.2, 1e-9)
	near(t, "10 territories = +50%", YieldFor(5, 50, 10), base*1.5, 1e-9)
}

// The Python ML service uses the same formula; this pins the contract between them:
// capacity * supply% * (1 + (purity-50)/200) * x * (1 - 0.6x).
func TestYieldMatchesTheMLServiceFormula(t *testing.T) {
	for _, purity := range []float64{20, 50, 75, 99.1} {
		for _, effort := range []float64{1, 4.2, 8.33, 10} {
			for _, territories := range []int{0, 3, 10} {
				x := effort / 10
				ml := Capacity(territories) * (100.0 / 100) * (1 + (purity-50)/200) * x * (1 - 0.6*x)
				near(t, "yield", YieldFor(effort, purity, territories), ml, 1e-9)
			}
		}
	}
}

func TestPurityDelta(t *testing.T) {
	for effort, want := range map[float64]float64{0: 0.6, 1: 0.6, 5: 0.6, 7: 0.6, 8.5: 0.6 - 1.5*1.4/3, 10: -0.8} {
		near(t, "delta", PurityDelta(effort), want, 1e-9)
	}
	near(t, "clamped below", PurityDelta(-4), 0.6, 1e-9)
	near(t, "clamped above", PurityDelta(40), -0.8, 1e-9)

	prev := math.Inf(1)
	for e := 0.0; e <= 10; e += 0.1 {
		if d := PurityDelta(e); d > prev+1e-12 {
			t.Fatalf("purity delta must never rise with effort (effort %v)", e)
		} else {
			prev = d
		}
	}
}

func TestPurityStaysInRange(t *testing.T) {
	near(t, "floor", PurityAfter(20.1, 10), MinPurity, 1e-9)
	near(t, "ceiling", PurityAfter(99.0, 1), MaxPurity, 1e-9)
	near(t, "normal", PurityAfter(50, 1), 50.6, 1e-9)
	near(t, "rushed", PurityAfter(50, 10), 49.2, 1e-9)
}

func TestOutcomeIsRoundedToCents(t *testing.T) {
	o := CookOutcome(3.3333, 61.7, 3)
	for name, v := range map[string]float64{"yield": o.Yield, "dirty": o.Dirty, "heat": o.Heat, "purity": o.PurityDelta} {
		if v != round2(v) {
			t.Errorf("%s is not rounded: %v", name, v)
		}
	}
}

func TestHeatClampAndDecay(t *testing.T) {
	near(t, "clamp high", ClampHeat(180), 100, 0)
	near(t, "clamp low", ClampHeat(-5), 0, 0)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }

	heat, stamp := DecayHeat(50, nil, now)
	if heat != 50 || !stamp.Equal(now) {
		t.Errorf("no previous timestamp: %v %v", heat, stamp)
	}
	for _, c := range []struct {
		name string
		ago  time.Duration
		from float64
		want float64
	}{
		{"one hour", time.Hour, 50, 44},
		{"two and a half hours", 150 * time.Minute, 50, 35},
		{"ten minutes", 10 * time.Minute, 20, 19},
		{"long enough to cool completely", 100 * time.Hour, 50, 0},
		{"already cold", 5 * time.Hour, 0, 0},
		{"nothing elapsed", 0, 30, 30},
	} {
		got, _ := DecayHeat(c.from, at(c.ago), now)
		near(t, c.name, got, c.want, 1e-9)
	}

	future := now.Add(time.Hour)
	if got, _ := DecayHeat(50, &future, now); got != 50 {
		t.Errorf("a clock that went backwards must not decay or heat anything, got %v", got)
	}
	if got, _ := DecayHeat(250, nil, now); got != 100 {
		t.Errorf("out-of-range heat is clamped, got %v", got)
	}
}

func TestTerritoryCost(t *testing.T) {
	for current, want := range map[int]float64{0: 25000, 1: 50000, 4: 125000, 9: 250000} {
		near(t, "cost", TerritoryCost(current), want, 0)
	}
}

func TestFrontTypeCatalog(t *testing.T) {
	if len(FrontTypes) != 6 {
		t.Fatalf("expected 6 front types, got %d", len(FrontTypes))
	}
	seen := map[string]bool{}
	for _, ft := range FrontTypes {
		if ft.Type == "" || ft.Label == "" || ft.Description == "" || ft.Suspicion <= 0 || seen[ft.Type] {
			t.Errorf("bad catalog entry: %+v", ft)
		}
		seen[ft.Type] = true
	}
	for _, name := range []string{"restaurant", " Restaurant ", "CAR_WASH", "nail_salon"} {
		if _, ok := FrontTypeByName(name); !ok {
			t.Errorf("%q should be found", name)
		}
	}
	for _, name := range []string{"", "casino", "car wash", "restaurant; drop table"} {
		if _, ok := FrontTypeByName(name); ok {
			t.Errorf("%q should not be found", name)
		}
	}
}

func TestNormalizeFrontName(t *testing.T) {
	good := map[string]string{
		"Los Pollos Hermanos":       "Los Pollos Hermanos",
		"  Lavandería   Brillante ": "Lavandería Brillante",
		"AB":                        "AB",
		strings.Repeat("x", 40):     strings.Repeat("x", 40),
	}
	for in, want := range good {
		if got, err := NormalizeFrontName(in); err != nil || got != want {
			t.Errorf("%q -> %q, %v (want %q)", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "a", strings.Repeat("x", 41), "bad\x00name", "<script>x</script>"} {
		if _, err := NormalizeFrontName(bad); !errors.Is(err, ErrInvalidFront) {
			t.Errorf("%q should be rejected, got %v", bad, err)
		}
	}
}

func TestFrontNameWhitespaceIsNormalisedNotRejected(t *testing.T) {
	got, err := NormalizeFrontName("tab\tand\nnewline   spaces")
	if err != nil || got != "tab and newline spaces" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestLocalAdviceAgreesWithTheGame(t *testing.T) {
	a := LocalAdvice(50, 0, 5)
	if a.Source != "local" {
		t.Errorf("source = %q", a.Source)
	}
	near(t, "predicted equals what a cook really yields", a.PredictedYield, CookOutcome(5, 50, 0).Yield, 0.01)
	near(t, "optimal effort", a.OptimalEffort, 8.33, 0.01)
	near(t, "optimal yield", a.OptimalYield, 10.42, 0.01)
	near(t, "efficiency", a.EfficiencyScore, 8.75/10.4167*100, 0.05)
	near(t, "purity projection", a.PurityProjection, 49.98, 0.01)

	if best := LocalAdvice(50, 0, OptimalEffort()); best.EfficiencyScore < 99.99 || best.EfficiencyScore > 100 {
		t.Errorf("efficiency at the best effort should be 100, got %v", best.EfficiencyScore)
	}
	if LocalAdvice(50, 0, 10).EfficiencyScore >= LocalAdvice(50, 0, 8.33).EfficiencyScore {
		t.Error("full effort must be less efficient than the best effort")
	}
}
