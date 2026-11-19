package fixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the files of a failed simulated-network run are written beside
// ze's working directory, never inside it, and are still there once that
// directory is removed, which is what the runner does when the test ends.
// PREVENTS: a failure line that names files nobody can read (2026-11-18).
// METHOD: keep the files, remove the working directory, read them back.
func TestLGSimulatedKeepSurvivesTheWorkDirectory(t *testing.T) {
	work := filepath.Join(t.TempDir(), "ze-work-1")
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	sentence := lgSimulatedKeep(work, map[string]string{
		lgSimulatedReasonFile: "the reason\n",
		lgSimulatedOutputFile: "what ze wrote\n",
	})
	kept := filepath.Join(filepath.Dir(work), lgSimulatedEvidencePrefix+"ze-work-1")
	if !strings.Contains(sentence, "kept in "+kept+" (") {
		t.Fatalf("the sentence does not name %s: %s", kept, sentence)
	}
	if err := os.RemoveAll(work); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		lgSimulatedReasonFile: "the reason\n",
		lgSimulatedOutputFile: "what ze wrote\n",
	} {
		got, err := os.ReadFile(filepath.Join(kept, name))
		if err != nil {
			t.Fatalf("%s after the working directory was removed: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s holds %q, want %q", name, got, want)
		}
	}
}

// VALIDATES: a directory that cannot be created is said, with its path, in
// place of the sentence that names the kept files.
// PREVENTS: a failure line pointing at evidence that was never written.
// METHOD: make the parent of the working directory a plain file.
func TestLGSimulatedKeepSaysWhenItCannotWrite(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sentence := lgSimulatedKeep(filepath.Join(parent, "ze-work-1"), map[string]string{lgSimulatedReasonFile: "x"})
	if !strings.HasPrefix(sentence, "the evidence could not be written to ") {
		t.Fatalf("the sentence claims the evidence was kept: %s", sentence)
	}
}

// VALIDATES: the tail of ze's output is its last lgSimulatedTailLines lines on
// one line, each cut to lgSimulatedTailWidth bytes, and an empty output is
// said to be empty.
// PREVENTS: a failure line the runner elides because it runs to pages.
// METHOD: three literal outputs.
func TestLGSimulatedTailIsBounded(t *testing.T) {
	if got := lgSimulatedTail(""); got != "(ze wrote nothing)" {
		t.Errorf("an empty output answers %q", got)
	}
	if got := lgSimulatedTail("one\ntwo\nthree\nfour\n"); got != "two | three | four" {
		t.Errorf("four lines answer %q, want the last three", got)
	}
	long := strings.Repeat("x", lgSimulatedTailWidth+5)
	if got := lgSimulatedTail(long + "\n"); got != long[:lgSimulatedTailWidth]+"..." {
		t.Errorf("a long line answers %d bytes, want %d and the mark", len(got), lgSimulatedTailWidth)
	}
}
