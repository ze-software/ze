// Design: docs/architecture/core-design.md -- observing the red a record rests on
package rfc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: two observations of producers that share a basename, run at once
// from one session scratch, each stage their own replacement and their own
// overlay, and each overlay still selects its own break.
// PREVENTS: the IKE break compiling against the LDP agent's broken-register.go
// because the replacement was keyed on the basename alone
// (concurrent-session-corruption journal, 2026-09-28).
func TestWriteOverlayStagesEachRunApart(t *testing.T) {
	tree := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "overlay-sid")
	first, err := writeOverlay(tree, overlayFile{rel: "internal/a/register.go", content: "package a\n"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeOverlay(tree, overlayFile{rel: "internal/b/register.go", content: "package b\n"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("both runs wrote overlay %s", first)
	}
	for overlay, want := range map[string]string{first: "package a\n", second: "package b\n"} {
		raw, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct{ Replace map[string]string }
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		for _, replacement := range parsed.Replace {
			got, err := os.ReadFile(replacement)
			if err != nil || string(got) != want {
				t.Fatalf("overlay %s selects %s holding %q, %v; want %q", filepath.Base(overlay), replacement, got, err, want)
			}
		}
	}
}

// VALIDATES: an interop break reaches the selected native child through its
// overlay while the shared producer still holds its original bytes.
// PREVENTS: a killed observation stranding a broken producer in the checkout;
// checking only after requireRed returns would accept mutate-and-restore.
func TestInteropBreakLeavesSourceUntouched(t *testing.T) {
	const (
		childRoot = "RFC_INTEROP_OVERLAY_CHILD_ROOT"
		selector  = "RFC_INTEROP_OVERLAY_SCENARIO"
		scenario  = "overlay-isolation"
		original  = "package producer\nfunc value() int { return 1 }\n"
		broken    = "package producer\nfunc value() int { panic(\"break\") }\n"
		observed  = "native child observed the isolated break"
	)
	if root := os.Getenv(childRoot); root != "" {
		source := filepath.Join(root, "producer.go")
		content, err := os.ReadFile(source)
		if err != nil || string(content) != original {
			t.Fatalf("shared source during break = %q, %v; want %q", content, err, original)
		}
		if got := os.Getenv(selector); got != scenario {
			t.Fatalf("native child scenario = %q, want %q", got, scenario)
		}
		overlay, held := strings.CutPrefix(os.Getenv("GOFLAGS"), "-overlay=")
		if !held || overlay == "" {
			t.Fatal("native child received no overlay")
		}
		raw, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct{ Replace map[string]string }
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		if len(parsed.Replace) != 1 {
			t.Fatalf("overlay replacements = %v, want only %s", parsed.Replace, source)
		}
		replacement := parsed.Replace[source]
		content, err = os.ReadFile(replacement)
		if err != nil || string(content) != broken {
			t.Fatalf("native child break = %q, %v; want %q", content, err, broken)
		}
		// Only this deliberate nonzero exit carries the evidence marker. An
		// earlier assertion failure must not be accepted as the native red.
		// The line is the shape interoplab.Suite prints for a failed scenario,
		// because the recorder accepts no red its cited assertion did not name.
		fmt.Fprintln(os.Stderr, "interop: [1/1] "+scenario+": FAIL (0s): scenario "+
			scenario+" assertion 1: "+observed)
		os.Exit(1)
	}

	tree := t.TempDir()
	source := filepath.Join(tree, "producer.go")
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "interop-overlay-sid")
	t.Setenv(childRoot, tree)
	t.Setenv(selector, "unselected")
	t.Setenv("GOFLAGS", "-overlay=stale-overlay.json")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := observationRunner{
		tree:     tree,
		carrier:  Carrier{Kind: kindInterop},
		scenario: scenario,
		record:   DiscriminationRecord{Citation: "1"},
		self:     binary,
		suite: interoplab.Catalog{
			Action:           "-test.run=^TestInteropBreakLeavesSourceUntouched$/-test.v",
			SelectorVariable: selector,
		},
	}
	red, err := runner.requireRed(overlayFile{rel: "producer.go", content: broken})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(red.Red, observed) {
		t.Fatalf("native child did not observe an isolated break:\n%s", red.Red)
	}
	wantCommand := fmt.Sprintf("%s -test.run=^TestInteropBreakLeavesSourceUntouched$ -test.v", binary)
	if red.Command != wantCommand {
		t.Errorf("recorded command = %q, want %q", red.Command, wantCommand)
	}
	content, err := os.ReadFile(source)
	if err != nil || string(content) != original {
		t.Fatalf("shared source after break = %q, %v; want %q", content, err, original)
	}
}
