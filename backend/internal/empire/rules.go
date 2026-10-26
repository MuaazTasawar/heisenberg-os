package empire

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ----------------------------------------------------------------- constants

const (
	MinEffort = 1.0
	MaxEffort = 10.0

	// LabCapacity is the product (kg) a cook can yield at purity 50 with perfect effort
	// before bonuses. The real yield also depends on purity, territories and effort.
	LabCapacity = 25.0
	// overheat is the share of yield wasted by rushing: yield ~ x * (1 - overheat * x),
	// where x = effort / 10. It makes the best effort sit at 1 / (2 * overheat) = 83 %.
	overheat = 0.6

	DirtyPerKg       = 1500.0
	HeatPerEffort    = 2.5
	MaxHeat          = 100.0
	HeatDecayPerHour = 6.0

	StartPurity     = 50.0
	StartBatchYield = 10.0
	MinPurity       = 20.0
	MaxPurity       = 99.1

	TerritoryBonus    = 0.05 // +5 % yield per territory
	TerritoryBaseCost = 25000.0
	TerritoryHeat     = 5.0
	MaxTerritories    = 10

	MaxActiveFronts = 5
	MinFrontNameLen = 2
	MaxFrontNameLen = 40
)

// ------------------------------------------------------------------- errors

var (
	ErrInvalidEffort     = errors.New("effort must be a number from 1 to 10")
	ErrInsufficientFunds = errors.New("insufficient clean money")
	ErrMaxTerritories    = errors.New("you already control the maximum number of territories")
	ErrInvalidFront      = errors.New("invalid front business")
	ErrFrontLimit        = errors.New("you already run the maximum number of active front businesses")
	ErrDuplicateFront    = errors.New("you already run a front business with that name")
	ErrFrontNotFound     = errors.New("front business not found")
	ErrFrontClosed       = errors.New("that front business is already closed")
)

// -------------------------------------------------------------------- cooking

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ValidateEffort accepts effort from 1 to 10 (decimals allowed).
func ValidateEffort(effort float64) error {
	if math.IsNaN(effort) || math.IsInf(effort, 0) || effort < MinEffort || effort > MaxEffort {
		return ErrInvalidEffort
	}
	return nil
}

func purityFactor(purity float64) float64 { return 1 + (purity-50)/200 }

func effortFactor(effort float64) float64 {
	x := math.Min(math.Max(effort, 0), MaxEffort) / MaxEffort
	return x * (1 - overheat*x)
}

// Capacity is the lab capacity with the territory bonus applied.
func Capacity(territories int) float64 {
	return LabCapacity * (1 + TerritoryBonus*float64(territories))
}

// YieldFor is the kilograms one cook produces.
func YieldFor(effort, purity float64, territories int) float64 {
	return Capacity(territories) * purityFactor(purity) * effortFactor(effort)
}

// PurityDelta is the purity change of one cook. Careful cooking (effort up to 7) adds
// 0.6 points; from 7 to 10 the gain falls linearly to a 0.8 point loss (rushed batches).
func PurityDelta(effort float64) float64 {
	effort = math.Min(math.Max(effort, 0), MaxEffort)
	if effort <= 7 {
		return 0.6
	}
	return 0.6 - (effort-7)*(1.4/3.0)
}

// PurityAfter applies a cook at the given effort to purity, within the allowed range.
func PurityAfter(purity, effort float64) float64 {
	return math.Min(math.Max(purity+PurityDelta(effort), MinPurity), MaxPurity)
}

// Outcome is what a cook produces before limits (heat cap, purity range) are applied.
type Outcome struct {
	Yield       float64 // kg
	Dirty       float64 // dollars
	Heat        float64
	PurityDelta float64
}

func CookOutcome(effort, purity float64, territories int) Outcome {
	yield := round2(YieldFor(effort, purity, territories))
	return Outcome{
		Yield:       yield,
		Dirty:       round2(yield * DirtyPerKg),
		Heat:        round2(effort * HeatPerEffort),
		PurityDelta: round2(PurityDelta(effort)),
	}
}

