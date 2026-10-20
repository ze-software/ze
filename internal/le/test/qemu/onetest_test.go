package testqemu

import "testing"

// VALIDATES: the guest command for one .ci points the harness at the cross-built daemons
// and narrows `all-tests` to that one file, through paths the guest resolves from the
// checkout it mounts.
// METHOD: OneCICommand over a fixture directory and file.
// PREVENTS: a guest run of the host-built ze (no break, or the wrong architecture) and a
// guest run of a whole suite where one .ci was asked for.
func TestOneCICommand(t *testing.T) {
	got := OneCICommand("tmp/qemu/x/linux-amd64/bin", "test/plugin/gated.ci", ArchAMD64)
	want := "ZE_BIN=tmp/qemu/x/linux-amd64/bin/ze ZE_STRIPPED_BIN=tmp/qemu/x/linux-amd64/bin/ze-stripped" +
		" tmp/qemu/linux-amd64/le test qemu all-tests test test/plugin/gated.ci"
	if got != want {
		t.Errorf("OneCICommand\n got %q\nwant %q", got, want)
	}
}
