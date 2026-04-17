// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: default_backend_linux.go, default_backend_other.go -- tested constants

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// VALIDATES: The CLI-side ifaceDefaultBackend() agrees with the runtime
//
//	iface.DefaultBackendName() on every build target, so `ze
//	config validate` and the daemon diagnose the same rejection
//	on a config that omits the `interface backend` leaf.
//
// PREVENTS: silent cross-platform drift when someone changes one side
//
//	(e.g. adds an "auto" default on Linux) without updating the
//	other. This test fires at compile+test time for whichever
//	GOOS the developer is running on.
func TestIfaceDefaultBackendMatchesRuntime(t *testing.T) {
	assert.Equal(t, iface.DefaultBackendName(), ifaceDefaultBackend(),
		"cmd/ze/config ifaceDefaultBackend() MUST match internal/component/iface.DefaultBackendName() (see default_backend_{linux,other}.go)")
}
