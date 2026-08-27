// The migration proof for `ze-rfc-check`: the Python producer and the Go
// command return the same diagnostics and exit code over committed fixtures.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11. Every HEAD ratchet is
// driven through rfc.Check, not through its leaf helper.
// PREVENTS: two silent outputs compared as parity, or a ratchet implemented but
// omitted from the public check driver.
package main

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

func rfcCheckBase(t *testing.T, extra map[string]string) string {
	t.Helper()
	files := rfcPyWith(map[string]string{
		"test/parse/rfc.ci": rfcPyTagLine("RFC9999-2-1", "positive") +
			rfcPyTagLine("RFC9999-2-1", "negative") +
			rfcPyTagLine("RFC9999-2-2", "positive") +
			rfcPyTagLine("RFC9999-2-2", "negative"),
		"rfc/extraction/rfc9999.json": rfcPyArtifact(rfcPyMappedSites, rfcPySections),
		"docs/features/rfc-status.md": "| RFC 9999 | Widgets | Supported | full | No tracked gap |\n",
		"rfc/drain-budget.txt":        "start 2026-01-01\nrate 0\n",
		"feature-gates.txt":           "ze_widget ./internal/widget\n",
		"Makefile":                    "GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)\n",
		"go.mod":                      "module fixture.invalid/rfc\n\ngo 1.25\n",
	})
	maps.Copy(files, extra)
	tree := rfcPyTree(t, files)
	if _, supplied := extra["internal/le/functional/suites.go"]; !supplied {
		root := devPyRoot(t)
		suites, err := os.ReadFile(filepath.Join(root, "internal/le", "functional", "suites.go"))
		if err != nil {
			t.Fatalf("reading the Go suite source: %v", err)
		}
		rfcPyWrite(t, tree, "internal/le/functional/suites.go", string(suites))
	}
	if result := rfcPyRunScript(t, tree, "--write"); result.Code != 0 {
		t.Fatalf("seeding the generated ledger: %s%s", result.Stdout, result.Stderr)
	}
	git(t, tree, "init", "-q")
	git(t, tree, "config", "user.email", "fixture@example.invalid")
	git(t, tree, "config", "user.name", "RFC fixture")
	git(t, tree, "add", ".")
	git(t, tree, "commit", "-qm", "baseline")
	return tree
}

func rfcCheckAgree(t *testing.T, tree, signal string) {
	t.Helper()
	script := rfcPyRunScript(t, tree, "--check")
	report, code := rfc.Check(tree)
	commandText := report.Text()
	scriptText := ansiRE.ReplaceAllString(script.Stdout+script.Stderr, "")
	if script.Code != code {
		t.Fatalf("exit mismatch: Python %d, Go %d\nPython:\n%s\nGo:\n%s",
			script.Code, code, scriptText, commandText)
	}
	if scriptText != commandText {
		t.Fatalf("diagnostic mismatch\nPython:\n%s\nGo:\n%s", scriptText, commandText)
	}
	if signal != "" && !strings.Contains(commandText, signal) {
		t.Fatalf("the public check did not fire %q:\n%s", signal, commandText)
	}
}

func TestRFCCheckBothHalvesAgreeOnACompleteFixture(t *testing.T) {
	tree := rfcCheckBase(t, nil)
	rfcCheckAgree(t, tree, "rfc-requirements OK")
}

// VALIDATES: The HEAD carrier baseline reads HEAD's Go suite list, not the
// legacy Python dispatcher or today's compiled list.
// PREVENTS: Relabeling both sides with a source that did not define HEAD.
func TestRFCCheckHEADCarrierReadsGoFunctionalSuites(t *testing.T) {
	tree := rfcCheckBase(t, map[string]string{
		"internal/le/functional/suites.go": "package functional\n\n" +
			"const suitePlugin = \"plugin\"\n\nvar Gating = []string{suitePlugin}\n",
	})
	if err := os.Remove(filepath.Join(tree, "test", "parse", "rfc.ci")); err != nil {
		t.Fatalf("removing the HEAD-unrun carrier: %v", err)
	}
	body := rfcPyTagLine("RFC9999-2-1", "positive") + rfcPyTagLine("RFC9999-2-1", "negative") +
		rfcPyTagLine("RFC9999-2-2", "positive") + rfcPyTagLine("RFC9999-2-2", "negative")
	rfcPyWrite(t, tree, "test/interop/scenarios/widget/check.py", body)
	rfcPyWrite(t, tree, ".github/workflows/nightly.yml",
		"on:\n  schedule:\n    - cron: '0 1 * * *'\njobs:\n  check:\n    steps:\n      - run: make ze-interop-test\n")

	report, _ := rfc.Check(tree)
	if strings.Contains(report.Text(), "has lost its functional/verify evidence") {
		t.Fatalf("the baseline credited test/parse from a source other than HEAD's Go suite list:\n%s",
			report.Text())
	}
}

