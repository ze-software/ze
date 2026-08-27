// The migration's proof for the spec inventory: the script and the command
// agree.
//
// scripts/status/spec_status.go is being replaced by internal/le/specstatus, and the
// two live side by side until the swap (plan/spec-le-is-a-ze-binary.md, step
// 14). This file is what makes that safe, and it is deliberately HERE rather
// than beside the new package: it is a migration artifact, so it is deleted by
// the same commit that deletes the script it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree, the script and the
// command print the same page, byte for byte, and answer the same records.
// PREVENTS: a silent behavior change in a port nobody reads the output of. The
// page is a fixed-width table of 228 rows on this checkout; a column that moved,
// a bucket that changed its count or a status that stopped sorting first would
// pass every other test in this repository.
//
// TWO differences are DELIBERATE and are asserted rather than compared:
//
//   - The JSON. The script prints its own array layout with HTML escaping
//     turned off; the command answers a payload and the engine renders it, so
//     `| json` is the engine's spelling. The records are compared DECODED,
//     which is the level at which the two are required to agree.
//   - A tree that holds no plan/ directory. filepath.Glob answers an empty list
//     and no error for a pattern whose directory does not exist, so the script
//     prints "Specs: 0 total" and exits 0 over a tree it never read. The
//     command refuses. TestScriptStillFailsOpenOnATreeWithNoPopulation pins that
//     difference in the direction of the fix, so it reddens the day somebody
//     fixes the script and this file can go.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/specstatus"
)

// parityTree writes a fixture plan/ tree and answers its root. It is a
// temporary directory outside any repository, so git answers nothing for every
// fixture and both implementations record "unknown".
func parityTree(t *testing.T, specs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatalf("create the fixture plan directory: %v", err)
	}
	for name, body := range specs {
		if err := os.WriteFile(filepath.Join(root, "plan", name), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture spec %s: %v", name, err)
		}
	}
	return root
}

// parityRunScript runs the compiled script with root as its working directory
// and answers stdout, stderr and the exit code.
func parityRunScript(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), specStatusTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, specStatusBinary(ctx, t), args...)
	cmd.Dir = root
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run the spec inventory: %v\nstderr: %s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	if ctx.Err() != nil {
		t.Fatalf("the spec inventory did not return inside %s", specStatusTimeout)
	}
	return stdout.String(), stderr.String(), code
}

// parityCollect runs the COMMAND's collection over the same tree and answers the
// records with every warning it wrote.
func parityCollect(t *testing.T, root string) (specstatus.Inventory, []string) {
	t.Helper()
	var warnings []string
	inventory, err := specstatus.Collect(context.Background(), root, time.Now(), func(line string) {
		warnings = append(warnings, line)
	})
	if err != nil {
		t.Fatalf("the command refused a tree the script reads: %v", err)
	}
	return inventory, warnings
}

// parityFixtureSpec returns a spec of the shape plan/TEMPLATE.md produces.
func parityFixtureSpec(status, updated string) string {
	var sb strings.Builder
	sb.WriteString("# Spec: fixture\n\n")
	sb.WriteString("<!-- Authoring note line 1\n     line 2\n     line 3\n     line 4\n     line 5 -->\n\n")
	sb.WriteString("| Field | Value |\n|-------|-------|\n| Status | ")
	sb.WriteString(status)
	sb.WriteString(" |\n| Depends | - |\n| Phase | 2/5 |\n| Updated | ")
	sb.WriteString(updated)
	sb.WriteString(" |\n\n## Task\n\nFixture prose.\n")
	return sb.String()
}

// parityFixtures are the trees both implementations are compared over.
//
// The real checkout exercises only the shapes it happens to hold today, which is
// the vacuity the lint port measured: a comparison over one tree tests the
// checks that tree trips and nothing else. Each tree here reaches a branch of
// the page the checkout may not.
func parityFixtures() map[string]map[string]string {
	return map[string]map[string]string{
		"every bucket at once": {
			"spec-fixture-inprogress.md":   parityFixtureSpec("in-progress", "2026-08-20"),
			"spec-fixture-verification.md": parityFixtureSpec("verification", "2026-08-19"),
			"spec-fixture-ready.md":        parityFixtureSpec("ready", "2026-08-18"),
			"spec-fixture-design.md":       parityFixtureSpec("design", "2026-08-17"),
			"spec-fixture-skeleton.md":     parityFixtureSpec("skeleton", "2026-08-16"),
			"spec-fixture-blocked.md":      parityFixtureSpec("blocked", "2026-08-15"),
			"spec-fixture-deferred.md":     parityFixtureSpec("deferred", "2026-08-14"),
		},
		"a status the reporting order never heard of": {
			"spec-fixture-done.md":     parityFixtureSpec("done", "2026-08-20"),
			"spec-fixture-invented.md": parityFixtureSpec("never-heard-of", "2026-08-19"),
		},
		"an unreadable spec beside one whose table omits Status": {
			"spec-fixture-no-table.md":  "# Spec: no table\n\nThis file carries no metadata table at all.\n\n## Task\n\nFixture prose.\n",
			"spec-fixture-no-status.md": "# Spec: no status row\n\n| Field | Value |\n|-------|-------|\n| Depends | - |\n| Phase | - |\n| Updated | 2026-08-20 |\n\n## Task\n\nFixture prose.\n",
		},
		"a skeleton long past its TTL beside a fresh one": {
			"spec-fixture-ancient.md": parityFixtureSpec("skeleton", "2020-01-01"),
			"spec-fixture-today.md":   parityFixtureSpec("skeleton", time.Now().Format("2006-01-02")),
		},
		"the template, which is not a spec": {
			"spec-template.md":     parityFixtureSpec("design", "2026-08-20"),
			"spec-fixture-real.md": parityFixtureSpec("ready", "2026-08-20"),
		},
		"three specs of one status, so the date order is the only thing separating them": {
			"spec-fixture-oldest.md": parityFixtureSpec("ready", "2026-01-01"),
			"spec-fixture-newest.md": parityFixtureSpec("ready", "2026-08-20"),
			"spec-fixture-middle.md": parityFixtureSpec("ready", "2026-04-10"),
		},
		"a set-numbered family beside a standalone spec": {
			"spec-rib-arch-3-reactor.md": parityFixtureSpec("ready", "2026-08-20"),
			"spec-standalone.md":         parityFixtureSpec("ready", "2026-08-19"),
		},
		"an empty population, which is an ANSWER rather than an unread tree": {},
	}
}

