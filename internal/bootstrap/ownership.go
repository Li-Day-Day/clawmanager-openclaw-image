package bootstrap

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

// applyOwnership recursively chowns the directories that were just seeded
// by syncDefaults / syncAutostart to the drop user. It is a no-op when the
// agent is not running as root.
func applyOwnership(cfg appconfig.Config) error {
	if os.Geteuid() != 0 {
		log.Printf("bootstrap: skipping chown because euid=%d", os.Geteuid())
		return nil
	}
	if cfg.DropUserName == "" {
		return nil
	}

	uid, gid, err := lookupDropUser(cfg.DropUserName)
	if err != nil {
		return err
	}

	stateDir := filepath.Dir(cfg.OpenClawConfigPath)
	if pathExists(stateDir) {
		if err := chownRecursive(stateDir, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", stateDir, err)
		}
	}
	if cfg.AutostartTargetDir != "" && pathExists(cfg.AutostartTargetDir) {
		if err := chownRecursive(cfg.AutostartTargetDir, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", cfg.AutostartTargetDir, err)
		}
	}
	return nil
}

func chownRecursive(root string, uid, gid int) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return fmt.Errorf("lchown %s: %w", path, err)
		}
		return nil
	})
}

func lookupDropUser(name string) (int, int, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, fmt.Errorf("lookup user %q: %w", name, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse uid of %q: %w", name, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse gid of %q: %w", name, err)
	}
	return uid, gid, nil
}
