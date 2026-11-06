package rfc

import (
	"slices"
	"testing"

	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// VALIDATES: a guest .ci observation boots Ze's own system on the named runtime kernel,
// with the harness `test qemu all` needs, and runs the ONE .ci against this run's own
// cross-built daemons.
// METHOD: guestCIArgv over a fixture runner, reading the whole argv.
// PREVENTS: `le rfc record` stopping before any suite on a needs-linux .ci, because the
// guest action refuses any guest whose PID 1 is not /gokrazy/init.
func TestGuestCIObservationRunsInZeSystem(t *testing.T) {
	runner := &observationRunner{
		tree: t.TempDir(), carrier: Carrier{Kind: kindFunctional, Name: "functional-plugin"},
		tag: Tag{File: "test/plugin/gated.ci"}, self: "/bin/le",
		kernel: "tmp/kernel/build/vmlinuz", guest: true,
	}
	binaries := "tmp/qemu/rfc-discriminate/pid-1/linux-amd64/bin"
	got := runner.guestCIArgv(binaries, testqemu.ArchAMD64)
	want := []string{"/bin/le", "test", "qemu", "docker-lab", "kernel", "tmp/kernel/build/vmlinuz",
		testqemu.KeywordHarness, "timeout", guestCICommandTimeout,
		"command", testqemu.OneCICommand(binaries, "test/plugin/gated.ci", testqemu.ArchAMD64)}
	if !slices.Equal(got, want) {
		t.Errorf("guest .ci argv\n got %q\nwant %q", got, want)
	}
}
