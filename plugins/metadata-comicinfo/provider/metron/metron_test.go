package metron

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

func TestSearchAndFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad auth: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/issue/":
			w.Write([]byte(`{"count":1,"results":[{"id":7,"number":"2","title":"Issue Two","desc":"<b>Desc</b>","cover_date":"2021-03-04","image":"http://cover","series":{"id":1,"name":"Metron Series"},"publisher":{"name":"Pub"},"credits":[{"creator":{"name":"Bob"},"role":{"name":"writer"}}],"characters":[{"name":"Hero"}],"teams":[{"name":"Crew"}]}]}`))
		case "/issue/7/":
			w.Write([]byte(`{"id":7,"number":"2","title":"Issue Two","desc":"Desc","cover_date":"2021-03-04","image":"http://cover","series":{"name":"Metron Series"},"publisher":{"name":"Pub"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProviderWithClient(&Client{Token: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()})
	p.gate = newGate(0) // search then fetch back to back, without the 1s spacing
	matches, err := p.Search(metadata.SearchQuery{Series: "Metron Series", Number: "2"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d", len(matches))
	}
	m := matches[0]
	if m.Series != "Metron Series" || m.Number != "2" || m.Year != 2021 || m.Publisher != "Pub" {
		t.Errorf("unexpected: %+v", m)
	}
	if len(m.Creators) != 1 || len(m.Characters) != 1 || len(m.Teams) != 1 {
		t.Errorf("credits: %+v", m)
	}
	if m.ProviderID != "metron:7" {
		t.Errorf("provider id = %q", m.ProviderID)
	}
	if _, err := p.Fetch("metron:7"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
}
