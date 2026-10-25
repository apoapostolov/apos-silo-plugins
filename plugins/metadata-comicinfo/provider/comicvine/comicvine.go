package comicvine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

const baseURL = "https://comicvine.gamespot.com/api"

type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type cvImage struct {
	OriginalURL string `json:"original_url"`
}

type cvCredit struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type cvNamed struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type cvIssue struct {
	ID              int        `json:"id"`
	Name            string     `json:"name"`
	IssueNumber     string     `json:"issue_number"`
	Description     string     `json:"description"`
	Image           cvImage    `json:"image"`
	Volume          cvNamed    `json:"volume"`
	CoverDate       string     `json:"cover_date"`
	StoreDate       string     `json:"store_date"`
	PersonCredits   []cvCredit `json:"person_credits"`
	CharCredits     []cvNamed  `json:"character_credits"`
	TeamCredits     []cvNamed  `json:"team_credits"`
	LocationCredits []cvNamed  `json:"location_credits"`
	Publisher       cvNamed    `json:"publisher"`
}

type cvResponse struct {
	Error               string    `json:"error"`
	NumberOfPageResults int       `json:"number_of_page_results"`
	Results             []cvIssue `json:"results"`
}

type Provider struct {
	client *Client
	budget *budget
	cache  *cache
}

func NewProvider(apiKey string) *Provider {
	return &Provider{client: New(apiKey), budget: newBudget(hourlyBudget, budgetWindow), cache: newCache()}
}

// limits lazily fills the gate so a provider built by a test with a bare client
// still spends budget and caches its answers.
func (p *Provider) limits() {
	if p.budget == nil {
		p.budget = newBudget(hourlyBudget, budgetWindow)
	}
	if p.cache == nil {
		p.cache = newCache()
	}
}

func (p *Provider) Name() string { return "comicvine" }

