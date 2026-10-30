package laundering

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/heisenberg-os/backend/internal/models"
)

const (
	// LaunderRate is the share of dirty money that comes out clean (the rest is the fee).
	LaunderRate = 0.75
	MinAmount   = 100.0
	MaxAmount   = 100_000_000.0

	// HeatPerDollar is the heat a transfer adds: one point per $100,000.
	HeatPerDollar = 1.0 / 100_000

	// LargeTransfer flags any single transfer above this amount.
	LargeTransfer = 50_000.0
	// FlagRisk flags a transfer when the analysis scores it at least this risky.
	FlagRisk = 60.0

	// Front business suspicion. A transfer is measured against the front's revenue so far
	// (at least BaselineRevenue): moving a lot of money through a front that has earned
	// little looks wrong, moving it through an established one does not.
	BaselineRevenue  = 20_000.0
	SuspicionScale   = 10.0
	MaxSuspicionGain = 30.0
	RiskBump         = 0.1 // extra suspicion per risk point from the analysis
	RaidThreshold    = 100.0
	RaidHeat         = 10.0

	// HistoryWindow is how many earlier transfers through the same front are analysed
	// together with the new one.
	HistoryWindow = 19

	structuringSpread  = 100.0
	highFrequencyHours = 1.0
	revenueRatioLimit  = 0.5
	smallBusinessLimit = 20_000.0
)

var (
	ErrInvalidAmount     = errors.New("amount must be between $100 and $100,000,000")
	ErrInsufficientDirty = errors.New("insufficient dirty money")
	ErrInvalidPage       = errors.New("invalid paging parameters")
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func ValidateAmount(amount float64) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < MinAmount || amount > MaxAmount {
		return ErrInvalidAmount
	}
	return nil
}

// CleanedAmount is what comes out of laundering the given dirty amount.
func CleanedAmount(amount float64) float64 { return round2(amount * LaunderRate) }

// HeatFor is the heat a transfer adds.
func HeatFor(amount float64) float64 { return round2(amount * HeatPerDollar) }

// SuspicionGain is the attention one transfer draws to its front business. typeFactor is the
// front type's multiplier (lower is safer) and risk the analysis score (0-100).
func SuspicionGain(amount, frontRevenue, typeFactor, risk float64) float64 {
	base := math.Max(frontRevenue, BaselineRevenue)
	gain := math.Min(amount/base*SuspicionScale*typeFactor, MaxSuspicionGain)
	return round2(gain + risk*RiskBump)
}

// ------------------------------------------------------------------- analysis

// Analysis is the verdict on a transfer. Source is "ml" when the ML service produced it and
// "local" when the built-in rules did (the local rules cannot detect statistical anomalies).
type Analysis struct {
	Source         string   `json:"source"`
	RiskScore      float64  `json:"risk_score"`
	Anomaly        bool     `json:"anomaly"`
	Patterns       []string `json:"patterns"`
	Recommendation string   `json:"recommendation"`
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func spread(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m, sum := mean(values), 0.0
	for _, v := range values {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(values)))
}

func maxOf(values []float64) float64 {
	best := math.Inf(-1)
	for _, v := range values {
		best = math.Max(best, v)
	}
	return best
}

// LocalAnalysis applies the same rules as the ML service's pattern checks, without its
// statistical anomaly model. gaps are the hours between consecutive transfers.
func LocalAnalysis(amounts, gaps []float64, revenue float64, businessType string) Analysis {
	a := Analysis{Source: "local", Patterns: []string{}}
	if len(amounts) == 0 {
		a.Recommendation = "No data"
		return a
	}
	avg := mean(amounts)

	if maxOf(amounts) > LargeTransfer {
		a.Patterns = append(a.Patterns, "Large single transaction detected")
	}
	if len(amounts) > 1 && spread(amounts) < structuringSpread {
		a.Patterns = append(a.Patterns, "Structuring pattern — identical amounts")
	}
	if len(gaps) > 0 && mean(gaps) < highFrequencyHours {
		a.Patterns = append(a.Patterns, "High-frequency transactions")
	}
	if t := strings.ToLower(businessType); (t == "car_wash" || t == "nail_salon") && avg > smallBusinessLimit {
		a.Patterns = append(a.Patterns, "Revenue inconsistent with business type")
	}
	if revenue > 0 {
		if ratio := avg / revenue; ratio > revenueRatioLimit {
			a.Patterns = append(a.Patterns, fmt.Sprintf("Laundered volume is %.0f%% of declared revenue", ratio*100))
		}
	}

	a.RiskScore = math.Min(100, float64(len(a.Patterns))*20)
	if a.RiskScore > 50 {
		a.Recommendation = "Reduce transaction sizes"
	} else {
		a.Recommendation = "Patterns look normal"
	}
	return a
}

// BuildSeries turns earlier transfers (oldest first) plus the new amount into the amount and
// time-gap series the analysis works on. Gaps are in hours and never negative; the last gap
// runs from the previous transfer to now.
func BuildSeries(history []models.Transaction, amount float64, now time.Time) (amounts, gaps []float64) {
	amounts = make([]float64, 0, len(history)+1)
	gaps = []float64{}
	for i, h := range history {
		amounts = append(amounts, h.Amount)
		if i > 0 {
			gaps = append(gaps, math.Max(0, h.CreatedAt.Sub(history[i-1].CreatedAt).Hours()))
		}
	}
	if n := len(history); n > 0 {
		gaps = append(gaps, math.Max(0, now.Sub(history[n-1].CreatedAt).Hours()))
	}
	return append(amounts, amount), gaps
}

// FlagReasons lists why a transfer is flagged for the DEA; empty means it is not.
func FlagReasons(amount float64, a Analysis) []string {
	var reasons []string
	if amount > LargeTransfer {
		reasons = append(reasons, fmt.Sprintf("large transfer ($%.0f is over $%.0f)", amount, LargeTransfer))
	}
	if a.RiskScore >= FlagRisk {
		detail := a.Recommendation
		if len(a.Patterns) > 0 {
			detail = strings.Join(a.Patterns, ", ")
		}
		reasons = append(reasons, fmt.Sprintf("analysis risk %.0f%%: %s", a.RiskScore, detail))
	}
	if a.Anomaly {
		reasons = append(reasons, "unusual activity pattern")
	}
	return reasons
}
