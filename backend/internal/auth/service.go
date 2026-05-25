package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func Register(username, email, password, alias string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Alias:        alias,
	}
	result := db.DB.Create(user)
	return user, result.Error
}

func Login(email, password string) (string, error) {
	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return "", errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
