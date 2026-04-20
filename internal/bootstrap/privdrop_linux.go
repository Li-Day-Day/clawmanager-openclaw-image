//go:build linux

// The privilege-drop logic is split into privdrop_linux.go and
// privdrop_other.go because syscall.Setuid/Setgid/Setgroups are only
// available under the linux build constraint; privdrop_other.go exists
// purely so `go build ./...` succeeds on non-Linux developer machines.
// Only this file is compiled into the container image at runtime.

package bootstrap

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"syscall"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

// dropPrivileges drops the effective user/group of every Go-managed thread
// to cfg.DropUserName. The call is a no-op unless the process is currently
// running as root.
func dropPrivileges(cfg appconfig.Config) error {
	if os.Geteuid() != 0 {
		log.Printf("bootstrap: skipping privilege drop because euid=%d", os.Geteuid())
		return nil
	}
	if cfg.DropUserName == "" {
		return nil
	}

	uid, gid, err := lookupDropUser(cfg.DropUserName)
	if err != nil {
		return err
	}

	u, err := user.Lookup(cfg.DropUserName)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", cfg.DropUserName, err)
	}

	if err := syscall.Setgroups([]int{gid}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid %d: %w", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid %d: %w", uid, err)
	}

	if u.HomeDir != "" {
		if err := os.Setenv("HOME", u.HomeDir); err != nil {
			return fmt.Errorf("set HOME: %w", err)
		}
	}
	if err := os.Setenv("USER", cfg.DropUserName); err != nil {
		return fmt.Errorf("set USER: %w", err)
	}
	if err := os.Setenv("LOGNAME", cfg.DropUserName); err != nil {
		return fmt.Errorf("set LOGNAME: %w", err)
	}
	return nil
}
