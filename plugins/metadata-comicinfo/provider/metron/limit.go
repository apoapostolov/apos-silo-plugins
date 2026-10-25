package metron

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

// Metron publishes X-RateLimit-Limit, X-RateLimit-Remaining and X-RateLimit-Reset
// on every response, and answers 429 once the window is spent. This gate is the
// reason a sweep cannot hammer it: a request is refused locally while a cooldown
// stands, and the cooldown is taken from the server's own reset header rather
// than guessed. Two thousand immediate retries after a 429 is what earned a rate
// limit warning from Metron in the first place.
const (
	defaultCooldown = time.Minute
	minSpacing      = time.Second
	cacheTTL        = 24 * time.Hour
	cacheLimit      = 4096
)

type gate struct {
	mu            sync.Mutex
	minSpacing    time.Duration
	last          time.Time
	cooldownUntil time.Time
	now           func() time.Time
}

func newGate(spacing time.Duration) *gate {
	return &gate{minSpacing: spacing, now: time.Now}
}

// take reports whether a request may go out now. A refusal carries how long the
// caller must wait, so the host can queue the item instead of retrying at once.
func (g *gate) take() (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if now.Before(g.cooldownUntil) {
		return false, g.cooldownUntil.Sub(now)
	}
	if !g.last.IsZero() {
		if elapsed := now.Sub(g.last); elapsed < g.minSpacing {
			return false, g.minSpacing - elapsed
		}
	}
	g.last = now
	return true, 0
}

// observe applies the server's own accounting. When nothing is left in the
// window the gate closes until the reset the response names, so the next request
// is never the one that draws a 429.
func (g *gate) observe(resp *http.Response) {
	remaining := strings.TrimSpace(resp.Header.Get("X-RateLimit-Remaining"))
	if remaining != "" && remaining != "0" {
		return
	}
	if remaining == "0" {
		g.cool(g.resetAfter(resp))
	}
}

// resetAfter reads X-RateLimit-Reset, which Metron sends as a unix timestamp but
// other proxies send as seconds from now. Both are handled rather than assumed.
func (g *gate) resetAfter(resp *http.Response) time.Duration {
	raw := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset"))
	if raw == "" {
		return defaultCooldown
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultCooldown
	}
	if value > 1_000_000_000 {
		until := time.Unix(value, 0)
		if wait := until.Sub(g.now()); wait > 0 {
			return wait
		}
		return defaultCooldown
	}
	if value > 0 {
		return time.Duration(value) * time.Second
	}
	return defaultCooldown
}

func (g *gate) cool(wait time.Duration) {
	if wait <= 0 {
		wait = defaultCooldown
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if until := g.now().Add(wait); until.After(g.cooldownUntil) {
		g.cooldownUntil = until
	}
}

// remaining reports the cooldown left, for error messages that tell the operator
// how long the provider is quiet for.
func (g *gate) remaining() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	if wait := g.cooldownUntil.Sub(g.now()); wait > 0 {
		return wait.Round(time.Second)
	}
	return 0
}

// cache keeps one answered query from costing a second request, which matters
// when the budget is measured in hundreds an hour.
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
