package empire

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/mlclient"
	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// defaultName names a new empire after the player's alias.
func defaultName(tx *gorm.DB, userID uint) string {
	var alias string
	tx.Model(&models.User{}).Select("alias").Where("id = ?", userID).Scan(&alias)
	if alias = strings.TrimSpace(alias); alias == "" {
		return "Empire"
	}
	return alias + "'s Empire"
}

// lockOrCreate returns the player's empire row locked FOR UPDATE, creating it first when it
// does not exist. The unique index on user_id makes concurrent first requests safe.
func lockOrCreate(tx *gorm.DB, userID uint) (*models.Empire, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var emp models.Empire
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&emp).Error
		if err == nil {
			return &emp, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		fresh := models.Empire{
			UserID:       userID,
			Name:         defaultName(tx, userID),
			PurityRating: StartPurity,
			BatchYield:   StartBatchYield,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fresh).Error; err != nil {
			return nil, err
		}
	}
	return nil, errors.New("could not create the empire")
}

func loadFronts(tx *gorm.DB, emp *models.Empire) error {
	emp.FrontBusinesses = []models.FrontBusiness{}
	return tx.Where("empire_id = ?", emp.ID).Order("id").Find(&emp.FrontBusinesses).Error
}

// WithEmpire runs fn inside a database transaction while holding a row lock on the player's
// empire, so two requests can never read the same balance and overwrite each other. Heat
// has already cooled down for the time since the last change when fn runs. If fn returns an
// error nothing is saved.
func WithEmpire(userID uint, fn func(tx *gorm.DB, emp *models.Empire) error) (*models.Empire, error) {
	var out *models.Empire
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		emp, err := lockOrCreate(tx, userID)
		if err != nil {
			return err
		}
		now := time.Now()
		emp.HeatLevel, _ = DecayHeat(emp.HeatLevel, emp.HeatUpdatedAt, now)
		if err := fn(tx, emp); err != nil {
			return err
		}
		emp.HeatUpdatedAt = &now
		if err := tx.Omit(clause.Associations).Save(emp).Error; err != nil {
			return err
		}
		if err := loadFronts(tx, emp); err != nil {
			return err
		}
		out = emp
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns the player's empire (created on first use) with its front businesses. Heat is
// shown as it is right now; reading never writes.
func Get(userID uint) (*models.Empire, error) {
	var emp models.Empire
	err := db.DB.Where("user_id = ?", userID).First(&emp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WithEmpire(userID, func(*gorm.DB, *models.Empire) error { return nil })
	}
	if err != nil {
		return nil, err
	}
	emp.HeatLevel, _ = DecayHeat(emp.HeatLevel, emp.HeatUpdatedAt, time.Now())
	if err := loadFronts(db.DB, &emp); err != nil {
		return nil, err
	}
	return &emp, nil
}

// GetOrCreate is kept for the packages that still call it; the name argument is ignored
// (new empires are named after the player's alias).
func GetOrCreate(userID uint, _ string) (*models.Empire, error) { return Get(userID) }

// GetEmpireID returns the id of the player's empire, or 0 when it cannot be loaded.
func GetEmpireID(userID uint) uint {
	emp, err := Get(userID)
	if err != nil {
		return 0
	}
	return emp.ID
}

// ---------------------------------------------------------------------- cooking

type CookResult struct {
	Empire      *models.Empire `json:"empire"`
	Effort      float64        `json:"effort"`
	YieldGained float64        `json:"yield_gained"`
	DirtyGained float64        `json:"dirty_gained"`
	HeatAdded   float64        `json:"heat_added"`
	PurityDelta float64        `json:"purity_delta"`
}

// Cook runs one batch at the given effort (1-10).
func Cook(userID uint, effort float64) (*CookResult, error) {
	if err := ValidateEffort(effort); err != nil {
		return nil, err
	}
	res := &CookResult{Effort: effort}
	emp, err := WithEmpire(userID, func(_ *gorm.DB, emp *models.Empire) error {
		heatBefore, purityBefore := emp.HeatLevel, emp.PurityRating
		o := CookOutcome(effort, emp.PurityRating, emp.TerritoryCount)

		emp.BatchYield = round2(emp.BatchYield + o.Yield)
		emp.DirtyMoney = round2(emp.DirtyMoney + o.Dirty)
		emp.HeatLevel = round2(ClampHeat(emp.HeatLevel + o.Heat))
		emp.PurityRating = round2(PurityAfter(emp.PurityRating, effort))

		res.YieldGained = o.Yield
		res.DirtyGained = o.Dirty
		res.HeatAdded = round2(emp.HeatLevel - heatBefore)
		res.PurityDelta = round2(emp.PurityRating - purityBefore)
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.Empire = emp
	return res, nil
}

// GetAdvice compares a planned effort with the best one. The ML service answers when it is
// available and sane; otherwise the same formulas run locally.
func GetAdvice(ctx context.Context, userID uint, effort float64) (*Advice, error) {
	if err := ValidateEffort(effort); err != nil {
		return nil, err
	}
	emp, err := Get(userID)
	if err != nil {
		return nil, err
	}

	res, mlErr := mlclient.Default().OptimizeYield(ctx, mlclient.YieldInput{
		CurrentPurity: emp.PurityRating,
		EffortLevel:   effort * 10, // the ML service works on a 0-100 scale
		SupplyQuality: 100,
		LabCapacity:   Capacity(emp.TerritoryCount),
	})
	if mlErr == nil && sane(res) {
		return &Advice{
			Source:           "ml",
			Effort:           effort,
			PredictedYield:   res.PredictedYield,
			OptimalEffort:    round2(res.OptimalEffort / 10),
			OptimalYield:     res.OptimalYield,
			PurityProjection: res.PurityProjection,
			EfficiencyScore:  res.EfficiencyScore,
		}, nil
	}
	local := LocalAdvice(emp.PurityRating, emp.TerritoryCount, effort)
	return &local, nil
}

func sane(r *mlclient.YieldResult) bool {
	for _, v := range []float64{r.PredictedYield, r.OptimalEffort, r.OptimalYield, r.PurityProjection, r.EfficiencyScore} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	return r.OptimalEffort <= 100 && r.PurityProjection <= 100 && r.EfficiencyScore <= 100
}

// ------------------------------------------------------------------ territories

// ExpandTerritory spends clean money to claim one more territory.
func ExpandTerritory(userID uint) (*models.Empire, error) {
	return WithEmpire(userID, func(_ *gorm.DB, emp *models.Empire) error {
		if emp.TerritoryCount >= MaxTerritories {
			return ErrMaxTerritories
		}
		cost := TerritoryCost(emp.TerritoryCount)
		if emp.CleanMoney < cost {
			return fmt.Errorf("%w: territory %d costs $%.0f", ErrInsufficientFunds, emp.TerritoryCount+1, cost)
		}
		emp.CleanMoney = round2(emp.CleanMoney - cost)
		emp.TerritoryCount++
		emp.HeatLevel = round2(ClampHeat(emp.HeatLevel + TerritoryHeat))
		return nil
	})
}

// ----------------------------------------------------------------------- fronts

// ListFronts returns every front business of the player, closed ones included.
func ListFronts(userID uint) ([]models.FrontBusiness, error) {
	emp, err := Get(userID)
	if err != nil {
		return nil, err
	}
	return emp.FrontBusinesses, nil
}

// AddFront opens a front business.
func AddFront(userID uint, name, btype string) (*models.FrontBusiness, error) {
	ft, ok := FrontTypeByName(btype)
	if !ok {
		return nil, fmt.Errorf("%w: unknown type %q", ErrInvalidFront, btype)
	}
	clean, err := NormalizeFrontName(name)
	if err != nil {
		return nil, err
	}

	var created models.FrontBusiness
	_, err = WithEmpire(userID, func(tx *gorm.DB, emp *models.Empire) error {
		var active int64
		if err := tx.Model(&models.FrontBusiness{}).Where("empire_id = ? AND is_active = ?", emp.ID, true).Count(&active).Error; err != nil {
			return err
		}
		if active >= MaxActiveFronts {
			return ErrFrontLimit
		}
		var same int64
		if err := tx.Model(&models.FrontBusiness{}).
			Where("empire_id = ? AND is_active = ? AND lower(name) = lower(?)", emp.ID, true, clean).Count(&same).Error; err != nil {
			return err
		}
		if same > 0 {
			return ErrDuplicateFront
		}
		created = models.FrontBusiness{EmpireID: emp.ID, Name: clean, Type: ft.Type, IsActive: true}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// CloseFront shuts one of the player's front businesses down.
func CloseFront(userID, frontID uint) (*models.FrontBusiness, error) {
	var fb models.FrontBusiness
	_, err := WithEmpire(userID, func(tx *gorm.DB, emp *models.Empire) error {
		if err := tx.Where("id = ? AND empire_id = ?", frontID, emp.ID).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFrontNotFound
			}
			return err
		}
		if !fb.IsActive {
			return ErrFrontClosed
		}
		fb.IsActive = false
		fb.ShutdownReason = "closed by owner"
		return tx.Save(&fb).Error
	})
	if err != nil {
		return nil, err
	}
	return &fb, nil
}
