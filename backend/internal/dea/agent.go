// Package dea is the Drug Enforcement Administration's investigation of the player: how
// worried Hank should be, how that has developed over time, and what evidence he holds.
package dea

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	LeadAgent = "Hank Schrader"

	// A new history row is written only when the picture changed: a different risk level, a
	// score move of at least minScoreChange, new evidence, or snapshotEvery has passed.
	snapshotEvery  = 5 * time.Minute
	minScoreChange = 0.5
)

// maxHistory is a variable so tests can use a small number.
var maxHistory = 200

var ErrInvalidLimit = errors.New("limit must be a number from 1 to 200")

// ------------------------------------------------------------------- risk bands

type band struct {
	upper          float64
	level          string
	prediction     string
	recommendation string
}

// The same four bands (and wording) as the ML service.
var bands = []band{
	{25, "low", "Hank suspects nothing. Stay clean.", "Continue operations. Keep front businesses active."},
	{50, "medium", "Hank is sniffing around. Reduce activity.", "Pause large transactions. Increase laundering ratio."},
	{75, "high", "A case is being built. Lawyer up.", "Shut down one territory. Consult Saul immediately."},
	{math.Inf(1), "critical", "They know. It's over \u2014 unless you talk to Saul.", "Full operational freeze. Destroy evidence. Run."},
}

// Band maps a threat score (0-100) to its risk level, prediction and recommendation.
func Band(score float64) (level, prediction, recommendation string) {
	for _, b := range bands {
		if score < b.upper {
			return b.level, b.prediction, b.recommendation
		}
	}
	last := bands[len(bands)-1]
	return last.level, last.prediction, last.recommendation
}

