package preroll

import (
	"math/rand"
	"time"
)

// SelectionMode defines how prerolls are selected.
type SelectionMode string

const (
	ModeRandom          SelectionMode = "random"
	ModeSequential      SelectionMode = "sequential"
	ModeWeighted        SelectionMode = "weighted"
	ModeEqualRotation   SelectionMode = "equal_rotation"
	ModeRecencyBoost    SelectionMode = "recency_boost"
)

// Selector handles preroll selection logic.
type Selector struct {
	mode         SelectionMode
	cooldownHrs  int
	cooldownMap  map[string]time.Time // itemID -> last played time
	playCounts   map[string]int       // prerollID -> play count
	sequentialIdx int
}

// NewSelector creates a new selector with the given mode and cooldown.
func NewSelector(mode SelectionMode, cooldownHrs int) *Selector {
	return &Selector{
		mode:        mode,
		cooldownHrs: cooldownHrs,
		cooldownMap: make(map[string]time.Time),
		playCounts:  make(map[string]int),
	}
}

// Select picks a preroll from the pool based on the selection mode.
func (s *Selector) Select(pool []Preroll, itemID string, userID string) *Preroll {
	if len(pool) == 0 {
		return nil
	}

	// Filter by cooldown
	candidates := s.filterCooldown(pool, itemID)
	if len(candidates) == 0 {
		candidates = pool // fallback if all on cooldown
	}

	// Filter by schedule
	now := time.Now()
	var scheduled []Preroll
	for _, p := range candidates {
		if p.IsScheduled(now) {
			scheduled = append(scheduled, p)
		}
	}
	if len(scheduled) > 0 {
		candidates = scheduled
	}

	switch s.mode {
	case ModeRandom:
		return s.selectRandom(candidates)
	case ModeSequential:
		return s.selectSequential(candidates)
	case ModeWeighted:
		return s.selectWeighted(candidates)
	case ModeEqualRotation:
		return s.selectEqualRotation(candidates)
	case ModeRecencyBoost:
		return s.selectRecencyBoost(candidates)
	default:
		return s.selectRandom(candidates)
	}
}

func (s *Selector) filterCooldown(pool []Preroll, itemID string) []Preroll {
	if s.cooldownHrs <= 0 {
		return pool
	}
	cutoff := time.Now().Add(-time.Duration(s.cooldownHrs) * time.Hour)
	var result []Preroll
	for _, p := range pool {
		if last, ok := s.cooldownMap[itemID+"_"+p.ID]; ok {
			if last.Before(cutoff) {
				result = append(result, p)
			}
		} else {
			result = append(result, p)
		}
	}
	return result
}

func (s *Selector) selectRandom(pool []Preroll) *Preroll {
	idx := rand.Intn(len(pool))
	p := &pool[idx]
	return p
}

func (s *Selector) selectSequential(pool []Preroll) *Preroll {
	idx := s.sequentialIdx % len(pool)
	s.sequentialIdx++
	p := &pool[idx]
	return p
}

func (s *Selector) selectWeighted(pool []Preroll) *Preroll {
	totalWeight := 0
	for _, p := range pool {
		w := p.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	if totalWeight == 0 {
		return s.selectRandom(pool)
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for i := range pool {
		w := pool[i].Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			p := &pool[i]
			return p
		}
	}
	p := &pool[len(pool)-1]
	return p
}

func (s *Selector) selectEqualRotation(pool []Preroll) *Preroll {
	// Pick the preroll with the fewest plays
	minPlays := -1
	var candidates []int
	for i := range pool {
		plays := s.playCounts[pool[i].ID]
		if minPlays == -1 || plays < minPlays {
			minPlays = plays
			candidates = []int{i}
		} else if plays == minPlays {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return s.selectRandom(pool)
	}
	idx := candidates[rand.Intn(len(candidates))]
	p := &pool[idx]
	return p
}

func (s *Selector) selectRecencyBoost(pool []Preroll) *Preroll {
	// Newly added files (< 7 days) get 2x weight
	now := time.Now()
	totalWeight := 0
	weights := make([]int, len(pool))
	for i, p := range pool {
		w := p.Weight
		if w <= 0 {
			w = 1
		}
		if now.Sub(p.AddedAt) < 7*24*time.Hour {
			w *= 2
		}
		weights[i] = w
		totalWeight += w
	}
	if totalWeight == 0 {
		return s.selectRandom(pool)
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for i := range pool {
		cumulative += weights[i]
		if r < cumulative {
			p := &pool[i]
			return p
		}
	}
	p := &pool[len(pool)-1]
	return p
}

// RecordPlay records a preroll play for cooldown and stats.
func (s *Selector) RecordPlay(prerollID string, itemID string) {
	s.cooldownMap[itemID+"_"+prerollID] = time.Now()
	s.playCounts[prerollID]++
}

// RecordSkip records a skip for stats.
func (s *Selector) RecordSkip(prerollID string, seconds float64) {
	// Stats tracking would be handled by the storage layer
}