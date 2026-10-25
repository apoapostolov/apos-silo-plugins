package main

import (
	"encoding/json"
	"os"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
)

// TestManifestDecodesWithEncodingJSON guards the encoding the host uses for the
// repository index. encoding/json wants enum numbers, while the SDK's protojson
// path accepts the names, so a manifest that reads fine in the plugin can still
// make the whole catalog unparseable on the server.
func TestManifestDecodesWithEncodingJSON(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest pluginv1.PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("the host cannot decode this manifest: %v", err)
	}
	if manifest.GetPluginId() == "" || manifest.GetVersion() == "" {
		t.Fatalf("decoded manifest is empty: id=%q version=%q", manifest.GetPluginId(), manifest.GetVersion())
	}
}
