// Command plugin serves the ComicInfo metadata provider capability for Silo.
package main

import (
	"context"
	_ "embed"
	"strconv"
	"strings"
	"sync"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/metadata"
	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-comicinfo/provider"
	"google.golang.org/protobuf/types/known/structpb"
)

// version is set at build time with -ldflags "-X main.version=...".
var version string

const (
	capabilityID = "comicinfo-metadata"
	itemType     = "comic"
	maxResults   = 25
)

//go:embed manifest.json
var manifestJSON []byte

type server struct {
	pluginv1.UnimplementedMetadataProviderServer

	mu      sync.RWMutex
	manager *provider.Manager
	options provider.Options
}

func main() {
	s := &server{options: defaultOptions()}
	s.manager = provider.NewManager(s.options)
	runtime.ServeManifestWithOptions(manifestJSON, version, runtime.CapabilityServers{
		MetadataProvider: s,
	}, runtime.WithConfigure(s.configure))
}

// configure stores the settings the host sends. Entry values are objects with
// a "value" field, as the manifest's json_schema declares.
func (s *server) configure(_ context.Context, entries []*pluginv1.ConfigEntry) error {
	options := defaultOptions()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		value := entry.GetValue()
		switch strings.TrimSpace(entry.GetKey()) {
		case "enable_gcd":
			options.EnableGCD = configBool(value, options.EnableGCD)
		case "gcd_dump_path":
			options.GCDDBPath = configString(value)
		case "gcd_dump_url":
			options.GCDDumpURL = configString(value)
		case "gcd_dump_cookie":
			options.GCDDumpCookie = configString(value)
		case "gcd_refresh_hours":
			if n, ok := configNumber(value); ok && n > 0 {
				options.GCDRefreshHours = n
			}
		case "enable_comicvine":
			options.EnableComicVine = configBool(value, options.EnableComicVine)
		case "comicvine_api_key":
			options.ComicVineAPIKey = configString(value)
		case "enable_metron":
			options.EnableMetron = configBool(value, options.EnableMetron)
		case "metron_token":
			options.MetronToken = configString(value)
		case "enable_filename":
			options.EnableFilename = configBool(value, options.EnableFilename)
		}
	}

	manager := provider.NewManager(options)
	s.mu.Lock()
	old := s.manager
	s.manager = manager
	s.options = options
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func defaultOptions() provider.Options {
	return provider.Options{
		EnableGCD:       true,
		EnableComicVine: true,
		EnableFilename:  true,
		GCDRefreshHours: 168,
	}
}

func (s *server) state() (*provider.Manager, provider.Options) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	manager := s.manager
	if manager == nil {
		manager = provider.NewManager(s.options)
	}
	return manager, s.options
}

func (s *server) Search(_ context.Context, req *pluginv1.SearchMetadataRequest) (*pluginv1.SearchMetadataResponse, error) {
	manager, _ := s.state()
	query := metadata.SearchQuery{
		Title:       strings.TrimSpace(req.GetQuery()),
		Year:        int(req.GetYear()),
		ProviderIDs: stringMap(req.GetProviderIds()),
	}
	if query.Title == "" && query.ProviderIDs["filename"] == "" {
		return &pluginv1.SearchMetadataResponse{}, nil
	}
	if filename := query.ProviderIDs["filename"]; filename != "" {
		query.Filename = filename
	}

	seen := make(map[string]struct{})
	results := make([]*pluginv1.ProviderSearchResult, 0, maxResults)
	for _, matches := range manager.SearchAll(query) {
		for _, match := range matches {
			providerID := strings.TrimSpace(match.ProviderID)
			if providerID == "" {
				continue
			}
			if _, ok := seen[providerID]; ok {
				continue
			}
			seen[providerID] = struct{}{}
			results = append(results, searchResult(match))
			if len(results) >= maxResults {
				return &pluginv1.SearchMetadataResponse{Results: results}, nil
			}
		}
	}
	return &pluginv1.SearchMetadataResponse{Results: results}, nil
}

func (s *server) GetMetadata(_ context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	manager, _ := s.state()
	providerID := strings.TrimSpace(req.GetProviderId())
	if providerID == "" {
		providerID = providerIDFromStruct(req.GetProviderIds())
	}
	if providerID == "" {
		return &pluginv1.GetMetadataResponse{}, nil
	}

	match, err := manager.Fetch(providerID)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return &pluginv1.GetMetadataResponse{}, nil
	}
	item, err := metadataItem(*match)
	if err != nil {
		return nil, err
	}
	return &pluginv1.GetMetadataResponse{Item: item}, nil
}

func searchResult(match metadata.Match) *pluginv1.ProviderSearchResult {
	providerIDs, _ := structpb.NewStruct(map[string]any{
		match.Provider: providerIDValue(match.ProviderID),
	})
	return &pluginv1.ProviderSearchResult{
		ProviderId:    strings.TrimSpace(match.ProviderID),
		ItemType:      itemType,
		Title:         displayTitle(match),
		OriginalTitle: displayTitle(match),
		Year:          int32(match.Year),
		Overview:      strings.TrimSpace(match.Summary),
		ProviderIds:   providerIDs,
		ImageUrl:      strings.TrimSpace(match.CoverURL),
	}
}

