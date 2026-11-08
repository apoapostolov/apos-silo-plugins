package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	goruntime "runtime"
	"time"

	"github.com/hashicorp/go-hclog"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	sdkruntime "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtimedefault"

	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/api"
	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/config"
	"github.com/apoapostolov/apos-silo-plugins/plugins/silo-prerolls/internal/preroll"
)

//go:embed manifest.json
var manifestJSON []byte

type runtimeServer struct {
	runtimedefault.Server
	manifest *pluginv1.PluginManifest
}

func (s *runtimeServer) GetManifest(_ context.Context, _ *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

type httpRoutesServer struct {
	logger  hclog.Logger
	handler *api.Handler
}

func (s *httpRoutesServer) Handle(ctx context.Context, req *pluginv1.HandleHTTPRequest) (*pluginv1.HandleHTTPResponse, error) {
	return s.handler.Handle(req), nil
}

type scheduledTaskServer struct {
	logger  hclog.Logger
	handler *api.Handler
}

func (s *scheduledTaskServer) Run(ctx context.Context, req *pluginv1.RunScheduledTaskRequest) (*pluginv1.RunScheduledTaskResponse, error) {
	// Trigger folder rescan
	folders := make([]string, len(s.handler.Config.Folders))
	for i, f := range s.handler.Config.Folders {
		folders[i] = f.Path
	}
	defaultTags := make(map[string][]string)
	for _, f := range s.handler.Config.Folders {
		defaultTags[f.Path] = f.DefaultTags
	}
	pool, err := preroll.Discover(folders, defaultTags, s.handler.Config.AllowedFormats)
	if err != nil {
		return nil, err
	}
	s.handler.Pool.Prerolls = pool
	s.handler.Pool.LastScan = time.Now()
	_ = pool
	return &pluginv1.RunScheduledTaskResponse{}, nil
}

type eventConsumerServer struct {
	logger  hclog.Logger
	handler *api.Handler
}

func (s *eventConsumerServer) HandleEvent(ctx context.Context, req *pluginv1.HandleEventRequest) (*pluginv1.HandleEventResponse, error) {
	// Log events for debugging
	s.logger.Debug("event received", "name", req.GetEventName())
	return &pluginv1.HandleEventResponse{}, nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{Name: "silo-prerolls"})

	manifest, err := loadManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig()
	pool := &preroll.PrerollPool{}
	engine := preroll.NewEngine(preroll.SelectionMode(cfg.SelectionMode), cfg.CooldownHours)
	engine.Config = &preroll.ConfigRef{
		CooldownHours:  cfg.CooldownHours,
		MaturityGating: cfg.MaturityGating,
		SessionMode:    cfg.SessionMode,
		PerUserRules:   make(map[string]preroll.UserRuleRef),
		LibraryRules:   []preroll.LibraryRuleRef{},
		OptOuts:        cfg.OptOuts,
	}
	for k, v := range cfg.PerUserRules {
		engine.Config.PerUserRules[k] = preroll.UserRuleRef{Mode: v.Mode, Users: v.Users}
	}
	for _, r := range cfg.LibraryRules {
		engine.Config.LibraryRules = append(engine.Config.LibraryRules, preroll.LibraryRuleRef{
			Name:        r.Name,
			Library:     r.Library,
			ItemType:    r.ItemType,
			RequireTags: r.RequireTags,
			ExcludeTags: r.ExcludeTags,
			Enabled:     r.Enabled,
		})
	}

	handler := &api.Handler{
		Config: &cfg,
		Pool:   pool,
		Engine: engine,
	}

	httpServer := &httpRoutesServer{logger: logger, handler: handler}
	scanServer := &scheduledTaskServer{logger: logger, handler: handler}
	eventServer := &eventConsumerServer{logger: logger, handler: handler}

	runtime := &runtimeServer{manifest: manifest}

	sdkruntime.Serve(sdkruntime.ServeConfig{
		Servers: sdkruntime.CapabilityServers{
			Runtime:       runtime,
			HttpRoutes:    httpServer,
			ScheduledTask: scanServer,
			EventConsumer: eventServer,
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binaryData, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binaryData)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	if len(manifest.GetSupportedPlatforms()) == 0 {
		manifest.SupportedPlatforms = []*pluginv1.SupportedPlatform{
			{Os: goruntime.GOOS, Arch: goruntime.GOARCH},
		}
	}

	return manifest, nil
}