func (p *Provider) Search(q metadata.SearchQuery) ([]metadata.Match, error) {
	if p.client.APIKey == "" {
		return nil, fmt.Errorf("comicvine: missing API key")
	}
	p.limits()
	title := q.Series
	if title == "" {
		title = q.Title
	}
	if title == "" {
		return nil, fmt.Errorf("comicvine: empty query")
	}
	// A filename like "128. Justice League of America's Vibe #1-2, Katana #2
	// (2011) (Digital) (TPB)" has no single issue behind it, and ComicVine
	// cannot answer it. Spending budget on that shape is what exhausts the key
	// before a single usable item is reached, so the caller's parse decides
	// whether this provider is asked at all.
	if strings.TrimSpace(q.Number) == "" {
		return nil, nil
	}
	cacheKey := fmt.Sprintf("search|%s|%s|%d", title, q.Number, q.Year)
	if cached, ok := p.cache.get(cacheKey); ok {
		return cached, nil
	}
	if ok, wait := p.budget.take(); !ok {
		return nil, fmt.Errorf("comicvine: hourly request budget spent, retry in %s (HTTP 429 semantics)", wait.Round(time.Minute))
	}
	params := url.Values{}
	params.Set("api_key", p.client.APIKey)
	params.Set("format", "json")
	params.Set("query", title)
	params.Set("resources", "issue")
	params.Set("limit", "10")
	params.Set("filter", "issue_number:"+issueNumberFilter(q.Number))

	req, err := http.NewRequest("GET", p.client.BaseURL+"/search/?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "silo-plugin-metadata-comicinfo")
	resp, err := p.client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// ComicVine answers 420, not 429, when the key's hourly window is spent, and
	// the window is shared with every other client using that key.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 420 {
		p.budget.exhaust()
		return nil, fmt.Errorf("comicvine: HTTP %d, the API key is rate limited for this hour", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("comicvine: http %d: %s", resp.StatusCode, string(b))
	}
	var r cvResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	if r.Error != "OK" {
		return nil, fmt.Errorf("comicvine: %s", r.Error)
	}

	out := make([]metadata.Match, 0, len(r.Results))
	for _, issue := range r.Results {
		out = append(out, toMatch(issue))
	}
	p.cache.put(cacheKey, out)
	return out, nil
}

func (p *Provider) Fetch(providerID string) (*metadata.Match, error) {
	if p.client.APIKey == "" {
		return nil, fmt.Errorf("comicvine: missing API key")
	}
	p.limits()
	if ok, wait := p.budget.take(); !ok {
		return nil, fmt.Errorf("comicvine: hourly request budget spent, retry in %s (HTTP 429 semantics)", wait.Round(time.Minute))
	}
	id := strings.TrimPrefix(providerID, "comicvine:")
	params := url.Values{}
	params.Set("api_key", p.client.APIKey)
	params.Set("format", "json")
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/issue/4000-%s/?%s", p.client.BaseURL, id, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "silo-plugin-metadata-comicinfo")
	resp, err := p.client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comicvine: http %d", resp.StatusCode)
	}
	var wrap struct {
		Error   string  `json:"error"`
		Results cvIssue `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil {
		return nil, err
	}
	if wrap.Error != "OK" {
		return nil, fmt.Errorf("comicvine: %s", wrap.Error)
	}
	m := toMatch(wrap.Results)
	return &m, nil
}

func (p *Provider) Close() error { return nil }

// issueNumberFilter turns a parsed issue number into the form ComicVine filters
// on. File names are padded ("048") and the filter is not, so an unpadded value
// would return nothing at all.
func issueNumberFilter(number string) string {
	number = strings.TrimPrefix(strings.TrimSpace(number), "#")
	if number == "" {
		return number
	}
	if trimmed := strings.TrimLeft(number, "0"); trimmed != "" {
		return trimmed
	}
	return "0"
}

func toMatch(issue cvIssue) metadata.Match {
	// The host scores a candidate against the title it derived for the item, so
	// the title is the volume and issue the way a reader would name it. ComicVine
	// puts the story name in "name", which is empty on plenty of issues and never
	// matches an item title on its own.
	title := strings.TrimSpace(strings.Join([]string{issue.Volume.Name, issue.IssueNumber}, " "))
	if title == "" {
		title = issue.Name
	}
	m := metadata.Match{
		Provider:   "comicvine",
		ProviderID: fmt.Sprintf("comicvine:%d", issue.ID),
		Title:      title,
		Series:     issue.Volume.Name,
		Number:     issue.IssueNumber,
		Summary:    stripHTML(issue.Description),
		CoverURL:   issue.Image.OriginalURL,
	}
	if issue.CoverDate != "" {
		parts := strings.Split(issue.CoverDate, "-")
		if len(parts) >= 1 {
			fmt.Sscan(parts[0], &m.Year)
		}
		if len(parts) >= 2 {
			fmt.Sscan(parts[1], &m.Month)
		}
		if len(parts) >= 3 {
			fmt.Sscan(parts[2], &m.Day)
		}
	}
	if issue.Publisher.Name != "" {
		m.Publisher = issue.Publisher.Name
	}
	for _, c := range issue.PersonCredits {
		role := c.Role
		if role == "" {
			role = "creator"
		}
		m.Creators = append(m.Creators, fmt.Sprintf("%s (%s)", c.Name, role))
	}
	for _, c := range issue.CharCredits {
		m.Characters = append(m.Characters, c.Name)
	}
	for _, c := range issue.TeamCredits {
		m.Teams = append(m.Teams, c.Name)
	}
	for _, c := range issue.LocationCredits {
		m.Locations = append(m.Locations, c.Name)
	}
	return m
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// NewComicVine returns a metadata.Provider for ComicVine.
func NewComicVine(apiKey string) metadata.Provider {
	return NewProvider(apiKey)
}

// NewProviderWithClient lets tests inject a client with a custom base URL.
func NewProviderWithClient(c *Client) *Provider { return &Provider{client: c} }
