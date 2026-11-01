// Package game assembles what a client needs to draw the whole game (GET /game/state) and the
// public statistics shown on the landing page (GET /stats).
package game

import (
	"context"
	"math"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/dea"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/laundering"
	"github.com/heisenberg-os/backend/internal/models"
)

// recentCases is how many legal cases the state carries (newest first).
const recentCases = 10

// Rules are the game constants the clients need. They are exposed so that no client has to
// hard-code a number that the server can change.
type Rules struct {
	MinEffort        float64 `json:"min_effort"`
	MaxEffort        float64 `json:"max_effort"`
	OptimalEffort    float64 `json:"optimal_effort"`
	DirtyPerKg       float64 `json:"dirty_per_kg"`
	HeatPerEffort    float64 `json:"heat_per_effort"`
	HeatDecayPerHour float64 `json:"heat_decay_per_hour"`
	LaunderRate      float64 `json:"launder_rate"`
	LaunderMin       float64 `json:"launder_min"`
	LargeTransfer    float64 `json:"large_transfer"`
	TerritoryBonus   float64 `json:"territory_bonus"`
	// NextTerritoryCost is the clean money the next territory costs; null once the player
	// holds the maximum.
	NextTerritoryCost *float64 `json:"next_territory_cost"`
	MaxTerritories    int      `json:"max_territories"`
	MaxActiveFronts   int      `json:"max_active_fronts"`
}

// CurrentRules returns the rules as they apply to a player who holds `territories` territories.
func CurrentRules(territories int) Rules {
	r := Rules{
		MinEffort:        empire.MinEffort,
		MaxEffort:        empire.MaxEffort,
		OptimalEffort:    math.Round(empire.OptimalEffort()*100) / 100,
		DirtyPerKg:       empire.DirtyPerKg,
		HeatPerEffort:    empire.HeatPerEffort,
		HeatDecayPerHour: empire.HeatDecayPerHour,
		LaunderRate:      laundering.LaunderRate,
		LaunderMin:       laundering.MinAmount,
		LargeTransfer:    laundering.LargeTransfer,
		TerritoryBonus:   empire.TerritoryBonus,
		MaxTerritories:   empire.MaxTerritories,
		MaxActiveFronts:  empire.MaxActiveFronts,
	}
	if territories < empire.MaxTerritories {
		cost := empire.TerritoryCost(territories)
		r.NextTerritoryCost = &cost
	}
	return r
}

// State is everything the dashboard shows, in one response.
type State struct {
	Empire     *models.Empire     `json:"empire"`
	DEAThreat  *models.DEAThreat  `json:"dea_threat"`
	LegalCases []models.LegalCase `json:"legal_cases"`
	Rules      Rules              `json:"rules"`
}

// GetState loads the player's empire (with its fronts), a freshly evaluated DEA threat, the
// latest legal cases and the rules. The threat is evaluated here, so it is never missing or
// stale just because the player has not opened the DEA page yet.
func GetState(ctx context.Context, userID uint) (*State, error) {
	threat, err := dea.Evaluate(ctx, userID)
	if err != nil {
		return nil, err
	}
	emp, err := empire.Get(userID)
	if err != nil {
		return nil, err
	}
	if emp.FrontBusinesses == nil {
		emp.FrontBusinesses = []models.FrontBusiness{}
	}

	cases := []models.LegalCase{}
	if err := db.DB.Where("empire_id = ?", emp.ID).Order("created_at DESC, id DESC").
		Limit(recentCases).Find(&cases).Error; err != nil {
		return nil, err
	}
	return &State{Empire: emp, DEAThreat: threat, LegalCases: cases, Rules: CurrentRules(emp.TerritoryCount)}, nil
}
