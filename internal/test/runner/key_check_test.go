// Design: docs/architecture/testing/ci-format.md -- decoded tree key checks

package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// A key expectation must reject a bad frame even when the searched text remains
// intact, and must not mistake netcapstring padding for the stored value.
func TestKeyCheckVerifiesFrameBeforeContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "database"), 0o700); err != nil {
		t.Fatal(err)
	}
	frame, err := zefs.EncodeNetcapstring([]byte("hello"), 64)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "database", "value")
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatal(err)
	}
	check := fileCheck{Key: true, Path: "value", Contains: "hello", NotContains: "   "}
	if err := validateOneFileCheck(dir, check); err != nil {
		t.Fatalf("decoded value: %v", err)
	}
	frame = append(frame, '\n')
	if err := os.WriteFile(path, frame, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateOneFileCheck(dir, check); err == nil {
		t.Fatal("trailing frame bytes were accepted")
	}
	check = fileCheck{Key: true, Path: "value", Absent: true}
	if err := validateOneFileCheck(dir, check); err == nil {
		t.Fatal("corrupt key satisfied absent")
	}
}
