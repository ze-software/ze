// Design: docs/architecture/appliance/iso-installer.md -- selected GRUB modules.

package appliance

import (
	"errors"
	"os"
	"path/filepath"
)

// checkISOGRUBModules returns the admitted module directory so the builder uses
// the same files that prerequisite checking accepted.
func checkISOGRUBModules(builder, target string) (string, error) {
	prefixes := []string{filepath.Dir(filepath.Dir(builder))}
	if real, err := filepath.EvalSymlinks(builder); err == nil {
		prefixes = append(prefixes, filepath.Dir(filepath.Dir(real)))
	}
	prefixes = append(prefixes, "/usr", "/usr/local", "/opt/homebrew")
	for _, prefix := range prefixes {
		for _, library := range []string{"lib/grub", "lib/grub2", "lib64/grub", "lib64/grub2", "share/grub"} {
			path := filepath.Join(prefix, library, target, "modinfo.sh")
			info, err := os.Stat(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return "", isoGRUBModuleError(err, builder, target, path)
			}
			if info.Mode().IsRegular() {
				return filepath.Dir(path), nil
			}
		}
	}
	return "", isoGRUBModuleError(nil, builder, target, "")
}
