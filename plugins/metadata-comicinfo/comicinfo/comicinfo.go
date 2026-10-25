// Package comicinfo parses ComicInfo.xml metadata embedded in western comic
// archives (CBZ/CBR). The schema is defined by the Anansi Project:
// https://github.com/anansi-project/comicinfo
package comicinfo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// ComicInfo is the root element of a ComicInfo.xml document.
type ComicInfo struct {
	XMLName xml.Name `xml:"ComicInfo" json:"-"`

	Title           string `xml:"Title"`
	Series          string `xml:"Series"`
	Number          string `xml:"Number"`
	Count           string `xml:"Count"`
	Volume          string `xml:"Volume"`
	AlternateSeries string `xml:"AlternateSeries"`
	AlternateNumber string `xml:"AlternateNumber"`
	AlternateCount  string `xml:"AlternateCount"`

	Summary string `xml:"Summary"`
	Notes   string `xml:"Notes"`

	Year  int `xml:"Year"`
	Month int `xml:"Month"`
	Day   int `xml:"Day"`

	Writer      string `xml:"Writer"`
	Penciller   string `xml:"Penciller"`
	Inker       string `xml:"Inker"`
	Colorist    string `xml:"Colorist"`
	Letterer    string `xml:"Letterer"`
	CoverArtist string `xml:"CoverArtist"`
	Editor      string `xml:"Editor"`
	Translator  string `xml:"Translator"`

	Publisher   string `xml:"Publisher"`
	Imprint     string `xml:"Imprint"`
	Genre       string `xml:"Genre"`
	Tags        string `xml:"Tags"`
	Web         string `xml:"Web"`
	LanguageISO string `xml:"LanguageISO"`

	PageCount     int    `xml:"PageCount"`
	Format        string `xml:"Format"`
	BlackAndWhite string `xml:"BlackAndWhite"`
	Manga         string `xml:"Manga"`

	Characters      string `xml:"Characters"`
	Teams           string `xml:"Teams"`
	Locations       string `xml:"Locations"`
	ScanInformation string `xml:"ScanInformation"`

	StoryArc    string `xml:"StoryArc"`
	SeriesGroup string `xml:"SeriesGroup"`

	AgeRating       string  `xml:"AgeRating"`
	CommunityRating float64 `xml:"CommunityRating"`

	GTIN string `xml:"GTIN"`
	ISBN string `xml:"ISBN"`

	Pages []Page `xml:"Pages>Page"`
}

// Page describes a single page entry in ComicInfo.xml.
type Page struct {
	Image       int    `xml:"Image,attr"`
	Type        string `xml:"Type,attr"`
	DoublePage  bool   `xml:"DoublePage,attr"`
	ImageSize   int64  `xml:"ImageSize,attr"`
	Key         string `xml:"Key,attr"`
	Bookmark    string `xml:"Bookmark,attr"`
	ImageWidth  int    `xml:"ImageWidth,attr"`
	ImageHeight int    `xml:"ImageHeight,attr"`
}

// Parse reads a ComicInfo.xml document from r and returns the parsed metadata.
func Parse(r io.Reader) (*ComicInfo, error) {
	if r == nil {
		return nil, fmt.Errorf("comicinfo: nil reader")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("comicinfo: read: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses a ComicInfo.xml document.
//
// Files in the wild are not strict XML. They carry HTML entities such as
// &nbsp;, unescaped ampersands, a byte order mark, and a Windows-1252 or
// ISO-8859-1 declaration inherited from ComicRack. The decoder is configured to
// survive all of that, because rejecting the document over one character loses
// every field it carried.
func ParseBytes(data []byte) (*ComicInfo, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("comicinfo: empty document")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity
	decoder.CharsetReader = charsetReader
	var ci ComicInfo
	if err := decoder.Decode(&ci); err != nil {
		return nil, fmt.Errorf("comicinfo: unmarshal: %w", err)
	}
	return &ci, nil
}

// charsetReader transcodes a declared legacy charset into UTF-8, so a document
// that says ISO-8859-1 parses instead of failing on the declaration.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	encoding, err := htmlindex.Get(charset)
	if err != nil {
		return nil, fmt.Errorf("comicinfo: unknown charset %q: %w", charset, err)
	}
	if encoding == nil {
		return input, nil
	}
	return transform.NewReader(input, encoding.NewDecoder()), nil
}

// Creators returns a de-duplicated, trimmed list of all creator names found
// across the creator fields, in priority order.
func (c *ComicInfo) Creators() []string {
	if c == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	for _, v := range []string{c.Writer, c.Penciller, c.Inker, c.Colorist, c.Letterer, c.CoverArtist, c.Editor, c.Translator} {
		add(v)
	}
	return out
}

// GenreList returns the comma-separated Genre field split into trimmed items.
func (c *ComicInfo) GenreList() []string {
	return splitCSV(c.Genre)
}

// TagList returns the comma-separated Tags field split into trimmed items.
func (c *ComicInfo) TagList() []string {
	return splitCSV(c.Tags)
}

// CharacterList returns the comma-separated Characters field split into trimmed items.
func (c *ComicInfo) CharacterList() []string {
	return splitCSV(c.Characters)
}

func splitCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// IsManga reports whether the Manga field indicates a manga (right-to-left) book.
func (c *ComicInfo) IsManga() bool {
	if c == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(c.Manga), "Yes") ||
		strings.EqualFold(strings.TrimSpace(c.Manga), "YesAndRightToLeft")
}

// DisplayTitle returns the best available title: Title, falling back to Series.
func (c *ComicInfo) DisplayTitle() string {
	if c == nil {
		return ""
	}
	if t := strings.TrimSpace(c.Title); t != "" {
		return t
	}
	return strings.TrimSpace(c.Series)
}
