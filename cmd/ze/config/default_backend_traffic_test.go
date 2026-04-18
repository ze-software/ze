// Design: docs/architecture/core-design.md -- ze:backend commit-time feature gate
// Related: default_backend_traffic_linux.go, default_backend_traffic_other.go -- tested constants

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// VALIDATES: The CLI-side trafficDefaultBackend() agrees with the runtime
//
//	traffic.DefaultBackendName() on every build target, so
//	`ze config validate` and the daemon diagnose the same
//	rejection on a config that omits the `traffic-control
//	backend` leaf.
//
// PREVENTS: silent cross-platform drift when someone changes one side
//
//	(e.g. adds an "auto" default on Linux) without updating the
//	other. This test fires at compile+test time for whichever
//	GOOS the developer is running on.
func TestTrafficDefaultBackendMatchesRuntime(t *testing.T) {
	assert.Equal(t, traffic.DefaultBackendName(), trafficDefaultBackend(),
		"cmd/ze/config trafficDefaultBackend() MUST match internal/component/traffic.DefaultBackendName() (see default_backend_traffic_{linux,other}.go)")
}
