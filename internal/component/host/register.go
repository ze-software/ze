// Design: docs/features/ai-first.md -- platform name registration for doctor check validation

package host

import "github.com/ze-software/ze/internal/component/plugin/registry"

func init() {
	names := make([]string, 0, len(platformTypeNames))
	for _, name := range platformTypeNames {
		names = append(names, name)
	}
	registry.RegisterDoctorPlatforms(names)
}
