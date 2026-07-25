// Design: plan/learned/913-firewall-irr.md -- shared PrefixStore path for firewall-irr

package irr

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/core/paths"
)

func cacheStorePath() string {
	configDir := paths.DefaultConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "database.zefs")
}
