package db

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the shared connection used by the service packages.
var DB *gorm.DB

// These are variables so tests can shorten the retry loop.
var (
	connectAttempts = 10
	connectDelay    = 2 * time.Second
)

var schemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Open creates a connection without retrying or touching the global.
func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true, // turns unique violations into gorm.ErrDuplicatedKey
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold:             300 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // "no such row" is a normal answer, not an error
		}),
	})
}

// WithSchema returns dsn with the connection's search_path set to schema, so every table is
// created and queried inside that schema. It understands both URL and key=value DSNs.
func WithSchema(dsn, schema string) (string, error) {
	if !schemaName.MatchString(schema) {
		return "", fmt.Errorf("invalid schema name %q", schema)
	}
	if !strings.Contains(dsn, "://") {
		return dsn + " search_path=" + schema, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid database URL: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// EnsureSchema creates the schema if it does not exist yet.
func EnsureSchema(g *gorm.DB, schema string) error {
	if !schemaName.MatchString(schema) {
		return fmt.Errorf("invalid schema name %q", schema)
	}
	return g.Exec(`CREATE SCHEMA IF NOT EXISTS "` + schema + `"`).Error
}

// Connect opens the database (retrying while it starts up, as happens under Docker
// Compose), configures the pool and stores the result in DB.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL
	if cfg.DatabaseSchema != "" {
		var err error
		if dsn, err = WithSchema(dsn, cfg.DatabaseSchema); err != nil {
			return nil, err
		}
	}

	var (
		gdb *gorm.DB
		err error
	)
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		gdb, err = Open(dsn)
		if err == nil {
			if raw, derr := gdb.DB(); derr != nil {
				err = derr
			} else {
				err = raw.Ping()
			}
		}
		if err == nil {
			break
		}
		if attempt < connectAttempts {
			log.Printf("database not ready (attempt %d/%d): %v", attempt, connectAttempts, err)
			time.Sleep(connectDelay)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not connect to the database after %d attempts: %w", connectAttempts, err)
	}

	if cfg.DatabaseSchema != "" {
		if err := EnsureSchema(gdb, cfg.DatabaseSchema); err != nil {
			return nil, fmt.Errorf("could not create schema %q: %w", cfg.DatabaseSchema, err)
		}
	}

	raw, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(25)
	raw.SetMaxIdleConns(5)
	raw.SetConnMaxLifetime(30 * time.Minute)

	DB = gdb
	return gdb, nil
}

// Migrate creates or updates every table. Changes are additive.
func Migrate(g *gorm.DB) error {
	if err := g.AutoMigrate(
		&models.User{},
		&models.Empire{},
		&models.FrontBusiness{},
		&models.Transaction{},
		&models.LegalCase{},
		&models.DEAThreat{},
	); err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}
	return nil
}
