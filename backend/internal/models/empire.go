package models

import "time"

type Empire struct {
	Base
	UserID          uint            `gorm:"uniqueIndex" json:"user_id"`
	Name            string          `json:"name"`
	PurityRating    float64         `json:"purity_rating"`
	BatchYield      float64         `json:"batch_yield"`
	DirtyMoney      float64         `json:"dirty_money"`
	CleanMoney      float64         `json:"clean_money"`
	HeatLevel       float64         `json:"heat_level"`
	HeatUpdatedAt   *time.Time      `json:"heat_updated_at,omitempty"`
	TerritoryCount  int             `json:"territory_count"`
	FrontBusinesses []FrontBusiness `json:"front_businesses,omitempty"`
}

type FrontBusiness struct {
	Base
	EmpireID       uint    `gorm:"index" json:"empire_id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Revenue        float64 `json:"revenue"`
	Suspicion      float64 `json:"suspicion"`
	IsActive       bool    `json:"is_active"`
	ShutdownReason string  `json:"shutdown_reason,omitempty"`
}
