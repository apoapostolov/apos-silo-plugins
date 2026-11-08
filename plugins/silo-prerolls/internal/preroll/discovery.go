package preroll

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/config"
)

// Discover scans configured folders for preroll video files.
func Discover(folders []string, defaultTags map[string][]string, allowedFormats []string) ([]Preroll, error) {
	var pool []Preroll
	formatSet := make(map[string]bool)
	for _, f := range allowedFormats {
		formatSet[strings.ToLower(f)] = true
	}

	for _, folder := range folders {
		tags := defaultTags[folder]
		if tags == nil {
			tags = []string{}
		}

		// Load folder-level metadata
		folderMeta := loadFolderMeta(folder)

		err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !formatSet[ext] {
				return nil
			}

			// Skip sidecar json files
			if strings.HasSuffix(path, ".json") {
				return nil
			}

			// Check for sidecar metadata
			sidecarPath := path + ".json"
			meta := loadSidecar(sidecarPath)
			if meta == nil && folderMeta != nil {
				meta = folderMeta.FileMeta(filepath.Base(path))
			}

			prerollTags := tags
			weight := 1
			rating := ""
			var schedule *Schedule

			if meta != nil {
				if len(meta.Tags) > 0 {
					prerollTags = meta.Tags
				}
				if meta.Weight > 0 {
					weight = meta.Weight
				}
				rating = meta.Rating
				if meta.Schedule != nil {
					schedule = &Schedule{Start: meta.Schedule.Start, End: meta.Schedule.End}
				}
			}

			// Apply folder-level defaults
			if folderMeta != nil && folderMeta.Defaults != nil {
				if len(prerollTags) == 0 && len(folderMeta.Defaults.Tags) > 0 {
					prerollTags = folderMeta.Defaults.Tags
				}
				if weight == 1 && folderMeta.Defaults.Weight > 0 {
					weight = folderMeta.Defaults.Weight
				}
			}

			info2, _ := os.Stat(path)
			addedAt := time.Now()
			if info2 != nil {
				addedAt = info2.ModTime()
			}

			preroll := Preroll{
				ID:       filepath.Base(path),
				Path:     path,
				Filename: filepath.Base(path),
				Folder:   folder,
				Tags:     prerollTags,
				Weight:   weight,
				Rating:   rating,
				Schedule: schedule,
				AddedAt:  addedAt,
			}
			pool = append(pool, preroll)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk folder %s: %w", folder, err)
		}
	}

	return pool, nil
}

// FolderMeta represents folder-level prerolls.json metadata.
type FolderMeta struct {
	Defaults *config.SidecarMeta            `json:"defaults"`
	Files    map[string]*config.SidecarMeta `json:"files"`
}

func loadFolderMeta(folder string) *FolderMeta {
	path := filepath.Join(folder, "prerolls.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta FolderMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

func (fm *FolderMeta) FileMeta(filename string) *config.SidecarMeta {
	if fm.Files != nil {
		if m, ok := fm.Files[filename]; ok {
			return m
		}
	}
	return nil
}

func loadSidecar(path string) *config.SidecarMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta config.SidecarMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}
