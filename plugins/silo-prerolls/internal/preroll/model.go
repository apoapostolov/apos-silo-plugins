package preroll

import (
	"path/filepath"
	"time"
)

// Preroll represents a discovered preroll video file.
type Preroll struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Filename    string    `json:"filename"`
	Folder      string    `json:"folder"`
	Tags        []string  `json:"tags"`
	Weight      int       `json:"weight"`
	Rating      string    `json:"rating"`
	Schedule    *Schedule `json:"schedule,omitempty"`
	AddedAt     time.Time `json:"added_at"`
	PlayCount   int       `json:"play_count"`
	LastPlayed  time.Time `json:"last_played"`
	SkipCount   int       `json:"skip_count"`
	SkipSeconds float64   `json:"skip_seconds_avg"`
}

type Schedule struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// PrerollPool holds all discovered prerolls.
type PrerollPool struct {
	Prerolls []Preroll `json:"prerolls"`
	LastScan time.Time  `json:"last_scan"`
}

// IsScheduled checks if the preroll is active based on its schedule.
func (p *Preroll) IsScheduled(now time.Time) bool {
	if p.Schedule == nil {
		return true
	}
	if p.Schedule.Start != "" {
		start, err := time.Parse("2006-01-02", p.Schedule.Start)
		if err == nil && now.Before(start) {
			return false
		}
	}
	if p.Schedule.End != "" {
		end, err := time.Parse("2006-01-02", p.Schedule.End)
		if err == nil && now.After(end) {
			return false
		}
	}
	return true
}

// HasTag checks if the preroll has a specific tag.
func (p *Preroll) HasTag(tag string) bool {
	for _, t := range p.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasAnyTag checks if the preroll has any of the given tags.
func (p *Preroll) HasAnyTag(tags []string) bool {
	for _, tag := range tags {
		if p.HasTag(tag) {
			return true
		}
	}
	return false
}

// HasAllTags checks if the preroll has all of the given tags.
func (p *Preroll) HasAllTags(tags []string) bool {
	for _, tag := range tags {
		if !p.HasTag(tag) {
			return false
		}
	}
	return true
}

// FileExtension returns the file extension of the preroll.
func (p *Preroll) FileExtension() string {
	return filepath.Ext(p.Filename)
}