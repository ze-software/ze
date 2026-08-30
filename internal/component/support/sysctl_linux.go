// Design: docs/architecture/core-design.md — sysctl collection for support bundle

package support

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/sysctl"
)

func collectSysctlInfo() (any, error) {
	keys := sysctl.All()
	results := make(map[string]string, len(keys))
	for _, k := range keys {
		if k.Template {
			continue
		}
		path := "/proc/sys/" + strings.ReplaceAll(k.Name, ".", "/")
		data, err := os.ReadFile(path) //nolint:gosec // reading known sysctl paths from registry
		if err != nil {
			continue
		}
		results[k.Name] = strings.TrimSpace(string(data))
	}
	return map[string]any{"sysctls": results, keyCount: len(results)}, nil
}
