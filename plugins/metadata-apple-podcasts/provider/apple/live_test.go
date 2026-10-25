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

// TestLiveTurncloaks records what the directory actually returns for a show that
// Apo's library carries. It is documentation as much as a test: on 2026-10-25
// Apple listed nothing named Turncloaks in us, gb or ca, under the show name or
// its publisher EncounterRoleplay, so no query scores it above the floor.
func TestLiveTurncloaks(t *testing.T) {
	client := NewClient("us")
	scored, err := client.SearchScored(context.Background(), "Turncloaks - D&D5E Dark Fantasy Actual Play", 10)
	if err != nil {
		t.Fatalf("SearchScored: %v", err)
	}
	t.Logf("candidates above floor %.2f: %d", MatchFloor, len(scored))
	for _, candidate := range scored {
		t.Logf("  %.2f %q (%s)", candidate.Score, candidate.Title, candidate.AppleID)
	}
	for _, candidate := range scored {
		if candidate.Score >= 0.9 {
			t.Logf("strong match found: %q id=%s", candidate.Title, candidate.AppleID)
			return
		}
	}
	t.Log("no strong Turncloaks match in the directory")
}
