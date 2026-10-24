package auth

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	Issuer     = "heisenberg-os"
	CookieName = "heisenberg_token"
	// UserIDKey is the gin context key the JWT middleware stores the caller's id under.
	UserIDKey = "user_id"

	MinPasswordLen   = 8
	MaxPasswordBytes = 72 // bcrypt ignores everything after 72 bytes
	MinUsernameLen   = 3
	MaxUsernameLen   = 32
	MaxAliasLen      = 64
	MaxEmailLen      = 254
)

var (
	ErrUserExists         = errors.New("username or email is already taken")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")

	usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)

	// bcryptCost is a variable so tests can use the cheapest cost.
	bcryptCost = bcrypt.DefaultCost

	// dummyHash is compared against when the email is unknown, so a missing account and a
	// wrong password take about the same time.
	dummyHash = sync.OnceValue(func() []byte {
		h, _ := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcryptCost)
		return h
	})
)

// ------------------------------------------------------------------- tokens

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// IssueToken signs an HS256 token that expires ttl after now.
func IssueToken(secret []byte, userID uint, ttl time.Duration, now time.Time) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   fmt.Sprint(userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// ParseToken verifies signature (HS256 only), issuer and expiry.
func ParseToken(secret []byte, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.UserID == 0 {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ------------------------------------------------------------- registration

type RegisterInput struct {
	Username string
	Email    string
	Password string
	Alias    string
}

// Validate normalises the input in place (trimmed, lower-cased username and email) and
// returns every problem found.
func (in *RegisterInput) Validate() []string {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Alias = strings.TrimSpace(in.Alias)

	var problems []string

	if n := utf8.RuneCountInString(in.Username); n < MinUsernameLen || n > MaxUsernameLen || !usernamePattern.MatchString(in.Username) {
		problems = append(problems, fmt.Sprintf(
			"username must be %d-%d characters: letters, numbers, dot, dash or underscore", MinUsernameLen, MaxUsernameLen))
	}

	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || len(in.Email) > MaxEmailLen || !strings.Contains(in.Email[strings.LastIndex(in.Email, "@")+1:], ".") {
		problems = append(problems, "email must be a valid email address")
	}

	if n := len(in.Password); utf8.RuneCountInString(in.Password) < MinPasswordLen {
		problems = append(problems, fmt.Sprintf("password must be at least %d characters", MinPasswordLen))
	} else if n > MaxPasswordBytes {
		problems = append(problems, fmt.Sprintf("password must be at most %d bytes", MaxPasswordBytes))
	}

	if utf8.RuneCountInString(in.Alias) > MaxAliasLen {
		problems = append(problems, fmt.Sprintf("alias must be at most %d characters", MaxAliasLen))
	}
	if in.Alias == "" {
		in.Alias = in.Username
	}
	return problems
}

// Register creates a user. Call Validate first.
func Register(in RegisterInput) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: string(hash),
		Alias:        in.Alias,
	}
	if err := db.DB.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrUserExists
		}
		return nil, err
	}
	return user, nil
}

// Login checks the credentials. Unknown email and wrong password give the same error.
func Login(email, password string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var user models.User
	err := db.DB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return &user, nil
}

// GetUser loads a user by id.
func GetUser(id uint) (*models.User, error) {
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}
