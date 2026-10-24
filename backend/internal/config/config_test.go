package config

import (
	"strings"
	"testing"
	"time"
)

const goodSecret = "5f0c2a9e7d1b48e6a3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7081920"

func env(pairs ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) string { return m[k] }
}

// valid returns an environment that passes validation, plus any extra settings.
func valid(extra ...string) func(string) string {
	base := []string{"DATABASE_URL", "postgres://u:p@localhost:5432/db", "JWT_SECRET", goodSecret}
	return env(append(base, extra...)...)
}

func mustFail(t *testing.T, get func(string) string, wants ...string) {
	t.Helper()
	_, err := LoadFrom(get)
	if err == nil {
		t.Fatalf("expected an error containing %v, got none", wants)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

func TestMinimalEnvironmentGetsSensibleDefaults(t *testing.T) {
	cfg, err := LoadFrom(valid())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.Port != "8080" || cfg.IsProduction() {
		t.Errorf("unexpected env/port: %s %s", cfg.Env, cfg.Port)
	}
	if cfg.JWTTTL != 72*time.Hour || cfg.CookieSecure {
		t.Errorf("unexpected auth defaults: %v %v", cfg.JWTTTL, cfg.CookieSecure)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("unexpected origins: %v", cfg.CORSOrigins)
	}
	if cfg.TrustedProxies == nil || len(cfg.TrustedProxies) != 0 {
		t.Errorf("trusted proxies should be an empty, non-nil list: %#v", cfg.TrustedProxies)
	}
	if cfg.MLServiceURL != "http://localhost:8000" || cfg.MLTimeout != 1500*time.Millisecond || cfg.MLAPIKey != "" {
		t.Errorf("unexpected ML defaults: %q %v", cfg.MLServiceURL, cfg.MLTimeout)
	}
	if cfg.AnthropicModel != "claude-sonnet-5-5" || cfg.AnthropicBaseURL != "https://api.anthropic.com" ||
		cfg.AnthropicTimeout != 25*time.Second || cfg.SaulConfigured() {
		t.Errorf("unexpected Anthropic defaults: %+v", cfg)
	}
	if cfg.RateAuthPerMin != 10 || cfg.RateLegalPerMin != 5 || cfg.RateAPIPerMin != 240 {
		t.Errorf("unexpected rate limits: %d %d %d", cfg.RateAuthPerMin, cfg.RateLegalPerMin, cfg.RateAPIPerMin)
	}
}

func TestRequiredSettingsAreReportedTogether(t *testing.T) {
	mustFail(t, env(), "DATABASE_URL is required", "JWT_SECRET is required")
}

func TestJWTSecretRules(t *testing.T) {
	cases := []struct {
		name, secret, want string
	}{
		{"too short", strings.Repeat("x7", 15), "at least 32"},
		{"old default", "iamtheonewhoknocks-and-some-padding-1234", "placeholder"},
		{"placeholder", "replace-me-with-a-long-random-string-please", "placeholder"},
		{"repeated character", strings.Repeat("a", 40), "placeholder"},
		{"contains the word password", "my-super-PASSWORD-that-is-long-enough-1", "placeholder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustFail(t, env("DATABASE_URL", "postgres://x", "JWT_SECRET", tc.secret), tc.want)
		})
	}
	if _, err := LoadFrom(valid()); err != nil {
		t.Errorf("a random 64-character secret must be accepted: %v", err)
	}
}

func TestAllProblemsAreReportedAtOnce(t *testing.T) {
	_, err := LoadFrom(env("APP_ENV", "staging", "PORT", "99999", "JWT_TTL_HOURS", "0",
		"ML_TIMEOUT_MS", "1", "RATE_LIMIT_API_PER_MIN", "-5"))
	if err == nil {
		t.Fatal("expected errors")
	}
	if lines := strings.Count(err.Error(), "\n") + 1; lines < 7 {
		t.Errorf("expected at least 7 problems, got %d:\n%v", lines, err)
	}
}

func TestPort(t *testing.T) {
	for _, bad := range []string{"0", "65536", "abc", "-1", "80.5"} {
		mustFail(t, valid("PORT", bad), "PORT")
	}
	cfg, err := LoadFrom(valid("PORT", "9090"))
	if err != nil || cfg.Port != "9090" {
		t.Errorf("port 9090 should be accepted: %v %v", cfg, err)
	}
}

func TestListenHost(t *testing.T) {
	if cfg, _ := LoadFrom(valid()); cfg.Host != "" {
		t.Errorf("default host should be empty (all interfaces), got %q", cfg.Host)
	}
	for _, ok := range []string{"127.0.0.1", "localhost", "::1", "0.0.0.0"} {
		cfg, err := LoadFrom(valid("LISTEN_HOST", ok))
		if err != nil || cfg.Host != ok {
			t.Errorf("%q should be accepted: %v %v", ok, cfg, err)
		}
	}
	mustFail(t, valid("LISTEN_HOST", "not a host"), "LISTEN_HOST")
	mustFail(t, valid("LISTEN_HOST", "example.com"), "LISTEN_HOST")
}

func TestDatabaseSchema(t *testing.T) {
	if cfg, _ := LoadFrom(valid()); cfg.DatabaseSchema != "" {
		t.Errorf("no schema by default, got %q", cfg.DatabaseSchema)
	}
	for _, ok := range []string{"hos_smoke", "_private", "a", "schema_2"} {
		cfg, err := LoadFrom(valid("DATABASE_SCHEMA", ok))
		if err != nil || cfg.DatabaseSchema != ok {
			t.Errorf("%q should be accepted: %v %v", ok, cfg, err)
		}
	}
	for _, bad := range []string{"Upper", "a-b", "9lives", "with space", `x"; DROP SCHEMA public; --`, strings.Repeat("a", 64)} {
		mustFail(t, valid("DATABASE_SCHEMA", bad), "DATABASE_SCHEMA")
	}
}

func TestAppEnvAndSecureCookie(t *testing.T) {
	mustFail(t, valid("APP_ENV", "staging"), "APP_ENV")
	mustFail(t, valid("APP_ENV", "production"), "COOKIE_SECURE must be true")
	mustFail(t, valid("COOKIE_SECURE", "maybe"), "COOKIE_SECURE")

	cfg, err := LoadFrom(valid("APP_ENV", "PRODUCTION", "COOKIE_SECURE", "true"))
	if err != nil || !cfg.IsProduction() || !cfg.CookieSecure {
		t.Errorf("production with secure cookies must load: %v %v", cfg, err)
	}
}

func TestJWTTTL(t *testing.T) {
	for _, bad := range []string{"0", "721", "x", "-3"} {
		mustFail(t, valid("JWT_TTL_HOURS", bad), "JWT_TTL_HOURS")
	}
	cfg, _ := LoadFrom(valid("JWT_TTL_HOURS", "12"))
	if cfg.JWTTTL != 12*time.Hour {
		t.Errorf("ttl = %v", cfg.JWTTTL)
	}
}

func TestCORSOrigins(t *testing.T) {
	cfg, err := LoadFrom(valid("CORS_ALLOWED_ORIGINS", " https://app.example.com/ , http://localhost:3000 "))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.CORSOrigins, ","); got != "https://app.example.com,http://localhost:3000" {
		t.Errorf("origins not normalised: %s", got)
	}
	for _, bad := range []string{"*", "https://*.example.com", "ftp://example.com", "https://example.com/app",
		"localhost:3000", "https://example.com?x=1"} {
		mustFail(t, valid("CORS_ALLOWED_ORIGINS", bad), "CORS_ALLOWED_ORIGINS")
	}
}

