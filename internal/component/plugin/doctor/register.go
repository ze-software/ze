// Design: docs/features/ai-first.md -- plugin binary readiness check
// Related: check_managed_listener.go -- the managed listener collision check registered here

package doctor

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codePluginMissing is the diagnostic code this check publishes and raises.
const codePluginMissing = "doctor-plugin-missing"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "plugin-binaries",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        700,
		Component:    "plugin",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codePluginMissing},
		Check:        checkPlugins,
	}); err != nil {
		exitOnRegistrationFailure(err)
	}

	// The managed listener check reads the config tree. It runs after the tree
	// is parsed and it declares that dependency. It sits beside the binary
	// check because the plugin component owns both producers it compares. Those
	// are the acceptor's address, and the hub server block list the managed
	// listener picks its own addresses from.
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "hub-managed-listener",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        701,
		Component:    "plugin",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeHubManagedCollision},
		Check:        checkManagedListener,
	}); err != nil {
		exitOnRegistrationFailure(err)
	}
}

// exitOnRegistrationFailure ends the process on a refused doctor check.
// Registration runs in init(). A refusal therefore means this binary would run
// without a check it claims to carry. A missing check nobody is told about
// reads to the operator as a clean report.
func exitOnRegistrationFailure(err error) {
	var report textbuf.Buffer
	report.Str("doctor check registration: ").Err(err).Byte('\n')
	report.StdErr() //nolint:errcheck // the process is exiting on the next line
	os.Exit(2)
}
