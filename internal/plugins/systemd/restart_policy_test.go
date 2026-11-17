// Design: docs/guide/ze-install.md -- intentional restart supervisor policy.
package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestUnitFileIntentionalRestartPolicy reads the generated service assignments
// as sections and keys. Intentional exit 75 must be successful and forced to
// restart; ordinary exit 0 stays stopped and genuine failures retain recovery.
func TestUnitFileIntentionalRestartPolicy(t *testing.T) {
	unit := buildUnitFile(unitSpec{BinaryPath: "/usr/local/bin/ze", ConfigDir: "/etc/ze"})
	sections := make(map[string]map[string]string)
	section := ""
	for line := range strings.SplitSeq(unit, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if name, found := strings.CutPrefix(line, "["); found {
			section = strings.TrimSuffix(name, "]")
			if sections[section] == nil {
				sections[section] = make(map[string]string)
			}
			continue
		}
		key, value, assigned := strings.Cut(line, "=")
		require.True(t, assigned, "generated unit contains a non-assignment: %q", line)
		require.NotEmpty(t, section, "generated unit contains an assignment outside a section")
		sections[section][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	service := sections["Service"]
	require.NotNil(t, service)
	assert.Equal(t, "on-failure", service["Restart"])
	assert.Equal(t, []string{"75"}, strings.Fields(service["SuccessExitStatus"]))
	assert.Equal(t, []string{"75"}, strings.Fields(service["RestartForceExitStatus"]))
}

// TestDoctorInstalledRestartPolicy drives the registered installed-service
// check with effective manager properties, including an older fragment and a
// drop-in that removes either required setting. The file itself is deliberately
// insufficient: doctor must diagnose the policy systemd actually applies.
func TestDoctorInstalledRestartPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		properties string
		warning    bool
	}{
		{"required policy", "Restart=on-failure\nSuccessExitStatus=75\nRestartForceExitStatus=75\n", false},
		{"older unit", "Restart=on-failure\nSuccessExitStatus=\nRestartForceExitStatus=\n", true},
		{"drop-in clears force", "Restart=on-failure\nSuccessExitStatus=75\nRestartForceExitStatus=\n", true},
		{"drop-in clears success", "Restart=on-failure\nSuccessExitStatus=\nRestartForceExitStatus=75\n", true},
		{"ordinary stop forced", "Restart=on-failure\nSuccessExitStatus=75\nRestartForceExitStatus=0 75\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "ze")
			require.NoError(t, os.WriteFile(binary, []byte("test executable"), 0o755))
			withServiceUnit(t, "[Service]\nExecStart="+binary+" start\n")
			withServiceStop(t, "TimeoutStopUSec=1min 30s\nKillMode=mixed\n"+tc.properties, nil)
			found := false
			for _, report := range checkServiceInstall(diagnostic.DoctorCheckContext{}) {
				if report.Code != "doctor-service-restart" {
					continue
				}
				found = true
				assert.Equal(t, diagnostic.SeverityWarning, report.Severity)
				assert.Contains(t, report.Message, "ze install systemd --force")
				assert.NotEmpty(t, report.Fix)
			}
			assert.Equal(t, tc.warning, found, "installed restart policy diagnostic")
		})
	}
}
