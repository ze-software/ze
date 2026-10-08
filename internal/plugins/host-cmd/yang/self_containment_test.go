package yang

import (
	"strings"
	"testing"
)

func TestHostCmdSchemaOwnsShowHost(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-host:show-cpu"`,
		`ze:command "ze-host:show-nic"`,
		`ze:command "ze-host:show-dmi"`,
		`ze:command "ze-host:show-memory"`,
		`ze:command "ze-host:show-thermal"`,
		`ze:command "ze-host:show-storage"`,
		`ze:command "ze-host:show-kernel"`,
		`ze:command "ze-host:show-platform"`,
		`ze:command "ze-host:show-all"`,
		"container host",
		"container cpu",
		"container nic",
		"container dmi",
		"container thermal",
		"container storage",
		"container all",
	} {
		if !strings.Contains(ZeHostCmdYANG, want) {
			t.Errorf("ze-host-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}

// TestHostCmdSchemaDeclaresBareShowHost pins the bare `show host` path as an
// alias of `show host all`.
//
// VALIDATES: `container host` carries ze:command, so a wire method serves the
// bare path and the daemon answers it.
// REGRESSION: the container declared no ze:command, so nothing was keyed on
// `show host`. The offline fallback in internal/plugins/host/register.go made
// the path dispatchable, which took it past the grouping-container branch in
// cmdutil.RunCommand and sent it to a daemon with no handler for it. The
// command therefore succeeded with the daemon DOWN and answered `unknown
// command` with it UP (test/ui/cli-verb-daemon-dispatch.ci, checks 5 and 7).
func TestHostCmdSchemaDeclaresBareShowHost(t *testing.T) {
	const decl = `ze:command "ze-host:show-all"`
	if got := strings.Count(ZeHostCmdYANG, decl); got != 2 {
		t.Errorf("ze-host-cmd.yang has %d %s declarations, want 2 (container host and container all)", got, decl)
	}
}

func TestHostCmdSchemaOwnsSystemKernelLog(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-host:show-system-kernel-log"`,
		"container kernel-log",
	} {
		if !strings.Contains(ZeHostCmdYANG, want) {
			t.Errorf("ze-host-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}

func TestHostSetCmdSchemaOwnsSetFileDescriptors(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-host:set-system-file-descriptors"`,
		"container file-descriptors",
	} {
		if !strings.Contains(ZeHostSetCmdYANG, want) {
			t.Errorf("ze-host-set-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}
