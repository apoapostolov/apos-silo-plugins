package config

// Config holds the prerolls plugin configuration.
type Config struct {
	Enabled            bool     `json:"enabled"`
	Folders            []Folder `json:"folders"`
	Triggers           Triggers `json:"triggers"`
	SelectionMode      string   `json:"selection_mode"`
	CooldownHours      int      `json:"cooldown_hours"`
	SessionMode        string   `json:"session_mode"`
	MaturityGating     bool     `json:"maturity_gating"`
	Skippable          bool     `json:"skippable"`
	SkipDelaySeconds   int      `json:"skip_delay_seconds"`
	FeaturePreload     bool     `json:"feature_preload"`
	PostRollsEnabled   bool     `json:"post_rolls_enabled"`
	PostRollsFolder    string   `json:"post_rolls_folder"`
	TrailerMode        bool     `json:"trailer_mode"`
	TrailerCount       int      `json:"trailer_count"`
	ComingSoonTrailer  bool     `json:"coming_soon_trailer"`
	SeriesSpecific     bool     `json:"series_specific_prerolls"`
	SeriesPrerollName  string   `json:"series_preroll_filename"`
	AllowedFormats     []string `json:"allowed_formats"`
	OptOuts            []string `json:"opt_outs"`
	PerUserRules       map[string]UserRule `json:"per_user_rules"`
	LibraryRules       []LibraryRule `json:"library_rules"`
}

type Folder struct {
	Path        string   `json:"path"`
	DefaultTags []string `json:"default_tags"`
}

type Triggers struct {
	Movies      bool `json:"movies"`
	Episodes    bool `json:"episodes"`
	MusicVideos bool `json:"music_videos"`
}

type UserRule struct {
	Mode  string   `json:"mode"` // "all", "only", "except"
	Users []string `json:"users"`
}

type LibraryRule struct {
	Name          string   `json:"name"`
	Library       string   `json:"library"`
	ItemType      string   `json:"item_type"`
	RequireTags   []string `json:"require_tags"`
	ExcludeTags   []string `json:"exclude_tags"`
	Enabled       bool     `json:"enabled"`
}

// SidecarMeta holds per-file metadata from sidecar .json files.
type SidecarMeta struct {
	Tags     []string `json:"tags"`
	Weight   int      `json:"weight"`
	Rating   string   `json:"rating"`
	Schedule *Schedule `json:"schedule,omitempty"`
}

type Schedule struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		Folders:           []Folder{},
		Triggers:          Triggers{Movies: true, Episodes: true, MusicVideos: false},
		SelectionMode:     "random",
		CooldownHours:     12,
		SessionMode:       "every_playback",
		MaturityGating:    true,
		Skippable:         true,
		SkipDelaySeconds:  5,
		FeaturePreload:    true,
		PostRollsEnabled:  false,
		TrailerMode:       false,
		TrailerCount:      2,
		ComingSoonTrailer: false,
		SeriesSpecific:    true,
		SeriesPrerollName: "theme-preroll",
		AllowedFormats:    []string{".mp4", ".mkv", ".mov", ".webm", ".avi", ".m4v"},
		OptOuts:           []string{},
		PerUserRules:      map[string]UserRule{},
		LibraryRules:      []LibraryRule{},
	}
}