package gcd

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
	_ "modernc.org/sqlite"
)

func buildFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gcd.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	stmts := []string{
		`CREATE TABLE gcd_publisher (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE gcd_series (id INTEGER PRIMARY KEY, name TEXT, sort_name TEXT, year_began INTEGER, year_ended INTEGER, issue_count INTEGER, publisher_id INTEGER, first_issue_id INTEGER, notes TEXT)`,
		`CREATE TABLE gcd_issue (id INTEGER PRIMARY KEY, number TEXT, key_date TEXT, title TEXT, series_id INTEGER, variant_of_id INTEGER, volume TEXT, price TEXT, isbn TEXT, notes TEXT)`,
		`CREATE TABLE gcd_story (id INTEGER PRIMARY KEY, issue_id INTEGER, sequence_number INTEGER, title TEXT, type_id INTEGER, synopsis TEXT, genre TEXT)`,
		`CREATE TABLE gcd_story_credit (id INTEGER PRIMARY KEY, story_id INTEGER, creator_id INTEGER, credit_type_id INTEGER)`,
		`CREATE TABLE gcd_issue_credit (id INTEGER PRIMARY KEY, issue_id INTEGER, creator_id INTEGER, credit_name TEXT)`,
		`CREATE TABLE gcd_creator_name_detail (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE gcd_credit_type (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO gcd_publisher (id,name) VALUES (1,'ACME')`,
		`INSERT INTO gcd_series (id,name,year_began,issue_count,publisher_id) VALUES (1,'Test Series',2020,3,1)`,
		`INSERT INTO gcd_issue (id,number,key_date,title,series_id,variant_of_id,notes) VALUES (10,'1','2020-06-01','First Issue',1,NULL,'Great issue')`,
		`INSERT INTO gcd_story (id,issue_id,sequence_number,title,type_id,synopsis,genre) VALUES (100,10,1,'The Beginning',19,'A hero rises','Superhero')`,
		`INSERT INTO gcd_creator_name_detail (id,name) VALUES (5,'Jane Writer')`,
		`INSERT INTO gcd_credit_type (id,name) VALUES (1,'writer')`,
		`INSERT INTO gcd_story_credit (id,story_id,creator_id,credit_type_id) VALUES (1,100,5,1)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
	return path
}

func TestSearchAndFetch(t *testing.T) {
	path := buildFixture(t)
	p, err := New(Options{DBPath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	matches, err := p.Search(metadata.SearchQuery{Series: "Test Series", Number: "1", Year: 2020})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches", len(matches))
	}
	m := matches[0]
	if m.Series != "Test Series" || m.Number != "1" || m.Year != 2020 || m.Publisher != "ACME" {
		t.Errorf("unexpected: %+v", m)
	}
	if m.ProviderID != "gcd:issue:10" {
		t.Errorf("provider id = %q", m.ProviderID)
	}
	if len(m.Creators) != 1 || m.Creators[0] != "Jane Writer (writer)" {
		t.Errorf("creators: %+v", m.Creators)
	}

	fetched, err := p.Fetch("gcd:issue:10")
	if err != nil {
		t.Fatal(err)
	}
	if fetched == nil || fetched.Number != "1" {
		t.Fatalf("fetch: %+v", fetched)
	}
}

func TestMissingPath(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error for empty path")
	}
}

// A series title containing a wildcard character must match itself and nothing
// else. Without ESCAPE in the query the backslashes escapeLike adds are literal,
// so a search for "100%" finds nothing at all and a search for "A_B" matches any
// single character in that position.
func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	path := buildFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	rows := []string{
		`INSERT INTO gcd_series (id,name,year_began,issue_count,publisher_id) VALUES (2,'100% Comics',1990,1,1)`,
		`INSERT INTO gcd_series (id,name,year_began,issue_count,publisher_id) VALUES (3,'100X Comics',1991,1,1)`,
		`INSERT INTO gcd_series (id,name,year_began,issue_count,publisher_id) VALUES (4,'A_B Comics',1992,1,1)`,
		`INSERT INTO gcd_series (id,name,year_began,issue_count,publisher_id) VALUES (5,'AXB Comics',1993,1,1)`,
	}
	for _, row := range rows {
		if _, err := db.Exec(row); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	_ = db.Close()

	p, err := New(Options{DBPath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	seriesNames := func(query string) []string {
		t.Helper()
		matches, err := p.Search(metadata.SearchQuery{Series: query})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.Series)
		}
		return names
	}

	if got := seriesNames("100%"); len(got) != 1 || got[0] != "100% Comics" {
		t.Errorf("search for 100%% = %v, want [100%% Comics]", got)
	}
	if got := seriesNames("A_B"); len(got) != 1 || got[0] != "A_B Comics" {
		t.Errorf("search for A_B = %v, want [A_B Comics]", got)
	}
}
