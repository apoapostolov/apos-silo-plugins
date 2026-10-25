package filename

import (
	"testing"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		series string
		number string
		year   int
	}{
		// The three names the parser always handled.
		{"Batman #12 (2020).cbz", "Batman", "12", 2020},
		{"The Amazing Spider-Man 001 (2018).cbr", "The Amazing Spider-Man", "001", 2018},
		{"Saga_54.cbz", "Saga", "54", 0},

		// Reprint volume markers used to be read as the issue number.
		{"Amazing Spider-Man Vol. 2 #14.cbz", "Amazing Spider-Man", "14", 0},
		{"Amazing Spider-Man Volume 3 014.cbz", "Amazing Spider-Man", "014", 0},
		{"Batman v2 #12.cbz", "Batman", "12", 0},

		// A bracketed year is a year even when it sits before the issue, and it
		// must not survive into the series name.
		{"Batman (2016) #12.cbz", "Batman", "12", 2016},
		{"Batman (2016) 012.cbz", "Batman", "012", 2016},

		// Numbers inside the title belong to the title.
		{"X-Men 2099 #001 (1993).cbz", "X-Men 2099", "001", 1993},
		{"2000 AD 1234.cbz", "2000 AD", "1234", 0},
		{"Judge Dredd 1977 001.cbz", "Judge Dredd 1977", "001", 0},

		// Packaging noise after the issue must not become the issue.
		{"Spawn #300 (2019) (Digital).cbz", "Spawn", "300", 2019},
		{"Spawn #300 c2c.cbz", "Spawn", "300", 0},
		{"Saga 54 scan.cbz", "Saga", "54", 0},

		// Other markers.
		{"Invincible issue 144.cbz", "Invincible", "144", 0},
		{"Invincible No. 144 (2017).cbz", "Invincible", "144", 2017},
		{"Hellboy_-_001.cbz", "Hellboy", "001", 0},

		// No issue at all: the whole name is the series.
		{"Saga.cbz", "Saga", "", 0},
		{"Judge Dredd 1977.cbz", "Judge Dredd", "", 1977},
	}
	p := New()
	for _, c := range cases {
		matches, err := p.Search(metadata.SearchQuery{Filename: c.name})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(matches) != 1 {
			t.Fatalf("%s: got %d matches", c.name, len(matches))
		}
		m := matches[0]
		if m.Series != c.series || m.Number != c.number || m.Year != c.year {
			t.Errorf("%s:\n  got  series=%q number=%q year=%d\n  want series=%q number=%q year=%d",
				c.name, m.Series, m.Number, m.Year, c.series, c.number, c.year)
		}
	}
}

func TestParseFallsBackToTheTitle(t *testing.T) {
	p := New()
	matches, err := p.Search(metadata.SearchQuery{Title: "Batman #12 (2020)"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Series != "Batman" || matches[0].Number != "12" {
		t.Fatalf("title-only query = %+v", matches)
	}
}

func TestParseIgnoresAnEmptyQuery(t *testing.T) {
	p := New()
	matches, err := p.Search(metadata.SearchQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("empty query returned %d matches", len(matches))
	}
}

func TestParseKeepsTheSeriesWhenTheNameIsOnlyANumber(t *testing.T) {
	series, number, year := Parse("2024.cbz")
	if series != "2024" || number != "" || year != 0 {
		t.Fatalf("got series=%q number=%q year=%d", series, number, year)
	}
}

func TestParseHandlesDirectoriesAndUnicode(t *testing.T) {
	series, number, year := Parse("/library/comics/Corto Maltese – Ballade de la mer salée #03 (2002).cbz")
	if series != "Corto Maltese - Ballade de la mer salée" || number != "03" || year != 2002 {
		t.Fatalf("got series=%q number=%q year=%d", series, number, year)
	}
}

// The title a candidate is offered under is what the host scores against the
// item's own title. Echoing the archive name scores 0.17 against "Fables 048",
// which is below the host's 0.5 admission bar; the parsed title scores 0.62.
func TestSearchPresentsTheParsedTitle(t *testing.T) {
	cases := []struct {
		file  string
		title string
	}{
		{"/mnt/media/comics/western/Fables/Fables 048 (2015) (Digital) (Zone-Empire).cbz", "Fables 048"},
		{"Saga #54.cbr", "Saga 54"},
		{"Saga (2012).cbz", "Saga"},
		{"Collected Works (Digital) (TPB).cbz", "Collected Works"},
	}
	p := New()
	for _, c := range cases {
		matches, err := p.Search(metadata.SearchQuery{Filename: c.file})
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if len(matches) != 1 {
			t.Fatalf("%s: got %d matches", c.file, len(matches))
		}
		if got := matches[0].Title; got != c.title {
			t.Errorf("%s: title = %q, want %q", c.file, got, c.title)
		}
	}
}

// The host admits a candidate and then fetches its metadata by provider id.
func TestFetchServesTheSameParse(t *testing.T) {
	p := New()
	matches, err := p.Search(metadata.SearchQuery{Filename: "Fables 048 (2015) (Digital) (Zone-Empire).cbz"})
	if err != nil || len(matches) != 1 {
		t.Fatalf("search: %v %d", err, len(matches))
	}
	offered := matches[0]

	fetched, err := p.Fetch(offered.ProviderID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if fetched == nil {
		t.Fatal("fetch returned nothing for its own provider id")
	}
	if fetched.Title != offered.Title || fetched.Series != offered.Series || fetched.Number != offered.Number || fetched.Year != offered.Year {
		t.Fatalf("fetch = %+v, want the parse of %+v", fetched, offered)
	}

	if unknown, err := p.Fetch("gcd:123"); err != nil || unknown != nil {
		t.Fatalf("a foreign provider id must answer nothing, got %+v %v", unknown, err)
	}
}
