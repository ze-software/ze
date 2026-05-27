//go:build integration && linux

package doctor

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/component/host"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
)

func TestCheckMachineIDIntegration(t *testing.T) {
	data, err := os.ReadFile(machineIDPath) //nolint:gosec // fixed platform path
	diags := checkMachineID(&host.PlatformInfo{Type: host.PlatformGokrazy})

	if err != nil || strings.TrimSpace(string(data)) == "" {
		requireDiag(t, diags, "doctor-machine-id-missing", diagnostic.SeverityWarning)
		return
	}
	assert.Empty(t, diags)
}
