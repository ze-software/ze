package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// gnmiExposedConfig binds gNMI to every interface with no token: unauthenticated
// Get AND Set, which is what the daemon refuses to start on.
const gnmiExposedConfig = `environment {
	gnmi {
		enabled true
	}
}`

const gnmiLoopbackConfig = `environment {
	gnmi {
		enabled true
		server main {
			ip 127.0.0.1
			port 9339
		}
	}
}`

func validationCodes(t *testing.T, input string) []string {
	t.Helper()
	result := runValidation(input, "test.conf")
	codes := make([]string, 0, len(result.Diagnostics))
	for i := range result.Diagnostics {
		codes = append(codes, result.Diagnostics[i].Code)
	}
	return codes
}

// TestValidateFlagsGnmiExposure drives the gNMI semantic check from the
// `ze config validate` entry point, not from GNMIListenConfig.Validate.
//
// VALIDATES: spec-fixit-mgmt-listener-auth-guard AC-6 -- `ze config validate`
// and `ze doctor` agree with the boot guard about which configs expose gNMI.
// The two commands reach the check by different routes (doctor through
// config.ValidateSemantics, this one through its own inline block), so one
// working proves nothing about the other.
// PREVENTS: an operator validating a config clean and then finding the daemon
// refuses to start on it.
func TestValidateFlagsGnmiExposure(t *testing.T) {
	assert.Contains(t, validationCodes(t, gnmiExposedConfig), "config-gnmi-invalid",
		"a tokenless 0.0.0.0 gNMI listener must be reported")

	assert.NotContains(t, validationCodes(t, gnmiLoopbackConfig), "config-gnmi-invalid",
		"a loopback gNMI listener exposes nothing off-box")
}

// TestValidateGnmiExposureFailsValidation pins the exit-code consequence: the
// diagnostic is an error, so `ze config validate` reports the config invalid
// rather than mentioning the exposure and returning success.
func TestValidateGnmiExposureFailsValidation(t *testing.T) {
	assert.False(t, runValidation(gnmiExposedConfig, "test.conf").Valid,
		"an exposed gNMI listener must make the config invalid")
}
