package provider

import (
	"testing"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
)

func TestManagerFilenameFallback(t *testing.T) {
	m := NewManager(Options{EnableFilename: true})
	defer func() { _ = m.Close() }()
	if got := m.Names(); len(got) != 1 || got[0] != "filename" {
		t.Fatalf("names = %v", got)
	}
	matches, provider, err := m.Search(metadata.SearchQuery{Filename: "Batman #7 (2021).cbz"})
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil || provider.Name() != "filename" {
		t.Fatalf("provider = %v", provider)
	}
	if len(matches) != 1 || matches[0].Number != "7" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestManagerPriority(t *testing.T) {
	// With GCD and ComicVine disabled, only filename is registered.
	m := NewManager(Options{EnableGCD: true, GCDDBPath: "", EnableComicVine: true, ComicVineAPIKey: "", EnableMetron: true, MetronToken: "", EnableFilename: true})
	defer func() { _ = m.Close() }()
	if got := m.Names(); len(got) != 1 || got[0] != "filename" {
		t.Fatalf("names = %v", got)
	}
}
