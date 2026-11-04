package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmptySuccessfulStdlibListingRefused starts a real executable that exits
// zero without writing stdout. Neither listing nor comparing two such roots
// may certify an empty standard library. No Go compiler or copied tree runs.
func TestEmptySuccessfulStdlibListingRefused(t *testing.T) {
	t.Parallel()
	program, err := exec.LookPath("true")
	if err != nil {
		t.Fatalf("locate the POSIX empty-success executable: %v", err)
	}
	program, err = filepath.Abs(program)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	goroot := filepath.Join(root, "empty toolchain")
	if err := os.MkdirAll(filepath.Join(goroot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(program, filepath.Join(goroot, "bin", "go")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		run  func() error
	}{
		{"listing", func() error {
			_, err := listStdlib(goroot, root)
			return err
		}},
		{"comparison", func() error {
			return sameStdlib(Toolchain{GOROOT: goroot}, goroot, root)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if err == nil {
				t.Fatal("a successful subprocess with no packages certified an empty standard library")
			}
			for _, want := range []string{"go list std cmd", goroot, "no packages"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}
