// Package apple reads the Apple Podcasts directory and show feeds.
//
// Apple publishes the same catalogue the Podcasts app shows over a public JSON
// API, so nothing here scrapes HTML. Episode-level detail comes from each
// show's own RSS feed, which is what the Podcasts app reads too.
package apple

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	searchEndpoint = "https://itunes.apple.com/search"
	lookupEndpoint = "https://itunes.apple.com/lookup"

	defaultTimeout = 20 * time.Second
	maxBodyBytes   = 8 << 20

	// defaultCountry is the storefront searched when the plugin config names none.
	defaultCountry = "us"
)

// UserAgent identifies the plugin to Apple, as their API etiquette asks.
const UserAgent = "apos-silo-plugins/apple-podcasts"

// Client reads the Apple directory and podcast feeds.
type Client struct {
	HTTP       *http.Client
	searchURL  string
	lookupURL  string
	country    string
	feedTTL    time.Duration
	cacheMu    sync.Mutex
	feedCache  map[string]cachedFeed
	artworkURL map[string]cachedArtwork
}

type cachedFeed struct {
	channel Channel
	expires time.Time
}

type cachedArtwork struct {
	url     string
	expires time.Time
}

// NewClient builds a client with sane defaults. country is the ISO storefront
// (us, gb, bg, …); empty means us.
func NewClient(country string) *Client {
	country = strings.ToLower(strings.TrimSpace(country))
	if country == "" {
		country = defaultCountry
	}
	return &Client{
		HTTP:       &http.Client{Timeout: defaultTimeout},
		searchURL:  searchEndpoint,
		lookupURL:  lookupEndpoint,
		country:    country,
		feedTTL:    6 * time.Hour,
		feedCache:  map[string]cachedFeed{},
		artworkURL: map[string]cachedArtwork{},
	}
}

// WithEndpoints overrides the directory endpoints. Tests use it; production
// never calls it.
func (c *Client) WithEndpoints(search, lookup string) {
	if search != "" {
		c.searchURL = search
	}
	if lookup != "" {
		c.lookupURL = lookup
	}
}

// Show is one directory entry, normalized.
type Show struct {
	AppleID      string
	Title        string
	Artist       string
	FeedURL      string
	ArtworkURL   string
	Genre        string
	Genres       []string
	EpisodeCount int
	ReleaseDate  string
	Language     string
	Explicit     bool
	AppleURL     string
}

// Channel is the channel-level metadata of a show feed.
type Channel struct {
	Title            string
	Description      string
	Author           string
	Language         string
	Link             string
	ArtworkURL       string
	Categories       []string
	Explicit         bool
	FirstEpisodeDate string
	LastEpisodeDate  string
	EpisodeCount     int
}

type searchResponse struct {
	ResultCount int             `json:"resultCount"`
	Results     []directoryShow `json:"results"`
}

type directoryShow struct {
	WrapperType     string   `json:"wrapperType"`
	CollectionID    int64    `json:"collectionId"`
	CollectionName  string   `json:"collectionName"`
	ArtistName      string   `json:"artistName"`
	FeedURL         string   `json:"feedUrl"`
	ArtworkURL600   string   `json:"artworkUrl600"`
	ArtworkURL100   string   `json:"artworkUrl100"`
	PrimaryGenre    string   `json:"primaryGenreName"`
	Genres          []string `json:"genres"`
	TrackCount      int      `json:"trackCount"`
	ReleaseDate     string   `json:"releaseDate"`
	Country         string   `json:"country"`
	Language        string   `json:"language"`
	Explicitness    string   `json:"collectionExplicitness"`
	CollectionView  string   `json:"collectionViewUrl"`
	ContentAdvisory string   `json:"contentAdvisoryRating"`
}

func (d directoryShow) show() Show {
	artwork := d.ArtworkURL600
	if artwork == "" {
		artwork = d.ArtworkURL100
	}
	genres := d.Genres
	if len(genres) == 0 && d.PrimaryGenre != "" {
		genres = []string{d.PrimaryGenre}
	}
	return Show{
		AppleID:      strconv.FormatInt(d.CollectionID, 10),
		Title:        strings.TrimSpace(d.CollectionName),
		Artist:       strings.TrimSpace(d.ArtistName),
		FeedURL:      strings.TrimSpace(d.FeedURL),
		ArtworkURL:   artwork,
		Genre:        strings.TrimSpace(d.PrimaryGenre),
		Genres:       genres,
		EpisodeCount: d.TrackCount,
		ReleaseDate:  strings.TrimSpace(d.ReleaseDate),
		Language:     strings.TrimSpace(d.Language),
		Explicit:     strings.EqualFold(d.Explicitness, "explicit") || strings.EqualFold(d.ContentAdvisory, "explicit"),
		AppleURL:     strings.TrimSpace(d.CollectionView),
	}
}

// Search returns directory matches for a show name.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Show, error) {
	return c.searchIn(ctx, query, c.country, limit)
}

