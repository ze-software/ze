// Related: parity.go -- every producer golden row is parsed and judged in Go
// Related: fixtures.go -- every behavioral category has two-sided native probes
//
// VALIDATES: the native hook selftest maps the complete Python producer
// population and each fixture category distinguishes an allowed shape from the
// failure it exists to catch.
// PREVENTS: an omitted golden row, an omitted category, a constant checker, or
// a success after a producer file disappears.
package hookcheck

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/lepath"
)

func hookCheckout(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	return root
}

func TestParityMapsEveryProducerGoldenRow(t *testing.T) {
	results, population, golden := runParity(hookCheckout(t))
	want := Population{
		Bash:          bashRowsExpected,
		WriteEdit:     writeEditRowsExpected,
		Weakening:     weakeningRowsExpected,
		PostWriteEdit: postWriteEditRowsExpected,
	}
	if !reflect.DeepEqual(population, want) {
		t.Fatalf("population = %+v, want %+v", population, want)
	}
	if len(golden) != bashRowsExpected+writeEditRowsExpected+weakeningRowsExpected+
		postWriteEditRowsExpected {
		t.Fatalf("golden cases = %d, want the full producer population", len(golden))
	}
	for _, fixture := range golden {
		if !fixture.Passed {
			t.Errorf("%s %q: native code %d, expected %d", fixture.Table, fixture.Name,
				fixture.NativeCode, fixture.ExpectedCode)
		}
	}
	for _, result := range results {
		if !result.Passed {
			t.Errorf("%s: %s", result.Name, result.Message)
		}
	}
}

func TestBashNativeStatementBoundariesMatchDispatcher(t *testing.T) {
	cases := []struct {
		name    string
		command string
		code    int
	}{
		{
			name: "later unbounded loop is still refused",
			command: "timeout 60 bash -c 'until [ -f a ]; do sleep 1; done'; " +
				"while true; do sleep 5; done",
			code: 2,
		},
		{
			name:    "loop words in a grep pattern are text",
			command: "grep -rn 'until ! pgrep' ai/rules",
			code:    0,
		},
		{
			name:    "run-shaped quoted scratch redirect is refused",
			command: `bash -c "make ze-doc-verify > tmp/out.log"`,
			code:    2,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := bashCode(test.command); got != test.code {
				t.Errorf("bashCode(%q) = %d, want %d", test.command, got, test.code)
			}
		})
	}
}

func TestEveryBehavioralCategoryDiscriminates(t *testing.T) {
	for _, category := range fixtureCategories {
		t.Run(category.name, func(t *testing.T) {
			if !categoryVerdict(category.name, category.allow) {
				t.Errorf("allow probe was refused: %q", category.allow)
			}
			if categoryVerdict(category.name, category.refuse) {
				t.Errorf("refusal probe was allowed: %q", category.refuse)
			}
		})
	}
}

func TestEveryBehavioralCategoryFailsWhenItsProducerMappingIsOmitted(t *testing.T) {
	for _, category := range fixtureCategories {
		t.Run(category.name, func(t *testing.T) {
			root := t.TempDir()
			owner := filepath.Join(root, filepath.FromSlash(category.owner))
			if err := os.MkdirAll(filepath.Dir(owner), 0o750); err != nil {
				t.Fatalf("create owner directory: %v", err)
			}
			if err := os.WriteFile(owner, []byte(category.evidence), 0o600); err != nil {
				t.Fatalf("write owner: %v", err)
			}

			var tb textbuf.Buffer
			mapping := tb.Byte('\'').Str(category.name).Str("': ").Str(category.runner).String()
			tb.Reset()
			body := tb.Str(mapping).Byte('\n').Str("def ").Str(category.runner).Byte('(').String()
			if result := checkFixtureCategory(root, body, category); !result.Passed {
				t.Fatalf("complete category failed: %s", result.Message)
			}
			omitted := strings.Replace(body, mapping, "", 1)
			if result := checkFixtureCategory(root, omitted, category); result.Passed || result.Code != 2 {
				t.Fatalf("omitted mapping result = %+v, want code-2 failure", result)
			}
		})
	}
}

func TestFixtureProducerMapsEveryCategoryAndCheckCallsite(t *testing.T) {
	results, checks := runFixtures(hookCheckout(t))
	if checks != fixtureChecksExpected {
		t.Fatalf("fixture check callsites = %d, want %d", checks, fixtureChecksExpected)
	}
	if len(results) != len(fixtureCategories)+1 {
		t.Fatalf("fixture results = %d, want population plus %d categories", len(results), len(fixtureCategories))
	}
	for _, result := range results {
		if !result.Passed {
			t.Errorf("%s: %s", result.Name, result.Message)
		}
	}
}

func TestRunFailsClosedWhenProducerPopulationIsMissing(t *testing.T) {
	root := t.TempDir()
	report, code := Run(root)
	if code != 2 {
		t.Fatalf("Run code = %d, want 2", code)
	}
	if len(report.Results) == 0 || report.Results[0].Passed {
		t.Fatalf("missing producer report = %+v", report)
	}
}

func TestActionIsTheGatelessUnitVerb(t *testing.T) {
	listing := Actions()
	if listing.Area != area || len(listing.Actions) != 1 {
		t.Fatalf("listing = %+v", listing)
	}
	row := listing.Actions[0]
	if row.Verb != "unit" || row.Gate != "" || row.Writes || len(row.Forks) != 0 {
		t.Fatalf("unit action = %+v", row)
	}
	if Subs() != "unit" {
		t.Fatalf("Subs = %q, want unit", Subs())
	}
	if !registry.HasLocal(leroot.CommandPath(area)) {
		t.Fatalf("importing hookcheck did not register %q", area)
	}
}

func TestGoldenParserRejectsAnUnterminatedPopulation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(parityProducer))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create producer directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("BASH_GOLDEN = {\n\"x\": 0,\n"), 0o600); err != nil {
		t.Fatalf("write producer: %v", err)
	}
	rows, err := parseGolden([]byte("BASH_GOLDEN = {\n\"x\": 0,\n"), "BASH_GOLDEN")
	if err == nil || rows != nil {
		t.Fatalf("unterminated table rows=%v err=%v", rows, err)
	}
}
