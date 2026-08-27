// Design: ai/rules/protocol.md -- what this lens says about ITSELF
//
// selftest.go is the one mode of this tool that may fail. Report mode always
// exits 0, so a classifier that stopped working and a tree that conforms print
// the same summary line: these fixtures are the only thing that tells the two
// apart.
//
// The case tables are declared ONCE and read twice: the selftest action runs
// them, and the package test runs the same rows so a failure names the case
// rather than a line number.

package protocolskeleton

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
)

// classifyCase is one classification the lens must make. The table covers each
// of the five classes and the two pairs that are easy to confuse: ike/wire is
// an exception and ospf/wire is a domain module of the same name.
type classifyCase struct {
	Protocol string
	Module   string
	Want     string
}

var selftestCases = []classifyCase{
	{protoBFD, modulePacket, classCanonical},
	{protoBFD, "session", classRFCState},
	{protoISIS, "adjacency", classRFCState},
	{protoOSPF, "neighbor", classRFCState},
	{protoBGP, "fsm", classRFCState},
	{protoOSPF, "v3", classVersion},
	{protoBGP, "wireu", classLegacy},
	{protoIKE, moduleWire, classLegacy},
	{protoOSPF, moduleWire, classDomain},
	{protoISIS, "lsdb", classDomain},
}

// walkCase is one fact about reading a tree. Each answers an empty string when
// the fixture behaved as declared, and what went wrong otherwise.
type walkCase struct {
	Name  string
	Check func(root string) string
}

// The fixture the walk cases share: a protocol with a subpackage, a testdata
// directory that must be skipped, and a flat protocol carrying only yang.
var fixtureDirs = []string{
	"internal/plugins/demo/packet",
	"internal/plugins/demo/yang",
	"internal/plugins/demo/testdata",
	"internal/plugins/flat/yang",
}

var walkCases = []walkCase{
	{Name: "testdata is skipped", Check: func(root string) string {
		report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
		if err != nil {
			return err.Error()
		}
		var tb textbuf.Buffer
		names := make([]string, 0, len(report.Protocols[0].Modules))
		for _, module := range report.Protocols[0].Modules {
			names = append(names, module.Name)
		}
		if len(names) != 2 || names[0] != modulePacket || names[1] != moduleYang {
			return tb.Str("the walk read ").Join(names, ", ").Str(", want packet and yang").String()
		}
		return ""
	}},
	{Name: "a protocol with a subpackage is not single-package", Check: func(root string) string {
		report, err := Build(root, []Protocol{{Name: "demo", Root: "internal/plugins/demo"}})
		if err != nil {
			return err.Error()
		}
		if report.Protocols[0].SinglePackage {
			return "demo carries packet/ and reads as single-package"
		}
		return ""
	}},
	{Name: "a root carrying only yang is single-package", Check: func(root string) string {
		report, err := Build(root, []Protocol{{Name: "flat", Root: "internal/plugins/flat"}})
		if err != nil {
			return err.Error()
		}
		if !report.Protocols[0].SinglePackage {
			return "flat carries only yang/ and does not read as single-package"
		}
		return ""
	}},
	{Name: "a manifest row whose root is gone is flagged", Check: func(root string) string {
		report, err := Build(root, []Protocol{{Name: "gone", Root: "internal/plugins/gone"}})
		if err != nil {
			return err.Error()
		}
		switch {
		case !report.Protocols[0].Missing:
			return "a root that does not exist was not flagged"
		case report.Protocols[0].SinglePackage:
			return "a missing root reads as single-package"
		case !strings.Contains(report.Text(), "MISSING roots: gone"):
			return "the summary line does not name the missing root"
		}
		return ""
	}},
	{Name: "report mode over the real tree renders a summary", Check: func(string) string {
		tree, err := treeOfRecord()
		if err != nil {
			return err.Error()
		}
		report, err := Build(tree, Manifest())
		if err != nil {
			return err.Error()
		}
		if !strings.Contains(report.Text(), "protocol-skeleton advisory:") {
			return "the summary line is missing from a real-tree report"
		}
		return ""
	}},
}

// Selftest runs every declared case against the real classifier.
func Selftest() leroot.SelftestReport { return runCases(Classify) }

// runCases runs the tables against the classifier given. The classifier is a
// parameter so a test can drive a broken one through the same path the action
// runs, which is what proves these cases would catch it.
func runCases(classify func(protocol, module string) string) leroot.SelftestReport {
	results := make([]leroot.SelftestResult, 0, len(selftestCases)+len(walkCases))

	for _, item := range selftestCases {
		var tb textbuf.Buffer
		name := tb.Str(item.Protocol).Byte('/').Str(item.Module).Str(" is ").Str(item.Want).String()
		got := classify(item.Protocol, item.Module)
		if got == item.Want {
			results = append(results, leroot.Pass(name))
			continue
		}
		tb.Reset()
		results = append(results, leroot.Fail(name, tb.Str(name).Str(", classified ").Str(got).String()))
	}

	root, err := fixture()
	if err != nil {
		// Fail closed: the walk cases could not run, so each of them says so
		// rather than being counted as passed or quietly dropped.
		for _, item := range walkCases {
			results = append(results, leroot.Fail(item.Name, err.Error()))
		}
		return report(results)
	}
	defer os.RemoveAll(root) //nolint:errcheck // a temporary fixture

	for _, item := range walkCases {
		if detail := item.Check(root); detail != "" {
			results = append(results, leroot.Fail(item.Name, detail))
			continue
		}
		results = append(results, leroot.Pass(item.Name))
	}
	return report(results)
}

// report wraps the rows in the two verdict lines this tool's page uses.
func report(results []leroot.SelftestResult) leroot.SelftestReport {
	return leroot.NewSelftestReport(
		"protocol-skeleton selftest OK",
		"protocol-skeleton selftest FAILED:",
		results...)
}

// fixture builds the temporary tree the walk cases read.
func fixture() (string, error) {
	root, err := os.MkdirTemp("", "protocol-skeleton-selftest")
	if err != nil {
		return "", err
	}
	for _, rel := range fixtureDirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o750); err != nil {
			os.RemoveAll(root) //nolint:errcheck // the error being answered is the one that matters
			return "", err
		}
	}
	return root, nil
}
