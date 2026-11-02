//go:build ze_core

package main

import (
	"io"
	"os"
	"testing"

	zeversion "github.com/ze-software/ze/internal/core/version"
)

// TestPrintVersionOneProducer proves AC-14 for show version on the no-daemon
// route: printVersion, which `ze show version`, `ze version` and
// `ze --version` reach, prints zeversion.Answer, the producer the daemon's
// show version RPC answers (TestShowVersionOneProducer in
// internal/component/cmd/show judges that side).
//
// Method: capture stdout around printVersion, short and extended, and compare
// it with the producer's line.
func TestPrintVersionOneProducer(t *testing.T) {
	release, built := zeversion.Release(), zeversion.BuildDate()
	t.Cleanup(func() { zeversion.Stamp(release, built) })
	zeversion.Stamp("one-producer-release", "2026-11-02T00:00:00Z")

	for _, extended := range []bool{false, true} {
		got := capturePrintVersion(t, extended)
		if want := zeversion.Answer(extended) + "\n"; got != want {
			t.Errorf("extended=%v: printed %q, producer %q", extended, got, want)
		}
	}
}

func capturePrintVersion(t *testing.T, extended bool) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	code := printVersion(extended)
	os.Stdout = saved
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("printVersion exit %d", code)
	}
	return string(out)
}
