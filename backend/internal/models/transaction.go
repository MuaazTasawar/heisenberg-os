package models

import "gorm.io/gorm"

type Transaction struct {
	gorm.Model
	EmpireID    uint    `json:"empire_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"` // dirty, clean, laundered
	Description string  `json:"description"`
	BusinessID  uint    `json:"business_id"`
	Flagged     bool    `json:"flagged"`
}