func TestTrustedProxies(t *testing.T) {
	cfg, err := LoadFrom(valid("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8, ::1"))
	if err != nil || len(cfg.TrustedProxies) != 3 {
		t.Errorf("proxies not parsed: %v %v", cfg, err)
	}
	mustFail(t, valid("TRUSTED_PROXIES", "127.0.0.1, my-proxy"), "my-proxy")
}

func TestMLServiceSettings(t *testing.T) {
	for _, off := range []string{"none", "OFF", "Disabled"} {
		cfg, err := LoadFrom(valid("ML_SERVICE_URL", off))
		if err != nil || cfg.MLServiceURL != "" {
			t.Errorf("%q should disable the ML service: %v %v", off, cfg, err)
		}
	}
	cfg, _ := LoadFrom(valid("ML_SERVICE_URL", "http://ml:8000/", "ML_API_KEY", "k", "ML_TIMEOUT_MS", "2000"))
	if cfg.MLServiceURL != "http://ml:8000" || cfg.MLAPIKey != "k" || cfg.MLTimeout != 2*time.Second {
		t.Errorf("ML settings wrong: %+v", cfg)
	}
	mustFail(t, valid("ML_SERVICE_URL", "ml:8000"), "ML_SERVICE_URL")
	mustFail(t, valid("ML_TIMEOUT_MS", "49"), "ML_TIMEOUT_MS")
	mustFail(t, valid("ML_TIMEOUT_MS", "30001"), "ML_TIMEOUT_MS")
}

func TestAnthropicSettings(t *testing.T) {
	cfg, err := LoadFrom(valid("ANTHROPIC_API_KEY", "sk-test", "ANTHROPIC_MODEL", "claude-x",
		"ANTHROPIC_BASE_URL", "http://127.0.0.1:9999/", "ANTHROPIC_TIMEOUT_SEC", "5"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SaulConfigured() || cfg.AnthropicModel != "claude-x" ||
		cfg.AnthropicBaseURL != "http://127.0.0.1:9999" || cfg.AnthropicTimeout != 5*time.Second {
		t.Errorf("Anthropic settings wrong: %+v", cfg)
	}
	mustFail(t, valid("ANTHROPIC_BASE_URL", "api.anthropic.com"), "ANTHROPIC_BASE_URL")
	mustFail(t, valid("ANTHROPIC_TIMEOUT_SEC", "0"), "ANTHROPIC_TIMEOUT_SEC")
	mustFail(t, valid("ANTHROPIC_TIMEOUT_SEC", "121"), "ANTHROPIC_TIMEOUT_SEC")
}

func TestRateLimits(t *testing.T) {
	cfg, err := LoadFrom(valid("RATE_LIMIT_AUTH_PER_MIN", "0", "RATE_LIMIT_LEGAL_PER_MIN", "2", "RATE_LIMIT_API_PER_MIN", "1000"))
	if err != nil || cfg.RateAuthPerMin != 0 || cfg.RateLegalPerMin != 2 || cfg.RateAPIPerMin != 1000 {
		t.Errorf("rate limits wrong: %+v %v", cfg, err)
	}
	for _, bad := range []string{"-1", "abc", "100001"} {
		mustFail(t, valid("RATE_LIMIT_AUTH_PER_MIN", bad), "RATE_LIMIT_AUTH_PER_MIN")
	}
}

func TestCurrentFallsBackToDefaultsAndCanBeSet(t *testing.T) {
	t.Cleanup(func() { Set(nil) })
	Set(nil)
	if Current().AnthropicModel != "claude-sonnet-5-5" {
		t.Error("Current() should return defaults when nothing is set")
	}
	custom := Defaults()
	custom.AnthropicModel = "custom"
	Set(custom)
	if Current().AnthropicModel != "custom" {
		t.Error("Current() should return the installed config")
	}
}
