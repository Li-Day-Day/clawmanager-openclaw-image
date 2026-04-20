package configmanager

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

const pluginManifestName = "openclaw.plugin.json"

// channelOverrides captures the inputs needed to reconcile the `channels`
// subtree and to rewrite installed plugin paths, replacing the inline
// Node.js block that used to live in scripts/99-openclaw-sync.
type channelOverrides struct {
	RawJSON                   string
	HasRawJSON                bool
	BundledExtensionsDir      string
	UserExtensionsDir         string
	InstalledPluginPathPrefix string
}

func readChannelOverridesFromEnv(cfg appconfig.Config) channelOverrides {
	raw, has := os.LookupEnv("CLAWMANAGER_OPENCLAW_CHANNELS_JSON")
	return channelOverrides{
		RawJSON:                   raw,
		HasRawJSON:                has,
		BundledExtensionsDir:      cfg.OpenClawBundledExtensionsDir,
		UserExtensionsDir:         cfg.OpenClawExtensionsDir,
		InstalledPluginPathPrefix: cfg.InstalledPluginPathPrefix,
	}
}

// applyChannelOverrides mutates cfg to:
//   - rewrite plugins.installs[*].installPath prefixes so installs seeded
//     under /defaults/.openclaw/extensions/* point at the user extensions
//     directory on /config;
//   - sanitize the existing cfg.channels by dropping entries whose id is
//     not advertised by any bundled or user-installed plugin;
//   - merge any channels supplied via CLAWMANAGER_OPENCLAW_CHANNELS_JSON
//     after applying the same sanitization.
func applyChannelOverrides(cfg map[string]any, opts channelOverrides) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	envChannels, err := parseChannelsEnvJSON(opts.RawJSON, opts.HasRawJSON)
	if err != nil {
		return err
	}

	rewriteInstalledPluginPaths(cfg, opts.InstalledPluginPathPrefix, opts.UserExtensionsDir)

	supported := map[string]struct{}{}
	collectSupportedChannelIds(opts.BundledExtensionsDir, supported)
	collectSupportedChannelIds(opts.UserExtensionsDir, supported)

	existing := ensureObject(cfg, "channels")
	sanitized := sanitizeChannels(existing, supported, "existing config")

	fromEnv := sanitizeChannels(envChannels, supported, "CLAWMANAGER_OPENCLAW_CHANNELS_JSON")
	for id, value := range fromEnv {
		sanitized[id] = value
	}
	cfg["channels"] = sanitized
	return nil
}

func parseChannelsEnvJSON(raw string, present bool) (map[string]any, error) {
	if !present {
		return map[string]any{}, nil
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("parse CLAWMANAGER_OPENCLAW_CHANNELS_JSON: %w", err)
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("CLAWMANAGER_OPENCLAW_CHANNELS_JSON must be a JSON object")
	}
	return obj, nil
}

func rewriteInstalledPluginPaths(cfg map[string]any, prefix, userExtensionsDir string) {
	if prefix == "" || userExtensionsDir == "" {
		return
	}
	plugins, ok := cfg["plugins"].(map[string]any)
	if !ok {
		return
	}
	installs, ok := plugins["installs"].(map[string]any)
	if !ok {
		return
	}
	for _, entry := range installs {
		install, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		installPath, ok := install["installPath"].(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(installPath, prefix) {
			continue
		}
		install["installPath"] = path.Join(userExtensionsDir, installPath[len(prefix):])
	}
}

func collectSupportedChannelIds(rootDir string, out map[string]struct{}) {
	if rootDir == "" {
		return
	}
	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		return
	}
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		log.Printf("configmanager: read plugins dir %s: %v", rootDir, err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(rootDir, entry.Name(), pluginManifestName)
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			log.Printf("configmanager: read %s: %v", manifestPath, err)
			continue
		}
		var manifest struct {
			Channels []string `json:"channels"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			log.Printf("configmanager: parse %s: %v", manifestPath, err)
			continue
		}
		for _, id := range manifest.Channels {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" {
				out[trimmed] = struct{}{}
			}
		}
	}
}

func sanitizeChannels(source map[string]any, supported map[string]struct{}, label string) map[string]any {
	sanitized := make(map[string]any, len(source))
	for id, value := range source {
		if _, ok := supported[id]; ok {
			sanitized[id] = value
			continue
		}
		log.Printf("configmanager: skipping unsupported channel %q from %s; no matching extension was found", id, label)
	}
	return sanitized
}
