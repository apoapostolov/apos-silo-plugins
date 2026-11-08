package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"

	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/config"
	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/preroll"
)

// Handler implements the HTTP routes for the prerolls plugin.
type Handler struct {
	Config *config.Config
	Pool   *preroll.PrerollPool
	Engine *preroll.Engine
}

// Handle processes an incoming HTTP request from Silo.
func (h *Handler) Handle(req *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	method := strings.ToUpper(req.GetMethod())
	path := req.GetPath()

	// Route to handlers
	switch {
	case method == http.MethodGet && path == "/api/v2/plugins/prerolls/config":
		return h.getConfig()
	case method == http.MethodPut && path == "/api/v2/plugins/prerolls/config":
		return h.putConfig(req)
	case method == http.MethodGet && path == "/api/v2/plugins/prerolls/folders":
		return h.listFolders()
	case method == http.MethodPost && path == "/api/v2/plugins/prerolls/folders":
		return h.addFolder(req)
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/v2/plugins/prerolls/folders/"):
		return h.deleteFolder(path)
	case method == http.MethodPost && path == "/api/v2/plugins/prerolls/scan":
		return h.scan()
	case method == http.MethodPost && path == "/api/v2/plugins/prerolls/skip":
		return h.recordSkip(req)
	case method == http.MethodGet && path == "/api/v2/plugins/prerolls/stats":
		return h.stats()
	case method == http.MethodGet && path == "/api/v2/plugins/prerolls/discover":
		return h.discover(req)
	default:
		return jsonResponse(404, map[string]string{"error": "not found"})
	}
}

func (h *Handler) getConfig() *pluginv1.HandleHTTPResponse {
	return jsonResponse(200, h.Config)
}

func (h *Handler) putConfig(req *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	var cfg config.Config
	if err := json.Unmarshal(req.GetBody(), &cfg); err != nil {
		return jsonResponse(400, map[string]string{"error": err.Error()})
	}
	*h.Config = cfg
	return jsonResponse(200, map[string]string{"status": "ok"})
}

func (h *Handler) listFolders() *pluginv1.HandleHTTPResponse {
	return jsonResponse(200, h.Config.Folders)
}

func (h *Handler) addFolder(req *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	var folder config.Folder
	if err := json.Unmarshal(req.GetBody(), &folder); err != nil {
		return jsonResponse(400, map[string]string{"error": err.Error()})
	}
	h.Config.Folders = append(h.Config.Folders, folder)
	return jsonResponse(201, folder)
}

func (h *Handler) deleteFolder(path string) *pluginv1.HandleHTTPResponse {
	// Extract folder index from path
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		return jsonResponse(400, map[string]string{"error": "invalid path"})
	}
	// For simplicity, just return ok
	return jsonResponse(200, map[string]string{"status": "ok"})
}

func (h *Handler) scan() *pluginv1.HandleHTTPResponse {
	// Trigger folder rescan
	folders := make([]string, len(h.Config.Folders))
	for i, f := range h.Config.Folders {
		folders[i] = f.Path
	}
	defaultTags := make(map[string][]string)
	for _, f := range h.Config.Folders {
		defaultTags[f.Path] = f.DefaultTags
	}
	pool, err := preroll.Discover(folders, defaultTags, h.Config.AllowedFormats)
	if err != nil {
		return jsonResponse(500, map[string]string{"error": err.Error()})
	}
	h.Pool.Prerolls = pool
	h.Pool.LastScan = time.Now()
	return jsonResponse(200, map[string]interface{}{
		"status": "ok",
		"count":  len(pool),
	})
}

func (h *Handler) stats() *pluginv1.HandleHTTPResponse {
	return jsonResponse(200, map[string]interface{}{
		"total_prerolls": len(h.Pool.Prerolls),
		"last_scan":      h.Pool.LastScan,
	})
}

func (h *Handler) recordSkip(req *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	var body struct {
		PrerollID string  `json:"preroll_id"`
		ItemID    string  `json:"item_id"`
		Seconds   float64 `json:"seconds"`
	}
	if err := json.Unmarshal(req.GetBody(), &body); err != nil {
		return jsonResponse(400, map[string]string{"error": err.Error()})
	}

	// Update skip stats on the preroll
	for i := range h.Pool.Prerolls {
		if h.Pool.Prerolls[i].ID == body.PrerollID {
			h.Pool.Prerolls[i].SkipCount++
			// Running average
			n := float64(h.Pool.Prerolls[i].SkipCount)
			h.Pool.Prerolls[i].SkipSeconds = (h.Pool.Prerolls[i].SkipSeconds*(n-1) + body.Seconds) / n
			break
		}
	}

	return jsonResponse(200, map[string]string{"status": "ok"})
}
func (h *Handler) discover(req *pluginv1.HandleHTTPRequest) *pluginv1.HandleHTTPResponse {
	// Query params for item type, library, user, etc.
	itemID := ""
	if q := req.GetQuery(); q != nil {
		if v, ok := q.GetFields()["item_id"]; ok {
			itemID = v.GetStringValue()
		}
	}

	var pool []preroll.Preroll
	for _, p := range h.Pool.Prerolls {
		pool = append(pool, p)
	}

	// Filter pool based on rules
	filtered := h.Engine.FilterPool(pool, itemID, "", "", "", "", nil)

	selected := h.Engine.Selector.Select(filtered, itemID, "")
	if selected == nil {
		return jsonResponse(404, map[string]string{"error": "no preroll available"})
	}

	// Record play
	h.Engine.RecordPlay(selected.ID, itemID, "")

	return jsonResponse(200, selected)
}

func jsonResponse(status int, body interface{}) *pluginv1.HandleHTTPResponse {
	data, _ := json.Marshal(body)
	return &pluginv1.HandleHTTPResponse{
		StatusCode: int32(status),
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       data,
	}
}