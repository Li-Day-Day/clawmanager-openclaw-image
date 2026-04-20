package bootstrap

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

// syncAutostart installs every *.desktop entry from cfg.AutostartDefaultsDir
// into cfg.AutostartTargetDir exactly once. Existing targets are never
// overwritten so the user can customise their XFCE autostart freely.
func syncAutostart(cfg appconfig.Config) error {
	if cfg.AutostartDefaultsDir == "" || cfg.AutostartTargetDir == "" {
		return nil
	}
	if !pathExists(cfg.AutostartDefaultsDir) {
		return nil
	}

	entries, err := os.ReadDir(cfg.AutostartDefaultsDir)
	if err != nil {
		return fmt.Errorf("read autostart defaults: %w", err)
	}

	var desktopEntries []fs.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".desktop") {
			continue
		}
		desktopEntries = append(desktopEntries, entry)
	}
	if len(desktopEntries) == 0 {
		return nil
	}

	if err := os.MkdirAll(cfg.AutostartTargetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir autostart target: %w", err)
	}

	for _, entry := range desktopEntries {
		src := filepath.Join(cfg.AutostartDefaultsDir, entry.Name())
		dst := filepath.Join(cfg.AutostartTargetDir, entry.Name())
		if pathExists(dst) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", src, err)
		}
		if err := copyRegularFile(src, dst, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}
