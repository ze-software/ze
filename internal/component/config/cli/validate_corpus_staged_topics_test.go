//go:build ze_bgp

// Design: docs/contributing/testing.md -- repository configuration coverage includes staged demo inputs.
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: a topic section named relative to the daemon configuration
// directory is validated from its authored source, in sequence beside the
// existing repository-absolute input form.
func TestTapeSequencesRetainStagedTopicInputs(t *testing.T) {
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("demos/terminal/lab/ze.conf", "system { host base; }\n")
	write("demos/terminal/topics/first/1-section.conf", "system { host first; }\n")
	write("demos/terminal/topics/second/2-section.conf", "system { host second; }\n")
	write("demos/terminal/show/demo.tape", "Output x.webm\n"+
		"Type \"ze-demo run lab start\"\n"+
		"Type \"load file absolute merge topics/first/1-section.conf\"\n"+
		"Type \"load file absolute merge /src/demos/terminal/topics/second/2-section.conf\"\n")
	s := &sweepContext{t: t, root: root, report: &sweepReport{}}
	loaded := map[string]bool{}
	sequences := s.tapeSequences(filepath.Join(root, "demos", "terminal", "show", "demo.tape"), loaded)
	if len(sequences) != 1 {
		t.Fatalf("got %d sequences, want the lab's sequence", len(sequences))
	}
	steps := sequences[0].steps
	if len(steps) != 2 {
		t.Fatalf("load steps = %+v, want both staged and repository-absolute sections", steps)
	}
	for index, relative := range []string{"topics/first/1-section.conf", "topics/second/2-section.conf"} {
		want := filepath.Join(root, "demos", "terminal", filepath.FromSlash(relative))
		if steps[index].source != want {
			t.Errorf("step %d source = %q, want %q", index, steps[index].source, want)
		}
		if !loaded[want] {
			t.Errorf("authored input %s is missing from the loaded population", relative)
		}
	}
	if len(s.report.findings) != 0 {
		t.Fatalf("valid topic sources produced findings: %v", s.report.findings)
	}
}
