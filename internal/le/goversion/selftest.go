// Design: docs/architecture/core-design.md -- the gate, proved against fixtures
// Overview: goversion.go -- the comparison these cases drive
//
// selftest.go proves the comparison independent of the live tree: fifteen
// cases, each naming one thing the gate must say. A gate whose reader broke and
// a gate over a tree that agrees print the same page, and this is what tells
// them apart.
//
// The table is declared ONCE and read twice: `le go-version selftest` runs it,
// and goversion_test.go runs the same rows so a failure names the case rather
// than a line of output.

package goversion

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
)

// selftestDeclared is the version every synthetic case is judged against. It is
// its own number rather than the tree's, so a case says what the comparison
// does and never what this checkout holds.
const selftestDeclared = "1.27"

// dockerfileFixture and goFixture are the paths the synthetic cases name. The
// paths carry no meaning beyond their KIND, which is what picks the reader.
const (
	dockerfileFixture = "Dockerfile.fixture"
	goFixture         = "fixture.go"
)

// selftestCase is one synthetic comparison and what it must answer.
type selftestCase struct {
	// name is the word a failure names.
	name string
	// run answers the detail of a failure, or "" when the case behaved.
	run func() string
}

// judge compares one synthetic carrier against selftestDeclared, so a case
// names content rather than a tree.
func judge(carrier, body string) Result {
	result := Result{Declared: selftestDeclared}
	if isDockerfile(carrier) {
		result.judgeDockerfile(carrier, body)
	} else {
		result.judgeGoSource(carrier, body)
	}
	result.Valid = len(result.Findings) == 0
	return result
}

// selftestCases is the whole selftest. Cases 8 to 10 are about the EXCLUSION
// rather than the comparison: a stage that builds other sources costs nothing
// when it is passed over and costs the gate its meaning when it is judged.
// Cases 14 and 15 are about the Go READER: a comment that quotes an image is a
// sentence, and a raw literal is a carrier, which is the pair a regular
// expression over the file text gets backwards in both directions.
var selftestCases = []selftestCase{
	{"declared-minor-read", func() string {
		for _, body := range []string{"module x\n\ngo 1.27.0\n", "module x\n\ngo 1.27\n"} {
			minor, err := declaredMinor(body)
			if err != nil {
				return describeStrings("case1 (declared minor read): ", []string{body, err.Error()})
			}
			if minor != selftestDeclared {
				return describeStrings("case1 (declared minor read): got ", []string{minor})
			}
		}
		return ""
	}},
	{"one-go-directive-required", func() string {
		if _, err := declaredMinor("module x\n"); err == nil {
			return "case2 (one go directive required): a go.mod with no directive was read as a version"
		}
		if _, err := declaredMinor("module x\ngo 1.27.0\ngo 1.26.0\n"); err == nil {
			return "case2 (one go directive required): two directives were read as one version"
		}
		return ""
	}},
	{"agrees", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.27-alpine AS builder\nCOPY go.mod go.sum ./\n")
		if result.Carriers != 1 || len(result.Findings) != 0 {
			return describe("case3 (agrees): unexpected result ", result)
		}
		return ""
	}},
	{"patch-tag-agrees", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.27.1-alpine AS builder\nCOPY . .\n")
		if result.Carriers != 1 || len(result.Findings) != 0 {
			return describe("case4 (patch tag agrees): unexpected result ", result)
		}
		return ""
	}},
	{"minor-drift", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.26-alpine AS builder\nCOPY . .\n")
		if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonMismatch {
			return describe("case5 (minor drift): unexpected result ", result)
		}
		return ""
	}},
	{"unreadable-tag", func() string {
		for _, base := range []string{"golang:latest", "golang:alpine", "golang:${GO_VERSION}", "golang"} {
			result := judge(dockerfileFixture, "FROM "+base+"\nCOPY go.mod ./\n")
			if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonUnreadableTag {
				return describe("case6 (unreadable tag) for "+base+": unexpected result ", result)
			}
		}
		return ""
	}},
	{"unreadable-base", func() string {
		result := judge(dockerfileFixture,
			"FROM alpine:3.21 AS builder\nCOPY go.mod ./\n")
		if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonUnreadableBase {
			return describe("case7 (unreadable base): unexpected result ", result)
		}
		return ""
	}},
	{"remote-install-not-judged", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.23-alpine AS builder\nRUN go install example.com/other@v1.0.0\n")
		if result.Carriers != 0 || len(result.Findings) != 0 || len(result.Excluded) != 1 {
			return describe("case8 (remote install not judged): unexpected result ", result)
		}
		if result.Excluded[0].Reason != ExcludedNoModuleCopy {
			return describe("case8 (remote install not judged): unexpected reason ", result)
		}
		return ""
	}},
	{"one-file-copy-is-not-a-module-copy", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.26-alpine AS fixture\nCOPY tools/entrypoint/main.go .\n")
		if result.Carriers != 0 || len(result.Findings) != 0 || len(result.Excluded) != 1 {
			return describe("case9 (one file copy is not a module copy): unexpected result ", result)
		}
		return ""
	}},
	{"stage-copy-is-not-a-module-copy", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.27-alpine AS builder\nCOPY . .\nFROM alpine:3.21\nCOPY --from=builder /ze /ze\n")
		if result.Carriers != 1 || len(result.Findings) != 0 || len(result.Excluded) != 0 {
			return describe("case10 (stage copy is not a module copy): unexpected result ", result)
		}
		return ""
	}},
	{"continuation-joined", func() string {
		result := judge(dockerfileFixture,
			"FROM golang:1.26-alpine AS builder\nCOPY \\\n  go.mod \\\n  go.sum ./\n")
		if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonMismatch {
			return describe("case11 (continuation joined): unexpected result ", result)
		}
		return ""
	}},
	{"go-literal-drift", func() string {
		result := judge(goFixture, "package x\n\nconst image = \"golang:1.26\"\n")
		if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonMismatch {
			return describe("case12 (go literal drift): unexpected result ", result)
		}
		if result.Findings[0].Line != 3 {
			return detail("case12 (go literal drift): the finding names line ", result.Findings[0].Line)
		}
		return ""
	}},
	{"another-image-is-not-a-carrier", func() string {
		result := judge(goFixture, "package x\n\nconst image = \"mygolang:1.2\"\n")
		if result.Carriers != 0 || len(result.Findings) != 0 {
			return describe("case13 (another image is not a carrier): unexpected result ", result)
		}
		return ""
	}},
	{"a-comment-is-prose", func() string {
		result := judge(goFixture,
			"package x\n\n// The image is `golang:1.26`, and \"golang:1.26\" names it too.\nconst x = 1\n")
		if result.Carriers != 0 || len(result.Findings) != 0 {
			return describe("case14 (a comment is prose): unexpected result ", result)
		}
		return ""
	}},
	{"a-raw-literal-is-a-carrier", func() string {
		result := judge(goFixture, "package x\n\nconst image = `golang:1.26`\n")
		if len(result.Findings) != 1 || result.Findings[0].Reason != ReasonMismatch {
			return describe("case15 (a raw literal is a carrier): unexpected result ", result)
		}
		return ""
	}},
}

