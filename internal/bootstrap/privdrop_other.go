//go:build !linux

package bootstrap

import (
	"log"

	appconfig "github.com/iamlovingit/clawmanager-openclaw-image/internal/config"
)

// dropPrivileges is a no-op on non-Linux platforms. The agent only drops
// privileges on its target runtime (linuxserver/webtop container).
func dropPrivileges(cfg appconfig.Config) error {
	log.Printf("bootstrap: skipping privilege drop on non-linux build")
	return nil
}
