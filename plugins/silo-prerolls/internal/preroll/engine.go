package preroll

import (
	"strings"
	"time"
)

// Engine handles smart preroll selection with all rules.
type Engine struct {
	Selector     *Selector
	Config       *ConfigRef
	Cooldowns    map[string]time.Time // itemID_prerollID -> last played
	PlayCounts   map[string]int       // prerollID -> total plays
	UserPlays    map[string]map[string]int // userID -> prerollID -> plays
	SessionState map[string]*Session  // userID -> session state
	OptOuts      map[string]bool      // itemID -> opted out
}

// ConfigRef holds the configuration needed by the engine.
type ConfigRef struct {
	CooldownHours   int
	MaturityGating  bool
	SessionMode     string
	PerUserRules    map[string]UserRuleRef
	LibraryRules    []LibraryRuleRef
	OptOuts         []string
}

type UserRuleRef struct {
	Mode  string
	Users []string
}

type LibraryRuleRef struct {
	Name        string
	Library     string
	ItemType    string
	RequireTags []string
	ExcludeTags []string
	Enabled     bool
}

// Session tracks playback state within a session.
type Session struct {
	UserID         string
	StartedAt      time.Time
	PlayedItems    map[string]bool // itemIDs played this session
	FirstOfSession bool
	FirstOfBinge   bool
	LastPlayedAt   time.Time
}

// NewEngine creates a new selection engine.
func NewEngine(mode SelectionMode, cooldownHrs int) *Engine {
	return &Engine{
		Selector:     NewSelector(mode, cooldownHrs),
		Cooldowns:    make(map[string]time.Time),
		PlayCounts:   make(map[string]int),
		UserPlays:    make(map[string]map[string]int),
		SessionState: make(map[string]*Session),
		OptOuts:      make(map[string]bool),
	}
}

// IsOptedOut checks if an item is opted out of prerolls.
func (e *Engine) IsOptedOut(itemID string) bool {
	return e.OptOuts[itemID]
}

// CheckMaturity checks if a preroll is appropriate for the feature's rating.
func (e *Engine) CheckMaturity(prerollRating string, featureRating string, enabled bool) bool {
	if !enabled || prerollRating == "" || featureRating == "" {
		return true
	}
	// Simple rating hierarchy: G < PG < PG-13 < R < NC-17
	ratingOrder := map[string]int{
		"G":     1,
		"PG":    2,
		"PG-13": 3,
		"R":     4,
		"NC-17": 5,
		"TV-Y":  1,
		"TV-Y7": 2,
		"TV-G":  2,
		"TV-PG": 3,
		"TV-14": 4,
		"TV-MA": 5,
	}
	pr := ratingOrder[prerollRating]
	fr := ratingOrder[featureRating]
	if pr == 0 || fr == 0 {
		return true // unknown ratings, allow
	}
	return pr <= fr
}

// CheckPerUser checks if a user is allowed prerolls based on per-user rules.
func (e *Engine) CheckPerUser(userID string, rule UserRuleRef) bool {
	if rule.Mode == "" || rule.Mode == "all" {
		return true
	}
	inList := false
	for _, u := range rule.Users {
		if u == userID {
			inList = true
			break
		}
	}
	if rule.Mode == "only" {
		return inList
	}
	if rule.Mode == "except" {
		return !inList
	}
	return true
}

