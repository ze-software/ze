package doccheck

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// commandWordsMax is the dispatcher's bound: a command is at most two words
// after `le` (TestDispatchBoundsTheLookupAtTwoWords in leroot pins it). A row
// naming a longer new command names nothing a caller can run.
const commandWordsMax = 2

// retiredFound reports whether the row whose old form is old lists file:line.
func retiredFound(report retiredReport, old, file string, line int) bool {
	for _, row := range report.Rows {
		if row.Old != old {
			continue
		}
		for _, match := range row.Matches {
			if match.File == file && match.Line == line {
				return true
			}
		}
	}
	return false
}

// retiredLinesOf answers every row that lists a line of file, as "old@line".
func retiredLinesOf(report retiredReport, file string) []string {
	var found []string
	for _, row := range report.Rows {
		for _, match := range row.Matches {
			if match.File == file {
				found = append(found, row.Old+"@"+strconv.Itoa(match.Line))
			}
		}
	}
	return found
}

func TestRetiredCommandSweepFindsAnInjectedName(t *testing.T) {
	// VALIDATES: each match form AC-15 names is found, at its file and line,
	// under the rename-map row it retires.
	// PREVENTS: a caller the Phase 2 rewrite never sees because the sweep
	// does not recognize how it spells the old name.
	root := fixtureRepository(t, map[string]string{
		"docs/commands.md": strings.Join([]string{
			"./le test-unit all",
			"run `le verify lint run` first",
			"ze le tier check",
			"\"command\": \"$CLAUDE_PROJECT_DIR/le hook-check pretool-bash\"",
			"./le docs-to-code index-check",
		}, "\n") + "\n",
		"internal/x/argv.go": strings.Join([]string{
			"package x",
			`var one = []string{"le", "functional", "gating"}`,
			`var two = []string{"le", "verify", "lock", "run"}`,
		}, "\n") + "\n",
		"docs/programs.md": strings.Join([]string{
			"ze-chaos --in-process --web",
			"go build ./cmd/ze-gok",
			"//go:build ze_analyze",
			"go build -tags ze_perf,ze_bgp ./cmd/ze",
			"COPY bin/ze-test-linux-arm64 /usr/local/bin/ze-test",
			"docker cp bin/ze-perf-linux sender:/",
		}, "\n") + "\n",
		"docs/variables.md": strings.Join([]string{
			"ZE_TEST_BIN=bin/ze-test",
			"env.Get(\"ze.qemu.test.bin\")",
			"ze.test.no.build",
			"export ze_perf_bin=x",
		}, "\n") + "\n",
	})

	report, err := sweepRetired(root)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		form string
		old  string
		file string
		line int
	}{
		{"./le", "le test-unit", "docs/commands.md", 1},
		{"bare le, two words", "le verify lint", "docs/commands.md", 2},
		{"ze le", "le tier", "docs/commands.md", 3},
		{"$CLAUDE_PROJECT_DIR/le", "le hook-check", "docs/commands.md", 4},
		{"an action row", "le docs-to-code index-check", "docs/commands.md", 5},
		{"Go literal, one word", "le functional", "internal/x/argv.go", 2},
		{"Go literal, two words", "le verify lock", "internal/x/argv.go", 3},
		{"program", "ze-chaos", "docs/programs.md", 1},
		{"program path", "ze-gok", "docs/programs.md", 2},
		{"build constraint", "ze_analyze", "docs/programs.md", 3},
		{"tag list", "ze_perf", "docs/programs.md", 4},
		{"harness file prefix", "bin/ze-test-linux-", "docs/programs.md", 5},
		{"image path", "/usr/local/bin/ze-test", "docs/programs.md", 5},
		{"perf file", "bin/ze-perf-linux", "docs/programs.md", 6},
		{"upper-case variable", "ze.test.bin", "docs/variables.md", 1},
		{"harness file", "bin/ze-test", "docs/variables.md", 1},
		{"dotted variable", "ze.qemu.test.bin", "docs/variables.md", 2},
		{"lower-case dotted variable", "ze.test.no.build", "docs/variables.md", 3},
		{"lower-case underscored variable", "ze.perf.bin", "docs/variables.md", 4},
	}
	for _, tc := range cases {
		if !retiredFound(report, tc.old, tc.file, tc.line) {
			t.Errorf("%s: %s:%d is not listed under %q; the file matched %v",
				tc.form, tc.file, tc.line, tc.old, retiredLinesOf(report, tc.file))
		}
	}
	if report.Lines == 0 {
		t.Error("the report counts no line")
	}
}

