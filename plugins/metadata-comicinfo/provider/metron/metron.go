// Package metron is a client for the Metron comic database REST API.
//
// Metron (https://metron.cloud) is an open, community-maintained comic
// database. Authentication uses an API token from the account page. It is
// deliberately the last-resort online source in this plugin.
package metron

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

const baseURL = "https://metron.cloud/api"

type Client struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

func New(token string) *Client {
	return &Client{Token: token, BaseURL: baseURL, HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}

type mnNamed struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mnCredit struct {
	Creator mnNamed `json:"creator"`
	Role    mnNamed `json:"role"`
}

type mnIssue struct {
	ID         int        `json:"id"`
	Number     string     `json:"number"`
	Title      string     `json:"title"`
	Desc       string     `json:"desc"`
	CoverDate  string     `json:"cover_date"`
	StoreDate  string     `json:"store_date"`
	Image      string     `json:"image"`
	Series     mnNamed    `json:"series"`
	Publisher  mnNamed    `json:"publisher"`
	Credits    []mnCredit `json:"credits"`
	Characters []mnNamed  `json:"characters"`
	Teams      []mnNamed  `json:"teams"`
}

type mnResponse struct {
	Count    int       `json:"count"`
	Next     string    `json:"next"`
	Previous string    `json:"previous"`
	Results  []mnIssue `json:"results"`
}

type Provider struct {
	client *Client
	gate   *gate
	cache  *cache
}

func NewProvider(token string) *Provider {
	return &Provider{client: New(token), gate: newGate(minSpacing), cache: newCache()}
}

// limits lazily fills the gate so a provider built with a bare client still
// paces itself.
func (p *Provider) limits() {
	if p.gate == nil {
		p.gate = newGate(minSpacing)
	}
	if p.cache == nil {
		p.cache = newCache()
	}
}

func NewMetron(token string) metadata.Provider { return NewProvider(token) }

func (p *Provider) Name() string { return "metron" }

func (p *Provider) Search(q metadata.SearchQuery) ([]metadata.Match, error) {
	if p.client.Token == "" {
		return nil, fmt.Errorf("metron: missing token")
	}
	p.limits()
	title := q.Series
	if title == "" {
		title = q.Title
	}
	if title == "" {
		return nil, fmt.Errorf("metron: empty query")
	}
	// One issue per query: a name with no issue behind it cannot be answered, and
	// asking anyway spends a rate-limited budget on nothing.
	if strings.TrimSpace(q.Number) == "" {
		return nil, nil
	}
	cacheKey := fmt.Sprintf("search|%s|%s|%d", title, q.Number, q.Year)
	if cached, ok := p.cache.get(cacheKey); ok {
		return cached, nil
	}
	if ok, wait := p.gate.take(); !ok {
		return nil, fmt.Errorf("metron: rate limit cooldown, retry in %s", wait.Round(time.Second))
	}
	params := url.Values{}
	params.Set("series_name", title)
	if q.Number != "" {
		params.Set("number", q.Number)
	}
	if q.Year > 0 {
		params.Set("series_year_began", strconv.Itoa(q.Year))
	}
	req, err := http.NewRequest("GET", p.client.BaseURL+"/issue/?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.client.Token)
	req.Header.Set("User-Agent", "silo-plugin-metadata-comicinfo")
	resp, err := p.client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	p.gate.observe(resp)
	if resp.StatusCode == http.StatusTooManyRequests {
		// The cooldown came from the reset header when it was present. Nothing
		// retries immediately: this error reaches the host as transient, and the
		// next call is refused locally until the server's window reopens.
		return nil, fmt.Errorf("metron: HTTP 429, backing off for %s", p.gate.remaining())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metron: http %d", resp.StatusCode)
	}
	var r mnResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	out := make([]metadata.Match, 0, len(r.Results))
	for _, issue := range r.Results {
		out = append(out, toMatch(issue))
	}
	p.cache.put(cacheKey, out)
	return out, nil
}

func (p *Provider) Fetch(providerID string) (*metadata.Match, error) {
	if p.client.Token == "" {
		return nil, fmt.Errorf("metron: missing token")
	}
	p.limits()
	if ok, wait := p.gate.take(); !ok {
		return nil, fmt.Errorf("metron: rate limit cooldown, retry in %s", wait.Round(time.Second))
	}
	id := strings.TrimPrefix(providerID, "metron:")
	req, err := http.NewRequest("GET", p.client.BaseURL+"/issue/"+id+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.client.Token)
	req.Header.Set("User-Agent", "silo-plugin-metadata-comicinfo")
	resp, err := p.client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	p.gate.observe(resp)
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("metron: HTTP 429, backing off for %s", p.gate.remaining())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metron: http %d", resp.StatusCode)
	}
	var issue mnIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}
	m := toMatch(issue)
	return &m, nil
}

func (p *Provider) Close() error { return nil }

func toMatch(issue mnIssue) metadata.Match {
	m := metadata.Match{
		Provider:   "metron",
		ProviderID: fmt.Sprintf("metron:%d", issue.ID),
		Title:      issue.Title,
		Series:     issue.Series.Name,
		Number:     issue.Number,
		Summary:    stripHTML(issue.Desc),
		CoverURL:   issue.Image,
		Publisher:  issue.Publisher.Name,
	}
	for _, d := range []string{issue.CoverDate, issue.StoreDate} {
		if d == "" {
			continue
		}
		parts := strings.Split(d, "-")
		if len(parts) >= 1 {
			fmt.Sscan(parts[0], &m.Year)
		}
		if len(parts) >= 2 {
			fmt.Sscan(parts[1], &m.Month)
		}
		if len(parts) >= 3 {
			fmt.Sscan(parts[2], &m.Day)
		}
		break
	}
	for _, c := range issue.Credits {
		role := c.Role.Name
		if role == "" {
			role = "creator"
		}
		m.Creators = append(m.Creators, fmt.Sprintf("%s (%s)", c.Creator.Name, role))
	}
	for _, c := range issue.Characters {
		m.Characters = append(m.Characters, c.Name)
	}
	for _, t := range issue.Teams {
		m.Teams = append(m.Teams, t.Name)
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

// NewProviderWithClient lets tests inject a client with a custom base URL.
func NewProviderWithClient(c *Client) *Provider { return &Provider{client: c} }
