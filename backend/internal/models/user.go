package models

type User struct {
	Base
	Username     string  `gorm:"uniqueIndex;not null" json:"username"`
	Email        string  `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string  `gorm:"not null" json:"-"`
	Alias        string  `json:"alias"`
	Empire       *Empire `json:"empire,omitempty"`
}
