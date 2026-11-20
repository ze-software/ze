// Design: docs/architecture/testing/interop.md -- the persistent image cache.
// Related: docker_image_cache.go -- a host-CPU-specific key carries this identity.
// Related: host_cpu_linux.go -- the Linux reader of /proc/cpuinfo.
// Related: host_cpu_darwin.go -- the macOS reader of sysctl.
package interoplab

import (
	"errors"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// cpuinfoIdentityFields are the /proc/cpuinfo fields that decide what a
// compiler targeting the native CPU emits, in the order the identity lists
// them: the x86 vendor, family, model and flags, and the arm64 implementer,
// architecture, variant, part and features. Speed, cache size and the
// microcode revision change nothing a compiler emits.
var cpuinfoIdentityFields = []string{
	"vendor_id", "cpu family", "model", "model name", "flags",
	"CPU implementer", "CPU architecture", "CPU variant", "CPU part", "Features",
}

// cpuinfoIdentity answers the CPU identity in a /proc/cpuinfo text: the
// cpuinfoIdentityFields of its first processor, one `name=value` line each.
// A text that names no model (`model name` or `CPU part`) or no instruction
// set (`flags` or `Features`) is refused, because an identity without them
// would match a CPU of another kind. The loop is bounded by the text.
func cpuinfoIdentity(cpuinfo string) (string, error) {
	values := make(map[string]string, len(cpuinfoIdentityFields))
	for line := range strings.SplitSeq(cpuinfo, "\n") {
		if strings.TrimSpace(line) == "" {
			if len(values) > 0 {
				break
			}
			continue
		}
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		if !slices.Contains(cpuinfoIdentityFields, name) {
			continue
		}
		if _, seen := values[name]; seen {
			continue
		}
		values[name] = strings.TrimSpace(value)
	}
	if values["model name"] == "" && values["CPU part"] == "" {
		return "", errors.New("/proc/cpuinfo names no CPU model (`model name` or `CPU part`) for its first processor")
	}
	if values["flags"] == "" && values["Features"] == "" {
		return "", errors.New("/proc/cpuinfo names no instruction-set flags (`flags` or `Features`) for its first processor")
	}
	var tb textbuf.Buffer
	for _, name := range cpuinfoIdentityFields {
		value, present := values[name]
		if !present {
			continue
		}
		tb.Str(name).Byte('=').Str(value).Byte('\n')
	}
	return tb.String(), nil
}
