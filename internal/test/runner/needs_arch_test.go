package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNeedsArchSkipReason pins the host-arch gate as a pure decision.
//
// VALIDATES: option=needs-arch runs a test only on a host whose CPU arch is in
// the list, and otherwise answers a skip reason naming the arch the test needs
// and the arch the host is (owner decision 2026-10-13: a target arch is tested
// only on a CPU of that arch).
// PREVENTS: an arm64 build test failing on an amd64 host for want of CPU
// emulation, and a typo in the list being read as "runs everywhere".
func TestNeedsArchSkipReason(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		host       string
		wantReason string
		wantErr    bool
	}{
		{name: "host-matches", value: "arm64", host: "arm64"},
		{name: "host-in-list", value: "amd64, arm64", host: "arm64"},
		{
			name: "host-differs", value: "arm64", host: "amd64",
			wantReason: "needs-arch=arm64, host GOARCH=amd64: a target arch is tested only on a CPU of that arch",
		},
		{
			name: "host-outside-list", value: "arm64", host: "riscv64",
			wantReason: "needs-arch=arm64, host GOARCH=riscv64: a target arch is tested only on a CPU of that arch",
		},
		{name: "empty-value", value: "", host: "amd64", wantErr: true},
		{name: "empty-token", value: "arm64,", host: "arm64", wantErr: true},
		{name: "kernel-spelling", value: "aarch64", host: "arm64", wantErr: true},
		{name: "typo", value: "amd46", host: "amd64", wantErr: true},
		{name: "kernel-spelling-x86", value: "x86_64", host: "amd64", wantErr: true},
		{name: "typo-in-list", value: "arm64,amd46", host: "arm64", wantErr: true},
		{name: "uppercase", value: "ARM64", host: "arm64", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, err := needsArchSkipReason(tt.value, tt.host)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "needs-arch")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

// TestParseCIOptionNeedsArch drives the option through the .ci parser.
//
// VALIDATES: a .ci naming an arch other than the host's is recorded with a
// SkipReason naming both arches, one naming the host's arch is not, and a
// directive without value= or with an unknown key fails the parse.
// PREVENTS: the option being parsed but never reaching the Record the runner
// reads its skip from.
func TestParseCIOptionNeedsArch(t *testing.T) {
	other := "arm64"
	if runtime.GOARCH == other {
		other = "amd64"
	}
	tests := []struct {
		name     string
		option   string
		wantSkip bool
		wantErr  bool
	}{
		{name: "host-arch", option: "option=needs-arch:value=" + runtime.GOARCH},
		{name: "other-arch", option: "option=needs-arch:value=" + other, wantSkip: true},
		{name: "missing-value", option: "option=needs-arch", wantErr: true},
		{name: "unknown-key", option: "option=needs-arch:value=" + runtime.GOARCH + ":arch=x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetNickCounter()
			tmpDir := t.TempDir()
			ciFile := filepath.Join(tmpDir, "test.ci")
			confFile := filepath.Join(tmpDir, "test.conf")
			ciContent := "option=file:path=test.conf\n" + tt.option + "\n"
			require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
			require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

			et := NewEncodingTests(tmpDir)
			_, err := et.parseAndAdd(ciFile)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "needs-arch")
				return
			}
			require.NoError(t, err)
			rec := et.GetByNick("1")
			require.NotNil(t, rec)
			if !tt.wantSkip {
				assert.Empty(t, rec.SkipReason)
				return
			}
			assert.Contains(t, rec.SkipReason, "needs-arch="+other)
			assert.Contains(t, rec.SkipReason, "host GOARCH="+runtime.GOARCH)
		})
	}
}
