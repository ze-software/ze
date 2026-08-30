// Design: docs/features/ai-first.md -- plugin binary readiness check

package doctor

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
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
		fmt.Fprintf(os.Stderr, "doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