// searchIn runs one directory query against a specific storefront.
func (c *Client) searchIn(ctx context.Context, query, country string, limit int) ([]Show, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("apple: empty search query")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if country == "" {
		country = c.country
	}
	url := fmt.Sprintf("%s?term=%s&media=podcast&entity=podcast&limit=%d&country=%s",
		c.searchURL, urlEncode(query), limit, urlEncode(country))
	var payload searchResponse
	if err := c.getJSON(ctx, url, &payload); err != nil {
		return nil, err
	}
	shows := make([]Show, 0, len(payload.Results))
	for _, entry := range payload.Results {
		// Only shows carry a feed; audiobook hits sneak into podcast searches.
		if strings.TrimSpace(entry.FeedURL) == "" {
			continue
		}
		shows = append(shows, entry.show())
	}
	return shows, nil
}

// SearchScored gathers candidates from several query shapes, widens to other
// storefronts when the first attempt finds nothing, and ranks everything against
// the original title. This is what makes a long on-disk title usable.
func (c *Client) SearchScored(ctx context.Context, query string, limit int) ([]ScoredShow, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("apple: empty search query")
	}
	if limit <= 0 {
		limit = 10
	}
	variants := QueryVariants(query)
	gathered := make([]Show, 0, 64)
	seen := map[string]struct{}{}
	collect := func(shows []Show) {
		for _, show := range shows {
			if _, ok := seen[show.AppleID]; ok {
				continue
			}
			seen[show.AppleID] = struct{}{}
			gathered = append(gathered, show)
		}
	}

	for _, variant := range variants {
		shows, err := c.searchIn(ctx, variant, c.country, 50)
		if err != nil {
			// A rate-limited or empty variant must not lose the others.
			continue
		}
		collect(shows)
	}

	// Nothing at all: the show may be carried only in another storefront.
	if len(gathered) == 0 {
		for _, country := range []string{"us", "gb", "ca", "au"} {
			if country == c.country {
				continue
			}
			if shows, err := c.searchIn(ctx, variants[0], country, 50); err == nil && len(shows) > 0 {
				collect(shows)
				break
			}
		}
	}

	return rankCandidates(query, gathered, MatchFloor, limit), nil
}

// Lookup returns one show by Apple collection id.
func (c *Client) Lookup(ctx context.Context, appleID string) (Show, error) {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return Show{}, errors.New("apple: empty collection id")
	}
	url := fmt.Sprintf("%s?id=%s&country=%s", c.lookupURL, urlEncode(appleID), urlEncode(c.country))
	var payload searchResponse
	if err := c.getJSON(ctx, url, &payload); err != nil {
		return Show{}, err
	}
	for _, entry := range payload.Results {
		if strconv.FormatInt(entry.CollectionID, 10) == appleID {
			return entry.show(), nil
		}
	}
	return Show{}, fmt.Errorf("apple: no show for collection id %s", appleID)
}

// Resolve accepts an Apple id or an Apple Podcasts page URL and returns the show.
func (c *Client) Resolve(ctx context.Context, identifier string) (Show, error) {
	identifier = strings.TrimSpace(identifier)
	if id := CollectionIDFromURL(identifier); id != "" {
		return c.Lookup(ctx, id)
	}
	return Show{}, fmt.Errorf("apple: %q is not an Apple collection id or show URL", identifier)
}

var (
	collectionIDPattern = regexp.MustCompile(`id(\d{4,})`)
	bareIDPattern       = regexp.MustCompile(`^\d{4,}$`)
	artworkSizePattern  = regexp.MustCompile(`/\d+x\d+bb\.(jpg|jpeg|png|webp)$`)
	htmlTagPattern      = regexp.MustCompile(`<[^>]+>`)
	spacePattern        = regexp.MustCompile(`\s+`)
)

// CollectionIDFromURL pulls the collection id out of an Apple Podcasts URL or
// a bare id string. It returns "" when neither shape matches.
func CollectionIDFromURL(value string) string {
	value = strings.TrimSpace(value)
	if bareIDPattern.MatchString(value) {
		return value
	}
	if match := collectionIDPattern.FindStringSubmatch(value); len(match) == 2 {
		return match[1]
	}
	return ""
}

// ArtworkAt rewrites an Apple artwork URL to a square of the given size. Apple
// serves one image at any size by swapping the "<n>x<n>bb" path segment.
func ArtworkAt(url string, size int) string {
	if url == "" {
		return ""
	}
	if size < 64 {
		size = 64
	}
	if size > 3000 {
		size = 3000
	}
	if artworkSizePattern.MatchString(url) {
		return artworkSizePattern.ReplaceAllString(url, fmt.Sprintf("/%dx%dbb.jpg", size, size))
	}
	return url
}

// SizeForVariant maps a host variant hint onto a pixel size. The variant
// vocabulary is open, so anything unknown degrades to the featured size rather
// than failing.
func SizeForVariant(variant string) int {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "card":
		return 400
	case "featured":
		return 800
	case "large":
		return 1200
	case "full":
		return 2000
	case "original":
		return 3000
	case "":
		return 1200
	default:
		return 800
	}
}

