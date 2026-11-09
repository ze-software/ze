// Design: docs/features/dns-resolver.md — the resolv-conf-path option
// Related: validators_register.go — registers resolv-conf-path
// Related: system/resolv.go — Resolvers.flush, the writer of the file this leaf names

package config

import (
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// ResolvConfPathValidator refuses a `system dns resolv-conf-path` that is not
// an absolute path in clean form. The empty string stays valid: it is how an
// operator stops Ze writing the file.
//
// Before this refusal the extractor turned such a path into the empty string
// in silence, so the config committed, Ze wrote no resolv.conf, and nothing
// said why. A relative path also has no meaning a reader can check: it names a
// file under whatever directory the daemon was started from.
func ResolvConfPathValidator() yang.CustomValidator {
	return yang.CustomValidator{
		ValidateFn: func(_ string, value any) error {
			path, ok := value.(string)
			if !ok {
				return fmt.Errorf("resolv-conf-path must be a string, got %T", value)
			}
			return ValidateResolvConfPath(path)
		},
	}
}

// ValidateResolvConfPath answers whether Ze will write its DNS servers to path.
// The empty string is accepted and means Ze writes no file.
func ValidateResolvConfPath(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("resolv-conf-path %q is not an absolute path, so it would name a file under "+
			"whatever directory Ze was started from; write an absolute path such as /etc/resolv.conf, "+
			"or \"\" to stop Ze writing the file", path)
	}
	clean := filepath.Clean(path)
	if clean != path {
		return fmt.Errorf("resolv-conf-path %q is not in clean form, and a .. or repeated / element hides "+
			"which file Ze writes; write %q instead", path, clean)
	}
	return nil
}
