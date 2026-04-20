package configmanager

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
	"github.com/iamlovingit/clawmanager-openclaw-image/internal/protocol"
	"github.com/iamlovingit/clawmanager-openclaw-image/internal/store"
)

func TestNormalizeActiveConfigSupportsGatewayModelList(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "openclaw.json")
	if err := os.WriteFile(configPath, []byte(sampleOpenClawConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAWMANAGER_LLM_MODEL", `["auto","gpt-4.1","claude-3.7-sonnet","deepseek-r1"]`)
	t.Setenv("CLAWMANAGER_LLM_BASE_URL", "https://gateway.example/v1")
	t.Setenv("CLAWMANAGER_LLM_API_KEY", "")

	bundledDir := t.TempDir()
	userDir := t.TempDir()
	manager := New(appconfig.Config{
		OpenClawConfigPath:           configPath,
		OpenClawBundledExtensionsDir: bundledDir,
		OpenClawExtensionsDir:        userDir,
		InstalledPluginPathPrefix:    "/does-not-exist/",
	}, nil, nil)
	if err := manager.NormalizeActiveConfig(); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigForTest(t, configPath)
	provider := nestedMapForTest(t, cfg, "models", "providers", "auto")

	if got := provider["baseUrl"]; got != "https://gateway.example/v1" {
		t.Fatalf("expected gateway baseUrl override, got %#v", got)
	}
	if got := provider["apiKey"]; got != "" {
		t.Fatalf("expected empty apiKey override, got %#v", got)
	}

	expectedModels := []string{"auto", "gpt-4.1", "claude-3.7-sonnet", "deepseek-r1"}
	if got := providerModelIDsForTest(t, provider); !equalStringSlices(got, expectedModels) {
		t.Fatalf("unexpected provider models: got %v want %v", got, expectedModels)
	}

	defaults := nestedMapForTest(t, cfg, "agents", "defaults")
	model := nestedMapForTest(t, defaults, "model")
	if got := model["primary"]; got != "auto/auto" {
		t.Fatalf("expected primary auto/auto, got %#v", got)
	}

	gotKeys := mapKeysForTest(t, defaults["models"])
	expectedKeys := []string{
		"auto/auto",
		"auto/claude-3.7-sonnet",
		"auto/deepseek-r1",
		"auto/gpt-4.1",
	}
	sort.Strings(expectedKeys)
	if !equalStringSlices(gotKeys, expectedKeys) {
		t.Fatalf("unexpected agent models keys: got %v want %v", gotKeys, expectedKeys)
	}
}

func TestApplyRevisionKeepsSingleModelCompatibility(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "active", "openclaw.json")
	stateDir := filepath.Join(root, "state")

	st, err := store.New(stateDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAWMANAGER_LLM_MODEL", "gpt-4.1")
	t.Setenv("OPENAI_BASE_URL", "https://gateway.example/v1")
	t.Setenv("OPENAI_API_KEY", "")

	manager := New(appconfig.Config{
		AgentDataDir:                 filepath.Join(root, "agent-data"),
		OpenClawConfigPath:           configPath,
		OpenClawBundledExtensionsDir: t.TempDir(),
		OpenClawExtensionsDir:        t.TempDir(),
		InstalledPluginPathPrefix:    "/does-not-exist/",
	}, stubRevisionClient{
		resp: protocol.ConfigRevisionResponse{
			Content: []byte(sampleOpenClawConfig),
		},
	}, st)

	if _, err := manager.ApplyRevision(context.Background(), "42"); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigForTest(t, configPath)
	provider := nestedMapForTest(t, cfg, "models", "providers", "auto")

	if got := provider["baseUrl"]; got != "https://gateway.example/v1" {
		t.Fatalf("expected OPENAI_BASE_URL fallback, got %#v", got)
	}
	if got := provider["apiKey"]; got != "" {
		t.Fatalf("expected empty apiKey from env fallback, got %#v", got)
	}

	expectedModels := []string{"gpt-4.1"}
	if got := providerModelIDsForTest(t, provider); !equalStringSlices(got, expectedModels) {
		t.Fatalf("unexpected provider models: got %v want %v", got, expectedModels)
	}

	defaults := nestedMapForTest(t, cfg, "agents", "defaults")
	model := nestedMapForTest(t, defaults, "model")
	if got := model["primary"]; got != "auto/gpt-4.1" {
		t.Fatalf("expected primary auto/gpt-4.1, got %#v", got)
	}

	gotKeys := mapKeysForTest(t, defaults["models"])
	expectedKeys := []string{"auto/gpt-4.1"}
	if !equalStringSlices(gotKeys, expectedKeys) {
		t.Fatalf("unexpected agent models keys: got %v want %v", gotKeys, expectedKeys)
	}
}

func TestChannelsMergeRejectsUnknownIds(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "openclaw.json")
	bundledDir := filepath.Join(root, "bundled-extensions")
	userDir := filepath.Join(root, "user-extensions")

	if err := os.MkdirAll(filepath.Join(bundledDir, "dingtalk-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"channels":["dingtalk"]}`)
	if err := os.WriteFile(filepath.Join(bundledDir, "dingtalk-plugin", "openclaw.plugin.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(sampleOpenClawConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAWMANAGER_OPENCLAW_CHANNELS_JSON", `{"dingtalk":{"enabled":true},"slack":{"enabled":true}}`)
	t.Setenv("CLAWMANAGER_LLM_MODEL", "")
	t.Setenv("CLAWMANAGER_LLM_BASE_URL", "")
	os.Unsetenv("CLAWMANAGER_LLM_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	manager := New(appconfig.Config{
		OpenClawConfigPath:           configPath,
		OpenClawBundledExtensionsDir: bundledDir,
		OpenClawExtensionsDir:        userDir,
	}, nil, nil)
	if err := manager.NormalizeActiveConfig(); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigForTest(t, configPath)
	channels := nestedMapForTest(t, cfg, "channels")
	if _, ok := channels["dingtalk"]; !ok {
		t.Fatalf("expected dingtalk channel to be preserved, got %#v", channels)
	}
	if _, ok := channels["slack"]; ok {
		t.Fatalf("expected unsupported slack channel to be dropped, got %#v", channels)
	}
}

func TestPluginInstallPathRewritten(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "openclaw.json")
	userDir := "/config/.openclaw/extensions"

	sample := `{
		"plugins": {
			"installs": {
				"foo": {
					"installPath": "/defaults/.openclaw/extensions/foo"
				},
				"bar": {
					"installPath": "/opt/vendor/bar"
				}
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Unsetenv("CLAWMANAGER_OPENCLAW_CHANNELS_JSON")
	os.Unsetenv("CLAWMANAGER_LLM_MODEL")
	os.Unsetenv("CLAWMANAGER_LLM_BASE_URL")
	os.Unsetenv("CLAWMANAGER_LLM_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	manager := New(appconfig.Config{
		OpenClawConfigPath:        configPath,
		OpenClawExtensionsDir:     userDir,
		InstalledPluginPathPrefix: "/defaults/.openclaw/extensions/",
	}, nil, nil)
	if err := manager.NormalizeActiveConfig(); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigForTest(t, configPath)
	installs := nestedMapForTest(t, cfg, "plugins", "installs")
	foo, _ := installs["foo"].(map[string]any)
	bar, _ := installs["bar"].(map[string]any)
	wantFoo := path.Join(userDir, "foo")
	if got, _ := foo["installPath"].(string); got != wantFoo {
		t.Fatalf("expected foo installPath to be rewritten to %q, got %q", wantFoo, got)
	}
	if got, _ := bar["installPath"].(string); got != "/opt/vendor/bar" {
		t.Fatalf("expected bar installPath to be untouched, got %q", got)
	}
}

type stubRevisionClient struct {
	resp protocol.ConfigRevisionResponse
	err  error
}

func (s stubRevisionClient) FetchConfigRevision(context.Context, string) (protocol.ConfigRevisionResponse, error) {
	return s.resp, s.err
}

func readConfigForTest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func nestedMapForTest(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()
	current := root
	for _, part := range path {
		next, ok := current[part].(map[string]any)
		if !ok {
			t.Fatalf("expected object at %v, got %#v", path, current[part])
		}
		current = next
	}
	return current
}

func providerModelIDsForTest(t *testing.T, provider map[string]any) []string {
	t.Helper()
	items, ok := provider["models"].([]any)
	if !ok {
		t.Fatalf("expected provider models array, got %#v", provider["models"])
	}
	modelIDs := make([]string, 0, len(items))
	for _, item := range items {
		model, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected model object, got %#v", item)
		}
		id, ok := model["id"].(string)
		if !ok {
			t.Fatalf("expected string model id, got %#v", model["id"])
		}
		modelIDs = append(modelIDs, id)
	}
	return modelIDs
}

func mapKeysForTest(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected model map, got %#v", value)
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const sampleOpenClawConfig = `{
    "models": {
        "mode": "merge",
        "providers": {
            "auto": {
                "baseUrl": "https://legacy.example/v1",
                "apiKey": "legacy-api-key",
                "api": "openai-completions",
                "models": [
                    {
                        "id": "legacy-model",
                        "name": "Legacy Model",
                        "reasoning": false,
                        "input": [
                            "text"
                        ],
                        "cost": {
                            "input": 0,
                            "output": 0,
                            "cacheRead": 0,
                            "cacheWrite": 0
                        },
                        "contextWindow": 64000,
                        "maxTokens": 8192
                    }
                ]
            }
        }
    },
    "agents": {
        "defaults": {
            "model": {
                "primary": "auto/legacy-model"
            },
            "models": {
                "auto/legacy-model": {}
            }
        }
    }
}`
