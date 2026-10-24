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
