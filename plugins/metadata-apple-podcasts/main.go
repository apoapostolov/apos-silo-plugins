// Command plugin serves the Apple Podcasts metadata provider and artwork
// resolver capabilities for Silo.
//
// The provider answers for podcast shows: search reads the Apple Podcasts
// directory, and detail reads the show's own RSS feed for the description,
// publisher, language, and latest episode date. Artwork is served through an
// image resolver so a stored "apple-podcasts://<id>/poster.jpg" path resolves
// to the Apple CDN at whatever size the host asks for.
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/apoapostolov/apos-silo-plugins/plugins/metadata-apple-podcasts/provider/apple"
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"google.golang.org/protobuf/types/known/structpb"
)

// version is set at build time with -ldflags "-X main.version=...".
var version string

const (
	capabilityID = "apple-podcasts"
	itemType     = "podcast"
	maxResults   = 25

	// posterScheme is the stored-artwork prefix this plugin resolves. It matches
	// the scheme the manifest advertises for image_resolver.v1.
	posterScheme = "apple-podcasts://"
	posterSuffix = "/poster.jpg"
	posterWidth  = 600
)

//go:embed manifest.json
var manifestJSON []byte

type server struct {
	pluginv1.UnimplementedMetadataProviderServer
	pluginv1.UnimplementedImageResolverServer

	mu      sync.RWMutex
	country string
}

func main() {
	s := &server{country: "us"}
	runtime.ServeManifestWithOptions(manifestJSON, version, runtime.CapabilityServers{
		MetadataProvider: s,
		ImageResolver:    s,
	}, runtime.WithConfigure(s.configure))
}

// configure stores the settings the host sends. Entry values are objects with a
// "value" field, as the manifest's json_schema declares.
func (s *server) configure(_ context.Context, entries []*pluginv1.ConfigEntry) error {
	country := s.currentCountry()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		value := entry.GetValue()
		switch strings.TrimSpace(entry.GetKey()) {
		case "country":
			if text := strings.ToLower(strings.TrimSpace(structString(value, "value"))); len(text) == 2 {
				country = text
			}
		}
	}
	s.mu.Lock()
	s.country = country
	s.mu.Unlock()
	return nil
}

func (s *server) currentCountry() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.country == "" {
		return "us"
	}
	return s.country
}

func (s *server) client() *apple.Client {
	return apple.NewClient(s.currentCountry())
}

// Search reads the Apple directory for a show name.
func (s *server) Search(ctx context.Context, req *pluginv1.SearchMetadataRequest) (*pluginv1.SearchMetadataResponse, error) {
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return &pluginv1.SearchMetadataResponse{}, nil
	}
	if requested := strings.TrimSpace(req.GetItemType()); requested != "" && requested != itemType {
		return &pluginv1.SearchMetadataResponse{}, nil
	}
	if id := apple.CollectionIDFromURL(query); id != "" {
		show, err := s.client().Lookup(ctx, id)
		if err != nil {
			return &pluginv1.SearchMetadataResponse{}, nil
		}
		return &pluginv1.SearchMetadataResponse{Results: []*pluginv1.ProviderSearchResult{searchResult(apple.ScoredShow{Show: show, Score: 1})}}, nil
	}
	scored, err := s.client().SearchScored(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}
	results := make([]*pluginv1.ProviderSearchResult, 0, len(scored))
	for _, candidate := range scored {
		results = append(results, searchResult(candidate))
	}
	return &pluginv1.SearchMetadataResponse{Results: results}, nil
}

