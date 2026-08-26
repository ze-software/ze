// The fixture checkout the testing-health parity cases judge.
//
// It is a complete small repository instead of a stub for one collector. The
// tool reads its nine inputs in one pass and refuses a missing input. Thus, no
// smaller tree can reach the comparison.
//
// Every result is small and stated here. A case can therefore assert a value
// instead of agreement on an unknown result. The RFC arithmetic must balance.
// The ledger value `gated - both` must equal the annotations in the summaries.
// Otherwise, the tool refuses to publish a non-partition.
//
//	gated       3 (rfc1000) + 2 (rfc1001)          = 5
//	both        1 (rfc1000) + 0 (rfc1001)          = 1
//	remainder                                       = 4
//	annotations gap, not-applicable, single-polarity, gap = 4
//
// Neither a ledger row nor a summary enrolls rfc1002. It states two gated MUSTs.
// Thus, a case fails if either half stops filtering the population by
// enrollment.

package main

import "maps"

// healthFixture answers the fixture tree, one file per entry.
func healthFixture() map[string]string {
	files := map[string]string{}
	for _, part := range [...]map[string]string{
		healthFixtureRepo(), healthFixtureRFC(), healthFixtureTests(), healthFixtureState(),
	} {
		maps.Copy(files, part)
	}
	return files
}

// healthFixtureRepo answers what makes the tree a Ze checkout: a module, the
// feature manifest, and one make file naming a tag universe.
//
// The go directive deliberately names a version before this repository's
// version. The script compiles its detector inside the fixture. A newer required
// toolchain would make the test search for that toolchain.
func healthFixtureRepo() map[string]string {
	return map[string]string{
		"go.mod": "module fixture\n\ngo 1.24\n",
		"feature-gates.txt": "# columns: <build-tag>   <gated-package>\n" +
			"ze_fixture internal/alpha\n",
		"Makefile": "check:\n\tgo test -tags 'ze_fixture' ./...\n",
	}
}

// healthFixtureRFC answers the ledger and the summaries the proof-density
// metric partitions.
func healthFixtureRFC() map[string]string {
	return map[string]string{
		"ai/RFC-REQUIREMENTS.md": "# RFC Requirements\n\n" +
			"| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | " +
			"Nightly-only | State |\n" +
			"|---|---|---|---|---|---|---|---|---|\n" +
			"| `rfc1000` | 3 | 1 | 0 | 2 | 0 | 0 | 0 | **enrolled** |\n" +
			"| `rfc1001` | 2 | 0 | 0 | 2 | 0 | 0 | 0 | **enrolled** |\n" +
			"| `rfc1002` | 2 | 0 | 0 | 0 | 2 | 2 | 0 | backlog |\n",

		"rfc/short/rfc1000.md": "# RFC 1000\n\n" +
			"- [ ] [RFC1000-1-1] [MUST] A speaker MUST do the first thing (§1)\n" +
			"- [ ] [RFC1000-1-2] [MUST] A speaker MUST do the second thing (§1) " +
			"{gap: nothing produces it yet}\n" +
			"- [ ] [RFC1000-1-3] [MUST NOT] A speaker MUST NOT do the third thing (§1) " +
			"{not-applicable: fixture has no such producer}\n" +
			"- [ ] [RFC1000-1-4] [SHOULD] A speaker SHOULD do a fourth thing (§1)\n",

		"rfc/short/rfc1001.md": "# RFC 1001\n\n" +
			"- [ ] [RFC1001-2-1] [SHALL] A speaker SHALL do a thing (§2) " +
			"{single-polarity: negative; the positive side needs a peer}\n" +
			"- [ ] [RFC1001-2-2] [REQUIRED] A speaker is REQUIRED to do a thing (§2) " +
			"{gap: unimplemented}\n",

		"rfc/short/rfc1002.md": "# RFC 1002\n\n" +
			"- [ ] [RFC1002-1-1] [MUST] A speaker MUST do a thing nobody gated (§1)\n" +
			"- [ ] [RFC1002-1-2] [MUST] A speaker MUST do a second such thing (§1)\n",
	}
}