func TestRetiredCommandSweepSkipsHistoricalRecords(t *testing.T) {
	// VALIDATES: every declared historical record and every declaration file
	// is skipped, while the same old name in an ordinary file beside them is
	// still found. A file beside the leroot tests is ordinary: the alias that
	// once lived under internal/le/le/root/retired* is gone, and so is the
	// prefix that excused it.
	// PREVENTS: an exclusion that is too wide hiding a live caller, and one
	// that is too narrow making a rewrite falsify history.
	const line = "./le test-unit all\n"
	files := map[string]string{
		"plan/spec-other.md":                   line,
		"internal/le/le/root/retired_extra.go": line,
	}
	var skipped []string
	for _, path := range retiredDeclarations {
		files[path] = line
		skipped = append(skipped, path)
	}
	for _, record := range retiredRecords {
		path := record.path
		if strings.HasSuffix(path, "/") {
			path += "nested/record.md"
		}
		files[path] = line
		skipped = append(skipped, path)
	}
	root := fixtureRepository(t, files)

	report, err := sweepRetired(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"plan/spec-other.md", "internal/le/le/root/retired_extra.go"} {
		if !retiredFound(report, "le test-unit", path, 1) {
			t.Errorf("%s:1 is not listed; the file matched %v", path, retiredLinesOf(report, path))
		}
	}
	for _, path := range skipped {
		if lines := retiredLinesOf(report, path); len(lines) != 0 {
			t.Errorf("%s is excluded, and the report lists it at %v", path, lines)
		}
	}
}

