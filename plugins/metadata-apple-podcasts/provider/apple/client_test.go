package apple

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const searchFixture = `{
  "resultCount": 2,
  "results": [
    {
      "wrapperType": "audiobook",
      "collectionId": 9,
      "collectionName": "Not a show",
      "artistName": "Nobody"
    },
    {
      "wrapperType": "collection",
      "collectionId": 1095138637,
      "collectionName": "The Magnus Archives",
      "artistName": "Rusty Quill",
      "feedUrl": "https://feeds.example.test/magnus",
      "artworkUrl600": "https://is1-ssl.mzstatic.com/image/thumb/x/100x100bb.jpg",
      "primaryGenreName": "Performing Arts",
      "genres": ["Performing Arts", "Fiction"],
      "trackCount": 401,
      "releaseDate": "2016-03-23T07:00:00Z",
      "language": "en",
      "collectionExplicitness": "notExplicit",
      "collectionViewUrl": "https://podcasts.apple.com/us/podcast/the-magnus-archives/id1095138637"
    }
  ]
}`

const feedFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>The Magnus Archives</title>
    <description>&lt;p&gt;A &lt;b&gt;horror&lt;/b&gt; anthology&lt;/p&gt;</description>
    <language>en</language>
    <itunes:author>Rusty Quill</itunes:author>
    <itunes:image href="https://is1-ssl.mzstatic.com/image/thumb/y/600x600bb.jpg"/>
    <itunes:category text="Arts"/>
    <itunes:explicit>no</itunes:explicit>
    <item>
      <title>Episode 58</title>
      <pubDate>Thu, 22 Oct 2026 17:45:00 +0300</pubDate>
    </item>
    <item>
      <title>Episode 57</title>
      <pubDate>Mon, 12 Oct 2026 17:45:00 +0300</pubDate>
    </item>
  </channel>
</rss>`

func testClient(t *testing.T) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search"):
			if got := r.URL.Query().Get("media"); got != "podcast" {
				t.Errorf("search media = %q, want podcast", got)
			}
			if got := r.URL.Query().Get("term"); got != "The Magnus Archives" {
				t.Errorf("search term = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(searchFixture))
		case strings.HasPrefix(r.URL.Path, "/lookup"):
			if got := r.URL.Query().Get("id"); got != "1095138637" {
				t.Errorf("lookup id = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(searchFixture))
		case r.URL.Path == "/feed":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(feedFixture))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := NewClient("US")
	client.WithEndpoints(server.URL+"/search", server.URL+"/lookup")
	return client
}

func TestSearchSkipsEntriesWithoutFeed(t *testing.T) {
	client := testClient(t)
	shows, err := client.Search(context.Background(), "The Magnus Archives", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(shows) != 1 {
		t.Fatalf("got %d shows, want 1 (the audiobook hit must be dropped)", len(shows))
	}
	show := shows[0]
	if show.AppleID != "1095138637" || show.Title != "The Magnus Archives" || show.Artist != "Rusty Quill" {
		t.Fatalf("unexpected show: %+v", show)
	}
	if show.FeedURL != "https://feeds.example.test/magnus" {
		t.Fatalf("feed URL = %q", show.FeedURL)
	}
	if show.EpisodeCount != 401 || show.Genre != "Performing Arts" {
		t.Fatalf("unexpected counts: %+v", show)
	}
}

func TestLookupAndResolve(t *testing.T) {
	client := testClient(t)
	show, err := client.Resolve(context.Background(), "https://podcasts.apple.com/us/podcast/the-magnus-archives/id1095138637")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if show.AppleID != "1095138637" {
		t.Fatalf("Resolve id = %q", show.AppleID)
	}
	if _, err := client.Resolve(context.Background(), "https://example.test/id1"); err == nil {
		t.Fatal("Resolve accepted a non-Apple URL")
	}
}

func TestFeedParsesChannelAndItems(t *testing.T) {
	client := NewClient("us")
	channel, err := client.Feed(context.Background(), "http://"+strings.TrimPrefix(testServerURL(t), "http://")+"/feed")
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if channel.Author != "Rusty Quill" || channel.Language != "en" {
		t.Fatalf("unexpected channel: %+v", channel)
	}
	if strings.Contains(channel.Description, "<") {
		t.Fatalf("description still has markup: %q", channel.Description)
	}
	if channel.Description != "A horror anthology" {
		t.Fatalf("description = %q", channel.Description)
	}
	if channel.EpisodeCount != 2 {
		t.Fatalf("episode count = %d", channel.EpisodeCount)
	}
	if channel.LastEpisodeDate != "2026-10-22" {
		t.Fatalf("last episode date = %q", channel.LastEpisodeDate)
	}
	// The feed lists newest first; the first episode date comes from the dates,
	// not from the item order.
	if channel.FirstEpisodeDate != "2026-10-12" {
		t.Fatalf("first episode date = %q", channel.FirstEpisodeDate)
	}
	if len(channel.Categories) != 1 || channel.Categories[0] != "Arts" {
		t.Fatalf("categories = %v", channel.Categories)
	}
}

// testServerURL is a helper for the feed test, which needs a real URL to fetch.
func testServerURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(feedFixture))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestArtworkAtAndVariant(t *testing.T) {
	const base = "https://is1-ssl.mzstatic.com/image/thumb/Podcasts115/v4/01/2b/f6/x/100x100bb.jpg"
	if got, want := ArtworkAt(base, 1200), "https://is1-ssl.mzstatic.com/image/thumb/Podcasts115/v4/01/2b/f6/x/1200x1200bb.jpg"; got != want {
		t.Fatalf("ArtworkAt = %q, want %q", got, want)
	}
	if got := ArtworkAt("", 600); got != "" {
		t.Fatalf("empty artwork = %q", got)
	}
	cases := map[string]int{
		"":         1200,
		"card":     400,
		"featured": 800,
		"large":    1200,
		"full":     2000,
		"original": 3000,
		"huge":     800,
	}
	for variant, want := range cases {
		if got := SizeForVariant(variant); got != want {
			t.Fatalf("SizeForVariant(%q) = %d, want %d", variant, got, want)
		}
	}
}

func TestArtworkForCachesLookup(t *testing.T) {
	client := testClient(t)
	first, err := client.ArtworkFor(context.Background(), "1095138637", 600)
	if err != nil {
		t.Fatalf("ArtworkFor: %v", err)
	}
	if !strings.Contains(first, "600x600bb") {
		t.Fatalf("artwork not resized: %q", first)
	}
	second, err := client.ArtworkFor(context.Background(), "1095138637", 300)
	if err != nil {
		t.Fatalf("ArtworkFor second: %v", err)
	}
	if !strings.Contains(second, "300x300bb") {
		t.Fatalf("cached artwork not resized: %q", second)
	}
}

func TestCollectionIDParsing(t *testing.T) {
	cases := map[string]string{
		"1095138637": "1095138637",
		"https://podcasts.apple.com/us/podcast/x/id1095138637": "1095138637",
		"id1542792309": "1542792309",
		"https://example.test/nothing": "",
		"not-an-id":                    "",
	}
	for input, want := range cases {
		if got := CollectionIDFromURL(input); got != want {
			t.Fatalf("CollectionIDFromURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCleanText(t *testing.T) {
	if got := CleanText("<p>Hello &amp;   welcome</p>", 40); got != "Hello & welcome" {
		t.Fatalf("CleanText = %q", got)
	}
	if got := CleanText(strings.Repeat("a", 50), 10); len(got) != 10 {
		t.Fatalf("CleanText limit = %d", len(got))
	}
}
