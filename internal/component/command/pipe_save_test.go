package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveWritesTheFormattedAnswer proves `| save` writes what the operator is
// looking at, not the dispatcher's JSON. The configured default format is
// appended to the END of a chain that names none, so a save applied where it
// sits in the chain would write something the terminal never showed.
func TestSaveWritesTheFormattedAnswer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "answer.json")

	_, format, errMsg := processPipesChecked("show test | json compact | save " + path)
	if errMsg != "" {
		t.Fatalf("the chain was refused: %s", errMsg)
	}
	shown := format(`{"peers":[{"address":"192.0.2.1"}]}`)

	saved, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("save wrote no file: %v", err)
	}
	if string(saved) != shown {
		t.Errorf("the file holds %q, the terminal showed %q", saved, shown)
	}
}

// TestSaveIsRefusedWhenTheDaemonExpandsTheChain is the security boundary. The
// daemon expands the chain for whoever connected, so a save honored there would
// write on the daemon's filesystem, with the daemon's privileges, at a path the
// remote caller chose.
func TestSaveIsRefusedWhenTheDaemonExpandsTheChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "written-by-the-daemon")

	_, format, errMsg := ProcessPipesDefaultFormatChecked("show test | save "+path, "json")
	if errMsg == "" {
		t.Fatal("the remote form accepted | save; it must refuse it")
	}
	if !strings.HasPrefix(errMsg, "save ") {
		t.Errorf("refusal %q does not name the operator", errMsg)
	}
	if format != nil {
		t.Error("a refused chain returned a formatter")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the refused chain wrote %s anyway", path)
	}

	// The same chain is accepted where the operator's own process expands it.
	_, format, errMsg = ProcessPipesDefaultFormatLocal("show test | save "+path, "json")
	if errMsg != "" {
		t.Fatalf("the local form refused | save: %s", errMsg)
	}
	if format == nil {
		t.Fatal("the local form returned no formatter")
	}
}

// TestSaveRequiresAPath refuses the argumentless form by name rather than
// writing to something surprising.
func TestSaveRequiresAPath(t *testing.T) {
	if msg := ValidatePipes([]pipeOp{{kind: pipeSave}}); msg == "" {
		t.Fatal("save with no path was accepted")
	}
}

// TestSaveLeavesTheDestinationAloneWhenItCannotWrite proves the write is atomic.
// The usual failure is a path the operator cannot write, and the usual
// destination is a file they still want.
func TestSaveLeavesTheDestinationAloneWhenItCannotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent-directory", "answer.json")

	_, format, errMsg := processPipesChecked("show test | json compact | save " + path)
	if errMsg != "" {
		t.Fatalf("the chain was refused at validation: %s", errMsg)
	}
	answer := format(`{"peers":[{"address":"192.0.2.1"}]}`)
	if !IsPipeError(answer) {
		t.Fatalf("saving to an unwritable path answered %q instead of refusing", answer)
	}
	if !strings.Contains(answer, path) {
		t.Errorf("the refusal does not say which path failed: %q", answer)
	}

	// No temporary file survives the failure.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".ze-save-") {
			t.Errorf("a partial file survived the failure: %s", e.Name())
		}
	}
}

// TestSaveFileIsOwnerOnly keeps an answer that can carry peer addresses, keys
// and topology from being readable by every account on the box.
func TestSaveFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "answer.json")

	_, format, errMsg := processPipesChecked("show test | json compact | save " + path)
	if errMsg != "" {
		t.Fatalf("the chain was refused: %s", errMsg)
	}
	format(`{"peers":[]}`)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("save wrote no file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != saveFileMode {
		t.Errorf("the saved answer is mode %04o, want %04o", mode, saveFileMode)
	}
}

// TestStreamSaveWriteFailureRemovesEveryTemp closes one staged file to force
// the callback write path to fail after all destinations were opened.
func TestStreamSaveWriteFailureRemovesEveryTemp(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.out")
	secondPath := filepath.Join(dir, "second.out")
	if err := os.WriteFile(firstPath, []byte("previous"), saveFileMode); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	saves, errMsg := openStreamSaves([]string{firstPath, secondPath})
	if errMsg != "" {
		t.Fatalf("open stream saves: %s", errMsg)
	}
	if err := saves.files[0].file.Close(); err != nil {
		t.Fatalf("close temporary file: %v", err)
	}
	if err := saves.WriteString("event\n"); err == nil {
		t.Fatal("write to a closed temporary file succeeded")
	}

	saved, err := os.ReadFile(firstPath) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(saved) != "previous" {
		t.Errorf("failed write replaced destination with %q", saved)
	}
	if _, err := os.Stat(secondPath); !os.IsNotExist(err) {
		t.Errorf("failed write created second destination: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ze-save-") {
			t.Errorf("failed write left temporary file %s", entry.Name())
		}
	}
}
