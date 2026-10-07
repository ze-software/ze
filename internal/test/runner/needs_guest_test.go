package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// VALIDATES: NeedsGuest answers the runner's own skip decision for a
// capability-gated .ci: true exactly when this host would skip it, false for a
// test that declares no needs-linux gate.
// PREVENTS: `le feature record-run` observing a host skip of a test whose home
// is the QEMU guest, which records nothing and says nothing about why.
func TestNeedsGuestAnswersTheRunnersOwnSkip(t *testing.T) {
	dir := t.TempDir()
	gated := filepath.Join(dir, "gated.ci")
	plain := filepath.Join(dir, "plain.ci")
	body := "cmd=foreground:seq=1:exec=ze\nexpect=exit:code=0\n"
	if err := os.WriteFile(gated, []byte("option=needs-linux:caps=net-admin\n"+body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := NeedsGuest(gated)
	if err != nil {
		t.Fatalf("NeedsGuest(gated): %v", err)
	}
	// The host decides the expected answer the way the parser does: a non-Linux
	// host, or a Linux host without CAP_NET_ADMIN, skips the gated test.
	want := runtime.GOOS != goosLinux || !hasCaps([]string{capsNetAdmin})
	if got != want {
		t.Fatalf("NeedsGuest(gated) = %v, want %v on this host", got, want)
	}

	got, err = NeedsGuest(plain)
	if err != nil {
		t.Fatalf("NeedsGuest(plain): %v", err)
	}
	if got {
		t.Fatal("a test with no needs-linux gate was sent to the guest")
	}
}