func TestRFCCheckRatchetsFireThroughThePublicEntryPoint(t *testing.T) {
	cases := []struct {
		name   string
		extra  map[string]string
		mutate func(*testing.T, string)
		signal string
	}{
		{
			name: "enrolment",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/enrolled.txt", "")
			},
			signal: "was un-enrolled. Enrolment is monotonic",
		},
		{
			name: "new summary",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/short/rfc1000.md", "# RFC 1000\n\n- [ ] [RFC1000-2-1] [MUST] A speaker MUST answer (§2)\n")
				rfcPyWrite(t, tree, "rfc/full/rfc1000.txt", "A speaker MUST answer.\n")
			},
			signal: "rfc/short/rfc1000.md is new and declares 1 gated MUST-level requirement",
		},
		{
			name: "retired requirement",
			mutate: func(t *testing.T, tree string) {
				body := strings.Replace(rfcPySummary, "- [ ] [RFC9999-2-2] [MUST NOT] A receiver MUST NOT drop the widget (§2)\n", "", 1)
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md", body)
			},
			signal: "RFC9999-2-2 was in rfc/short/rfc9999.md at HEAD and is now gone",
		},
		{
			name: "level",
			mutate: func(t *testing.T, tree string) {
				body := strings.Replace(rfcPySummary, "[MUST] A speaker MUST", "[SHOULD] A speaker SHOULD", 1)
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md", body)
			},
			signal: "moved [MUST] -> [SHOULD] and left the gated MUST-level population",
		},
		{
			name: "coverage",
			mutate: func(t *testing.T, tree string) {
				body := rfcPyTagLine("RFC9999-2-1", "positive") +
					rfcPyTagLine("RFC9999-2-2", "positive") + rfcPyTagLine("RFC9999-2-2", "negative")
				rfcPyWrite(t, tree, "test/parse/rfc.ci", body)
			},
			signal: "RFC9999-2-1 is no longer proven -- the negative test(s) that covered it at HEAD are gone",
		},
		{
			name: "evidence tier",
			mutate: func(t *testing.T, tree string) {
				if err := os.Remove(filepath.Join(tree, "test", "parse", "rfc.ci")); err != nil {
					t.Fatalf("removing the functional carrier: %v", err)
				}
				body := rfcPyTagLine("RFC9999-2-1", "positive") + rfcPyTagLine("RFC9999-2-1", "negative") +
					rfcPyTagLine("RFC9999-2-2", "positive") + rfcPyTagLine("RFC9999-2-2", "negative")
				rfcPyWrite(t, tree, "test/interop/scenarios/widget/check.py", body)
				rfcPyWrite(t, tree, ".github/workflows/nightly.yml", "on:\n  schedule:\n    - cron: '0 1 * * *'\njobs:\n  check:\n    steps:\n      - run: make ze-interop-test\n")
			},
			signal: "has lost its functional/verify evidence",
		},
		{
			name: "audit verdict",
			extra: map[string]string{
				"rfc/audit/rfc9999.json": `{"rfc":"rfc9999","audited":"2026-01-01","requirements":{"RFC9999-2-1":{"verdict":"enforced","note":"fixture","requirement_sha":"0000000000000000","tests":{}}}}` + "\n",
			},
			mutate: func(t *testing.T, tree string) {
				if err := os.Remove(filepath.Join(tree, "rfc", "audit", "rfc9999.json")); err != nil {
					t.Fatalf("removing the audit: %v", err)
				}
			},
			signal: "carried a verdict at HEAD and carries none now. Audit coverage is monotonic",
		},
		{
			name: "audit finding",
			extra: map[string]string{
				"rfc/audit/rfc9999.json": `{"rfc":"rfc9999","audited":"2026-01-01","requirements":{"RFC9999-2-1":{"verdict":"weak","note":"fixture","requirement_sha":"0000000000000000","tests":{},"units":{"test/parse/rfc.ci":"0000000000000000"}}}}` + "\n",
			},
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/audit/rfc9999.json", `{"rfc":"rfc9999","audited":"2026-01-02","requirements":{"RFC9999-2-1":{"verdict":"enforced","note":"RFC9999 fixture","requirement_sha":"0000000000000000","tests":{"test/parse/rfc.ci":"0000000000000000"},"units":{"test/parse/rfc.ci":"0000000000000000"}}}}`+"\n")
			},
			signal: "went from 'weak' to 'enforced' while every tagged unit stayed byte-identical",
		},
		{
			name: "extraction",
			mutate: func(t *testing.T, tree string) {
				if err := os.Remove(filepath.Join(tree, "rfc", "extraction", "rfc9999.json")); err != nil {
					t.Fatalf("removing the extraction: %v", err)
				}
			},
			signal: "had an extraction sign-off at HEAD and has none now. Extraction sign-off is monotonic",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := rfcCheckBase(t, one.extra)
			one.mutate(t, tree)
			rfcCheckAgree(t, tree, one.signal)
		})
	}
}