// detail answers a lead-in followed by one number.
func detail(lead string, value int) string {
	var tb textbuf.Buffer
	return tb.Str(lead).Int(int64(value)).String()
}

// describe answers a lead-in followed by what the comparison produced, so a
// failing case says the whole result rather than the one field it tested.
func describe(lead string, result Result) string {
	var tb textbuf.Buffer
	tb.Str(lead).Str("carriers=").Int(int64(result.Carriers)).Str(" findings=[")
	for index, finding := range result.Findings {
		if index > 0 {
			tb.Byte(' ')
		}
		tb.Byte('{').Str(finding.Carrier).Byte(':').Int(int64(finding.Line)).Byte(' ').
			Str(finding.Names).Byte(' ').Str(finding.Reason).Byte('}')
	}
	tb.Str("] excluded=[")
	for index, excluded := range result.Excluded {
		if index > 0 {
			tb.Byte(' ')
		}
		tb.Byte('{').Str(excluded.Names).Byte(' ').Str(excluded.Reason).Byte('}')
	}
	tb.Byte(']')
	return tb.String()
}

// describeStrings answers a lead-in followed by a list of words.
func describeStrings(lead string, words []string) string {
	var tb textbuf.Buffer
	return tb.Str(lead).Byte('[').Join(words, " ").Byte(']').String()
}

// Selftest runs every case and answers one result per case.
func Selftest() leroot.SelftestReport {
	results := make([]leroot.SelftestResult, 0, len(selftestCases))
	for _, testCase := range selftestCases {
		if failure := testCase.run(); failure != "" {
			results = append(results, leroot.Fail(testCase.name, failure))
			continue
		}
		results = append(results, leroot.Pass(testCase.name))
	}
	return leroot.NewSelftestReport("go-version: selftest OK", "go-version: SELFTEST FAILED", results...)
}

// runSelftest is the `selftest` action. A failure answers 2: a selftest that did
// not hold is a broken gate rather than a tree that drifted, and a caller reads
// the two apart.
func runSelftest() (any, int) {
	report := Selftest()
	return report, report.Code(2)
}
