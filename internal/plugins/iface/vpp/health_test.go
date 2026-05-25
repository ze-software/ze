// Design: plan/learned/786-backend-command-dispatch.md -- VPP health check tests

package ifacevpp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/health"
)

func TestVPPHealthCheckSocketMissing(t *testing.T) {
	status, _ := checkVPPHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when VPP socket does not exist")
}