func TestRFCCheckSilentBranchesFireThroughThePublicEntryPoint(t *testing.T) {
	cases := []struct {
		name   string
		extra  map[string]string
		mutate func(*testing.T, string)
		signal string
	}{
		{
			name: "status row removal",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "docs/features/rfc-status.md", "")
			},
			signal: "had a row in docs/features/rfc-status.md at HEAD and does not now",
		},
		{
			name: "gap count disagreement",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "docs/features/rfc-status.md",
					"| RFC 9999 | Widgets | Partial | partial | One MUST gap remains |\n")
			},
			signal: "docs/features/rfc-status.md says rfc9999 has 1 MUST-level gap(s)",
		},
		{
			name: "audit schema claim",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/audit/rfc9999.json",
					`{"rfc":"rfc9999","audited":"2026-01-02","requirements":{"RFC9999-2-1":{"verdict":"enforced","note":"fixture","requirement_sha":"0000000000000000","tests":{}}}}`+"\n")
			},
			signal: "is 'enforced' with an empty 'tests' map",
		},
		{
			name: "audit disclosure",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/audit/rfc9999.json",
					`{"rfc":"rfc9999","audited":"2026-01-02","requirements":{"RFC9999-2-1":{"verdict":"wrong","note":"fixture","requirement_sha":"0000000000000000","tests":{}}}}`+"\n")
			},
			signal: "An audited requirement that is not met cannot be advertised as clean support",
		},
		{
			name: "audit note",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/audit/rfc9999.json",
					`{"rfc":"rfc9999","audited":"2026-01-02","requirements":{"RFC9999-2-1":{"verdict":"enforced","note":"zzzzz","requirement_sha":"0000000000000000","tests":{"test/parse/rfc.ci":"0000000000000000"}}}}`+"\n")
			},
			signal: "its note names nothing that occurs in the tagged unit(s)",
		},
		{
			name: "tagged package compile",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "internal/bad/bad_test.go",
					"package bad\n\n// "+strings.TrimPrefix(rfcPyTagLine("RFC9999-2-1", "positive"), "# ")+"func broken(\n")
			},
			signal: "`go vet` failed over the 1 package(s) that hold RFC requirement tags",
		},
		{
			name: "drain policy",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/drain-budget.txt", "start 2026-01-01\nrate 2\n")
			},
			signal: "rate 2.0 exceeds the whole enrolled set (1)",
		},
		{
			name: "missing generated shard",
			mutate: func(t *testing.T, tree string) {
				if err := os.Remove(filepath.Join(tree, "rfc", "requirements", "rfc9999.md")); err != nil {
					t.Fatalf("removing the generated shard: %v", err)
				}
			},
			signal: "rfc/requirements/rfc9999.md is missing",
		},
		{
			name: "live disposition removal",
			extra: map[string]string{
				"rfc/short/rfc1000.md": "# RFC 1000\n",
				"rfc/full/rfc1000.txt": "This Informational document describes widgets.\n",
				"rfc/not-enrolled.txt": "rfc1000 non-normative Informational document with no RFC 2119 key words\n",
			},
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/not-enrolled.txt", "")
			},
			signal: "left rfc/not-enrolled.txt without entering rfc/enrolled.txt",
		},
		{
			name: "retired id reuse",
			extra: map[string]string{
				"rfc/short/rfc9999.md": strings.Replace(rfcPySummary,
					"RFC9999-2-2", "RFC9999-2-3", 1),
				"test/parse/rfc.ci": rfcPyTagLine("RFC9999-2-1", "positive") +
					rfcPyTagLine("RFC9999-2-1", "negative") +
					rfcPyTagLine("RFC9999-2-3", "positive") +
					rfcPyTagLine("RFC9999-2-3", "negative"),
				"rfc/extraction/rfc9999.json": strings.Replace(
					rfcPyArtifact(rfcPyMappedSites, rfcPySections),
					"RFC9999-2-2", "RFC9999-2-3", 1),
			},
			mutate: func(t *testing.T, tree string) {
				body, err := os.ReadFile(filepath.Join(tree, "rfc", "short", "rfc9999.md"))
				if err != nil {
					t.Fatalf("reading the summary: %v", err)
				}
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md", string(body)+
					"- [ ] [RFC9999-2-2] [SHOULD] A receiver SHOULD log the widget (§2)\n")
			},
			signal: "RFC9999-2-2 reuses a retired id",
		},
		{
			name: "superseded requirement without lineage",
			mutate: func(t *testing.T, tree string) {
				body := strings.Replace(rfcPySummary, "| Obsoleted-by | None |",
					"| Obsoleted-by | RFC 1000 |", 1)
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md", body)
				rfcPyWrite(t, tree, "rfc/full/rfc1000.txt", "Successor.\n")
			},
			signal: "states an obligation of a document RFC1000 obsoletes",
		},
		{
			name: "private gap behind public support",
			mutate: func(t *testing.T, tree string) {
				body := strings.Replace(rfcPySummary, "A speaker MUST send the widget (§2)",
					"A speaker MUST send the widget (§2) {gap: no sender exists}", 1)
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md", body)
			},
			signal: "A known unmet MUST cannot be advertised as clean support",
		},
		{
			name: "support over an empty checklist",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/short/rfc9999.md",
					"# RFC 9999\n\n| Field | Value |\n|---|---|\n| Obsoleted-by | None |\n")
			},
			signal: "the claim rests on an empty checklist",
		},
		{
			name: "orphan audit file",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/audit/rfc1000.json",
					`{"rfc":"rfc1000","audited":"2026-01-02","requirements":{}}`+"\n")
			},
			signal: "there is no rfc/short/rfc1000.md",
		},
		{
			name: "exclusion rise without a new reason",
			mutate: func(t *testing.T, tree string) {
				artifact := strings.Replace(
					rfcPyArtifact(rfcPyMappedSites, rfcPySections),
					`"disposition": "mapped", "mapped-to": "RFC9999-2-2"`,
					`"disposition": "excluded", "excluded-kind": "not-a-requirement", "reason": "reclassified"`, 1)
				rfcPyWrite(t, tree, "rfc/extraction/rfc9999.json", artifact)
			},
			signal: "exclusions rose from 0 to 1 with no 'resign-reason'",
		},
		{
			name: "new enrolment without signoff",
			extra: map[string]string{
				"rfc/short/rfc1000.md": "# RFC 1000\n",
				"rfc/full/rfc1000.txt": "This Informational document describes widgets.\n",
				"rfc/not-enrolled.txt": "rfc1000 non-normative Informational document with no RFC 2119 key words\n",
			},
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/enrolled.txt", "rfc9999\nrfc1000\n")
				rfcPyWrite(t, tree, "rfc/not-enrolled.txt", "")
			},
			signal: "rfc1000 is newly enrolled with no valid extraction sign-off",
		},
		{
			name: "non-normative laundering",
			mutate: func(t *testing.T, tree string) {
				rfcPyWrite(t, tree, "rfc/short/rfc1000.md", "# RFC 1000\n")
				rfcPyWrite(t, tree, "rfc/full/rfc1000.txt", "This document describes widgets.\n")
				rfcPyWrite(t, tree, "rfc/not-enrolled.txt",
					"rfc1000 non-normative Ze does not need widgets\n")
			},
			signal: "judges what ZE owes rather than what the DOCUMENT states",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			tree := rfcCheckBase(t, one.extra)
			one.mutate(t, tree)
			rfcCheckAgree(t, tree, one.signal)
		})
	}
}
