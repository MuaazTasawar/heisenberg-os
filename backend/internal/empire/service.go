package empire

import (
	"errors"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
)

func GetOrCreate(userID uint, name string) (*models.Empire, error) {
	var emp models.Empire
	result := db.DB.Where("user_id = ?", userID).Preload("FrontBusinesses").First(&emp)
	if result.Error != nil {
		emp = models.Empire{
			UserID:       userID,
			Name:         name,
			PurityRating: 50.0,
			BatchYield:   10.0,
			HeatLevel:    0.0,
		}
		db.DB.Create(&emp)
	}
	return &emp, nil
}

func Cook(empireID uint, effort float64) (*models.Empire, error) {
	var emp models.Empire
	if err := db.DB.First(&emp, empireID).Error; err != nil {
		return nil, errors.New("empire not found")
	}
	yieldGain := effort * (emp.PurityRating / 100.0) * 5.0
	emp.BatchYield += yieldGain
	emp.DirtyMoney += yieldGain * 1500
	emp.HeatLevel += effort * 2.5
	if emp.HeatLevel > 100 {
		emp.HeatLevel = 100
	}
	db.DB.Save(&emp)
	return &emp, nil
}

func AddFrontBusiness(empireID uint, name, btype string) (*models.FrontBusiness, error) {
	fb := &models.FrontBusiness{
		EmpireID: empireID,
		Name:     name,
		Type:     btype,
		IsActive: true,
	}
	db.DB.Create(fb)
	return fb, nil
}
func GetEmpireID(userID uint) uint {
	emp, _ := GetOrCreate(userID, "Empire")
	return emp.ID
}
