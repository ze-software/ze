package doccheck

import (
	"strconv"
	"strings"
	"testing"
)

// retiredFound reports whether the row whose old form is old lists file:line.
func retiredFound(report RetiredReport, old, file string, line int) bool {
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
func retiredLinesOf(report RetiredReport, file string) []string {
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
		"docs/elsewhere.md":                             "the ze-test binary\n",
		"internal/le/leroot/retired.go":                 "{Retired: \"test-unit\"} ze-chaos ze_test\n",
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
		"internal/le/leroot/retired.go",
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
