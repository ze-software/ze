package dockerhost

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRefuseRestartFollowsTheBootMarker proves the guest program refuses a
// run after the boot sequence ran, and boots on the first run. Method: ask
// refuseRestart about a marker path in a temporary directory, absent, then
// present, then behind a regular file so it cannot be read.
//
// VALIDATES: review findings R1-7 and R2-2: a second run of ze-docker-host in
// one boot exits rather than starting a second sshd and dockerd, and its
// message names both reboots (Colima and the QEMU docker-lab).
// PREVENTS: an inverted or dropped marker check, which either serves twice or
// never boots.
func TestRefuseRestartFollowsTheBootMarker(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "booted")

	if err := refuseRestart(marker); err != nil {
		t.Fatalf("refuseRestart with no marker = %v, want nil (first run of this boot)", err)
	}

	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := refuseRestart(marker)
	if !errors.Is(err, errRestartRefused) {
		t.Fatalf("refuseRestart with the marker = %v, want the restart refusal", err)
	}
	for _, want := range []string{"colima restart ze", "./le test qemu docker-lab"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name the recovery %q", err, want)
		}
	}

	unreadable := filepath.Join(marker, "booted")
	err = refuseRestart(unreadable)
	if err == nil {
		t.Fatalf("refuseRestart(%s) behind a regular file = nil, want a refusal", unreadable)
	}
	if errors.Is(err, errRestartRefused) {
		t.Errorf("refuseRestart(%s) = %v, want the cannot-tell refusal, not the restart one", unreadable, err)
	}
}
