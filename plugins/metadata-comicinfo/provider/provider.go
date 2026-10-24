// Package provider orchestrates metadata scrapers for western comics.
package provider

import (
	"fmt"
	"strings"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/provider/comicvine"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/provider/filename"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/provider/gcd"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/provider/metron"
)

// Options holds configuration shared across providers.
type Options struct {
	ComicVineAPIKey string
	MetronToken     string
	GCDDBPath       string
	GCDDumpURL      string
	GCDDumpCookie   string
	GCDRefreshHours int
	EnableGCD       bool
	EnableComicVine bool
	EnableMetron    bool
	EnableFilename  bool
}

// Manager orchestrates providers in priority order.
type Manager struct {
	opts      Options
	providers []metadata.Provider
}

// NewManager creates a manager and registers providers in priority order:
// GCD local dump, ComicVine, Metron (distant last), then filename fallback.
func NewManager(opts Options) *Manager {
	m := &Manager{opts: opts}
	if opts.EnableGCD && opts.GCDDBPath != "" {
		if p, err := gcd.NewGCD(gcd.Options{
			DBPath:       opts.GCDDBPath,
			DumpURL:      opts.GCDDumpURL,
			DumpCookie:   opts.GCDDumpCookie,
			RefreshHours: opts.GCDRefreshHours,
		}); err == nil {
			m.providers = append(m.providers, p)
		}
	}
	if opts.EnableComicVine && opts.ComicVineAPIKey != "" {
		m.providers = append(m.providers, comicvine.NewComicVine(opts.ComicVineAPIKey))
	}
	if opts.EnableMetron && opts.MetronToken != "" {
		m.providers = append(m.providers, metron.NewMetron(opts.MetronToken))
	}
	if opts.EnableFilename {
		m.providers = append(m.providers, filename.NewFilename())
	}
	return m
}

// Search tries providers in priority order and returns the first non-empty result.
func (m *Manager) Search(q metadata.SearchQuery) ([]metadata.Match, metadata.Provider, error) {
	for _, p := range m.providers {
		matches, err := p.Search(q)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches, p, nil
		}
	}
	return nil, nil, fmt.Errorf("no matches from any provider")
}

// SearchAll returns results from all providers (for aggregation).
func (m *Manager) SearchAll(q metadata.SearchQuery) map[string][]metadata.Match {
	out := map[string][]metadata.Match{}
	for _, p := range m.providers {
		if matches, err := p.Search(q); err == nil && len(matches) > 0 {
			out[p.Name()] = matches
		}
	}
	return out
}

// Fetch returns full metadata for a provider item key such as
// "comicvine:123", "gcd:issue:9", or "metron:7". The provider named by the
// key's prefix answers; an unknown prefix returns nil.
func (m *Manager) Fetch(providerID string) (*metadata.Match, error) {
	name, _, _ := strings.Cut(providerID, ":")
	for _, p := range m.providers {
		if p.Name() == name {
			return p.Fetch(providerID)
		}
	}
	return nil, nil
}

// ProviderNames returns the names of the registered providers in priority
// order.
func (m *Manager) ProviderNames() []string {
	return m.Names()
}

// Close releases all providers.
func (m *Manager) Close() error {
	var first error
	for _, p := range m.providers {
		if err := p.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Names returns registered provider names.
func (m *Manager) Names() []string {
	out := make([]string, 0, len(m.providers))
	for _, p := range m.providers {
		out = append(out, p.Name())
	}
	return out
}