// CheckLibraryRules checks if prerolls match library rules.
func (e *Engine) CheckLibraryRules(prerollTags []string, library string, itemType string, rules []LibraryRuleRef) bool {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.Library != "" && rule.Library != library {
			continue
		}
		if rule.ItemType != "" && rule.ItemType != itemType {
			continue
		}
		// Check require tags
		if len(rule.RequireTags) > 0 {
			hasAll := true
			for _, tag := range rule.RequireTags {
				found := false
				for _, pt := range prerollTags {
					if strings.EqualFold(pt, tag) {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if !hasAll {
				continue
			}
		}
		// Check exclude tags
		if len(rule.ExcludeTags) > 0 {
			hasExcluded := false
			for _, tag := range rule.ExcludeTags {
				for _, pt := range prerollTags {
					if strings.EqualFold(pt, tag) {
						hasExcluded = true
						break
					}
				}
				if hasExcluded {
					break
				}
			}
			if hasExcluded {
				continue
			}
		}
		return true
	}
	// If no rules match, allow by default
	if len(rules) == 0 {
		return true
	}
	return false
}

// CheckSessionMode checks if a preroll should play based on session mode.
func (e *Engine) CheckSessionMode(userID string, itemID string, mode string) bool {
	session := e.GetOrCreateSession(userID)

	switch mode {
	case "every_playback":
		return true
	case "first_of_session":
		if session.FirstOfSession {
			session.FirstOfSession = false
			return true
		}
		return false
	case "once_per_day":
		// Check if any preroll played today
		today := time.Now().Truncate(24 * time.Hour)
		if session.LastPlayedAt.Before(today) {
			session.LastPlayedAt = time.Now()
			return true
		}
		return false
	case "first_of_binge":
		// First episode of a series binge
		if !session.PlayedItems[itemID] {
			session.PlayedItems[itemID] = true
			return true
		}
		return false
	default:
		return true
	}
}

// GetOrCreateSession gets or creates a session for a user.
func (e *Engine) GetOrCreateSession(userID string) *Session {
	if s, ok := e.SessionState[userID]; ok {
		// Check if session expired (4 hours)
		if time.Since(s.StartedAt) < 4*time.Hour {
			return s
		}
	}
	s := &Session{
		UserID:         userID,
		StartedAt:      time.Now(),
		PlayedItems:    make(map[string]bool),
		FirstOfSession: true,
		FirstOfBinge:   true,
		LastPlayedAt:   time.Now(),
	}
	e.SessionState[userID] = s
	return s
}

// RecordPlay records a preroll play.
func (e *Engine) RecordPlay(prerollID string, itemID string, userID string) {
	key := itemID + "_" + prerollID
	e.Cooldowns[key] = time.Now()
	e.PlayCounts[prerollID]++
	if e.UserPlays[userID] == nil {
		e.UserPlays[userID] = make(map[string]int)
	}
	e.UserPlays[userID][prerollID]++
	e.Selector.RecordPlay(prerollID, itemID)
}

// CheckCooldown checks if a preroll is on cooldown for an item.
func (e *Engine) CheckCooldown(prerollID string, itemID string, cooldownHrs int) bool {
	if cooldownHrs <= 0 {
		return true
	}
	key := itemID + "_" + prerollID
	if last, ok := e.Cooldowns[key]; ok {
		if time.Since(last) < time.Duration(cooldownHrs)*time.Hour {
			return false
		}
	}
	return true
}

// IsSeasonal checks if a preroll should play based on seasonal dates.
func (e *Engine) IsSeasonal(preroll *Preroll, now time.Time) bool {
	if preroll.Schedule == nil {
		return true
	}
	return preroll.IsScheduled(now)
}

// FilterPool filters the preroll pool based on all rules.
func (e *Engine) FilterPool(pool []Preroll, itemID string, userID string, featureRating string, library string, itemType string, tags []string) []Preroll {
	var result []Preroll

	for _, p := range pool {
		// Check opt-outs
		if e.IsOptedOut(itemID) {
			continue
		}

		// Check cooldown
		if !e.CheckCooldown(p.ID, itemID, e.Config.CooldownHours) {
			continue
		}

		// Check maturity
		if !e.CheckMaturity(p.Rating, featureRating, e.Config.MaturityGating) {
			continue
		}

		// Check per-user rules
		userRule := e.Config.PerUserRules[itemID]
		if userRule.Mode != "" {
			if !e.CheckPerUser(userID, userRule) {
				continue
			}
		}

		// Check library rules
		if !e.CheckLibraryRules(p.Tags, library, itemType, e.Config.LibraryRules) {
			continue
		}

		// Check seasonal
		if !e.IsSeasonal(&p, time.Now()) {
			continue
		}

		// Check tag matching (if tags specified, preroll must have at least one)
		if len(tags) > 0 && !p.HasAnyTag(tags) {
			continue
		}

		result = append(result, p)
	}

	return result
}