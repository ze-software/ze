// Design: docs/architecture/plugin/plugin-system.md -- data-plane writer resolution
// Related: registry.go -- PluginForDataPlane, NeedsDataPlane, the declarations it reads
// Related: gated_plugins.go -- GatedDataPlaneWriter, the compiled-out writer
// Related: ../server/startup_autoload.go -- appendDataPlaneWriter, the boot and reload refusal
// Related: ../doctor/check_dataplane_writer.go -- the doctor check reporting the same refusal

package registry

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/opererr"
)

// DataPlaneWriter answers the plugin that programs dataPlane for producer, a
// selected plugin whose routes reach forwarding only through a data-plane
// writer (Registration.NeedsDataPlane), or the refusal: an
// UnloadedDependencyError naming the build tags when this build compiled the
// writer out, else an operator error naming the data plane and the fix.
//
// It is the one judgement both the boot and reload selection
// (appendDataPlaneWriter) and `ze doctor` (doctor-plugin-no-dataplane-writer)
// make, so the doctor row says what the daemon would refuse, in its words.
// The caller decides first that a writer is needed: a NeedsDataPlane producer
// is selected and no plugin that ProgramsFIB is chosen.
func DataPlaneWriter(producer, dataPlane string) (string, error) {
	if writer, ok := PluginForDataPlane(dataPlane); ok {
		return writer, nil
	}
	if gatedWriter, gated := GatedDataPlaneWriter(dataPlane); gated {
		return "", fmt.Errorf("data plane %q: %w", dataPlane, UnloadedDependencyError(producer, gatedWriter))
	}
	return "", opererr.New(
		"no forwarding plugin programs data plane \""+dataPlane+"\"",
		"the requested routes cannot reach forwarding; startup or reload is refused before their plugins start",
		"remove the configuration that loads \""+producer+"\", or select an interface backend with a forwarding plugin")
}
