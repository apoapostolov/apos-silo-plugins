package preroll

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// PostRoll represents an outro video played after the feature.
type PostRoll struct {
	Preroll
}

// SeriesPreroll looks up a series-specific preroll file.
func SeriesPreroll(seriesPath string, filename string, allowedFormats []string) *Preroll {
	for _, ext := range allowedFormats {
		path := seriesPath + "/" + filename + ext
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return &Preroll{
				ID:       filename + ext,
				Path:     path,
				Filename: filename + ext,
				Folder:   seriesPath,
				Tags:     []string{"series-specific"},
				Weight:   5,
				AddedAt:  info.ModTime(),
			}
		}
	}
	return nil
}

// TrailerChain returns up to N trailer paths from a feature's metadata.
func TrailerChain(trailerPaths []string, count int) []string {
	if count <= 0 || len(trailerPaths) == 0 {
		return nil
	}
	if count > len(trailerPaths) {
		count = len(trailerPaths)
	}
	return trailerPaths[:count]
}

// NeXrollClient provides integration with NeXroll's API.
type NeXrollClient struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewNeXrollClient creates a new NeXroll API client.
func NewNeXrollClient(baseURL, apiKey string) *NeXrollClient {
	return &NeXrollClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Client:  &http.Client{},
	}
}

// GetLibrary fetches prerolls from NeXroll's library.
func (c *NeXrollClient) GetLibrary() ([]Preroll, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/external/prerolls", nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Api-Key", c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nexroll API returned %d", resp.StatusCode)
	}

	var result struct {
		Prerolls []struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			Path     string   `json:"path"`
			Category string   `json:"category"`
			Tags     []string `json:"tags"`
			Weight   int      `json:"weight"`
		} `json:"prerolls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var pool []Preroll
	for _, p := range result.Prerolls {
		pool = append(pool, Preroll{
			ID:       p.ID,
			Path:     p.Path,
			Filename: p.Name,
			Folder:   "nexroll",
			Tags:     p.Tags,
			Weight:   p.Weight,
			AddedAt:  time.Now(),
		})
	}
	return pool, nil
}

// GetSchedule fetches the active schedule from NeXroll.
func (c *NeXrollClient) GetSchedule() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/external/active-schedules", nil)
	if err != nil {
		return nil, err
	}
	if c.APIKey != "" {
		req.Header.Set("X-Api-Key", c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nexroll API returned %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}