func metadataItem(match metadata.Match) (*pluginv1.MetadataItem, error) {
	providerIDs, err := structpb.NewStruct(map[string]any{
		match.Provider: providerIDValue(match.ProviderID),
	})
	if err != nil {
		return nil, err
	}
	extra, err := structpb.NewStruct(map[string]any{
		"series":     strings.TrimSpace(match.Series),
		"number":     strings.TrimSpace(match.Number),
		"count":      strings.TrimSpace(match.Count),
		"volume":     strings.TrimSpace(match.Volume),
		"publisher":  strings.TrimSpace(match.Publisher),
		"imprint":    strings.TrimSpace(match.Imprint),
		"tags":       stringsOrEmpty(match.Tags),
		"characters": stringsOrEmpty(match.Characters),
		"teams":      stringsOrEmpty(match.Teams),
		"locations":  stringsOrEmpty(match.Locations),
		"creators":   stringsOrEmpty(match.Creators),
		"age_rating": strings.TrimSpace(match.AgeRating),
		"page_count": match.PageCount,
		"gtin":       strings.TrimSpace(match.GTIN),
		"isbn":       strings.TrimSpace(match.ISBN),
		"language":   strings.TrimSpace(match.Language),
		"confidence": match.Confidence,
	})
	if err != nil {
		return nil, err
	}

	return &pluginv1.MetadataItem{
		ProviderId:       strings.TrimSpace(match.ProviderID),
		ItemType:         itemType,
		Title:            displayTitle(match),
		OriginalTitle:    displayTitle(match),
		Year:             int32(match.Year),
		Overview:         strings.TrimSpace(match.Summary),
		Genres:           splitList(match.Genre),
		ProviderIds:      providerIDs,
		Metadata:         extra,
		Studios:          nonEmpty(match.Publisher),
		ContentRating:    strings.TrimSpace(match.AgeRating),
		PosterPath:       strings.TrimSpace(match.CoverURL),
		OriginalLanguage: strings.TrimSpace(match.Language),
		People:           people(match.Creators),
		ReleaseDate:      releaseDate(match),
	}, nil
}

func people(creators []string) []*pluginv1.PersonRecord {
	out := make([]*pluginv1.PersonRecord, 0, len(creators))
	for index, creator := range creators {
		name, role := creatorNameAndRole(creator)
		if name == "" {
			continue
		}
		out = append(out, &pluginv1.PersonRecord{
			Name:      name,
			Kind:      "creator",
			Character: role,
			SortOrder: int32(index),
		})
	}
	return out
}

func creatorNameAndRole(creator string) (string, string) {
	creator = strings.TrimSpace(creator)
	if creator == "" {
		return "", ""
	}
	if open := strings.LastIndex(creator, "("); open > 0 && strings.HasSuffix(creator, ")") {
		return strings.TrimSpace(creator[:open]), strings.TrimSpace(creator[open+1 : len(creator)-1])
	}
	return creator, ""
}

func displayTitle(match metadata.Match) string {
	title := strings.TrimSpace(match.Title)
	if title != "" {
		return title
	}
	series := strings.TrimSpace(match.Series)
	if series == "" {
		return ""
	}
	if number := strings.TrimSpace(match.Number); number != "" {
		return series + " #" + number
	}
	return series
}

func releaseDate(match metadata.Match) string {
	if match.Year == 0 {
		return ""
	}
	date := strconv.Itoa(match.Year)
	if match.Month > 0 {
		date += "-" + pad2(match.Month)
		if match.Day > 0 {
			date += "-" + pad2(match.Day)
		}
	}
	return date
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func nonEmpty(value string) []string {
	if value = strings.TrimSpace(value); value == "" {
		return nil
	}
	return []string{value}
}

func stringsOrEmpty(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func providerIDValue(providerID string) string {
	_, value, found := strings.Cut(providerID, ":")
	if !found {
		return providerID
	}
	return value
}

func providerIDFromStruct(value *structpb.Struct) string {
	fields := stringMap(value)
	for _, name := range []string{"comicvine", "gcd", "metron", "filename"} {
		if id := strings.TrimSpace(fields[name]); id != "" {
			if strings.HasPrefix(id, name+":") {
				return id
			}
			return name + ":" + id
		}
	}
	return strings.TrimSpace(fields["provider_id"])
}

func stringMap(value *structpb.Struct) map[string]string {
	out := make(map[string]string)
	if value == nil {
		return out
	}
	for key, raw := range value.AsMap() {
		if text, ok := raw.(string); ok && text != "" {
			out[strings.TrimSpace(key)] = strings.TrimSpace(text)
		}
	}
	return out
}

func configString(value *structpb.Struct) string {
	if value == nil {
		return ""
	}
	if raw, ok := value.GetFields()["value"]; ok && raw != nil {
		return strings.TrimSpace(raw.GetStringValue())
	}
	return ""
}

func configBool(value *structpb.Struct, def bool) bool {
	if value == nil {
		return def
	}
	raw, ok := value.GetFields()["value"]
	if !ok || raw == nil {
		return def
	}
	switch raw.GetKind().(type) {
	case *structpb.Value_BoolValue:
		return raw.GetBoolValue()
	case *structpb.Value_NumberValue:
		return raw.GetNumberValue() != 0
	case *structpb.Value_StringValue:
		switch strings.ToLower(strings.TrimSpace(raw.GetStringValue())) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return def
}

func configNumber(value *structpb.Struct) (int, bool) {
	if value == nil {
		return 0, false
	}
	raw, ok := value.GetFields()["value"]
	if !ok || raw == nil {
		return 0, false
	}
	switch raw.GetKind().(type) {
	case *structpb.Value_NumberValue:
		return int(raw.GetNumberValue()), true
	case *structpb.Value_StringValue:
		if n, err := strconv.Atoi(strings.TrimSpace(raw.GetStringValue())); err == nil {
			return n, true
		}
	}
	return 0, false
}
