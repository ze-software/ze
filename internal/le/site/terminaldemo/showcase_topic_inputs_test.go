// Design: docs/contributing/gh-pages.md -- shared topic inputs stay inside the daemon configuration directory.
package siteterminaldemo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: one common preparation copies only the authored configuration
// sections, preserving topic names, into the directory remote loads may read.
// Source edits after preparation cannot change the staged section.
func TestShowcaseTopicInputsAreConfinedCopies(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZE_DEMO_ROOT", root)
	t.Setenv("ZE_DEMO_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "home"))
	configDir := scenarioConfigDir(showcaseLabID)
	previous := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", previous) })
	if err := env.Set("ze.config.dir", configDir); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(demoTree(), "topics", "first", "1-section.conf")
	second := filepath.Join(demoTree(), "topics", "second", "1-section.conf")
	mustWrite(t, first, "system { host edge-first; }\n")
	mustWrite(t, second, "system { host edge-second; }\n")
	mustWrite(t, filepath.Join(demoTree(), "topics", "first", "configure.tape"), "Type show\n")
	mustWrite(t, filepath.Join(demoTree(), "topics", "first", "notes.txt"), "not an input\n")
	if err := stageShowcaseTopicInputs(); err != nil {
		t.Fatalf("stage topic inputs: %v", err)
	}
	for _, source := range []string{first, second} {
		relative, err := filepath.Rel(demoTree(), source)
		if err != nil {
			t.Fatal(err)
		}
		staged := filepath.Join(configDir, relative)
		original, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(staged)
		if err != nil {
			t.Fatalf("read staged %s: %v", relative, err)
		}
		if !bytes.Equal(copied, original) {
			t.Errorf("staged %s = %q, want %q", relative, copied, original)
		}
		if _, err := command.CallerPath(command.RouteRemote, "load", nil, staged); err != nil {
			t.Errorf("remote load refused staged %s: %v", relative, err)
		}
		if _, err := command.CallerPath(command.RouteRemote, "load", nil, source); err == nil {
			t.Errorf("remote load admitted the original outside configuration: %s", source)
		}
		mustWrite(t, source, "changed after preparation\n")
		after, err := os.ReadFile(staged)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(after, original) {
			t.Errorf("staged %s followed a source edit: %q", relative, after)
		}
	}
	files, err := regularFiles(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("staged files = %v, want the two configuration sections only", files)
	}
}

// A missing authored input tree must stop preparation instead of presenting a
// lab whose later file imports cannot work.
func TestShowcaseTopicInputsRefuseEmptySource(t *testing.T) {
	t.Setenv("ZE_DEMO_ROOT", t.TempDir())
	t.Setenv("ZE_DEMO_STATE_ROOT", t.TempDir())
	if err := stageShowcaseTopicInputs(); err == nil {
		t.Fatal("an empty topic input source was accepted")
	}
}
