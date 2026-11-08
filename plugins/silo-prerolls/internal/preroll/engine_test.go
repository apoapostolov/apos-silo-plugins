package preroll

import (
	"testing"
)

func TestSelectorRandom(t *testing.T) {
	pool := []Preroll{
		{ID: "a", Weight: 1},
		{ID: "b", Weight: 1},
		{ID: "c", Weight: 1},
	}
	sel := NewSelector(ModeRandom, 0)
	for i := 0; i < 100; i++ {
		got := sel.Select(pool, "item1", "user1")
		if got == nil {
			t.Fatal("expected selection, got nil")
		}
	}
}

func TestSelectorWeighted(t *testing.T) {
	pool := []Preroll{
		{ID: "a", Weight: 10},
		{ID: "b", Weight: 1},
	}
	sel := NewSelector(ModeWeighted, 0)
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		got := sel.Select(pool, "item1", "user1")
		counts[got.ID]++
	}
	if counts["a"] < 800 {
		t.Errorf("expected 'a' to be selected more often, got %d", counts["a"])
	}
}

func TestSelectorEqualRotation(t *testing.T) {
	pool := []Preroll{
		{ID: "a", Weight: 1},
		{ID: "b", Weight: 1},
	}
	sel := NewSelector(ModeEqualRotation, 0)
	got1 := sel.Select(pool, "item1", "user1")
	sel.RecordPlay(got1.ID, "item1")
	got2 := sel.Select(pool, "item1", "user1")
	if got1.ID == got2.ID {
		t.Error("expected different preroll on second selection (equal rotation)")
	}
}

func TestMaturityGating(t *testing.T) {
	engine := NewEngine(ModeRandom, 0)
	if !engine.CheckMaturity("PG", "PG-13", true) {
		t.Error("PG should be allowed for PG-13")
	}
	if engine.CheckMaturity("R", "PG-13", true) {
		t.Error("R should not be allowed for PG-13")
	}
	if !engine.CheckMaturity("R", "PG-13", false) {
		t.Error("maturity gating disabled should allow all")
	}
}

func TestPerUserRules(t *testing.T) {
	engine := NewEngine(ModeRandom, 0)
	rule := UserRuleRef{Mode: "only", Users: []string{"user1"}}
	if !engine.CheckPerUser("user1", rule) {
		t.Error("user1 should be allowed")
	}
	if engine.CheckPerUser("user2", rule) {
		t.Error("user2 should not be allowed")
	}
}

func TestSessionMode(t *testing.T) {
	engine := NewEngine(ModeRandom, 0)
	if !engine.CheckSessionMode("user1", "item1", "every_playback") {
		t.Error("every_playback should always return true")
	}
	// first_of_session: first call true, second false
	if !engine.CheckSessionMode("user2", "item1", "first_of_session") {
		t.Error("first call should return true")
	}
	if engine.CheckSessionMode("user2", "item2", "first_of_session") {
		t.Error("second call should return false")
	}
}

func TestCooldown(t *testing.T) {
	engine := NewEngine(ModeRandom, 1)
	if !engine.CheckCooldown("p1", "item1", 1) {
		t.Error("first check should pass (no cooldown)")
	}
	engine.RecordPlay("p1", "item1", "user1")
	if engine.CheckCooldown("p1", "item1", 1) {
		t.Error("should be on cooldown after play")
	}
}

func TestFilterPool(t *testing.T) {
	engine := NewEngine(ModeRandom, 0)
	engine.Config = &ConfigRef{
		CooldownHours:  0,
		MaturityGating: true,
		LibraryRules:   []LibraryRuleRef{},
		OptOuts:        []string{},
	}
	pool := []Preroll{
		{ID: "a", Rating: "PG", Tags: []string{"action"}},
		{ID: "b", Rating: "R", Tags: []string{"horror"}},
	}
	filtered := engine.FilterPool(pool, "item1", "user1", "PG-13", "Movies", "movie", nil)
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered preroll, got %d", len(filtered))
	}
	if filtered[0].ID != "a" {
		t.Errorf("expected 'a', got %s", filtered[0].ID)
	}
}