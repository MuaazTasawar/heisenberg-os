package laundering

import (
	"errors"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
)

func LaunderMoney(empireID uint, amount float64, businessID uint) (*models.Transaction, error) {
	var emp models.Empire
	if err := db.DB.First(&emp, empireID).Error; err != nil {
		return nil, errors.New("empire not found")
	}
	if emp.DirtyMoney < amount {
		return nil, errors.New("insufficient dirty money")
	}
	launderRate := 0.75
	cleaned := amount * launderRate
	emp.DirtyMoney -= amount
	emp.CleanMoney += cleaned
	emp.HeatLevel += amount / 100000
	db.DB.Save(&emp)

	tx := &models.Transaction{
		EmpireID:    empireID,
		Amount:      cleaned,
		Type:        "laundered",
		Description: "Los Pollos transfer",
		BusinessID:  businessID,
		Flagged:     amount > 50000,
	}
	db.DB.Create(tx)
	return tx, nil
}

func GetTransactions(empireID uint) ([]models.Transaction, error) {
	var txs []models.Transaction
	db.DB.Where("empire_id = ?", empireID).Order("created_at desc").Limit(50).Find(&txs)
	return txs, nil
}