// OptimalEffort is the effort (1-10 scale) that yields the most per cook.
func OptimalEffort() float64 { return MaxEffort / (2 * overheat) }

// ------------------------------------------------------------------------ heat

func ClampHeat(h float64) float64 { return math.Min(math.Max(h, 0), MaxHeat) }

// DecayHeat cools heat down by HeatDecayPerHour for the time since last. With no previous
// timestamp there is nothing to decay yet. A clock that went backwards decays nothing.
func DecayHeat(heat float64, last *time.Time, now time.Time) (float64, time.Time) {
	if last == nil || !now.After(*last) {
		return ClampHeat(heat), now
	}
	hours := now.Sub(*last).Hours()
	return round2(ClampHeat(heat - HeatDecayPerHour*hours)), now
}

// ------------------------------------------------------------------ territories

// TerritoryCost is the clean money needed to go from `current` to current+1 territories.
func TerritoryCost(current int) float64 { return TerritoryBaseCost * float64(current+1) }

// ----------------------------------------------------------------------- fronts

// FrontType describes one kind of front business. Suspicion multiplies how fast attention
// builds when money is laundered through it (lower is safer).
type FrontType struct {
	Type        string  `json:"type"`
	Label       string  `json:"label"`
	Suspicion   float64 `json:"suspicion_factor"`
	Description string  `json:"description"`
}

var FrontTypes = []FrontType{
	{"bar", "Bar", 0.8, "Late nights, cash tips and no questions."},
	{"laundromat", "Laundromat", 0.9, "Coins in, coins out. Hard to audit."},
	{"restaurant", "Restaurant", 1.0, "Cash-heavy and forgiving. The classic."},
	{"auto_shop", "Auto shop", 1.1, "Parts, labour and plenty of paperwork to hide in."},
	{"car_wash", "Car wash", 1.2, "Steady cash, but the numbers must add up."},
	{"nail_salon", "Nail salon", 1.3, "Small tickets make big deposits stand out."},
}

func FrontTypeByName(name string) (FrontType, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, t := range FrontTypes {
		if t.Type == name {
			return t, true
		}
	}
	return FrontType{}, false
}

// NormalizeFrontName trims the name and checks its length and characters.
func NormalizeFrontName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	n := utf8.RuneCountInString(name)
	if n < MinFrontNameLen || n > MaxFrontNameLen {
		return "", fmt.Errorf("%w: the name must be %d-%d characters", ErrInvalidFront, MinFrontNameLen, MaxFrontNameLen)
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return "", fmt.Errorf("%w: the name contains characters that are not allowed", ErrInvalidFront)
		}
	}
	return name, nil
}

// --------------------------------------------------------------------- advice

// Advice compares a planned effort with the best one.
type Advice struct {
	Source           string  `json:"source"` // "ml" or "local"
	Effort           float64 `json:"effort"`
	PredictedYield   float64 `json:"predicted_yield"`
	OptimalEffort    float64 `json:"optimal_effort"`
	OptimalYield     float64 `json:"optimal_yield"`
	PurityProjection float64 `json:"purity_projection"`
	EfficiencyScore  float64 `json:"efficiency_score"`
}

// LocalAdvice answers without the ML service, using the same formulas the game uses.
func LocalAdvice(purity float64, territories int, effort float64) Advice {
	best := OptimalEffort()
	predicted := YieldFor(effort, purity, territories)
	optimal := YieldFor(best, purity, territories)
	efficiency := 0.0
	if optimal > 0 {
		efficiency = math.Min(100, predicted/optimal*100)
	}
	return Advice{
		Source:           "local",
		Effort:           effort,
		PredictedYield:   round2(predicted),
		OptimalEffort:    round2(best),
		OptimalYield:     round2(optimal),
		PurityProjection: round2(PurityAfter(purity, best)),
		EfficiencyScore:  round2(efficiency),
	}
}
