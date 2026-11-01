package game

import (
	"math"
	"sync"
	"time"

	"github.com/heisenberg-os/backend/internal/db"
	"github.com/heisenberg-os/backend/internal/empire"
	"github.com/heisenberg-os/backend/internal/models"
)

// statsTTL is how long the public statistics are reused. The endpoint needs no login, so the
// database must not be queried on every request.
const statsTTL = 30 * time.Second

// Stats are aggregates over every player. They contain no personal data.
type Stats struct {
	Empires          int64   `json:"empires"`
	KilogramsCooked  float64 `json:"kilograms_cooked"`
	DirtyLaundered   float64 `json:"dirty_laundered"`
	FlaggedTransfers int64   `json:"flagged_transfers"`
	Raids            int64   `json:"raids"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// computeStats queries the database.
func computeStats() (*Stats, error) {
	var s Stats
	if err := db.DB.Model(&models.Empire{}).Count(&s.Empires).Error; err != nil {
		return nil, err
	}

	// An empire starts with a 10 kg batch; only what was cooked on top of that counts.
	var kg float64
	if err := db.DB.Model(&models.Empire{}).
		Select("COALESCE(SUM(GREATEST(batch_yield - ?, 0)), 0)", empire.StartBatchYield).Scan(&kg).Error; err != nil {
		return nil, err
	}
	s.KilogramsCooked = round2(kg)

	// Transaction.Amount is the gross dirty amount that went into a transfer.
	var dirty float64
	if err := db.DB.Model(&models.Transaction{}).Where("type = ?", models.TxLaundered).
		Select("COALESCE(SUM(amount), 0)").Scan(&dirty).Error; err != nil {
		return nil, err
	}
	s.DirtyLaundered = round2(dirty)

	if err := db.DB.Model(&models.Transaction{}).Where("flagged = ?", true).Count(&s.FlaggedTransfers).Error; err != nil {
		return nil, err
	}
	if err := db.DB.Model(&models.FrontBusiness{}).Where("shutdown_reason LIKE ?", "raided%").Count(&s.Raids).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// statsCache remembers the last result for ttl. The clock and the loader are fields so tests
// can drive them.
type statsCache struct {
	mu   sync.Mutex
	ttl  time.Duration
	now  func() time.Time
	load func() (*Stats, error)
	val  Stats
	at   time.Time
	ok   bool
}

func newStatsCache(now func() time.Time, ttl time.Duration, load func() (*Stats, error)) *statsCache {
	return &statsCache{ttl: ttl, now: now, load: load}
}

// get returns the cached statistics, reloading them when they are older than ttl. A failed
// load is never cached and is not masked by older numbers: the caller sees the error.
func (c *statsCache) get() (Stats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	// A clock that went backwards (age < 0) counts as expired rather than as fresh forever.
	if age := now.Sub(c.at); c.ok && age >= 0 && age < c.ttl {
		return c.val, nil
	}
	fresh, err := c.load()
	if err != nil {
		return Stats{}, err
	}
	c.val, c.at, c.ok = *fresh, now, true
	return c.val, nil
}

var defaultStats = newStatsCache(time.Now, statsTTL, computeStats)

// GetStats returns the public statistics, at most statsTTL old.
func GetStats() (Stats, error) { return defaultStats.get() }
