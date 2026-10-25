// Package gcd reads comic metadata from a local Grand Comics Database SQLite dump.
//
// GCD publishes a full SQLite dump at https://www.comics.org/download/.
// The dump is large (several GB) and requires a free comics.org account.
package gcd

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
	_ "modernc.org/sqlite"
)

// Options configures the GCD provider.
type Options struct {
	DBPath       string
	DumpURL      string // optional direct download URL (may require cookie/basic auth)
	DumpCookie   string // optional Cookie header for the download
	RefreshHours int
}

// Provider queries a local GCD SQLite dump.
type Provider struct {
	opts Options
	db   *sql.DB
	mu   sync.Mutex
}

// New opens the GCD dump at opts.DBPath.
func New(opts Options) (*Provider, error) {
	if opts.DBPath == "" {
		return nil, fmt.Errorf("gcd: empty database path")
	}
	abs, err := filepath.Abs(opts.DBPath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", abs+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("gcd: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("gcd: ping: %w", err)
	}
	return &Provider{opts: opts, db: db}, nil
}

// NewGCD returns a metadata.Provider for GCD.
func NewGCD(opts Options) (metadata.Provider, error) {
	p, err := New(opts)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) Name() string { return "gcd" }

// Download fetches the dump to opts.DBPath. It is a no-op when DumpURL is empty.
func (p *Provider) Download() error {
	if p.opts.DumpURL == "" {
		return fmt.Errorf("gcd: no dump URL configured")
	}
	if err := os.MkdirAll(filepath.Dir(p.opts.DBPath), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequest("GET", p.opts.DumpURL, nil)
	if err != nil {
		return err
	}
	if p.opts.DumpCookie != "" {
		req.Header.Set("Cookie", p.opts.DumpCookie)
	}
	req.Header.Set("User-Agent", "silo-plugin-metadata-comicinfo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gcd: dump http %d", resp.StatusCode)
	}
	tmp := p.opts.DBPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p.opts.DBPath)
}

