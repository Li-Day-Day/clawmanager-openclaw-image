package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

func TestSyncDefaultsCopiesWhenStateMissing(t *testing.T) {
	root := t.TempDir()
	defaultsDir := filepath.Join(root, "defaults")
	stateDir := filepath.Join(root, "state")

	if err := os.MkdirAll(filepath.Join(defaultsDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	configBytes := []byte(`{"hello":"world"}`)
	if err := os.WriteFile(filepath.Join(defaultsDir, "openclaw.json"), configBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultsDir, "nested", "child.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := appconfig.Config{
		OpenClawDefaultsDir: defaultsDir,
		OpenClawConfigPath:  filepath.Join(stateDir, "openclaw.json"),
	}
	if err := syncDefaults(cfg); err != nil {
		t.Fatal(err)
	}

	gotConfig, err := os.ReadFile(filepath.Join(stateDir, "openclaw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotConfig) != string(configBytes) {
		t.Fatalf("expected copied config %q, got %q", configBytes, gotConfig)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "nested", "child.json")); err != nil {
		t.Fatalf("expected nested child to be copied: %v", err)
	}
}

func TestSyncDefaultsDoesNotOverwriteExistingConfig(t *testing.T) {
	root := t.TempDir()
	defaultsDir := filepath.Join(root, "defaults")
	stateDir := filepath.Join(root, "state")

	if err := os.MkdirAll(defaultsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultsDir, "openclaw.json"), []byte(`{"from":"defaults"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userContent := []byte(`{"from":"user"}`)
	if err := os.WriteFile(filepath.Join(stateDir, "openclaw.json"), userContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := appconfig.Config{
		OpenClawDefaultsDir: defaultsDir,
		OpenClawConfigPath:  filepath.Join(stateDir, "openclaw.json"),
	}
	if err := syncDefaults(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(stateDir, "openclaw.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(userContent) {
		t.Fatalf("expected user content preserved, got %q", got)
	}
}

func TestEnsureExtensionsDirCreates(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "extensions")
	cfg := appconfig.Config{OpenClawExtensionsDir: target}
	if err := ensureExtensionsDir(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory at %s", target)
	}
}

func TestSyncAutostartInstallsOnlyMissing(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "defaults-autostart")
	dst := filepath.Join(root, "user-autostart")

	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	newEntry := []byte("[Desktop Entry]\nName=New\n")
	existingEntry := []byte("[Desktop Entry]\nName=Existing Default\n")
	userOverride := []byte("[Desktop Entry]\nName=User Override\n")

	if err := os.WriteFile(filepath.Join(src, "new.desktop"), newEntry, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "existing.desktop"), existingEntry, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "existing.desktop"), userOverride, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := appconfig.Config{
		AutostartDefaultsDir: src,
		AutostartTargetDir:   dst,
	}
	if err := syncAutostart(cfg); err != nil {
		t.Fatal(err)
	}

	gotNew, err := os.ReadFile(filepath.Join(dst, "new.desktop"))
	if err != nil {
		t.Fatalf("expected new.desktop to be installed: %v", err)
	}
	if string(gotNew) != string(newEntry) {
		t.Fatalf("new.desktop content mismatch: %q", gotNew)
	}

	gotExisting, err := os.ReadFile(filepath.Join(dst, "existing.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotExisting) != string(userOverride) {
		t.Fatalf("expected user override preserved, got %q", gotExisting)
	}
}

func TestApplyOwnershipNoopWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test only meaningful for non-root execution")
	}
	cfg := appconfig.Config{
		OpenClawConfigPath: filepath.Join(t.TempDir(), "nonexistent", "openclaw.json"),
		DropUserName:       "abc",
	}
	if err := applyOwnership(cfg); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}

func TestDropPrivilegesNoopWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test only meaningful for non-root execution")
	}
	cfg := appconfig.Config{DropUserName: "abc"}
	if err := dropPrivileges(cfg); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}
