package comicinfo

import "testing"

const sample = `<?xml version="1.0"?>
<ComicInfo xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>The Amazing Spider-Man</Title>
  <Series>The Amazing Spider-Man</Series>
  <Number>1</Number>
  <Count>33</Count>
  <Volume>2018</Volume>
  <Summary>Spider-Man returns.</Summary>
  <Year>2018</Year>
  <Month>7</Month>
  <Day>11</Day>
  <Writer>Dan Slott, Nick Spencer</Writer>
  <Penciller>Ryan Ottley</Penciller>
  <Colorist>Laura Martin</Colorist>
  <Publisher>Marvel Comics</Publisher>
  <Genre>Superhero, Action</Genre>
  <Tags>spider-man, marvel</Tags>
  <LanguageISO>en</LanguageISO>
  <PageCount>32</PageCount>
  <Manga>No</Manga>
  <Characters>Peter Parker, Mary Jane Watson</Characters>
  <Teams>Avengers</Teams>
  <Pages>
    <Page Image="0" Type="FrontCover" ImageSize="12345" />
  </Pages>
</ComicInfo>`

func TestParse(t *testing.T) {
	ci, err := ParseBytes([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ci.DisplayTitle() != "The Amazing Spider-Man" {
		t.Errorf("title = %q", ci.DisplayTitle())
	}
	if ci.Series != "The Amazing Spider-Man" {
		t.Errorf("series = %q", ci.Series)
	}
	if ci.Number != "1" || ci.Count != "33" {
		t.Errorf("number/count = %s/%s", ci.Number, ci.Count)
	}
	if ci.Year != 2018 || ci.Month != 7 || ci.Day != 11 {
		t.Errorf("date = %d-%d-%d", ci.Year, ci.Month, ci.Day)
	}
	if len(ci.Creators()) < 3 {
		t.Errorf("creators = %v", ci.Creators())
	}
	if len(ci.GenreList()) != 2 {
		t.Errorf("genres = %v", ci.GenreList())
	}
	if len(ci.TagList()) != 2 {
		t.Errorf("tags = %v", ci.TagList())
	}
	if len(ci.CharacterList()) != 2 {
		t.Errorf("chars = %v", ci.CharacterList())
	}
	if ci.IsManga() {
		t.Error("expected not manga")
	}
	if len(ci.Pages) != 1 || ci.Pages[0].Type != "FrontCover" {
		t.Errorf("pages = %+v", ci.Pages)
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := ParseBytes([]byte("")); err == nil {
		t.Fatal("expected error for empty")
	}
	if _, err := ParseBytes([]byte("   ")); err == nil {
		t.Fatal("expected error for whitespace")
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := ParseBytes([]byte("<ComicInfo><Title>")); err == nil {
		t.Fatal("expected error for invalid xml")
	}
}

func TestParseNilReader(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("expected error for nil reader")
	}
}

// Real ComicRack files are not strict XML. Each of these would have failed the
// whole document before the decoder was relaxed, taking every field with it.
func TestParseAcceptsRealWorldDocuments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		title string
	}{
		{
			name: "html entity",
			// &nbsp; resolves to U+00A0, a non-breaking space, not U+0020.
			input: `<ComicInfo><Title>Batman&nbsp;Annual</Title></ComicInfo>`,
			title: "Batman\u00a0Annual",
		},
		{
			name:  "accented html entity",
			input: `<ComicInfo><Title>Pok&eacute;mon Adventures</Title></ComicInfo>`,
			title: "Pokémon Adventures",
		},
		{
			name:  "unescaped ampersand",
			input: `<ComicInfo><Title>Batman & Robin</Title></ComicInfo>`,
			title: "Batman & Robin",
		},
		{
			name:  "byte order mark",
			input: "\xef\xbb\xbf" + `<ComicInfo><Title>Batman</Title></ComicInfo>`,
			title: "Batman",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ci, err := ParseBytes([]byte(c.input))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if ci.Title != c.title {
				t.Fatalf("title = %q, want %q", ci.Title, c.title)
			}
		})
	}
}

// A Windows-1252 or ISO-8859-1 declaration is normal in ComicRack output and
// must not fail the parse.
func TestParseTranscodesLegacyCharsets(t *testing.T) {
	latin := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?><ComicInfo><Title>Pok` + "\xe9" + `mon</Title></ComicInfo>`)
	ci, err := ParseBytes(latin)
	if err != nil {
		t.Fatalf("ISO-8859-1: %v", err)
	}
	if ci.Title != "Pokémon" {
		t.Fatalf("title = %q, want Pokémon", ci.Title)
	}

	win := []byte(`<?xml version="1.0" encoding="windows-1252"?><ComicInfo><Title>Caf` + "\xe9" + `</Title></ComicInfo>`)
	ci, err = ParseBytes(win)
	if err != nil {
		t.Fatalf("windows-1252: %v", err)
	}
	if ci.Title != "Café" {
		t.Fatalf("title = %q, want Café", ci.Title)
	}
}

// The wrong root element is a different document, not an empty one.
func TestParseRejectsAWrongRoot(t *testing.T) {
	if _, err := ParseBytes([]byte(`<Foo><Title>Batman</Title></Foo>`)); err == nil {
		t.Fatal("expected error for a non ComicInfo root")
	}
}
