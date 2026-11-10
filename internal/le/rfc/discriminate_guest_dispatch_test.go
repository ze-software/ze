// Design: docs/contributing/rfc-conformance-gates.md -- own-system tagged-unit transport.
// Related: ../test/qemu/actions.go -- the actual dispatcher accepting the unit command.
package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// TestGuestUnitTransportReachesOwnSystemDispatcher passes the generated unit
// command to the real QEMU dispatcher. A conflicting lab must reach the command
// mode guard before kernel/image work; the old Alpine route rejects its grammar
// instead. This proves dispatch, not successful guest execution or path mounting.
func TestGuestUnitTransportReachesOwnSystemDispatcher(t *testing.T) {
	root := t.TempDir()
	runner := observationRunner{tree: root, self: "/unused/le", kernel: filepath.Join(root, "absent-vmlinuz")}
	argv := runner.guestRunArgv(filepath.Join(root, "unit script.sh"))
	if len(argv) < 4 {
		t.Fatalf("unit transport has no QEMU action: %q", argv)
	}
	if argv[1] != "test" {
		t.Fatalf("unit transport lost test dispatch: %q", argv)
	}
	if argv[2] != "qemu" {
		t.Fatalf("unit transport lost QEMU dispatch: %q", argv)
	}
	args := slices.Concat(argv[3:], []string{"lab", "test integration"})
	stderr, err := os.CreateTemp(root, "dispatch-refusal-")
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = saved
		if err := stderr.Close(); err != nil {
			t.Error(err)
		}
	})
	answer, code := testqemu.Answer(args)
	output, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("dispatcher answered %d (%v), want own-system mode refusal: %s", code, answer, output)
	}
	if !strings.Contains(string(output), "docker-lab command and lab are mutually exclusive") {
		t.Fatalf("generated unit transport reached another refusal: %s", output)
	}
}
