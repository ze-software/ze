// Related: shimprogram.go -- shimRecorded and shimLineProgram, the reading these cases drive
// Related: exec.go -- Program, the post-edit lint's resolution

package leexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shimFor answers the body `le setup` writes for a shim starting program
// through le at lePath (setup.shimScript), quoting each path as it does.
func shimFor(lePath, program string) string {
	quote := func(path string) string { return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'" }
	return "#!/bin/sh\n" + ShimMarker + "\n" +
		"if [ ! -x " + quote(lePath) + " ]; then\n\texit 1\nfi\n" +
		"exec " + quote(lePath) + " exec " + quote(program) + " \"$@\"\n"
}

// TestProgramRunsTheToolTheShimRecords asks whether Program, the post-edit
// lint's resolution, answers the tool the first PATH entry's shim records,
// rather than the first real tool of that name further down PATH. Its
// method: a shim recording one binary, a second binary of the same name later
// on PATH, and the answer's path and argv; then a real tool first on PATH,
// and a shim whose line names no quoted program.
//
// VALIDATES: the post-edit lint and the golangci-lint shim run the same
// binary, a path holding a quote included.
// PREVENTS: the post-edit lint judging a file with another golangci-lint than
// the one every shimmed run uses.
func TestProgramRunsTheToolTheShimRecords(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	recorded := program(t, filepath.Join(t.TempDir(), "it's", "golangci-lint"), "#!/bin/sh\nexit 0\n")
	later := program(t, filepath.Join(t.TempDir(), "golangci-lint"), "#!/bin/sh\nexit 0\n")
	shims := t.TempDir()
	program(t, filepath.Join(shims, "golangci-lint"), shimFor("/checkout/le", recorded))
	oldShims := t.TempDir()
	program(t, filepath.Join(oldShims, "golangci-lint"), "#!/bin/sh\n"+ShimMarker+"\nexec le exec "+recorded+" \"$@\"\n")

	for _, row := range []struct {
		name string
		path []string
		want string
	}{
		{"shim first", []string{shims, filepath.Dir(later)}, recorded},
		{"real tool first", []string{filepath.Dir(later), shims}, later},
		{"unreadable shim line", []string{oldShims, filepath.Dir(later)}, later},
	} {
		t.Run(row.name, func(t *testing.T) {
			environ := []string{"PATH=" + strings.Join(row.path, string(filepath.ListSeparator)), "XDG_CACHE_HOME=" + os.Getenv("XDG_CACHE_HOME")}
			tool, err := Program(t.TempDir(), []string{"golangci-lint", "run"}, environ)
			if err != nil {
				t.Fatalf("Program refused: %v", err)
			}
			if tool.Path != row.want {
				t.Errorf("Program answered %q, want %q", tool.Path, row.want)
			}
			if len(tool.Argv) != 2 || tool.Argv[0] != "golangci-lint" || tool.Argv[1] != "run" {
				t.Errorf("argv = %q, want the name as typed and its arguments", tool.Argv)
			}
		})
	}
}