// TestSpecStatusPagesAgree compares the human page byte for byte.
func TestSpecStatusPagesAgree(t *testing.T) {
	for name, specs := range parityFixtures() {
		t.Run(name, func(t *testing.T) {
			root := parityTree(t, specs)
			scriptOut, scriptErr, scriptCode := parityRunScript(t, root)
			inventory, warnings := parityCollect(t, root)

			if scriptCode != 0 {
				t.Fatalf("the script exited %d over the fixture tree\nstderr: %s", scriptCode, scriptErr)
			}
			if got := inventory.Text(); got != scriptOut {
				t.Errorf("the page differs.\n--- script ---\n%s\n--- command ---\n%s", scriptOut, got)
			}
			wantWarnings := strings.Count(scriptErr, "\n")
			if len(warnings) != wantWarnings {
				t.Errorf("the command wrote %d warning(s) and the script %d:\ncommand: %q\nscript: %q",
					len(warnings), wantWarnings, warnings, scriptErr)
			}
			for _, line := range warnings {
				if !strings.Contains(scriptErr, line) {
					t.Errorf("the command warns %q and the script does not:\n%s", line, scriptErr)
				}
			}
		})
	}
}

// TestSpecStatusRecordsAgree compares the machine answer.
//
// The comparison is over DECODED records: the script prints its own array
// layout and the command answers a payload the engine renders, so the bytes
// were never required to agree and the records always were.
func TestSpecStatusRecordsAgree(t *testing.T) {
	for name, specs := range parityFixtures() {
		t.Run(name, func(t *testing.T) {
			root := parityTree(t, specs)
			scriptOut, _, scriptCode := parityRunScript(t, root, "--json")
			if scriptCode != 0 {
				t.Fatalf("the script exited %d in JSON mode", scriptCode)
			}
			inventory, _ := parityCollect(t, root)

			var fromScript, fromCommand []map[string]any
			if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
				t.Fatalf("decode the script's JSON: %v\n%s", err, scriptOut)
			}
			raw, err := json.Marshal(inventory)
			if err != nil {
				t.Fatalf("the command's payload does not encode: %v", err)
			}
			if err := json.Unmarshal(raw, &fromCommand); err != nil {
				t.Fatalf("decode the command's payload: %v\n%s", err, raw)
			}

			if len(fromScript) != len(fromCommand) {
				t.Fatalf("the script answers %d records and the command %d", len(fromScript), len(fromCommand))
			}
			for i := range fromScript {
				if len(fromScript[i]) != len(fromCommand[i]) {
					t.Errorf("record %d: the script carries %d keys and the command %d\nscript:  %v\ncommand: %v",
						i, len(fromScript[i]), len(fromCommand[i]), fromScript[i], fromCommand[i])
					continue
				}
				for key, want := range fromScript[i] {
					if got := fromCommand[i][key]; got != want {
						t.Errorf("record %d key %q: the script answers %v and the command %v", i, key, want, got)
					}
				}
			}
		})
	}
}

// TestScriptStillFailsOpenOnATreeWithNoPopulation is the ONE deliberate
// behavioral difference, written in the direction of the fix.
//
// The script's filepath.Glob answers an empty list and no error for a pattern
// whose directory does not exist, so it reports a complete inventory of nothing
// over any tree it is not standing in, and exits 0. The command refuses.
//
// This test asserts the SCRIPT still fails open. It therefore starts failing the
// day somebody fixes the script, and the answer then is to delete the script and
// this file with it (step 14). It is not an endorsement of the behavior: it is
// the record that the difference is known and deliberate.
func TestScriptStillFailsOpenOnATreeWithNoPopulation(t *testing.T) {
	bare := t.TempDir()

	scriptOut, scriptErr, scriptCode := parityRunScript(t, bare)
	if scriptCode != 0 {
		t.Fatalf("the script no longer exits 0 over a tree with no plan/ directory; delete this file with the script\nstderr: %s", scriptErr)
	}
	if !strings.Contains(scriptOut, "Specs: 0 total") {
		t.Fatalf("the script no longer reports an empty inventory over a tree it never read:\n%s", scriptOut)
	}

	if _, err := specstatus.Collect(context.Background(), bare, time.Now(), nil); err == nil {
		t.Error("the command reports an inventory for a tree that holds no plan/ directory")
	}
}
