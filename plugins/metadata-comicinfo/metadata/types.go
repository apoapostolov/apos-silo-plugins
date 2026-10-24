// Package metadata defines the common metadata model and provider contract
// used by the comic scraping providers.
package metadata

// Match is a normalized metadata result from any provider.
type Match struct {
	Provider   string
	ProviderID string
	Title      string
	Series     string
	Number     string
	Count      string
	Volume     string
	Year       int
	Month      int
	Day        int
	Summary    string
	Publisher  string
	Imprint    string
	Genre      string
	Tags       []string
	Characters []string
	Teams      []string
	Locations  []string
	Creators   []string // "Name (Role)"
	AgeRating  string
	PageCount  int
	CoverURL   string
	Language   string
	GTIN       string
	ISBN       string
	Confidence float64
}

// SearchQuery is the input to a provider search.
type SearchQuery struct {
	Title       string
	Series      string
	Number      string
	Year        int
	Filename    string
	ProviderIDs map[string]string
}

// Provider is implemented by each metadata source.
type Provider interface {
	Name() string
	// Search returns candidate matches for a query.
	Search(q SearchQuery) ([]Match, error)
	// Fetch returns full metadata for a known provider ID (if supported).
	Fetch(providerID string) (*Match, error)
	// Close releases resources.
	Close() error
}