func TestRetiredCommandSweepBlanksDeclaredWords(t *testing.T) {
	// VALIDATES: a declared word that contains a retired name (the skill,
	// the EAP identity, the two old make targets, the chaos MCP server name, the
	// pkg/ze external test package) is not a match, and the harness
	// name beside it on the same line still is.
	// PREVENTS: a declared word hiding a live caller on its line.
	root := fixtureRepository(t, map[string]string{
		"docs/words.md": strings.Join([]string{
			"run /ze-test first",
			"identity ze-test-client",
			"make ze-test-all",
			"contains=ze-chaos-mcp",
			"package ze_test",
			"make ze-perf-bench PERF_DUT=ze",
			"run /ze-test && ze-test peer",
		}, "\n") + "\n",
	})

	report, err := sweepRetired(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, found := range retiredLinesOf(report, "docs/words.md") {
		if !strings.HasSuffix(found, "@7") {
			t.Errorf("docs/words.md lists %s, and that line holds only a declared word", found)
		}
	}
	if !retiredFound(report, "ze-test", "docs/words.md", 7) {
		t.Errorf("docs/words.md:7 names the harness beside a declared word, and the report misses it")
	}
}

func TestRetiredHarnessMatchesOnlyInProgramPosition(t *testing.T) {
	// VALIDATES: the harness name is matched where it is run or shipped as a
	// program (a command's first word, exec=, cmd/, bin/, the cross-build
	// suffix, a Dockerfile COPY or RUN), and not where the same word is a
	// fixture: a hostname in a config string, a NAS id, a module name.
	// PREVENTS: some 300 fixture strings reported as callers of the harness,
	// and a tightening that also loses the forms which really run it.
	root := fixtureRepository(t, map[string]string{
		"test/plugin/run.ci": strings.Join([]string{
			"exec=ze-test bgp plugin",
			"ze-test peer --port 1790",
			"out=$(bin/ze-test bgp parse 1)",
			"go build -o bin/x ./cmd/ze-test",
			"cp build/ze-test-linux-arm64 guest/",
			"make build; ./bin/ze-test editor",
		}, "\n") + "\n",
		"test/interop/Dockerfile.x": "COPY le-test /usr/bin/\nCOPY ze-test /usr/bin/\n",
		"internal/component/x/fixture_test.go": strings.Join([]string{
			"package x",
			`const config = "system { host-name ze-test; }"`,
			`nas := "ze-test"`,
			`module := "ze-test-module"`,
			`// Connects as the ze-test client.`,
			`cert, err := root.IssueLeaf("ze-test", nil)`,
		}, "\n") + "\n",
		"internal/test/fixture/start.go": strings.Join([]string{
			`p, err := startFixtureProcess(ctx, env, "", "ze-test", "peer")`,
			`	run "ze-test fixture plugin/probe"`,
			`// The ze-test binary serves the peer half.`,
		}, "\n") + "\n",
	})

	report, err := sweepRetired(root)
	if err != nil {
		t.Fatal(err)
	}
	for line := 1; line <= 6; line++ {
		if !retiredFound(report, "ze-test", "test/plugin/run.ci", line) {
			t.Errorf("test/plugin/run.ci:%d runs or builds the harness, and the report misses it", line)
		}
	}
	if retiredFound(report, "ze-test", "test/interop/Dockerfile.x", 1) {
		t.Error("test/interop/Dockerfile.x:1 copies le-test, and the report lists it")
	}
	if !retiredFound(report, "ze-test", "test/interop/Dockerfile.x", 2) {
		t.Error("test/interop/Dockerfile.x:2 copies the harness, and the report misses it")
	}
	for line := 1; line <= 3; line++ {
		if !retiredFound(report, "ze-test", "internal/test/fixture/start.go", line) {
			t.Errorf("internal/test/fixture/start.go:%d starts or names the harness, and the report misses it", line)
		}
	}
	if found := retiredLinesOf(report, "internal/component/x/fixture_test.go"); len(found) != 0 {
		t.Errorf("fixture_test.go names ze-test only as fixture text, and the sweep listed %v", found)
	}
}

func TestRetiredEveryDeclaredExceptionIsFileScoped(t *testing.T) {
	// VALIDATES: every row of retiredExceptions, the per-file ones for the
	// Discord channel and the skill among them, hides its spelling in its own
	// file and nowhere else. The method writes one matching line into the
	// declared file and the same line into a file no exception names.
	// PREVENTS: an exception that hides nothing (a dead row) or that hides the
	// spelling in every file. A new exception with no sample line fails here.
	samples := map[string]string{
		"ze-test":                     "exec=ze-test bgp plugin",
		"ze_test":                     "//go:build ze_test",
		"le rules":                    "./le rules index-update",
		"le repository tracked-build": "./le repository tracked-build check",
		"le repository":               "./le repository generate",
		"le qemu":                     "./le qemu run",
		"ze-terminal-pty":             "go build -o bin/ze-terminal-pty ./cmd/ze-terminal-pty",
		"le.test.bin":                 "LE_TEST_BIN=/stale/harness",
		"ze.test.bin":                 "ZE_TEST_BIN=/stale/harness",
		"le.qemu.test.bin":            "LE_QEMU_TEST_BIN=/stale/harness",
		"ze.qemu.test.bin":            "ZE_QEMU_TEST_BIN=/stale/harness",
		"le-test":                     "exec=le-test bgp plugin",
		"bin/le-test":                 "run bin/le-test ospf",
		"ze-perf":                     "ze-perf report results.json",
		"ze-perf-run":                 "go build -o bin/ze-perf-run ./cmd/ze-perf-run",
		"ze-analyze":                  "bin/ze-analyze density",
		"ze-chaos":                    "exec=ze-chaos --quiet",
		"ze_chaos":                    "//go:build ze_chaos",
		"ze.test.no.build":            "ZE_TEST_NO_BUILD=1",
		"ze-peer":                     "exec=ze-peer --mode sink",
		"bin/ze-perf":                 "run bin/ze-perf track --check",
		"le test-health":              "./le test-health update",
	}
	for _, exception := range retiredExceptions {
		sample, known := samples[exception.old]
		if !known {
			t.Errorf("exception %s %q has no sample line in this test", exception.file, exception.old)
			continue
		}
		root := fixtureRepository(t, map[string]string{
			exception.file:       sample + "\n",
			"docs/unexcepted.md": sample + "\n",
		})
		report, err := sweepRetired(root)
		if err != nil {
			t.Fatal(err)
		}
		if found := retiredLinesOf(report, exception.file); len(found) != 0 {
			t.Errorf("%s declares %q, and the sweep listed %v", exception.file, exception.old, found)
		}
		if !retiredFound(report, exception.old, "docs/unexcepted.md", 1) {
			t.Errorf("the %q exception of %s hid the line in docs/unexcepted.md, or the sample does not match",
				exception.old, exception.file)
		}
	}
}

func TestRetiredCommandSweepHonorsDeclaredExceptions(t *testing.T) {
	// VALIDATES: a spelling that names something else is not a match: the
	// ze-test publication channel, the nftables table ze_test, the ze_chaos_
	// metric prefix, the zetest tag and the product key ze_test_bgp_port. An
	// exception covers its own file only. The map's declaration is outside the
	// sweep.
	// PREVENTS: a Phase 3 gate that can never be green, and an exception that
	// silently hides the same word everywhere.
	root := fixtureRepository(t, map[string]string{
		"internal/le/weekly/answer.go":                  "package weekly\nvar channels = []string{\"ze-test\"}\n",
		"internal/component/firewall/validate_test.go":  "package firewall\nconst table = \"ze_test\"\n",
		"docs/architecture/testing/qemu-integration.md": "nft list table inet ze_test\n",
		"docs/metrics.md":                               "ze_chaos_peers_total\n//go:build zetest\nZE_TEST_BGP_PORT=1790 ze_test_bgp_port\nwhile tier holds\n./le test-unitx\n",
		"docs/elsewhere.md":                             "exec=ze-test bgp\n",
		"internal/le/doc/check/retirednames.go":         "{Retired: \"test-unit\"} ze-chaos ze_test\n",
		"vendor/example.com/x/ze_test.txt":              "ze-chaos ze_test\n",
	})

	report, err := sweepRetired(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		"internal/le/weekly/answer.go",
		"internal/component/firewall/validate_test.go",
		"docs/architecture/testing/qemu-integration.md",
		"docs/metrics.md",
		"internal/le/doc/check/retirednames.go",
		"vendor/example.com/x/ze_test.txt",
	} {
		if found := retiredLinesOf(report, file); len(found) != 0 {
			t.Errorf("%s is declared or a different name, and the sweep listed %v", file, found)
		}
	}
	if !retiredFound(report, "ze-test", "docs/elsewhere.md", 1) {
		t.Error("the ze-test exception of internal/le/weekly/answer.go hid the harness name in another file")
	}
}

// TestRenameMapRowsAreDisjoint checks the shape of the rename map the gate
// reads: every row names both sides, no two rows retire one word sequence, and
// no old word sequence starts a new invocation.
func TestRenameMapRowsAreDisjoint(t *testing.T) {
	rows := Renames()
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		old := strings.Join(row.Old(), " ")
		if row.Retired == "" || row.Command == "" {
			t.Errorf("row %+v names no retired command or no new command", row)
			continue
		}
		if words := len(strings.Fields(row.Command)); words > commandWordsMax {
			t.Errorf("row `le %s`: the new command `le %s` has %d words, the dispatcher reads %d", old, row.Command, words, commandWordsMax)
		}
		if seen[old] {
			t.Errorf("two rows retire `le %s`", old)
		}
		seen[old] = true

		// An old word sequence that starts a new invocation would make the
		// gate refuse a caller who already typed the new name.
		for _, other := range rows {
			fresh := other.New()
			if len(row.Old()) > len(fresh) {
				continue
			}
			if slices.Equal(fresh[:len(row.Old())], row.Old()) {
				t.Errorf("row `le %s` captures the new name `le %s`", old, strings.Join(fresh, " "))
			}
		}
	}

	names := make(map[string]bool, len(Retirements()))
	for _, retired := range Retirements() {
		if retired.Kind == RetiredKindUnspecified {
			t.Errorf("retired name %q declares no kind", retired.Old)
		}
		if retired.Replacement == "" {
			t.Errorf("retired name %q names no replacement", retired.Old)
		}
		if names[retired.Old] {
			t.Errorf("two rows retire %q", retired.Old)
		}
		names[retired.Old] = true
	}
}