func (p *Provider) Search(q metadata.SearchQuery) ([]metadata.Match, error) {
	title := q.Series
	if title == "" {
		title = q.Title
	}
	if title == "" {
		return nil, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find matching series.
	rows, err := p.db.Query(`
		SELECT s.id, s.name, s.year_began, s.issue_count, COALESCE(p.name,'')
		FROM gcd_series s
		LEFT JOIN gcd_publisher p ON p.id = s.publisher_id
		WHERE s.name LIKE ? ESCAPE '\' COLLATE NOCASE
		LIMIT 10`, "%"+escapeLike(title)+"%")
	if err != nil {
		return nil, fmt.Errorf("gcd: series query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []metadata.Match
	for rows.Next() {
		var seriesID int
		var name, publisher string
		var yearBegan, issueCount sql.NullInt64
		if err := rows.Scan(&seriesID, &name, &yearBegan, &issueCount, &publisher); err != nil {
			return nil, err
		}
		m := metadata.Match{
			Provider:   "gcd",
			ProviderID: fmt.Sprintf("gcd:series:%d", seriesID),
			Title:      name,
			Series:     name,
			Publisher:  publisher,
			Count:      nullInt(issueCount),
			Year:       int(nullInt2(yearBegan)),
		}
		// If an issue number is requested, fetch that issue.
		if q.Number != "" {
			if issue, err := p.fetchIssue(seriesID, q.Number, q.Year); err == nil && issue != nil {
				m = *issue
			}
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (p *Provider) Fetch(providerID string) (*metadata.Match, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var issueID int
	if _, err := fmt.Sscanf(providerID, "gcd:issue:%d", &issueID); err != nil {
		return nil, fmt.Errorf("gcd: bad provider id %q", providerID)
	}
	return p.fetchIssueByID(issueID)
}

func (p *Provider) fetchIssue(seriesID int, number string, year int) (*metadata.Match, error) {
	query := `SELECT i.id, i.number, i.key_date, i.title, s.name, COALESCE(p.name,''), i.notes
		FROM gcd_issue i
		JOIN gcd_series s ON s.id = i.series_id
		LEFT JOIN gcd_publisher p ON p.id = s.publisher_id
		WHERE i.series_id = ? AND i.number = ? AND i.variant_of_id IS NULL`
	args := []any{seriesID, number}
	if year > 0 {
		query += ` AND (i.key_date LIKE ? OR i.key_date = '')`
		args = append(args, fmt.Sprintf("%d%%", year))
	}
	query += ` LIMIT 1`
	row := p.db.QueryRow(query, args...)
	return scanIssue(row, p.db)
}

func (p *Provider) fetchIssueByID(issueID int) (*metadata.Match, error) {
	row := p.db.QueryRow(`SELECT i.id, i.number, i.key_date, i.title, s.name, COALESCE(p.name,''), i.notes
		FROM gcd_issue i
		JOIN gcd_series s ON s.id = i.series_id
		LEFT JOIN gcd_publisher p ON p.id = s.publisher_id
		WHERE i.id = ?`, issueID)
	return scanIssue(row, p.db)
}

func scanIssue(row *sql.Row, db *sql.DB) (*metadata.Match, error) {
	var id int
	var number, keyDate, title, series, publisher, notes sql.NullString
	if err := row.Scan(&id, &number, &keyDate, &title, &series, &publisher, &notes); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m := &metadata.Match{
		Provider:   "gcd",
		ProviderID: fmt.Sprintf("gcd:issue:%d", id),
		Title:      title.String,
		Series:     series.String,
		Number:     number.String,
		Publisher:  publisher.String,
		Summary:    notes.String,
	}
	if keyDate.String != "" {
		parts := strings.Split(keyDate.String, "-")
		if len(parts) >= 1 {
			if year, err := strconv.Atoi(parts[0]); err == nil {
				m.Year = year
			}
		}
		if len(parts) >= 2 {
			if month, err := strconv.Atoi(parts[1]); err == nil {
				m.Month = month
			}
		}
		if len(parts) >= 3 {
			if day, err := strconv.Atoi(parts[2]); err == nil {
				m.Day = day
			}
		}
	}
	// Story titles, genres, synopsis.
	var storyTitle, genre, synopsis sql.NullString
	_ = db.QueryRow(`SELECT GROUP_CONCAT(title, ' / '), GROUP_CONCAT(genre, ','), GROUP_CONCAT(synopsis, ' ')
		FROM gcd_story WHERE issue_id = ?`, id).Scan(&storyTitle, &genre, &synopsis)
	if storyTitle.String != "" && m.Title == "" {
		m.Title = storyTitle.String
	}
	if genre.String != "" {
		m.Genre = genre.String
	}
	if synopsis.String != "" && m.Summary == "" {
		m.Summary = synopsis.String
	}
	// Credits (issue-level and story-level).
	credRows, err := db.Query(`SELECT c.name, ct.name FROM gcd_story_credit sc
		JOIN gcd_story st ON st.id = sc.story_id
		JOIN gcd_creator_name_detail c ON c.id = sc.creator_id
		JOIN gcd_credit_type ct ON ct.id = sc.credit_type_id
		WHERE st.issue_id = ?`, id)
	if err == nil {
		defer func() { _ = credRows.Close() }()
		for credRows.Next() {
			var name, role string
			if err := credRows.Scan(&name, &role); err == nil {
				m.Creators = append(m.Creators, fmt.Sprintf("%s (%s)", name, role))
			}
		}
	}
	return m, nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func nullInt(v sql.NullInt64) string {
	if !v.Valid {
		return ""
	}
	return fmt.Sprintf("%d", v.Int64)
}

func nullInt2(v sql.NullInt64) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// RefreshLoop downloads the dump at startup and then on the configured
// interval. It is intended to run in a goroutine.
func (p *Provider) RefreshLoop(stop <-chan struct{}) {
	if p.opts.DumpURL == "" {
		return
	}
	hours := p.opts.RefreshHours
	if hours <= 0 {
		hours = 168
	}
	ticker := time.NewTicker(time.Duration(hours) * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			_ = p.Download()
		}
	}
}
