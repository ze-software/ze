package workflowcheck

import (
	"strings"
	"testing"

	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// TestQEMUNightlyBuildsEveryGuestDaemon pins the needs-linux job to the one
// list of daemons a guest run checks for.
// VALIDATES: the job cross-builds through `le test qemu guest-binaries`
// (testfunctional.PrepareGuest) and hands `test qemu all` the environment that
// producer's directory answers, so ZE_STRIPPED_FLOW_BIN and every later
// daemon reach the guest without a workflow edit.
// PREVENTS: the hand-written `go build` pair that left ze-stripped-flow out,
// so every nightly needs-linux run refused before its first test with
// `bin/ze-stripped-flow is missing or not executable`.
// METHOD: read the needs-linux job of qemu-nightly.yml with comments stripped.
func TestQEMUNightlyBuildsEveryGuestDaemon(t *testing.T) {
	const name = "qemu-nightly.yml"
	var body string
	for _, job := range jobBlocks(t, name) {
		if job.name == "needs-linux" {
			body = stripComments(job.body)
		}
	}
	if body == "" {
		t.Fatalf("%s has no needs-linux job", name)
	}
	if !strings.Contains(body, "run: ./le test qemu guest-binaries") {
		t.Errorf("%s needs-linux must build its guest daemons with `./le test qemu guest-binaries`", name)
	}
	actionExists(t, "test qemu/guest-binaries")
	want := testqemu.GuestBinaryEnvironment(testqemu.GuestBinariesRel(testqemu.ArchAMD64))
	if !strings.Contains(body, want) {
		t.Errorf("%s needs-linux must pass %q to `test qemu all`, the daemons guest-binaries builds", name, want)
	}
	if strings.Contains(body, "go build") {
		t.Errorf("%s needs-linux builds a daemon by hand; the guest set comes from `./le test qemu guest-binaries`", name)
	}
}
