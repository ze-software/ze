// Design: docs/architecture/testing/qemu-integration.md -- a record run boots the cached kernel and image and never builds one
// Related: discriminate_guest.go -- observationRunner.requireCachedGuest

package rfc

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// TestRecordRefusesAColdGuestBeforeObserving checks the record preflight over
// this checkout's real kernel key and an empty cache: a guest unit is refused
// before any observation, the refusal names the runtime kernel cache entry it
// looked for, and its last line is the one command that builds the kernel and
// the image. Method: XDG_CACHE_HOME names an empty directory, so the real
// docker-lab lookup (testqemu.CheckDockerLabInputs) meets a cold cache.
func TestRecordRefusesAColdGuestBeforeObserving(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	runner := observationRunner{tree: root, guest: true,
		kernel: filepath.Join(t.TempDir(), "vmlinuz"), record: DiscriminationRecord{Unit: "TestGuestUnit"}}

	err = runner.requireCachedGuest()
	if err == nil {
		t.Fatal("a guest run over an empty cache passed the preflight")
	}
	if !isParseError(err) {
		t.Errorf("the refusal is not a record refusal (ParseError): %v", err)
	}
	message := err.Error()
	namespace := filepath.Join(cache, "ze", "runtime-kernel") + string(filepath.Separator)
	if !strings.Contains(message, namespace) {
		t.Errorf("refusal does not name the runtime kernel entry under %s:\n%s", namespace, message)
	}
	if !strings.Contains(message, "TestGuestUnit") {
		t.Errorf("refusal does not name the unit:\n%s", message)
	}
	lines := strings.Split(message, "\n")
	if want := "fix: ./le setup docker host build arch " + testqemu.GuestArch(); lines[len(lines)-1] != want {
		t.Errorf("refusal's last line = %q, want %q", lines[len(lines)-1], want)
	}
}

// TestRecordPreflightAsksOnlyAGuestRun checks that a host run boots nothing and
// so is never refused by the guest preflight, while a guest run carries the
// lookup's refusal. Method: the lookup seam answers a fixed refusal.
func TestRecordPreflightAsksOnlyAGuestRun(t *testing.T) {
	refusal := errors.New("cold cache fixture")
	asked := 0
	old := guestInputsCached
	guestInputsCached = func(string, string, string, string) error {
		asked++
		return refusal
	}
	t.Cleanup(func() { guestInputsCached = old })

	host := observationRunner{record: DiscriminationRecord{Unit: "TestHostUnit"}}
	if err := host.requireCachedGuest(); err != nil || asked != 0 {
		t.Errorf("host run: err=%v, lookups=%d, want no refusal and no lookup", err, asked)
	}
	guest := observationRunner{guest: true, kernel: "/cache/vmlinuz", record: DiscriminationRecord{Unit: "TestGuestUnit"}}
	if err := guest.requireCachedGuest(); err == nil || !strings.Contains(err.Error(), refusal.Error()) || asked != 1 {
		t.Errorf("guest run: err=%v, lookups=%d, want the lookup's refusal after one lookup", err, asked)
	}
}
