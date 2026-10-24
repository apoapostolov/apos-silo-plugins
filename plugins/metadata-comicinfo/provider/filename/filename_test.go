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
		{"Batman #12 (2020).cbz", "Batman", "12", 2020},
		{"The Amazing Spider-Man 001 (2018).cbr", "The Amazing Spider-Man", "001", 2018},
		{"Saga_54.cbz", "Saga", "54", 0},
	}
	p := New()
	for _, c := range cases {
		matches, err := p.Search(metadata.SearchQuery{Filename: c.name})
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("%s: got %d", c.name, len(matches))
		}
		m := matches[0]
		if m.Series != c.series || m.Number != c.number || m.Year != c.year {
			t.Errorf("%s: got series=%q number=%q year=%d", c.name, m.Series, m.Number, m.Year)
		}
	}
}
