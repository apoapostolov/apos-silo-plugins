package apple

import (
	"context"
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"Turncloaks - D&D5E Dark Fantasy Actual Play": "turncloaks d and d5e dark fantasy actual play",
		"  The Magnus   Archives ":                    "the magnus archives",
		"WOE.BEGONE":                                  "woe begone",
		"Tale of The Manticore, a Dark Fantasy Dungeons & Dragons Audiodrama": "tale of the manticore a dark fantasy dungeons and dragons audiodrama",
	}
	for input, want := range cases {
		if got := NormalizeTitle(input); got != want {
			t.Fatalf("NormalizeTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSignificantTokensDropsFormatWords(t *testing.T) {
	got := SignificantTokens("Turncloaks - D&D5E Dark Fantasy Actual Play")
	if len(got) != 1 || got[0] != "turncloaks" {
		t.Fatalf("tokens = %v, want [turncloaks]", got)
	}
	// A title made only of noise still returns its words instead of nothing.
	if got := SignificantTokens("The Actual Play Podcast"); len(got) == 0 {
		t.Fatalf("all-noise title returned no tokens")
	}
}

func TestTitleSimilaritySeparatesTheCases(t *testing.T) {
	query := "Turncloaks - D&D5E Dark Fantasy Actual Play"

	strong := TitleSimilarity(query, "Turncloaks")
	if strong < 0.95 {
		t.Fatalf("Turncloaks scored %.2f, want >= 0.95", strong)
	}
	if plural := TitleSimilarity(query, "Turncloak"); plural < 0.8 {
		t.Fatalf("singular Turncloak scored %.2f, want >= 0.8", plural)
	}
	weak := TitleSimilarity(query, "You Meet In a Tavern - A TTRPG Actual Play Podcast")
	if weak >= MatchFloor {
		t.Fatalf("unrelated show scored %.2f, want below floor %.2f", weak, MatchFloor)
	}
	if weak := TitleSimilarity(query, "Dungeon Delve - An Official Dungeons & Dragons Podcast"); weak >= MatchFloor {
		t.Fatalf("genre neighbour scored %.2f, want below floor", weak)
	}

	magnus := TitleSimilarity("The Magnus Archives", "The Magnus Archives")
	if magnus < 0.99 {
		t.Fatalf("exact title scored %.2f", magnus)
	}
	extra := TitleSimilarity("The Magnus Archives", "The Magnus Archives - Rusty Quill")
	if extra < 0.7 {
		t.Fatalf("title with publisher suffix scored %.2f", extra)
	}
}

func TestQueryVariants(t *testing.T) {
	variants := QueryVariants("Turncloaks - D&D5E Dark Fantasy Actual Play")
	if len(variants) < 2 {
		t.Fatalf("expected several variants, got %v", variants)
	}
	if variants[0] != "Turncloaks - D&D5E Dark Fantasy Actual Play" {
		t.Fatalf("first variant must be the raw query, got %q", variants[0])
	}
	if variants[1] != "Turncloaks" {
		t.Fatalf("second variant should be the clause before the dash, got %q", variants[1])
	}
	for _, variant := range variants {
		if variant == "" {
			t.Fatalf("empty variant in %v", variants)
		}
	}
}

func TestRankCandidatesOrdersByConfidence(t *testing.T) {
	shows := []Show{
		{AppleID: "1", Title: "You Meet In a Tavern - A TTRPG Actual Play Podcast", Artist: "Dungeons and Dragons"},
		{AppleID: "2", Title: "Turncloaks", Artist: "EncounterRoleplay"},
		{AppleID: "3", Title: "Dungeon Delve", Artist: "Dungeons & Dragons"},
		{AppleID: "2", Title: "Turncloaks", Artist: "EncounterRoleplay"},
	}
	ranked := rankCandidates("Turncloaks - D&D5E Dark Fantasy Actual Play", shows, MatchFloor, 10)
	if len(ranked) == 0 || ranked[0].AppleID != "2" {
		t.Fatalf("expected Turncloaks first, got %+v", ranked)
	}
	if len(ranked) > 1 && ranked[0].Score < ranked[len(ranked)-1].Score {
		t.Fatalf("results not sorted by score: %+v", ranked)
	}
	for _, candidate := range ranked[1:] {
		if candidate.Title == "Dungeon Delve" {
			t.Fatalf("a genre neighbour above the floor slipped through: %+v", ranked)
		}
	}
}

// TestSearchScoredLive proves the multi-query path against the real directory.
// Run with: go test -tags live -run LiveSearchScored ./provider/apple
func TestSearchScoredLiveSkipsNoise(t *testing.T) {
	if testing.Short() {
		t.Skip("live directory call")
	}
	client := NewClient("us")
	scored, err := client.SearchScored(context.Background(), "Tale of The Manticore, a Dark Fantasy Dungeons & Dragons Audiodrama", 5)
	if err != nil {
		t.Fatalf("SearchScored: %v", err)
	}
	if len(scored) == 0 {
		t.Fatal("no candidates for a show that exists")
	}
	top := scored[0]
	t.Logf("top: %q score=%.2f id=%s", top.Title, top.Score, top.AppleID)
	if top.Score < 0.9 {
		t.Fatalf("top score %.2f for an exact show", top.Score)
	}
}
