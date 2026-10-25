package comicvine

import (
	"sync"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

// ComicVine allows 200 requests per hour per API key and answers 429 past that.
// A library sweep asks about every item at once, so the provider spends its
// budget, then answers with an error the host treats as transient and retries
// later. Blocking would be worse: the enrichment call carries a deadline, and a
// sleep long enough to stay under the limit would fail the item instead of
// queueing it.
const (
	hourlyBudget = 180
	budgetWindow = time.Hour
	cacheTTL     = 24 * time.Hour
	cacheLimit   = 4096
)

// budget meters the requests spent inside a rolling window.
type budget struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	start  time.Time
	used   int
	now    func() time.Time
}

func newBudget(limit int, window time.Duration) *budget {
	return &budget{limit: limit, window: window, now: time.Now}
}

// take reports whether one request may be spent, and how long the caller must
// wait when it may not.
func (b *budget) take() (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	if b.start.IsZero() || now.Sub(b.start) >= b.window {
		b.start, b.used = now, 0
	}
	if b.used >= b.limit {
		return false, b.start.Add(b.window).Sub(now)
	}
	b.used++
	return true, 0
}

// exhaust records that the provider itself says the window is spent, which
// happens when another client shares the key. Everything is refused until the
// window reopens rather than spending request after request on 420s.
func (b *budget) exhaust() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.start, b.used = b.now(), b.limit
}

// cache keeps a query answered once inside a sweep from costing budget twice.
type cache struct {
	mu      sync.Mutex
	entries map[string]cachedEntry
	now     func() time.Time
}

type cachedEntry struct {
	matches []metadata.Match
	expires time.Time
}

func newCache() *cache {
	return &cache{entries: make(map[string]cachedEntry), now: time.Now}
}

func (c *cache) get(key string) ([]metadata.Match, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || c.now().After(entry.expires) {
		return nil, false
	}
	return append([]metadata.Match(nil), entry.matches...), true
}

func (c *cache) put(key string, matches []metadata.Match) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= cacheLimit {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[key] = cachedEntry{
		matches: append([]metadata.Match(nil), matches...),
		expires: c.now().Add(cacheTTL),
	}
}
