// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/memlock.

package memlock

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeMemlockRlimitLow is raised when RLIMIT_MEMLOCK is smaller than the ze
// executable and no capability lifts the limit.
const codeMemlockRlimitLow = "doctor-memlock-rlimit-low"

// codeMemlockRlimitUnknown is raised when the host could not be read, so the
// check has no verdict. It is a separate code because the operator acts
// differently: one says raise the limit, the other says the probe failed.
const codeMemlockRlimitUnknown = "doctor-memlock-rlimit-unknown"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeMemlockRlimitLow,
		Title:       "RLIMIT_MEMLOCK too small for the ze executable",
		Description: "RLIMIT_MEMLOCK is smaller than the ze executable, so the kernel refuses to lock the binary in memory and its pages can be evicted under memory pressure. A daemon that waits on the disk holding its own binary drops sessions at the moment it must keep them. Raise the limit: the generated ze.service unit does it with LimitMEMLOCK=infinity, and under Docker use --ulimit memlock=-1 or --cap-add IPC_LOCK. This check reads the host before ze locks anything; `show plugin list` reports what a running ze process achieved.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-memlock-rlimit-low"},
	},
	{
		Code:        codeMemlockRlimitUnknown,
		Title:       "RLIMIT_MEMLOCK could not be read",
		Description: "The pre-flight memory-lock check could not read RLIMIT_MEMLOCK, the size of /proc/self/exe, or the effective capability set in /proc/self/status, so it has no verdict on whether this host can lock the ze executable. The usual cause is a restricted or absent /proc. Mount /proc, or read the `show plugin list` setup outcome on a running ze process instead.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-memlock-rlimit-unknown"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: memlock: doctor code registration refused: " + err.Error())
		}
	}
}
