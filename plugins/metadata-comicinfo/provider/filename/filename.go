// Package filename infers comic metadata from archive file names.
package filename

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

// Provider parses series/issue/year from a filename.
type Provider struct{}

func New() *Provider { return &Provider{} }

func NewFilename() metadata.Provider { return &Provider{} }

func (p *Provider) Name() string { return "filename" }

var (
	// "Series Name #12 (2020)" or "Series Name 012 (2020)"
	issueRe = regexp.MustCompile(`(?i)^(.*?)[\s._-]*(?:#|no\.?|issue)?\s*(\d{1,4})(?:\s*\((\d{4})\))?`)
	yearRe  = regexp.MustCompile(`\((\d{4})\)`)
)

func (p *Provider) Search(q metadata.SearchQuery) ([]metadata.Match, error) {
	name := q.Filename
	if name == "" {
		name = q.Title
	}
	if name == "" {
		return nil, nil
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	base = strings.ReplaceAll(base, "_", " ")

	var year int
	if m := yearRe.FindStringSubmatch(base); len(m) == 2 {
		year, _ = strconv.Atoi(m[1])
	}

	series := strings.TrimSpace(base)
	number := ""
	if m := issueRe.FindStringSubmatch(base); len(m) >= 3 {
		series = strings.TrimSpace(m[1])
		number = m[2]
	}
	series = strings.Trim(series, " ._-")

	match := metadata.Match{
		Provider:   "filename",
		ProviderID: "filename:" + base,
		Title:      strings.TrimSpace(base),
		Series:     series,
		Number:     number,
		Year:       year,
		Confidence: 0.3,
	}
	return []metadata.Match{match}, nil
}

func (p *Provider) Fetch(string) (*metadata.Match, error) { return nil, nil }
func (p *Provider) Close() error                          { return nil }
