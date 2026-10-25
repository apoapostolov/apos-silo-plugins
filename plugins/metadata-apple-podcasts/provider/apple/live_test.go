//go:build live

package apple

import (
	"context"
	"testing"
)

// These tests hit the real Apple directory and a real feed. Run them with:
//
//	go test -tags live -run Live ./provider/apple
func TestLiveSearchAndFeed(t *testing.T) {
	client := NewClient("us")
	shows, err := client.Search(context.Background(), "The Magnus Archives", 5)
	if err != nil {
		t.Fatalf("live search: %v", err)
	}
	if len(shows) == 0 {
		t.Fatal("live search returned no shows")
	}
	show := shows[0]
	t.Logf("show: id=%s title=%q artist=%q genre=%q episodes=%d", show.AppleID, show.Title, show.Artist, show.Genre, show.EpisodeCount)
	if show.FeedURL == "" {
		t.Fatal("show has no feed URL")
	}

	channel, err := client.Feed(context.Background(), show.FeedURL)
	if err != nil {
		t.Fatalf("live feed: %v", err)
	}
	t.Logf("channel: author=%q language=%q episodes=%d last=%q description=%.80q",
		channel.Author, channel.Language, channel.EpisodeCount, channel.LastEpisodeDate, channel.Description)
	if channel.Description == "" {
		t.Fatal("feed returned no description")
	}

	artwork, err := client.ArtworkFor(context.Background(), show.AppleID, 1200)
	if err != nil {
		t.Fatalf("live artwork: %v", err)
	}
	t.Logf("artwork: %s", artwork)
}