func validLevel(level string) bool {
	for _, b := range bands {
		if b.level == level {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------- scoring

// Inputs are the facts the threat model looks at.
type Inputs struct {
	Heat       float64
	Flagged    int
	BatchYield float64
	Territory  int
	DaysActive int
}

// Verdict is the DEA's assessment. Source is "ml" or "heuristic".
type Verdict struct {
	Score          float64
	Level          string
	Prediction     string
	Recommendation string
	Source         string
}

func clamp01(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Heuristic is the built-in threat model. It is the curve the ML model is trained on, so the
// two agree to within a couple of points: a weighted "pressure" from heat, flagged
// transactions, product, territory and time, pushed through a logistic curve.
func Heuristic(in Inputs) Verdict {
	pressure := 0.35*clamp01(in.Heat/100) +
		0.30*clamp01(float64(in.Flagged)/20) +
		0.15*clamp01(in.BatchYield/1000) +
		0.10*clamp01(float64(in.Territory)/10) +
		0.10*clamp01(float64(in.DaysActive)/365)
	score := round2(100 / (1 + math.Exp(-8*(pressure-0.5))))
	level, prediction, recommendation := Band(score)
	return Verdict{Score: score, Level: level, Prediction: prediction, Recommendation: recommendation, Source: "heuristic"}
}

func fromML(res *mlclient.ThreatResult) (Verdict, bool) {
	if math.IsNaN(res.ThreatScore) || res.ThreatScore < 0 || res.ThreatScore > 100 || !validLevel(res.RiskLevel) {
		return Verdict{}, false
	}
	return Verdict{
		Score:          round2(res.ThreatScore),
		Level:          res.RiskLevel,
		Prediction:     res.Prediction,
		Recommendation: res.Recommendation,
		Source:         "ml",
	}, true
}

// assess asks the ML service and falls back to the heuristic when it cannot answer sensibly.
func assess(ctx context.Context, empireID uint, in Inputs) Verdict {
	res, err := mlclient.Default().AnalyzeThreat(ctx, mlclient.ThreatInput{
		EmpireID:            empireID,
		HeatLevel:           in.Heat,
		FlaggedTransactions: in.Flagged,
		BatchYield:          in.BatchYield,
		TerritoryCount:      in.Territory,
		DaysActive:          in.DaysActive,
	})
	if err == nil {
		if v, ok := fromML(res); ok {
			return v
		}
	}
	return Heuristic(in)
}

// ------------------------------------------------------------------ evaluation

// Evaluate works out how much the DEA knows right now and records it. The result is stored in
// the history only when something changed, so polling does not flood the table.
func Evaluate(ctx context.Context, userID uint) (*models.DEAThreat, error) {
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	var flagged int64
	if err := db.DB.Model(&models.Transaction{}).Where("empire_id = ? AND flagged = ?", emp.ID, true).Count(&flagged).Error; err != nil {
		return nil, err
	}
	days := int(time.Since(emp.CreatedAt).Hours() / 24)
	if days < 0 {
		days = 0
	}

	verdict := assess(ctx, emp.ID, Inputs{
		Heat:       emp.HeatLevel,
		Flagged:    int(flagged),
		BatchYield: emp.BatchYield,
		Territory:  emp.TerritoryCount,
		DaysActive: days,
	})
	return persist(emp.ID, int(flagged), verdict, time.Now())
}

func unchanged(last models.DEAThreat, evidence int, v Verdict, now time.Time) bool {
	return last.RiskLevel == v.Level && last.Source == v.Source && last.EvidenceCount == evidence &&
		math.Abs(last.ThreatScore-v.Score) < minScoreChange && now.Sub(last.CreatedAt) < snapshotEvery
}

// persist stores the verdict as the new active row unless nothing worth recording changed.
// Evaluations of one empire are serialised with a row lock so concurrent requests cannot
// create duplicate rows.
func persist(empireID uint, evidence int, v Verdict, now time.Time) (*models.DEAThreat, error) {
	var out *models.DEAThreat
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var lock models.Empire
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").First(&lock, empireID).Error; err != nil {
			return err
		}

		var last models.DEAThreat
		err := tx.Where("empire_id = ? AND is_active = ?", empireID, true).Order("created_at DESC, id DESC").First(&last).Error
		if err == nil && unchanged(last, evidence, v, now) {
			out = &last
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Model(&models.DEAThreat{}).Where("empire_id = ? AND is_active = ?", empireID, true).
			Update("is_active", false).Error; err != nil {
			return err
		}
		row := models.DEAThreat{
			EmpireID:       empireID,
			ThreatScore:    v.Score,
			EvidenceCount:  evidence,
			LeadAgent:      LeadAgent,
			IsActive:       true,
			RiskLevel:      v.Level,
			Prediction:     v.Prediction,
			Recommendation: v.Recommendation,
			Source:         v.Source,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}

		// keep only the newest maxHistory rows
		newest := tx.Model(&models.DEAThreat{}).Select("id").Where("empire_id = ?", empireID).
			Order("created_at DESC, id DESC").Limit(maxHistory)
		if err := tx.Unscoped().Where("empire_id = ? AND id NOT IN (?)", empireID, newest).
			Delete(&models.DEAThreat{}).Error; err != nil {
			return err
		}
		out = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// History returns the player's threat history, newest first.
func History(userID uint, limit int) ([]models.DEAThreat, error) {
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	rows := []models.DEAThreat{}
	err = db.DB.Where("empire_id = ?", emp.ID).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// -------------------------------------------------------------------- evidence

// EvidenceFile is what the DEA holds against the player.
type EvidenceFile struct {
	Source      string                    `json:"source"` // "ml" (the evidence graph) or "local" (the ledger)
	Count       int                       `json:"count"`
	TotalWeight float64                   `json:"total_weight"`
	TopSources  []mlclient.EvidenceSource `json:"top_sources"`
}

// evidenceWeight is how heavy one flagged transfer weighs in the file (same rule as laundering).
func evidenceWeight(amount float64) float64 {
	return round2(math.Min(10, math.Max(0.1, amount/50_000)))
}

func localEvidence(empireID uint) (*EvidenceFile, error) {
	file := &EvidenceFile{Source: "local", TopSources: []mlclient.EvidenceSource{}}
	base := db.DB.Model(&models.Transaction{}).Where("empire_id = ? AND flagged = ?", empireID, true)

	var count int64
	if err := base.Count(&count).Error; err != nil {
		return nil, err
	}
	file.Count = int(count)

	var total float64
	if err := db.DB.Model(&models.Transaction{}).Where("empire_id = ? AND flagged = ?", empireID, true).
		Select("COALESCE(SUM(LEAST(10, GREATEST(0.1, amount / 50000))), 0)").Scan(&total).Error; err != nil {
		return nil, err
	}
	file.TotalWeight = round2(total)

	var top []models.Transaction
	if err := db.DB.Where("empire_id = ? AND flagged = ?", empireID, true).
		Order("amount DESC, id DESC").Limit(5).Find(&top).Error; err != nil {
		return nil, err
	}
	for _, tx := range top {
		file.TopSources = append(file.TopSources, mlclient.EvidenceSource{
			Source: "tx:" + strconv.FormatUint(uint64(tx.ID), 10), Kind: "transaction", Weight: evidenceWeight(tx.Amount),
		})
	}
	return file, nil
}

// Evidence returns the DEA's file on the player. The ML service's evidence graph is used when
// it knows at least as much as the ledger does (the graph lives in memory and is empty after
// a restart); otherwise the ledger is the source of truth.
func Evidence(ctx context.Context, userID uint) (*EvidenceFile, error) {
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	local, err := localEvidence(emp.ID)
	if err != nil {
		return nil, err
	}
	if res, mlErr := mlclient.Default().GetEvidence(ctx, emp.ID); mlErr == nil && res.NodeCount >= local.Count && res.NodeCount > 0 {
		top := res.TopSources
		if top == nil {
			top = []mlclient.EvidenceSource{}
		}
		return &EvidenceFile{Source: "ml", Count: res.NodeCount, TotalWeight: res.TotalWeight, TopSources: top}, nil
	}
	return local, nil
}
