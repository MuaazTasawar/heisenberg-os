package db

import (
	"log"
	"os"

	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	DB.AutoMigrate(
		&models.User{},
		&models.Empire{},
		&models.Transaction{},
		&models.LegalCase{},
		&models.DEAThreat{},
	)
	log.Println("Database connected and migrated")
}
