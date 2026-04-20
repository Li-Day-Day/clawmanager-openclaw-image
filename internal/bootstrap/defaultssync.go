package bootstrap

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

// syncDefaults copies cfg.OpenClawDefaultsDir into the parent directory of
// cfg.OpenClawConfigPath when the state directory or the active config file
// is missing. The copy preserves file modes and skips targets that already
// exist so we never overwrite a user's on-disk /config/.openclaw state.
func syncDefaults(cfg appconfig.Config) error {
	stateDir := filepath.Dir(cfg.OpenClawConfigPath)
	stateDirMissing := !pathExists(stateDir)
	configMissing := !pathExists(cfg.OpenClawConfigPath)
	if !stateDirMissing && !configMissing {
		return nil
	}

	if !pathExists(cfg.OpenClawDefaultsDir) {
		return fmt.Errorf("defaults source %q does not exist", cfg.OpenClawDefaultsDir)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	return copyTreeIfMissing(cfg.OpenClawDefaultsDir, stateDir)
}

// ensureExtensionsDir makes the user extensions directory exist so external
// channel plugins have a predictable mount target.
func ensureExtensionsDir(cfg appconfig.Config) error {
	if cfg.OpenClawExtensionsDir == "" {
		return nil
	}
	return os.MkdirAll(cfg.OpenClawExtensionsDir, 0o755)
}

func copyTreeIfMissing(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
			return nil
		}

		if pathExists(target) {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				return fmt.Errorf("symlink %s: %w", target, err)
			}
			return nil
		}

		return copyRegularFile(path, target, info.Mode().Perm())
	})
}

func copyRegularFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir parent of %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	if err := os.Chmod(dst, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", dst, err)
	}
	return nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
