package dea

import (
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
)

func UpdateThreat(empireID uint) (*models.DEAThreat, error) {
	var threat models.DEAThreat
	db.DB.Where("empire_id = ? AND is_active = true", empireID).First(&threat)

	var emp models.Empire
	db.DB.First(&emp, empireID)

	var flaggedCount int64
	db.DB.Model(&models.Transaction{}).Where("empire_id = ? AND flagged = true", empireID).Count(&flaggedCount)

	score := (emp.HeatLevel * 0.4) + (float64(flaggedCount) * 3.0) + (emp.BatchYield * 0.1)
	if score > 100 {
		score = 100
	}

	if threat.ID == 0 {
		threat = models.DEAThreat{
			EmpireID:      empireID,
			ThreatScore:   score,
			EvidenceCount: int(flaggedCount),
			LeadAgent:     "Hank Schrader",
			IsActive:      true,
		}
		db.DB.Create(&threat)
	} else {
		threat.ThreatScore = score
		threat.EvidenceCount = int(flaggedCount)
		db.DB.Save(&threat)
	}
	return &threat, nil
}
