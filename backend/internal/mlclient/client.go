// Package mlclient talks to the Python ML service. Every call is optional: when the service
// is switched off, down or slow, callers get an error and fall back to the built-in rules,
// so the game keeps working. A small circuit breaker stops a dead service from adding its
// timeout to every request.
package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/heisenberg-os/backend/internal/config"
)

var (
	// ErrDisabled means no ML service URL is configured.
	ErrDisabled = errors.New("ml service is disabled")
	// ErrUnavailable means the service could not be reached or failed.
	ErrUnavailable = errors.New("ml service is unavailable")
)

const (
	breakerThreshold = 3
	breakerCooldown  = 15 * time.Second
	maxResponseBytes = 1 << 20
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

	mu        sync.Mutex
	failures  int
	openUntil time.Time
	now       func() time.Time
}

// New builds a client from the configuration. An empty MLServiceURL gives a disabled client.
func New(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.MLServiceURL, "/"),
		apiKey:  cfg.MLAPIKey,
		http:    &http.Client{Timeout: cfg.MLTimeout},
		now:     time.Now,
	}
}

// Enabled reports whether an ML service URL is configured.
func (c *Client) Enabled() bool { return c != nil && c.baseURL != "" }

// ---------------------------------------------------------------- data types

type ThreatInput struct {
	EmpireID            uint    `json:"empire_id"`
	HeatLevel           float64 `json:"heat_level"`
	FlaggedTransactions int     `json:"flagged_transactions"`
	BatchYield          float64 `json:"batch_yield"`
	TerritoryCount      int     `json:"territory_count"`
	DaysActive          int     `json:"days_active"`
}

type ThreatResult struct {
	ThreatScore    float64 `json:"threat_score"`
	RiskLevel      string  `json:"risk_level"`
	Prediction     string  `json:"prediction"`
	Recommendation string  `json:"recommendation"`
}

// YieldInput.EffortLevel is on the ML service's 0-100 scale (cook effort 1-10 times 10).
type YieldInput struct {
	CurrentPurity float64 `json:"current_purity"`
	EffortLevel   float64 `json:"effort_level"`
	SupplyQuality float64 `json:"supply_quality"`
	LabCapacity   float64 `json:"lab_capacity"`
}

type YieldResult struct {
	PredictedYield   float64 `json:"predicted_yield"`
	OptimalEffort    float64 `json:"optimal_effort"`
	OptimalYield     float64 `json:"optimal_yield"`
	PurityProjection float64 `json:"purity_projection"`
	EfficiencyScore  float64 `json:"efficiency_score"`
}

type LaunderingInput struct {
	Amounts          []float64 `json:"amounts"`
	BusinessRevenues []float64 `json:"business_revenues"`
	TimeGaps         []float64 `json:"time_gaps"`
	BusinessType     string    `json:"business_type"`
}

type LaunderingResult struct {
	RiskScore          float64  `json:"risk_score"`
	AnomalyDetected    bool     `json:"anomaly_detected"`
	SuspiciousPatterns []string `json:"suspicious_patterns"`
	Recommendation     string   `json:"recommendation"`
	RevenueRatio       *float64 `json:"revenue_ratio"`
}

type EvidenceInput struct {
	EmpireID uint    `json:"empire_id"`
	Source   string  `json:"source"`
	Weight   float64 `json:"weight"`
	Kind     string  `json:"kind,omitempty"`
}

type EvidenceSource struct {
	Source string  `json:"source"`
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
}

type EvidenceSummary struct {
	EmpireID    uint             `json:"empire_id"`
	NodeCount   int              `json:"node_count"`
	TotalWeight float64          `json:"total_weight"`
	TopSources  []EvidenceSource `json:"top_sources"`
}

// ------------------------------------------------------------------- calls

func (c *Client) AnalyzeThreat(ctx context.Context, in ThreatInput) (*ThreatResult, error) {
	var out ThreatResult
	if err := c.do(ctx, http.MethodPost, "/ml/threat/analyze", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) OptimizeYield(ctx context.Context, in YieldInput) (*YieldResult, error) {
	var out YieldResult
	if err := c.do(ctx, http.MethodPost, "/ml/yield/optimize", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AnalyzeLaundering(ctx context.Context, in LaunderingInput) (*LaunderingResult, error) {
	var out LaunderingResult
	if err := c.do(ctx, http.MethodPost, "/ml/laundering/analyze", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) AddEvidence(ctx context.Context, in EvidenceInput) (*EvidenceSummary, error) {
	var out EvidenceSummary
	if err := c.do(ctx, http.MethodPost, "/ml/evidence", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetEvidence(ctx context.Context, empireID uint) (*EvidenceSummary, error) {
	var out EvidenceSummary
	path := "/ml/evidence/" + url.PathEscape(strconv.FormatUint(uint64(empireID), 10))
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Health checks that the service answers AND accepts our API key. It uses a cheap protected
// endpoint on purpose: the public /health page would still answer with a wrong key.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/ml/evidence/0", nil, nil)
}

// ---------------------------------------------------------------- plumbing

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	if !c.allow() {
		return fmt.Errorf("%w (cooling down after repeated failures)", ErrUnavailable)
	}

	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err // the caller gave up; that says nothing about the service
		}
		c.record(false)
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		c.record(false)
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if resp.StatusCode >= 500 {
		c.record(false)
		return fmt.Errorf("%w: HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	c.record(true) // a 4xx is our mistake, not an outage
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("ml service rejected the request (HTTP %d): %s", resp.StatusCode, snippet(data))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			c.record(false)
			return fmt.Errorf("%w: unreadable response: %v", ErrUnavailable, err)
		}
	}
	return nil
}

func (c *Client) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.now().Before(c.openUntil)
}

// record updates the breaker. After the cooldown one request is let through; if it fails
// the breaker re-opens at once because the failure count is still at the threshold.
func (c *Client) record(ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ok {
		c.failures = 0
		c.openUntil = time.Time{}
		return
	}
	c.failures++
	if c.failures >= breakerThreshold {
		c.openUntil = c.now().Add(breakerCooldown)
	}
}

func snippet(data []byte) string {
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// ------------------------------------------------------- process-wide client

var (
	defMu sync.RWMutex
	def   *Client
)

// SetDefault installs the client the service packages use.
func SetDefault(c *Client) {
	defMu.Lock()
	defer defMu.Unlock()
	def = c
}

// Default returns the installed client, or a disabled one. It is never nil.
func Default() *Client {
	defMu.RLock()
	defer defMu.RUnlock()
	if def == nil {
		return &Client{}
	}
	return def
}