// GetMetadata resolves one show and enriches it from the show's feed.
func (s *server) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	if requested := strings.TrimSpace(req.GetItemType()); requested != "" && requested != itemType {
		return &pluginv1.GetMetadataResponse{}, nil
	}
	appleID := strings.TrimSpace(req.GetProviderId())
	if appleID == "" {
		appleID = providerIDFromStruct(req.GetProviderIds())
	}
	appleID = apple.CollectionIDFromURL(appleID)
	if appleID == "" {
		return &pluginv1.GetMetadataResponse{}, nil
	}

	client := s.client()
	show, err := client.Lookup(ctx, appleID)
	if err != nil {
		return nil, err
	}

	// The feed is the only place with a real description and publisher, so a
	// feed failure degrades to directory data instead of failing the match.
	var channel apple.Channel
	if show.FeedURL != "" {
		if fetched, feedErr := client.Feed(ctx, show.FeedURL); feedErr == nil {
			channel = fetched
		}
	}

	item, err := metadataItem(show, channel)
	if err != nil {
		return nil, err
	}
	return &pluginv1.GetMetadataResponse{Item: item}, nil
}

// GetImages offers the square show artwork as the poster.
func (s *server) GetImages(ctx context.Context, req *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	appleID := providerIDFromStruct(req.GetProviderIds())
	if appleID == "" {
		appleID = apple.CollectionIDFromURL(req.GetProviderId())
	}
	if appleID == "" {
		return &pluginv1.GetImagesResponse{}, nil
	}
	url, err := s.client().ArtworkFor(ctx, appleID, posterWidth)
	if err != nil || url == "" {
		return &pluginv1.GetImagesResponse{}, nil
	}
	return &pluginv1.GetImagesResponse{Images: []*pluginv1.ImageRecord{{
		Kind:   "poster",
		Url:    url,
		Width:  posterWidth,
		Height: posterWidth,
	}}}, nil
}

// ResolveImageURL resolves one stored artwork path.
func (s *server) ResolveImageURL(ctx context.Context, req *pluginv1.ResolveImageURLRequest) (*pluginv1.ResolveImageURLResponse, error) {
	url, err := s.resolve(ctx, req.GetPath(), req.GetVariant())
	if err != nil {
		return nil, err
	}
	return &pluginv1.ResolveImageURLResponse{Url: url}, nil
}

// ResolveImageURLs resolves a batch. Unknown paths are skipped rather than
// failing the whole request, which the SDK rules allow.
func (s *server) ResolveImageURLs(ctx context.Context, req *pluginv1.ResolveImageURLsRequest) (*pluginv1.ResolveImageURLsResponse, error) {
	urls := make(map[string]string, len(req.GetPaths()))
	for _, path := range req.GetPaths() {
		url, err := s.resolve(ctx, path, req.GetVariant())
		if err != nil || url == "" {
			continue
		}
		urls[path] = url
	}
	return &pluginv1.ResolveImageURLsResponse{Urls: urls}, nil
}

func (s *server) resolve(ctx context.Context, path, variant string) (string, error) {
	appleID, err := CollectionIDFromPosterPath(path)
	if err != nil {
		return "", err
	}
	return s.client().ArtworkFor(ctx, appleID, apple.SizeForVariant(variant))
}

// CollectionIDFromPosterPath reads the collection id out of a stored path of
// the form "apple-podcasts://<id>/poster.jpg".
func CollectionIDFromPosterPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("apple: empty artwork path")
	}
	if !strings.HasPrefix(path, posterScheme) {
		return "", fmt.Errorf("apple: %q is not an %s path", path, posterScheme)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, posterScheme), posterSuffix)
	if id = apple.CollectionIDFromURL(strings.Trim(id, "/")); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("apple: %q carries no collection id", path)
}

// PosterPath is the stored artwork path for a show.
func PosterPath(appleID string) string {
	return posterScheme + appleID + posterSuffix
}

func searchResult(candidate apple.ScoredShow) *pluginv1.ProviderSearchResult {
	show := candidate.Show
	providerIDs, _ := structpb.NewStruct(map[string]any{capabilityID: show.AppleID})
	result := &pluginv1.ProviderSearchResult{
		ProviderId:       show.AppleID,
		ItemType:         itemType,
		Title:            show.Title,
		OriginalTitle:    show.Title,
		Overview:         confidenceLine(candidate),
		ProviderIds:      providerIDs,
		ImageUrl:         apple.ArtworkAt(show.ArtworkURL, posterWidth),
		OriginalLanguage: show.Language,
	}
	// No year: a podcast collection's Apple releaseDate is the latest episode
	// date, not the show's first year, and a wrong year is worse than none.
	return result
}

