package comicvine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

func TestBudgetStopsAtTheLimitAndReopensWithTheWindow(t *testing.T) {
	now := time.Date(2026, 10, 25, 18, 0, 0, 0, time.UTC)
	b := newBudget(2, time.Hour)
	b.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if ok, wait := b.take(); !ok {
			t.Fatalf("request %d refused, wait %s", i+1, wait)
		}
	}
	ok, wait := b.take()
	if ok {
		t.Fatal("a third request was allowed past a budget of two")
	}
	if wait != time.Hour {
		t.Fatalf("wait = %s, want the rest of the window", wait)
	}

	now = now.Add(time.Hour)
	if ok, _ := b.take(); !ok {
		t.Fatal("the window did not reopen")
	}
}

// A file name that carries no single issue cannot be answered, so the provider
// declines before it spends a request.
func TestSearchDeclinesANameWithoutAnIssue(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = io.WriteString(w, `{"error":"OK","results":[]}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()})
	matches, err := p.Search(metadata.SearchQuery{Series: "Justice League of America's Vibe"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("got %d matches for a name with no issue", len(matches))
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("server saw %d requests for a query it cannot answer, want 0", got)
	}
}

func TestSearchCachesAnIssueAndSpendsOneRequest(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if got := r.URL.Query().Get("filter"); got != "issue_number:48" {
			t.Errorf("filter = %q, want the issue number", got)
		}
		_, _ = io.WriteString(w, `{"error":"OK","results":[{"id":42,"name":"Fables","issue_number":"48","volume":{"id":7,"name":"Fables"},"cover_date":"2006-06-01"}]}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()})
	for i := 0; i < 3; i++ {
		matches, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48", Year: 2006})
		if err != nil {
			t.Fatalf("search %d: %v", i+1, err)
		}
		if len(matches) != 1 || matches[0].ProviderID != "comicvine:42" {
			t.Fatalf("search %d: %+v", i+1, matches)
		}
		if matches[0].Title != "Fables 48" {
			t.Fatalf("candidate title = %q, want the volume and issue the host compares against", matches[0].Title)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server saw %d requests, want 1: the repeats came from the cache", got)
	}
}

func TestSearchRefusesOnceTheBudgetIsSpent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"error":"OK","results":[]}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()})
	p.budget = newBudget(1, time.Hour)

	if _, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48"}); err != nil {
		t.Fatalf("first search: %v", err)
	}
	_, err := p.Search(metadata.SearchQuery{Series: "Hellblazer", Number: "74"})
	if err == nil {
		t.Fatal("a second search passed a budget of one")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("error %q must name the rate limit so the host can retry it", err)
	}
}

// ComicVine answers 420, not 429, when the key's hourly window is spent, and the
// window is shared with every other client using that key.
func TestA420StopsTheProviderSpendingMore(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(420)
		_, _ = io.WriteString(w, "{\"error\":\"Rate limit exceeded.  Slow down cowboy.\",\"status_code\":107}")
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48"})
	if err == nil || !strings.Contains(err.Error(), "420") {
		t.Fatalf("err = %v, want it to name HTTP 420", err)
	}
	for _, q := range []metadata.SearchQuery{
		{Series: "Hellblazer", Number: "74"},
		{Series: "100 Bullets", Number: "3"},
	} {
		if _, err := p.Search(q); err == nil {
			t.Fatalf("%s was answered after the key said 420", q.Series)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server saw %d requests after a 420, want 1", got)
	}
}

func TestFetchSpendsBudgetAndAnswersTheIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"error":"OK","results":{"id":42,"name":"Fables","issue_number":"48","volume":{"id":7,"name":"Fables"},"publisher":{"id":10,"name":"DC Comics"},"cover_date":"2006-06-01"}}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{APIKey: "key", BaseURL: server.URL, HTTPClient: server.Client()})
	match, err := p.Fetch("comicvine:42")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if match == nil || match.Series != "Fables" || match.Number != "48" || match.Publisher != "DC Comics" || match.Year != 2006 {
		t.Fatalf("fetch = %+v", match)
	}
	if match.Title != "Fables 48" {
		t.Fatalf("fetch title = %q, want the volume and issue", match.Title)
	}
}

// A padded file number ("048") must reach the API unpadded, or the filter matches
// no issue at all.
func TestIssueNumberFilterStripsPadding(t *testing.T) {
	cases := map[string]string{"048": "48", "#012": "12", "0": "0", "000": "0", "7": "7", "1.5": "1.5"}
	for in, want := range cases {
		if got := issueNumberFilter(in); got != want {
			t.Errorf("issueNumberFilter(%q) = %q, want %q", in, got, want)
		}
	}
}
