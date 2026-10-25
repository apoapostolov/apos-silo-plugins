package metron

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

// The incident this guards: 2636 answers of 429 in a day because every item
// asked again immediately. A 429 must close the gate until the server's reset,
// and every question asked in the meantime must be refused without a request.
func TestA429ClosesTheGateUntilTheServerReset(t *testing.T) {
	var calls int32
	reset := time.Now().Add(10 * time.Minute).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Limit", "200")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{Token: "token", BaseURL: server.URL, HTTPClient: server.Client()})
	p.gate = newGate(0)

	_, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48"})
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v, want it to report the 429", err)
	}
	for _, series := range []string{"Hellblazer", "Saga", "Y The Last Man", "Preacher"} {
		if _, err := p.Search(metadata.SearchQuery{Series: series, Number: "1"}); err == nil {
			t.Fatalf("%s was requested while the cooldown stood", series)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server saw %d requests, want 1: the rest must be refused locally", got)
	}
	if wait := p.gate.remaining(); wait < 9*time.Minute {
		t.Fatalf("cooldown = %s, want it to follow the reset header", wait)
	}
}

// A response that says the window is spent closes the gate before the next
// request can draw a 429 at all.
func TestExhaustedRemainingHeaderClosesTheGate(t *testing.T) {
	var calls int32
	reset := time.Now().Add(5 * time.Minute).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		io.WriteString(w, `{"count":0,"results":[]}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{Token: "token", BaseURL: server.URL, HTTPClient: server.Client()})
	p.gate = newGate(0)

	if _, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48"}); err != nil {
		t.Fatalf("first search: %v", err)
	}
	if _, err := p.Search(metadata.SearchQuery{Series: "Hellblazer", Number: "74"}); err == nil {
		t.Fatal("a second request went out after the window was reported spent")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server saw %d requests, want 1", got)
	}
}

func TestSearchCachesAnIssueAndDeclinesANameWithoutOne(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Remaining", "199")
		io.WriteString(w, `{"count":1,"results":[{"id":7,"number":"48","title":"Fables","series":{"id":3,"name":"Fables"},"publisher":{"name":"DC Comics"},"cover_date":"2006-06-01"}]}`)
	}))
	defer server.Close()

	p := NewProviderWithClient(&Client{Token: "token", BaseURL: server.URL, HTTPClient: server.Client()})
	p.gate = newGate(0)

	for i := 0; i < 3; i++ {
		if _, err := p.Search(metadata.SearchQuery{Series: "Fables", Number: "48"}); err != nil {
			t.Fatalf("search %d: %v", i+1, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("server saw %d requests, want 1: repeats come from the cache", got)
	}

	before := atomic.LoadInt32(&calls)
	if _, err := p.Search(metadata.SearchQuery{Series: "Fogtown HC"}); err != nil {
		t.Fatalf("declined search returned an error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != before {
		t.Fatalf("a name with no issue number was still requested (%d -> %d)", before, got)
	}
}

func TestGateSpacesRequests(t *testing.T) {
	now := time.Date(2026, 10, 25, 21, 0, 0, 0, time.UTC)
	g := newGate(time.Minute)
	g.now = func() time.Time { return now }

	if ok, _ := g.take(); !ok {
		t.Fatal("first request refused")
	}
	ok, wait := g.take()
	if ok {
		t.Fatal("a second request went out inside the spacing window")
	}
	if wait != time.Minute {
		t.Fatalf("wait = %s, want the spacing remainder", wait)
	}
	now = now.Add(time.Minute)
	if ok, _ := g.take(); !ok {
		t.Fatal("spacing did not clear")
	}
}
