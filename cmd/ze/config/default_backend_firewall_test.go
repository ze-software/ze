// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: default_backend_firewall_linux.go, default_backend_firewall_other.go -- tested constants

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
)

// VALIDATES: The CLI-side firewallDefaultBackend() agrees with the
// runtime firewall.DefaultBackendName() on every build target, so
// `ze config validate` and the daemon diagnose the same rejection on
// a config that omits the `firewall.backend` leaf.
//
// PREVENTS: silent cross-platform drift when someone changes one side
// (e.g. flips the Linux default to a future "auto" backend) without
// updating the other. The test fires on whichever GOOS runs the suite.
func TestFirewallDefaultBackendMatchesRuntime(t *testing.T) {
	assert.Equal(t, firewall.DefaultBackendName(), firewallDefaultBackend(),
		"cmd/ze/config firewallDefaultBackend() MUST match internal/component/firewall.DefaultBackendName() (see default_backend_firewall_{linux,other}.go)")
}
