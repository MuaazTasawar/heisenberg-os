package db

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/heisenberg-os/backend/internal/config"
	"github.com/heisenberg-os/backend/internal/models"
	"gorm.io/gorm"
)

func TestWithSchemaOnURLs(t *testing.T) {
	got, err := WithSchema("postgres://walt:p%40ss%2Fword@localhost:5432/heisenberg_os?sslmode=disable", "hos_smoke")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"search_path=hos_smoke", "sslmode=disable", "walt:p%40ss%2Fword@localhost:5432", "/heisenberg_os"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing %q", got, want)
		}
	}

	bare, err := WithSchema("postgres://u:p@h/db", "x")
	if err != nil || !strings.HasSuffix(bare, "/db?search_path=x") {
		t.Errorf("URL without a query: %q %v", bare, err)
	}
}

func TestWithSchemaOnKeyValueDSN(t *testing.T) {
	got, err := WithSchema("host=localhost user=walt dbname=heisenberg_os", "hos_smoke")
	if err != nil || got != "host=localhost user=walt dbname=heisenberg_os search_path=hos_smoke" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestSchemaNamesAreValidated(t *testing.T) {
	for _, bad := range []string{"", "Upper", "a-b", "9lives", "with space", `x"; DROP SCHEMA public; --`, strings.Repeat("a", 64)} {
		if _, err := WithSchema("postgres://u:p@h/db", bad); err == nil {
			t.Errorf("WithSchema accepted %q", bad)
		}
		if err := EnsureSchema(nil, bad); err == nil {
			t.Errorf("EnsureSchema accepted %q", bad)
		}
	}
}

func TestConnectGivesUpWithAClearError(t *testing.T) {
	oldAttempts, oldDelay := connectAttempts, connectDelay
	connectAttempts, connectDelay = 2, 0
	t.Cleanup(func() { connectAttempts, connectDelay = oldAttempts, oldDelay })

	cfg := config.Defaults()
	cfg.DatabaseURL = "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1"
	_, err := Connect(cfg)
	if err == nil || !strings.Contains(err.Error(), "could not connect to the database after 2 attempts") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectRejectsABadSchemaBeforeConnecting(t *testing.T) {
	cfg := config.Defaults()
	cfg.DatabaseURL = "postgres://u:p@127.0.0.1:1/db"
	cfg.DatabaseSchema = "Not Valid"
	if _, err := Connect(cfg); err == nil || !strings.Contains(err.Error(), "invalid schema name") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ------------------------------------------------------- database-backed tests

func testURL(t *testing.T) string {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set - skipping database tests")
	}
	return raw
}

func closeDB(g *gorm.DB) {
	if g == nil {
		return
	}
	if sqlDB, err := g.DB(); err == nil {
		sqlDB.Close()
	}
}

func dropSchema(t *testing.T, raw, schema string) {
	t.Helper()
	g, err := Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	g.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
	if sqlDB, err := g.DB(); err == nil {
		sqlDB.Close()
	}
}

func TestConnectUsesTheConfiguredSchema(t *testing.T) {
	raw := testURL(t)
	const schema = "db_connect_test"
	dropSchema(t, raw, schema)
	cfg := config.Defaults()
	cfg.DatabaseURL = raw
	cfg.DatabaseSchema = schema
	gdb, err := Connect(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDB(gdb); DB = nil; dropSchema(t, raw, schema) })
	if DB != gdb {
		t.Error("Connect must install the shared connection")
	}

	var current string
	if err := gdb.Raw("SELECT current_schema()").Scan(&current).Error; err != nil || current != schema {
		t.Errorf("current_schema = %q (%v), want %q", current, err, schema)
	}
	if err := Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	var inSchema int64
	gdb.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = 'users'", schema).Scan(&inSchema)
	if inSchema != 1 {
		t.Errorf("the users table should live in schema %s", schema)
	}
}

func TestMigrateCreatesEveryTableAndIsRepeatable(t *testing.T) {
	raw := testURL(t)
	const schema = "db_migrate_test"
	dropSchema(t, raw, schema)
	t.Cleanup(func() { dropSchema(t, raw, schema) })

	dsn, err := WithSchema(raw, schema)
	if err != nil {
		t.Fatal(err)
	}
	boot, err := Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDB(boot) })
	if err := EnsureSchema(boot, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(boot, schema); err != nil {
		t.Errorf("EnsureSchema must be idempotent: %v", err)
	}

	gdb, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDB(gdb) })
	for i := 0; i < 2; i++ {
		if err := Migrate(gdb); err != nil {
			t.Fatalf("migration %d: %v", i+1, err)
		}
	}
	for _, model := range []any{&models.User{}, &models.Empire{}, &models.FrontBusiness{},
		&models.Transaction{}, &models.LegalCase{}, &models.DEAThreat{}} {
		if !gdb.Migrator().HasTable(model) {
			t.Errorf("missing table for %T", model)
		}
	}

	// one empire per user is enforced by the database itself
	user := models.User{Username: "dbtest", Email: "dbtest@example.com", PasswordHash: "x"}
	if err := gdb.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.Empire{UserID: user.ID, Name: "first"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.Empire{UserID: user.ID, Name: "second"}).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("a second empire for the same user must be rejected, got %v", err)
	}
}
