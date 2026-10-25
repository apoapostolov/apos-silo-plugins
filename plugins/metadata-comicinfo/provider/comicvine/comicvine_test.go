package comicvine

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

func TestSearchAndFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/":
			if r.URL.Query().Get("api_key") != "key" {
				t.Errorf("missing api key")
			}
			_, _ = w.Write([]byte(`{"error":"OK","number_of_page_results":1,"results":[{"id":123,"name":"Issue One","issue_number":"1","description":"<p>Hello</p>","image":{"original_url":"http://img"},"volume":{"id":9,"name":"Test Series"},"cover_date":"2020-05-01","person_credits":[{"name":"Ann Author","role":"writer"}],"character_credits":[{"name":"Hero"}],"team_credits":[{"name":"Team"}],"publisher":{"name":"ACME"}}]}`))
		case "/issue/4000-123/":
			_, _ = w.Write([]byte(`{"error":"OK","results":{"id":123,"name":"Issue One","issue_number":"1","description":"Hello","volume":{"name":"Test Series"},"cover_date":"2020-05-01","publisher":{"name":"ACME"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: srv.URL, HTTPClient: srv.Client()})
	matches, err := p.Search(metadata.SearchQuery{Series: "Test Series", Number: "1"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("got %d matches", len(matches))
	}
	m := matches[0]
	if m.Series != "Test Series" || m.Number != "1" || m.Year != 2020 || m.Publisher != "ACME" {
		t.Errorf("unexpected match: %+v", m)
	}
	if m.Summary != "Hello" {
		t.Errorf("summary = %q", m.Summary)
	}
	if len(m.Creators) != 1 || len(m.Characters) != 1 {
		t.Errorf("credits: %+v", m)
	}
	if m.ProviderID != "comicvine:123" {
		t.Errorf("provider id = %q", m.ProviderID)
	}

	fetched, err := p.Fetch("comicvine:123")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if fetched.Series != "Test Series" {
		t.Errorf("fetched series = %q", fetched.Series)
	}
}
