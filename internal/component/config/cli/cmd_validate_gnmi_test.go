package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
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

// gnmiTokenNoTLSConfig authenticates gNMI with a token on every interface but
// names no TLS: the daemon refuses to boot on it, because the token and the
// config would cross the network in clear text.
const gnmiTokenNoTLSConfig = `environment {
	gnmi {
		enabled true
		token s3cret-token
	}
}`

// gnmiTokenTLSConfig is the same listener with both TLS files named.
const gnmiTokenTLSConfig = `environment {
	gnmi {
		enabled true
		token s3cret-token
		tls {
			cert /etc/ze/gnmi.crt
			key /etc/ze/gnmi.key
		}
	}
}`

// TestValidateFlagsGnmiTokenWithoutTLS drives the TLS half of the gNMI rule
// from the `ze config validate` entry point.
//
// VALIDATES: `ze config validate` refuses a token-only non-loopback gNMI
// listener, as boot does, and names the TLS fix; the same listener with both
// TLS files passes.
// PREVENTS: a config that validates clean and exits 1 at boot.
func TestValidateFlagsGnmiTokenWithoutTLS(t *testing.T) {
	result := runValidation(gnmiTokenNoTLSConfig, "test.conf")
	assert.False(t, result.Valid, "a token-only 0.0.0.0 gNMI listener must make the config invalid")
	found := false
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "config-gnmi-invalid" && result.Diagnostics[i].Fix == config.GNMITLSFix {
			found = true
		}
	}
	assert.True(t, found, "the refusal must carry the TLS fix, got %+v", result.Diagnostics)

	assert.NotContains(t, validationCodes(t, gnmiTokenTLSConfig), "config-gnmi-invalid",
		"a token with both TLS files secures a 0.0.0.0 gNMI listener")
}

// TestValidateGnmiExposureFailsValidation pins the exit-code consequence: the
// diagnostic is an error, so `ze config validate` reports the config invalid
// rather than mentioning the exposure and returning success.
func TestValidateGnmiExposureFailsValidation(t *testing.T) {
	assert.False(t, runValidation(gnmiExposedConfig, "test.conf").Valid,
		"an exposed gNMI listener must make the config invalid")
}
