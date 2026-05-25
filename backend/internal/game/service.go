package game

import (
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
)

type GameState struct {
	Empire models.Empire      `json:"empire"`
	Threat models.DEAThreat   `json:"dea_threat"`
	Cases  []models.LegalCase `json:"legal_cases"`
}

func GetGameState(userID uint) (*GameState, error) {
	var emp models.Empire
	db.DB.Where("user_id = ?", userID).Preload("FrontBusinesses").First(&emp)

	var threat models.DEAThreat
	db.DB.Where("empire_id = ? AND is_active = true", emp.ID).First(&threat)

	var cases []models.LegalCase
	db.DB.Where("empire_id = ?", emp.ID).Find(&cases)

	return &GameState{Empire: emp, Threat: threat, Cases: cases}, nil
}
