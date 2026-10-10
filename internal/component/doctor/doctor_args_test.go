package doctor

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = old
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

// TestDoctorArgsRefused proves the config file is only ever the value of the
// config keyword (ai/rules/cli.md, keyword before value).
// VALIDATES: a bare path, a config keyword with no file, a config keyword
// followed by an option, a second config keyword, and a config file beside
// kernel-capabilities each exit 1 with a stderr line that names the fix, and
// no readiness check runs (stdout carries no JSON verdict).
// PREVENTS: `ze doctor router.conf` silently checking a file through the
// removed positional slot, or a dash-leading token read as a file name.
func TestDoctorArgsRefused(t *testing.T) {
	cfgPath := writeTestConfig(t, minimalConfig)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"bare path", []string{"--json", cfgPath}, "ze doctor config <file>"},
		{"config without a file", []string{"--json", "config"}, "config needs a file: ze doctor config <file>"},
		{"config followed by an option", []string{"config", "-x"}, "not the option -x"},
		{"config twice", []string{"config", cfgPath, "config", cfgPath}, "config is named once"},
		{"kernel-capabilities with a config", []string{"kernel-capabilities", "config", cfgPath}, "kernel-capabilities takes no config file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			var stdout string
			stderr := captureStderr(t, func() {
				stdout = captureStdout(t, func() { code = Run(tt.args) })
			})
			if code != 1 {
				t.Fatalf("Run(%q) = %d, want 1", tt.args, code)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("Run(%q) stderr %q does not name %q", tt.args, stderr, tt.want)
			}
			if strings.Contains(stdout, `"ready"`) {
				t.Fatalf("Run(%q) ran the checks after refusing: %s", tt.args, stdout)
			}
		})
	}
}
