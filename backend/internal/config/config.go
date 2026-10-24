// Package config loads and validates every setting the server needs from environment
// variables. All problems are reported together, so one start-up attempt shows everything
// that has to be fixed.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Env         string // "development" or "production"
	Host        string // optional listen address, empty = all interfaces
	Port        string
	DatabaseURL string
	// DatabaseSchema optionally isolates this server's tables in their own Postgres schema
	// (used by the automated checks so they never touch your real tables).
	DatabaseSchema string

	JWTSecret    string
	JWTTTL       time.Duration
	CookieSecure bool

	CORSOrigins    []string
	TrustedProxies []string

	MLServiceURL string // empty means the ML service is disabled
	MLAPIKey     string
	MLTimeout    time.Duration

	AnthropicAPIKey  string
	AnthropicModel   string
	AnthropicBaseURL string
	AnthropicTimeout time.Duration

	RateAuthPerMin  int
	RateLegalPerMin int
	RateAPIPerMin   int
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// SaulConfigured reports whether an Anthropic API key is available.
func (c *Config) SaulConfigured() bool { return c.AnthropicAPIKey != "" }

// Defaults returns development defaults. It is NOT valid for serving (no database, no
// secret); Load fills those in from the environment.
func Defaults() *Config {
	return &Config{
		Env:              "development",
		Port:             "8080",
		JWTTTL:           72 * time.Hour,
		CORSOrigins:      []string{"http://localhost:3000"},
		TrustedProxies:   []string{},
		MLServiceURL:     "http://localhost:8000",
		MLTimeout:        1500 * time.Millisecond,
		AnthropicModel:   "claude-sonnet-5-5",
		AnthropicBaseURL: "https://api.anthropic.com",
		AnthropicTimeout: 25 * time.Second,
		RateAuthPerMin:   10,
		RateLegalPerMin:  5,
		RateAPIPerMin:    240,
	}
}

var schemaNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

var weakSecretMarkers = []string{
	"iamtheonewhoknocks", "changeme", "change-me", "replace-me", "your_secret", "your-secret",
	"secret123", "password",
}

// Load reads the process environment.
func Load() (*Config, error) { return LoadFrom(os.Getenv) }