// healthFixtureTests answers the test files the inventory, sensitivity,
// negative-test and adoption metrics read.
//
// One test asserts nothing. One file is behind a tag that no `go test`
// invocation supplies. Other files expect an error, contain a fuzz target, and
// contain an RFC tag. Each is the only input for one metric. If a metric stops
// reading that input, its stated number becomes zero.
func healthFixtureTests() map[string]string {
	return map[string]string{
		"internal/alpha/a_test.go": "package alpha\n\nimport \"testing\"\n\n" +
			"func TestOne(t *testing.T) {\n\tif 1 != 1 {\n\t\tt.Fatal(\"arithmetic\")\n\t}\n}\n\n" +
			"func TestTwo(t *testing.T) {\n\tif 2 != 2 {\n\t\tt.Error(\"arithmetic\")\n\t}\n}\n",

		"internal/alpha/inert_test.go": "package alpha\n\nimport \"testing\"\n\n" +
			"// TestInert executes code and passes unconditionally, which is what the\n" +
			"// sensitivity detector exists to find.\n" +
			"func TestInert(t *testing.T) {\n\t_ = t.Name()\n}\n",

		"internal/alpha/orphan_test.go": "//go:build ze_missing\n\npackage alpha\n\n" +
			"import \"testing\"\n\n" +
			"func TestOrphan(t *testing.T) {\n\tt.Fatal(\"no go test invocation supplies ze_missing\")\n}\n",

		"internal/beta/b_test.go": "package beta\n\nimport (\n\t\"errors\"\n\t\"testing\"\n)\n\n" +
			"func TestThree(t *testing.T) {\n\twantErr := true\n" +
			"\tif got := errors.New(\"x\"); (got != nil) != wantErr {\n\t\tt.Fatal(\"want an error\")\n\t}\n}\n\n" +
			"func FuzzFour(f *testing.F) {\n\tf.Fuzz(func(t *testing.T, in string) {\n" +
			"\t\tif len(in) < 0 {\n\t\t\tt.Fatal(\"impossible\")\n\t\t}\n\t})\n}\n",

		"cmd/tool/c_test.go": "package main\n\nimport \"testing\"\n\n" +
			"// RFC requirement: RFC1000-1-1\n" +
			"func TestFive(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"never\")\n\t}\n}\n\n" +
			"func BenchmarkSix(b *testing.B) {\n\tfor range b.N {\n\t\t_ = 1\n\t}\n}\n",

		"test/one.ci":  "# one\ntime.sleep(1)\ntime.sleep(1)\n",
		"test/two.ci":  "# two\ntime.sleep(2)\n",
		"test/edit.et": "# an editor test\n",
	}
}

// healthFixtureState answers the committed state the ratchets, the trends and
// the known-failure log are read from.
func healthFixtureState() map[string]string {
	return map[string]string{
		"test/.ci-sleep-baseline": "# the fixture's three sleeps\n3\n",

		"test/health/sensitivity-baseline.json": "{\n  \"assert-nothing\": 1,\n  \"tag-orphan\": 1\n}\n",

		"test/mutation/history.ndjson": "" +
			`{"ts":"2026-01-01T00:00:00Z","sha":"aaaaaaaaa","package":"internal/alpha",` +
			`"mutants":10,"killed":6,"score":60.0}` + "\n" +
			`{"ts":"2026-01-02T00:00:00Z","sha":"bbbbbbbbb","package":"internal/beta",` +
			`"mutants":10,"killed":9,"score":90.0}` + "\n",

		"plan/known-failures/README.md": "# How to log a known failure\n",
		"plan/known-failures/RESOLVED.md": "# Resolved\n\n" +
			"### a failure that was fixed\n\nIt was fixed.\n",
		"plan/known-failures/flaky.md": "### a live failure\n\nStill red.\n",
	}
}
