// Related: leroot.go -- the registration adapter these tests drive directly

package leroot

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// captureStderr runs fn with os.Stderr replaced by a pipe and answers what fn
// wrote. The refusal writes to os.Stderr by name, which is what keeps it inside
// the one exemption the no-Sprintf check makes, so reading it back means moving
// the file rather than injecting a writer.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("open a pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = write
	t.Cleanup(func() { os.Stderr = saved })

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := read.Read(buf)
			sb.Write(buf[:n]) //nolint:errcheck // strings.Builder never fails
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	write.Close() //nolint:errcheck // the reader answers what it has
	captured := <-done
	read.Close() //nolint:errcheck // read to EOF
	return captured
}

// VALIDATES: Run expands `| save` in the caller's local process, writes the
// exact formatted structured answer, and leaves the same answer on stdout.
// PREVENTS: selecting the remote pipe processor, which refuses save before the
// le command can run.
func TestRunAllowsLocalSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "answer.json")
	var stdout, stderr bytes.Buffer

	code := Run("example", func(command.ValidatedArgs) (any, int) {
		return struct {
			Answer string `json:"answer"`
		}{Answer: "saved locally"}, 0
	}, []string{"|", "save", path, "|", "json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run answered %d, want 0: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run wrote stderr on success: %q", stderr.String())
	}

	saved, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("read saved answer: %v", err)
	}
	wantSaved := "{\n  \"answer\": \"saved locally\"\n}"
	if string(saved) != wantSaved {
		t.Errorf("saved answer = %q, want %q", saved, wantSaved)
	}
	if wantStdout := wantSaved + "\n"; stdout.String() != wantStdout {
		t.Errorf("stdout = %q, want %q", stdout.String(), wantStdout)
	}

	// A local save still fails closed through the shared pipe API when the
	// caller names a destination whose parent does not exist.
	invalidPath := filepath.Join(t.TempDir(), "missing", "answer.json")
	stdout.Reset()
	stderr.Reset()
	code = Run("example", func(command.ValidatedArgs) (any, int) {
		return struct {
			Answer string `json:"answer"`
		}{Answer: "not saved"}, 0
	}, []string{"|", "save", invalidPath, "|", "json"}, &stdout, &stderr)

	// A destination whose parent does not exist is a write the environment
	// refused, not a malformed chain: le could not deliver its answer, 3
	// (D-P5-save). The pipe layer reports it through the post-run refusal path.
	if code != 3 {
		t.Errorf("Run with invalid destination answered %d, want 3", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("Run wrote stdout after save failure: %q", stdout.String())
	}
	wantErrorPrefix := "pipe error: save could not write " + invalidPath + ": "
	if !strings.HasPrefix(stderr.String(), wantErrorPrefix) {
		t.Errorf("save failure = %q, want prefix %q", stderr.String(), wantErrorPrefix)
	}
}

// TestAnAreaWithoutATableStopsLeNamingIt proves AC-16 (D-P3-tablecheck): an
// area that registered no action table stops le before its first answer, and
// the stop names the area and the call that fixes it. An area holding a table
// passes the same check.
// VALIDATES: requireEveryAreaHasATable, run once by Dispatch and Tree.
// PREVENTS: an area publishing no grammar, which the manifest, help and the
// dispatch guard would each answer for from nothing.
func TestAnAreaWithoutATableStopsLeNamingIt(t *testing.T) {
	tabled := "zz-tablecheck-tabled"
	setActions(tabled, func() leaction.List {
		return leaction.New(tabled, leaction.Action{Verb: "run", Why: "w", Answer: func() (any, int) { return nil, 0 }}).Actions()
	})
	requireEveryAreaHasATable([]string{tabled})

	missing := "zz-tablecheck-missing"
	defer func() {
		stop, _ := recover().(string)
		if !strings.Contains(stop, `"`+missing+`"`) {
			t.Fatalf("the stop does not name the area %q: %q", missing, stop)
		}
		if !strings.Contains(stop, `leroot.RegisterActions("`+missing+`", Actions)`) {
			t.Fatalf("the stop does not name the call that fixes it: %q", stop)
		}
	}()
	requireEveryAreaHasATable([]string{tabled, missing})
	t.Fatal("an area with no action table passed the check")
}
