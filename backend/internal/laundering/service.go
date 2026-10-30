package laundering

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/gorm"
)

// Result is everything a laundering transfer changed.
type Result struct {
	Transaction   models.Transaction   `json:"transaction"`
	CleanedAmount float64              `json:"cleaned_amount"`
	Fee           float64              `json:"fee"`
	SuspicionGain float64              `json:"suspicion_gain"`
	FrontClosed   bool                 `json:"front_closed"`
	Front         models.FrontBusiness `json:"front"`
	Empire        *models.Empire       `json:"empire"`
	Analysis      Analysis             `json:"analysis"`
}

func findFront(fronts []models.FrontBusiness, id uint) (models.FrontBusiness, error) {
	for _, f := range fronts {
		if f.ID == id {
			if !f.IsActive {
				return f, empire.ErrFrontClosed
			}
			return f, nil
		}
	}
	return models.FrontBusiness{}, empire.ErrFrontNotFound
}

// recentHistory returns the last HistoryWindow transfers through one front, oldest first.
func recentHistory(tx *gorm.DB, empireID, frontID uint) ([]models.Transaction, error) {
	var rows []models.Transaction
	err := tx.Where("empire_id = ? AND business_id = ? AND type = ?", empireID, frontID, models.TxLaundered).
		Order("created_at DESC, id DESC").Limit(HistoryWindow).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func sane(r *mlclient.LaunderingResult) bool {
	return !math.IsNaN(r.RiskScore) && r.RiskScore >= 0 && r.RiskScore <= 100
}

// analyze asks the ML service about the transfer and falls back to the built-in rules when it
// is off, down, slow or answers nonsense.
func analyze(ctx context.Context, front models.FrontBusiness, history []models.Transaction, amount float64, now time.Time) Analysis {
	amounts, gaps := BuildSeries(history, amount, now)
	revenue := math.Max(front.Revenue, BaselineRevenue)

	res, err := mlclient.Default().AnalyzeLaundering(ctx, mlclient.LaunderingInput{
		Amounts:          amounts,
		BusinessRevenues: []float64{revenue},
		TimeGaps:         gaps,
		BusinessType:     front.Type,
	})
	if err == nil && sane(res) {
		patterns := res.SuspiciousPatterns
		if patterns == nil {
			patterns = []string{}
		}
		return Analysis{
			Source:         "ml",
			RiskScore:      res.RiskScore,
			Anomaly:        res.AnomalyDetected,
			Patterns:       patterns,
			Recommendation: res.Recommendation,
		}
	}
	return LocalAnalysis(amounts, gaps, revenue, front.Type)
}

// Launder turns dirty money into clean money through one of the player's front businesses.
func Launder(ctx context.Context, userID, businessID uint, amount float64) (*Result, error) {
	if err := ValidateAmount(amount); err != nil {
		return nil, err
	}
	amount = round2(amount)

	// Look first, without holding any lock: the ML call is a network call and must not keep
	// the player's empire locked while it runs.
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	front, err := findFront(emp.FrontBusinesses, businessID)
	if err != nil {
		return nil, err
	}
	history, err := recentHistory(db.DB, emp.ID, businessID)
	if err != nil {
		return nil, err
	}
	analysis := analyze(ctx, front, history, amount, time.Now())

	// Then do it, atomically, with the empire row locked. Everything is re-checked here.
	var res Result
	updated, err := empire.WithEmpire(userID, func(tx *gorm.DB, e *models.Empire) error {
		var fb models.FrontBusiness
		if err := tx.Where("id = ? AND empire_id = ?", businessID, e.ID).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return empire.ErrFrontNotFound
			}
			return err
		}
		if !fb.IsActive {
			return empire.ErrFrontClosed
		}
		if e.DirtyMoney < amount {
			return ErrInsufficientDirty
		}

		factor := 1.0
		if ft, ok := empire.FrontTypeByName(fb.Type); ok {
			factor = ft.Suspicion
		}
		cleaned := CleanedAmount(amount)
		gain := SuspicionGain(amount, fb.Revenue, factor, analysis.RiskScore)
		reasons := FlagReasons(amount, analysis)

		e.DirtyMoney = round2(e.DirtyMoney - amount)
		e.CleanMoney = round2(e.CleanMoney + cleaned)
		e.HeatLevel = round2(empire.ClampHeat(e.HeatLevel + HeatFor(amount)))

		fb.Revenue = round2(fb.Revenue + cleaned)
		fb.Suspicion = round2(math.Min(100, fb.Suspicion+gain))
		closed := false
		if fb.Suspicion >= RaidThreshold {
			fb.IsActive = false
			fb.ShutdownReason = "raided by the DEA: suspicion reached 100"
			e.HeatLevel = round2(empire.ClampHeat(e.HeatLevel + RaidHeat))
			closed = true
		}
		if err := tx.Save(&fb).Error; err != nil {
			return err
		}

		record := models.Transaction{
			EmpireID:    e.ID,
			Amount:      amount,
			Type:        models.TxLaundered,
			Description: fmt.Sprintf("Laundered via %s: $%.2f in, $%.2f clean", fb.Name, amount, cleaned),
			BusinessID:  fb.ID,
			Flagged:     len(reasons) > 0,
			FlagReason:  strings.Join(reasons, "; "),
			RiskScore:   analysis.RiskScore,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		res = Result{
			Transaction:   record,
			CleanedAmount: cleaned,
			Fee:           round2(amount - cleaned),
			SuspicionGain: gain,
			FrontClosed:   closed,
			Front:         fb,
			Analysis:      analysis,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.Empire = updated

	if res.Transaction.Flagged {
		recordEvidence(res.Transaction)
	}
	return &res, nil
}

// recordEvidence adds a flagged transfer to the DEA's evidence file on the ML service. It
// happens after the transaction committed and is best effort: the game goes on without it.
func recordEvidence(tx models.Transaction) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	weight := round2(math.Min(10, math.Max(0.1, tx.Amount/50_000)))
	_, _ = mlclient.Default().AddEvidence(ctx, mlclient.EvidenceInput{
		EmpireID: tx.EmpireID,
		Source:   fmt.Sprintf("tx:%d", tx.ID),
		Weight:   weight,
		Kind:     "transaction",
	})
}

// Page is one page of the transaction ledger.
type Page struct {
	Items  []models.Transaction `json:"items"`
	Total  int64                `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

// ListTransactions returns the player's ledger, newest first. flagged filters when non-nil.
func ListTransactions(userID uint, limit, offset int, flagged *bool) (*Page, error) {
	if limit < 1 || limit > 200 || offset < 0 {
		return nil, ErrInvalidPage
	}
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	query := db.DB.Model(&models.Transaction{}).Where("empire_id = ?", emp.ID)
	if flagged != nil {
		query = query.Where("flagged = ?", *flagged)
	}
	page := &Page{Items: []models.Transaction{}, Limit: limit, Offset: offset}
	if err := query.Count(&page.Total).Error; err != nil {
		return nil, err
	}
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&page.Items).Error; err != nil {
		return nil, err
	}
	return page, nil
}