// LoadFrom reads settings through get, which makes the loader easy to test.
func LoadFrom(get func(string) string) (*Config, error) {
	cfg := Defaults()
	var problems []error
	bad := func(format string, args ...any) { problems = append(problems, fmt.Errorf(format, args...)) }
	val := func(key string) string { return strings.TrimSpace(get(key)) }

	// --- environment ------------------------------------------------------
	if v := strings.ToLower(val("APP_ENV")); v != "" {
		if v != "development" && v != "production" {
			bad("APP_ENV must be 'development' or 'production' (got %q)", v)
		} else {
			cfg.Env = v
		}
	}

	if v := val("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 65535 {
			bad("PORT must be a number between 1 and 65535 (got %q)", v)
		} else {
			cfg.Port = v
		}
	}

	if v := val("LISTEN_HOST"); v != "" {
		if v != "localhost" && net.ParseIP(v) == nil {
			bad("LISTEN_HOST must be an IP address or 'localhost' (got %q)", v)
		} else {
			cfg.Host = v
		}
	}

	if cfg.DatabaseURL = val("DATABASE_URL"); cfg.DatabaseURL == "" {
		bad("DATABASE_URL is required")
	}
	if v := val("DATABASE_SCHEMA"); v != "" {
		if !schemaNamePattern.MatchString(v) {
			bad("DATABASE_SCHEMA must be lower-case letters, digits and underscores, starting with a letter or underscore (got %q)", v)
		} else {
			cfg.DatabaseSchema = v
		}
	}

	// --- auth -------------------------------------------------------------
	cfg.JWTSecret = val("JWT_SECRET")
	switch {
	case cfg.JWTSecret == "":
		bad("JWT_SECRET is required (generate one with: openssl rand -hex 32)")
	case len(cfg.JWTSecret) < 32:
		bad("JWT_SECRET must be at least 32 characters (it is %d)", len(cfg.JWTSecret))
	case isWeakSecret(cfg.JWTSecret):
		bad("JWT_SECRET looks like a placeholder or a guessable value; generate a random one")
	}

	if v := val("JWT_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 720 {
			bad("JWT_TTL_HOURS must be between 1 and 720 (got %q)", v)
		} else {
			cfg.JWTTTL = time.Duration(n) * time.Hour
		}
	}

	if v := val("COOKIE_SECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err != nil {
			bad("COOKIE_SECURE must be true or false (got %q)", v)
		} else {
			cfg.CookieSecure = b
		}
	}
	if cfg.IsProduction() && !cfg.CookieSecure {
		bad("COOKIE_SECURE must be true when APP_ENV=production (serve the API over HTTPS)")
	}

	// --- network ----------------------------------------------------------
	if v := val("CORS_ALLOWED_ORIGINS"); v != "" {
		origins, errs := parseOrigins(v)
		problems = append(problems, errs...)
		if len(errs) == 0 {
			cfg.CORSOrigins = origins
		}
	}

	if v := val("TRUSTED_PROXIES"); v != "" {
		for _, p := range splitList(v) {
			if net.ParseIP(p) == nil {
				if _, _, err := net.ParseCIDR(p); err != nil {
					bad("TRUSTED_PROXIES entry %q is not an IP address or CIDR range", p)
					continue
				}
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	// --- ML service -------------------------------------------------------
	if v := val("ML_SERVICE_URL"); v != "" {
		switch strings.ToLower(v) {
		case "none", "off", "disabled":
			cfg.MLServiceURL = ""
		default:
			if u, err := parseHTTPURL(v); err != nil {
				bad("ML_SERVICE_URL: %v", err)
			} else {
				cfg.MLServiceURL = strings.TrimRight(u, "/")
			}
		}
	}
	cfg.MLAPIKey = val("ML_API_KEY")
	if v := val("ML_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 50 || n > 30000 {
			bad("ML_TIMEOUT_MS must be between 50 and 30000 (got %q)", v)
		} else {
			cfg.MLTimeout = time.Duration(n) * time.Millisecond
		}
	}

	// --- Saul Goodman -----------------------------------------------------
	cfg.AnthropicAPIKey = val("ANTHROPIC_API_KEY")
	if v := val("ANTHROPIC_MODEL"); v != "" {
		cfg.AnthropicModel = v
	}
	if v := val("ANTHROPIC_BASE_URL"); v != "" {
		if u, err := parseHTTPURL(v); err != nil {
			bad("ANTHROPIC_BASE_URL: %v", err)
		} else {
			cfg.AnthropicBaseURL = strings.TrimRight(u, "/")
		}
	}
	if v := val("ANTHROPIC_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 120 {
			bad("ANTHROPIC_TIMEOUT_SEC must be between 1 and 120 (got %q)", v)
		} else {
			cfg.AnthropicTimeout = time.Duration(n) * time.Second
		}
	}

	// --- rate limits (0 disables a limiter) --------------------------------
	for _, r := range []struct {
		key string
		dst *int
	}{
		{"RATE_LIMIT_AUTH_PER_MIN", &cfg.RateAuthPerMin},
		{"RATE_LIMIT_LEGAL_PER_MIN", &cfg.RateLegalPerMin},
		{"RATE_LIMIT_API_PER_MIN", &cfg.RateAPIPerMin},
	} {
		if v := val(r.key); v != "" {
			if n, err := strconv.Atoi(v); err != nil || n < 0 || n > 100000 {
				bad("%s must be between 0 and 100000 (got %q)", r.key, v)
			} else {
				*r.dst = n
			}
		}
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return cfg, nil
}

func isWeakSecret(s string) bool {
	lower := strings.ToLower(s)
	for _, marker := range weakSecretMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	// A single repeated character ("aaaa...") is not a secret.
	return strings.Count(s, string(s[0])) == len(s)
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseHTTPURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%q is not a valid http(s) URL", raw)
	}
	return raw, nil
}

func parseOrigins(raw string) ([]string, []error) {
	var origins []string
	var errs []error
	for _, o := range splitList(raw) {
		if strings.Contains(o, "*") {
			errs = append(errs, fmt.Errorf("CORS_ALLOWED_ORIGINS must list exact origins, wildcards are not allowed (%q)", o))
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			errs = append(errs, fmt.Errorf("CORS_ALLOWED_ORIGINS entry %q must look like https://example.com (no path)", o))
			continue
		}
		origins = append(origins, u.Scheme+"://"+u.Host)
	}
	return origins, errs
}

var (
	mu      sync.RWMutex
	current *Config
)

// Set makes cfg the process-wide configuration returned by Current.
func Set(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	current = cfg
}

// Current returns the configuration installed by Set, or development defaults when none
// has been installed (for example in unit tests).
func Current() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return Defaults()
	}
	return current
}