// confidenceLine describes why a candidate is offered, because the match dialog
// has no score column of its own.
func confidenceLine(candidate apple.ScoredShow) string {
	parts := []string{fmt.Sprintf("Apple match %d%%", int(candidate.Score*100+0.5))}
	if candidate.Artist != "" {
		parts = append(parts, candidate.Artist)
	}
	if candidate.EpisodeCount > 0 {
		parts = append(parts, fmt.Sprintf("%d episodes", candidate.EpisodeCount))
	}
	if candidate.Genre != "" {
		parts = append(parts, candidate.Genre)
	}
	return strings.Join(parts, " · ")
}

func metadataItem(show apple.Show, channel apple.Channel) (*pluginv1.MetadataItem, error) {
	providerIDs, err := structpb.NewStruct(map[string]any{capabilityID: show.AppleID})
	if err != nil {
		return nil, err
	}
	extra := map[string]any{
		"apple_id": show.AppleID,
	}
	if show.AppleURL != "" {
		extra["apple_url"] = show.AppleURL
	}
	if show.FeedURL != "" {
		extra["feed_url"] = show.FeedURL
	}
	author := firstNonEmpty(channel.Author, show.Artist)
	if author != "" {
		extra["author"] = author
	}
	if show.EpisodeCount > 0 {
		extra["episode_count"] = show.EpisodeCount
	}
	extra["explicit"] = show.Explicit || channel.Explicit
	metadataStruct, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}

	item := &pluginv1.MetadataItem{
		ProviderId:       show.AppleID,
		ItemType:         itemType,
		Title:            firstNonEmpty(show.Title, channel.Title),
		OriginalTitle:    firstNonEmpty(show.Title, channel.Title),
		Overview:         channel.Description,
		ProviderIds:      providerIDs,
		PosterPath:       PosterPath(show.AppleID),
		OriginalLanguage: firstNonEmpty(channel.Language, show.Language),
		Metadata:         metadataStruct,
	}
	if year := yearOf(channel.FirstEpisodeDate); year > 0 {
		item.Year = int32(year)
	}
	if channel.FirstEpisodeDate != "" {
		item.FirstAirDate = channel.FirstEpisodeDate
	}
	if channel.LastEpisodeDate != "" {
		item.LastAirDate = channel.LastEpisodeDate
	}
	if author != "" {
		// Silo has no podcast publisher field, so the publisher rides on studios,
		// which the item page already renders.
		item.Studios = []string{author}
	}
	if show.Explicit || channel.Explicit {
		item.ContentRating = "Explicit"
	}
	genres := make([]string, 0, len(show.Genres)+len(channel.Categories))
	seen := map[string]struct{}{}
	for _, value := range append(append([]string{}, show.Genres...), channel.Categories...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		genres = append(genres, value)
	}
	item.Genres = genres
	return item, nil
}

func providerIDFromStruct(value *structpb.Struct) string {
	if value == nil {
		return ""
	}
	fields := value.GetFields()
	if field, ok := fields[capabilityID]; ok {
		return strings.TrimSpace(field.GetStringValue())
	}
	// Older hosts hand back the bare provider id in a single-key struct.
	for _, field := range fields {
		if text := strings.TrimSpace(field.GetStringValue()); text != "" {
			return apple.CollectionIDFromURL(text)
		}
	}
	return ""
}

func structString(value *structpb.Struct, key string) string {
	if value == nil {
		return ""
	}
	field, ok := value.GetFields()[key]
	if !ok {
		return ""
	}
	if text := field.GetStringValue(); text != "" {
		return text
	}
	return strings.TrimSpace(fmt.Sprint(field.AsInterface()))
}

func yearOf(value string) int {
	value = strings.TrimSpace(value)
	if len(value) < 4 {
		return 0
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil || year < 1900 || year > 2200 {
		return 0
	}
	return year
}


func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
