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
	// A year is only trustworthy inside brackets, where comic namers put it:
	// "Batman (2016)". Bare four digit runs are part of the title far too often
	// ("X-Men 2099", "2000 AD") to be read as anything else.
	yearRe = regexp.MustCompile(`[\(\[]\s*(\d{4})\s*[\)\]]`)

	// Reprint volume: "Vol. 2", "Volume 3", "v2". Removed before anything else
	// so it cannot be mistaken for the issue number.
	volumeRe = regexp.MustCompile(`(?i)(?:[\s._-]*\bvol(?:ume)?\.?\s*\d{1,3}\b)|(?:[\s._-]+\bv\d{1,3}\b)`)

	// An explicit issue marker is the strongest signal a name can carry.
	issueMarkerRe = regexp.MustCompile(`(?i)^(.*?)[\s._-]*(?:#|issue\s*|no\.?\s*|nr\.?\s*)(\d{1,5})\b`)

	// Anything after the issue token is packaging noise: "(Digital)", "c2c",
	// "scan", "webrip". It must not become part of the issue number.
	trailingRe = regexp.MustCompile(`(?i)^[\s._-]*(?:digital|scan|scanned|c2c|fixed|repack|webrip|web-?dl|hq|hd|complete)\b`)

	numberTokenRe = regexp.MustCompile(`\d{1,5}`)
)

// minimumYear and maximumYear bound what may be read as a publication year.
const (
	minimumYear = 1900
	maximumYear = 2100
)

// Parse splits a file name into series, issue number and year.
//
// The order of the rules matters. An explicit "#12" wins because a name that
// carries one is never ambiguous. Without a marker the last number is the
// issue, since namers put the issue at the end, and a bracketed year was
// already removed. A leading number is part of the title: "2000 AD 1234" is
// issue 1234 of a series called 2000 AD, not issue 1 of a series called 2000.
func Parse(name string) (series, number string, year int) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.Join(strings.Fields(base), " ")

	if m := yearRe.FindStringSubmatch(base); len(m) == 2 {
		if parsed, err := strconv.Atoi(m[1]); err == nil && parsed >= minimumYear && parsed <= maximumYear {
			year = parsed
		}
	}

	// Drop bracketed groups and reprint volume markers: neither carries the
	// issue, and both pollute the series when left in place.
	base = yearRe.ReplaceAllString(base, " ")
	base = volumeRe.ReplaceAllString(base, " ")
	base = strings.Join(strings.Fields(base), " ")

	if m := issueMarkerRe.FindStringSubmatch(base); len(m) == 3 {
		series = cleanSeries(m[1])
		number = m[2]
		rest := base[len(m[0]):]
		if loc := trailingRe.FindStringIndex(rest); loc != nil {
			rest = rest[loc[1]:]
		}
		if strings.TrimSpace(rest) == "" && series != "" {
			return series, number, year
		}
		if series != "" {
			return series, number, year
		}
	}

	// No marker: the last number in the name is the issue, everything before it
	// is the series.
	locs := numberTokenRe.FindAllStringIndex(base, -1)
	seriesEnd := -1
	for index := len(locs) - 1; index >= 0; index-- {
		token := base[locs[index][0]:locs[index][1]]
		candidate := cleanSeries(base[:locs[index][0]])
		if len(token) == 4 && candidate != "" {
			// A bare four digit run with a real series name in front of it is a
			// year: "Judge Dredd 1977" has no issue number. A series whose name
			// ends in digits keeps them, so the run is only the year when the
			// text before it survives cleaning.
			if parsed, err := strconv.Atoi(token); err == nil && parsed >= minimumYear && parsed <= maximumYear {
				if year == 0 {
					year = parsed
				}
				seriesEnd = locs[index][0]
				continue
			}
		}
		if candidate == "" {
			continue
		}
		return candidate, token, year
	}
	if seriesEnd >= 0 {
		return cleanSeries(base[:seriesEnd]), "", year
	}

	return cleanSeries(base), "", year
}

// cleanSeries trims the separators a name keeps after the issue number is cut
// away, and folds the dash variants into a plain hyphen.
func cleanSeries(value string) string {
	value = strings.NewReplacer("–", "-", "—", "-").Replace(value)
	value = strings.Trim(value, " ._-#,")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func (p *Provider) Search(q metadata.SearchQuery) ([]metadata.Match, error) {
	name := q.Filename
	if name == "" {
		name = q.Title
	}
	if name == "" {
		return nil, nil
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	series, number, year := Parse(name)

	match := metadata.Match{
		Provider:   "filename",
		ProviderID: "filename:" + base,
		Title:      strings.Join(strings.Fields(base), " "),
		Series:     series,
		Number:     number,
		Year:       year,
		Confidence: 0.3,
	}
	return []metadata.Match{match}, nil
}

func (p *Provider) Fetch(string) (*metadata.Match, error) { return nil, nil }
func (p *Provider) Close() error                          { return nil }