func TestRetiredCommandGateGoesRedOnAnInjectedName(t *testing.T) {
	// VALIDATES: AC-16. `le doc check retired-commands` answers 1 when an
	// ordinary file names an old form, names that file and line, and answers
	// 0 on a tree that names none.
	// PREVENTS: a gate that reports a caller and still passes, which is what
	// the sweep was through Phases 1 and 2.
	dirty := fixtureRepository(t, map[string]string{
		"docs/guide.md": "Read the guide.\nThen run `./le test-unit all`.\n",
	})
	answer, code := runRetiredCheck(dirty)
	if code != 1 {
		t.Fatalf("an injected old name answered %d, want 1", code)
	}
	report, ok := answer.(retiredReport)
	if !ok {
		t.Fatalf("the gate answered %T, want retiredReport", answer)
	}
	if !retiredFound(report, "le test-unit", "docs/guide.md", 2) {
		t.Errorf("docs/guide.md:2 is not listed; the file matched %v", retiredLinesOf(report, "docs/guide.md"))
	}
	if !strings.Contains(report.Text(), "  docs/guide.md:2\n") {
		t.Errorf("the rendered report does not name the file and line:\n%s", report.Text())
	}

	clean := fixtureRepository(t, map[string]string{
		"docs/guide.md": "Then run `./le test unit all`.\n",
	})
	if _, code := runRetiredCheck(clean); code != 0 {
		t.Errorf("a tree naming only new forms answered %d, want 0", code)
	}
}
