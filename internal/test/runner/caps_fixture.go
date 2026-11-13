// Design: docs/functional-tests.md -- process control belongs to test fixtures.

package runner

import (
	"strings"

	"github.com/ze-software/ze/internal/core/privilege"
)

// FixtureProcessControlCapability is the test harness's authority to stop a
// privileged metadata child. It is deliberately outside DaemonCapabilities and
// the caps= vocabulary: the daemon under test MUST retain its production grant.
func FixtureProcessControlCapability() privilege.Capability {
	return privilege.Capability{Name: "CAP_KILL", Bit: 5, Use: "stop an owned privileged fixture child"}
}

// NetnsFixtureSetcap answers the grant for the guest's private le copy.
func NetnsFixtureSetcap() (string, error) {
	daemon, err := NetnsDaemonSetcap()
	if err != nil {
		return "", err
	}
	// NetnsDaemonSetcap owns the complete +ep spelling. Preserve every
	// production capability, then add the controller's separately owned bits.
	control := FixtureProcessControlCapability()
	inspection := FixtureProcessInspectionCapability()
	return strings.TrimSuffix(daemon, "+ep") + "," + strings.ToLower(control.Name) + "," + strings.ToLower(inspection.Name) + "+ep", nil
}