// ArtworkFor returns the artwork URL of a show at the requested size, using a
// short-lived cache because the image resolver calls it per request.
func (c *Client) ArtworkFor(ctx context.Context, appleID string, size int) (string, error) {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return "", errors.New("apple: empty collection id")
	}
	c.cacheMu.Lock()
	cached, ok := c.artworkURL[appleID]
	c.cacheMu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return ArtworkAt(cached.url, size), nil
	}
	show, err := c.Lookup(ctx, appleID)
	if err != nil {
		return "", err
	}
	if show.ArtworkURL == "" {
		return "", fmt.Errorf("apple: show %s has no artwork", appleID)
	}
	c.cacheMu.Lock()
	c.artworkURL[appleID] = cachedArtwork{url: show.ArtworkURL, expires: time.Now().Add(c.feedTTL)}
	c.cacheMu.Unlock()
	return ArtworkAt(show.ArtworkURL, size), nil
}

type rssFeed struct {
	Channel struct {
		Title       string     `xml:"title"`
		Description string     `xml:"description"`
		Summary     string     `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd summary"`
		Author      string     `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author"`
		Language    string     `xml:"language"`
		Link        string     `xml:"link"`
		Image       rssImage   `xml:"image"`
		ItunesImage rssImage   `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image"`
		Categories  []rssCat   `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd category"`
		Explicit    string     `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd explicit"`
		Items       []rssEntry `xml:"item"`
	} `xml:"channel"`
}

type rssImage struct {
	Href string `xml:"href,attr"`
	URL  string `xml:"url"`
}

type rssCat struct {
	Text string `xml:"text,attr"`
}

type rssEntry struct {
	Title   string `xml:"title"`
	PubDate string `xml:"pubDate"`
}

// Feed reads the channel metadata of a show feed, cached for a few hours.
func (c *Client) Feed(ctx context.Context, feedURL string) (Channel, error) {
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return Channel{}, errors.New("apple: feed URL is empty")
	}
	c.cacheMu.Lock()
	cached, ok := c.feedCache[feedURL]
	c.cacheMu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.channel, nil
	}

	body, err := c.get(ctx, feedURL)
	if err != nil {
		return Channel{}, err
	}
	var parsed rssFeed
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return Channel{}, fmt.Errorf("apple: feed %s is not valid XML: %w", feedURL, err)
	}
	channel := parsed.Channel
	artwork := channel.ItunesImage.Href
	if artwork == "" {
		artwork = channel.Image.URL
	}
	categories := make([]string, 0, len(channel.Categories))
	for _, category := range channel.Categories {
		if text := strings.TrimSpace(category.Text); text != "" {
			categories = append(categories, text)
		}
	}
	result := Channel{
		Title:       strings.TrimSpace(channel.Title),
		Description: CleanText(firstNonEmpty(channel.Description, channel.Summary), 4000),
		Author:      strings.TrimSpace(channel.Author),
		Language:    strings.ToLower(strings.TrimSpace(channel.Language)),
		Link:        strings.TrimSpace(channel.Link),
		ArtworkURL:  strings.TrimSpace(artwork),
		Categories:  categories,
		Explicit:    strings.EqualFold(strings.TrimSpace(channel.Explicit), "yes") || strings.EqualFold(strings.TrimSpace(channel.Explicit), "true"),
	}
	result.EpisodeCount = len(channel.Items)
	// The feed lists newest first, but rely on the dates rather than the order.
	for _, item := range channel.Items {
		date := parsePubDate(item.PubDate)
		if date == "" {
			continue
		}
		if result.FirstEpisodeDate == "" || date < result.FirstEpisodeDate {
			result.FirstEpisodeDate = date
		}
		if date > result.LastEpisodeDate {
			result.LastEpisodeDate = date
		}
	}

	c.cacheMu.Lock()
	c.feedCache[feedURL] = cachedFeed{channel: result, expires: time.Now().Add(c.feedTTL)}
	c.cacheMu.Unlock()
	return result, nil
}

// CleanText strips markup and collapses whitespace in feed copy.
func CleanText(value string, limit int) string {
	if value == "" {
		return ""
	}
	text := htmlTagPattern.ReplaceAllString(value, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(spacePattern.ReplaceAllString(text, " ")), " ")
	if limit > 0 && len(text) > limit {
		text = strings.TrimSpace(text[:limit])
	}
	return text
}

// parsePubDate normalizes RFC 1123-ish feed dates to YYYY-MM-DD.
func parsePubDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	layouts := []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, "Mon, 2 Jan 2006 15:04:05 -0700", time.RFC3339}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}

func (c *Client) getJSON(ctx context.Context, url string, target any) error {
	body, err := c.get(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("apple: response from %s is not JSON: %w", url, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json, application/rss+xml, application/xml;q=0.9, */*;q=0.5")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple: GET %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("apple: read %s: %w", url, err)
	}
	return body, nil
}

func urlEncode(value string) string {
	return url.QueryEscape(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
