// Design: (none -- test utility, no architecture doc)

package golden

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// TestGoldenOutputCapturesCandidateBytes writes through the registered flags
// and Compare, then checks that the candidate keeps the full fixture path and
// exact bytes while the original fixture remains unchanged.
func TestGoldenOutputCapturesCandidateBytes(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "testdata", "golden", "component", "panel.html")
	original := []byte("original fixture\n")
	writeGoldenOutputFile(t, fixture, original)

	output := t.TempDir()
	setGoldenOutputFlags(t, "true", output)
	captured := []byte("candidate\x00bytes\r\n")

	Compare(t, fixture, captured)

	assertGoldenOutputBytes(t, filepath.Join(output, fixture), captured)
	assertGoldenOutputBytes(t, fixture, original)
}

// TestGoldenOutputDoesNotChangeComparison gives the candidate different bytes
// and compares against the original fixture with capture disabled. The output
// option must neither select the candidate as the oracle nor rewrite it.
func TestGoldenOutputDoesNotChangeComparison(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "testdata", "handler", "panel.txt")
	original := []byte("original response\n")
	writeGoldenOutputFile(t, fixture, original)

	output := t.TempDir()
	candidate := filepath.Join(output, fixture)
	captured := []byte("unapproved candidate\n")
	writeGoldenOutputFile(t, candidate, captured)
	setGoldenOutputFlags(t, "false", output)

	Compare(t, fixture, original)

	assertGoldenOutputBytes(t, fixture, original)
	assertGoldenOutputBytes(t, candidate, captured)
}

// setGoldenOutputFlags uses the same registry a capture invocation parses.
// These tests MUST run serially because the flags belong to the test process.
func setGoldenOutputFlags(t *testing.T, capture, output string) {
	t.Helper()

	for _, setting := range []struct {
		name  string
		value string
	}{
		{name: "update-golden", value: capture},
		{name: "golden-output", value: output},
	} {
		entry := flag.Lookup(setting.name)
		if entry == nil {
			t.Fatalf("capture flag %q is not registered", setting.name)
		}

		previous := entry.Value.String()
		t.Cleanup(func() {
			if err := flag.Set(setting.name, previous); err != nil {
				t.Errorf("restore capture flag %s: %v", setting.name, err)
			}
		})

		if err := flag.Set(setting.name, setting.value); err != nil {
			t.Fatalf("set capture flag %s: %v", setting.name, err)
		}
	}
}

func writeGoldenOutputFile(t *testing.T, file string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func assertGoldenOutputBytes(t *testing.T, file string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read fixture %s: %v", file, err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("fixture %s = %q, want %q", file, got, want)
	}
}
