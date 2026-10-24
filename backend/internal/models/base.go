package models

import (
	"time"

	"gorm.io/gorm"
)

// Base replaces gorm.Model so every API response uses snake_case JSON (id, created_at, ...).
// The database columns are identical to gorm.Model's, so existing tables migrate untouched.
type Base struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
