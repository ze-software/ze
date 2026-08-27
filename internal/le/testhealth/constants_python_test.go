// MIGRATION ARTIFACT. Delete this file with scripts/dev/testing_health.py.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- every table, path, bound and
// pattern the port copied out of the script still says what the script says.
// PREVENTS: a shared value that drifts where no output comparison can see it. A
// table both halves read is invisible to a page diff until the day the tree
// contains a document that exercises the entry that moved, and by then the two
// halves disagree in a committed file.
//
// The values are compared BY VALUE through the Python module -- imported, and
// its objects read -- rather than by two literal lists side by side. Two lists
// that were correct when they were typed is the thing this file exists to stop
// relying on.
//
// The PATTERNS are compared by BEHAVIOR rather than by text. Go's regexp
// spells three things differently: the multi-line and dot-all flags are inline
// rather than arguments, and Python's `\s` over a str counts the vertical tab
// where Go's does not. So each pattern is run over a probe table on both sides
// and the verdicts are compared, which is the property the port actually needs.

package testhealth

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// pythonProbeDeadline bounds the one python run this file makes. Importing a
// module and printing a document is milliseconds.
const pythonProbeDeadline = 60 * time.Second

// pythonValues is what the reader below emits: the script's own objects.
type pythonValues struct {
	Paths          map[string]string   `json:"paths"`
	QualityMetrics []string            `json:"quality_metrics"`
	TestRoots      []string            `json:"test_roots"`
	GatedLevels    []string            `json:"gated_levels"`
	StatusValues   []string            `json:"status_values"`
	Statuses       map[string]string   `json:"statuses"`
	Questions      map[string][]string `json:"questions"`
	StatusMark     map[string]string   `json:"status_mark"`
	StatusOrder    map[string]int      `json:"status_order"`
	TableHeader    string              `json:"table_header"`
	StateEnrolled  string              `json:"state_enrolled"`
	MinSamples     int                 `json:"min_samples"`
	Timeout        int                 `json:"timeout"`
	SparkWidth     int                 `json:"spark_width"`
	SparkHeight    int                 `json:"spark_height"`
	Annotations    []string            `json:"annotations"`
	NonShards      []string            `json:"non_shards"`
	KnownFailures  string              `json:"known_failures"`
	Patterns       map[string][]bool   `json:"patterns"`
	SleepBaselines []any               `json:"sleep_baselines"`
}

// pythonReader is the program that reads the script's own objects.
//
// Three values are read out of the SOURCE rather than out of the module,
// because they are literals inside a function and nothing exports them: the
// three annotation kinds, the two bookkeeping file names of the known-failure
// log, and the directory that log lives in.
const pythonReader = `
import inspect, json, re, sys
sys.path.insert(0, "scripts/dev")
import testing_health as th
import verify_wiring_docs as wiring

source = inspect.getsource(th.collect_rfc)
kinds = re.search(r'for kind in \(([^)]*)\)', source).group(1)
shards = re.search(r'non_shards = \{([^}]*)\}', inspect.getsource(th.collect_known_failures)).group(1)
directory = re.search(r'root / "([^"]*)"', inspect.getsource(th.collect_known_failures)).group(1)
spark = inspect.signature(th.sparkline).parameters

probes = json.loads(sys.argv[1])
patterns = {}
for name, probe in probes.items():
    pattern = getattr(th, name)
    patterns[name] = [bool(pattern.search(text)) for text in probe]

print(json.dumps({
  "paths": {"page": th.PAGE, "latest": th.LATEST, "history": th.HISTORY,
            "baseline": th.BASELINE, "quality_baseline": th.QUALITY_BASELINE,
            "ledger": th.RFC_LEDGER, "summaries": th.RFC_SUMMARIES,
            "mutation": th.MUTATION_HISTORY, "sleep": th.SLEEP_BASELINE},
  "quality_metrics": list(th.QUALITY_METRICS),
  "test_roots": list(th.TEST_ROOTS),
  "gated_levels": sorted(th.GATED_LEVELS),
  "status_values": sorted(th.STATUS_VALUES),
  "statuses": {"ok": th.OK, "warn": th.WARN, "unknown": th.UNKNOWN},
  "questions": {k: list(v) for k, v in th.QUESTIONS.items()},
  "status_mark": th.STATUS_MARK,
  "status_order": th.STATUS_ORDER,
  "table_header": th.RFC_TABLE_HEADER,
  "state_enrolled": th.RFC_STATE_ENROLLED,
  "min_samples": th.MIN_SAMPLES,
  "timeout": th.SUBPROCESS_TIMEOUT,
  "spark_width": spark["width"].default,
  "spark_height": spark["height"].default,
  "annotations": [s.strip().strip('"\'') for s in kinds.split(",") if s.strip()],
  "non_shards": sorted(s.strip().strip('"\'') for s in shards.split(",") if s.strip()),
  "known_failures": directory,
  "patterns": patterns,
  "sleep_baselines": [wiring.parse_sleep_baseline(t) for t in json.loads(sys.argv[2])],
}))
`

// patternProbes are the strings each pattern is asked about. Each list carries
// a case the pattern must accept and a case it must refuse, so a probe table
// two broken patterns agree on is not reachable by accident.
var patternProbes = map[string][]string{
	"RFC_ROW": {
		"| `rfc4271` | 12 | 3 | 0 | 9 | 0 | 0 | 0 | **enrolled** |",
		"| `rfc4271` | 12 | 3 | 0 | 9 | 0 | 0 | **enrolled** |",
		"| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | Nightly-only | State |",
	},
	"RFC_LEVEL": {
		"- [ ] [RFC4271-1-1] [MUST] a thing (§1)",
		"- [x] [RFC4271-1-2] [MUST NOT] a thing (§1)",
		"- [ ] [RFC4271-1-3] [should] a thing (§1)",
		"  - [ ] [RFC4271-1-4] [MUST] an indented thing",
	},
	"TEST_FUNC": {
		"func TestOne(t *testing.T) {}",
		"package x\n\nfunc TestTwo(t *testing.T) {}",
		"\tfunc TestThree(t *testing.T) {}",
		"func testLower(t *testing.T) {}",
		"func TestingHelper() {}",
	},
	"FUZZ_FUNC":  {"func FuzzOne(f *testing.F) {}", "func fuzzOne(f *testing.F) {}"},
	"BENCH_FUNC": {"func BenchmarkOne(b *testing.B) {}", "func Benchmarking() {}"},
	"NEGATIVE_ASSERT": {
		"if wantErr {",
		"require.Error(t, err)",
		"if err == nil {",
		"if err  ==\tnil {",
		"if !errors.Is(err, target) {",
		"if err != nil {\n\t\tt.Fatal(err)\n\t}",
		"// wantErr is discussed here",
		"assert.NoError(t, err)",
	},
	"GO_LINE_COMMENT":  {"code // trailing", "code /* not a line comment */"},
	"GO_BLOCK_COMMENT": {"a /* one\ntwo */ b", "a // one\nb"},
}

// goPatterns maps each probed name to this package's own pattern.
var goPatterns = map[string]*regexp.Regexp{
	"RFC_ROW":          rfcRow,
	"RFC_LEVEL":        rfcLevel,
	"TEST_FUNC":        testFunc,
	"FUZZ_FUNC":        fuzzFunc,
	"BENCH_FUNC":       benchFunc,
	"NEGATIVE_ASSERT":  negativeAssert,
	"GO_LINE_COMMENT":  goLineComment,
	"GO_BLOCK_COMMENT": goBlockComment,
}

// sleepBaselineProbes are the delta ledgers both parsers are asked about: a
// plain absolute, a comment-only file that leaves the ratchet unenforced, a
// signed delta ledger, and a file whose only lines are prose.
var sleepBaselineProbes = []string{
	"3\n",
	"# nothing but a comment\n",
	"# start\n125\n-3\n+1\n",
	"not a number\n",
	"",
}

func readPythonValues(t *testing.T) pythonValues {
	t.Helper()

	root := repositoryRoot(t)
	probes, err := json.Marshal(patternProbes)
	if err != nil {
		t.Fatalf("encoding the probe table: %v", err)
	}
	baselines, err := json.Marshal(sleepBaselineProbes)
	if err != nil {
		t.Fatalf("encoding the baseline table: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), pythonProbeDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pythonReader, string(probes), string(baselines)) // #nosec G204 -- this file's own program and tables
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reading the script's values: %v", err)
	}

	var values pythonValues
	if err := json.Unmarshal(out, &values); err != nil {
		t.Fatalf("decoding the script's values: %v", err)
	}
	return values
}

// repositoryRoot answers this checkout, walking up from the test's own
// directory rather than calling lepath.Root: a case may point ZE_REPO_ROOT at a
// fixture, and the script lives here.
func repositoryRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the checkout: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "feature-gates.txt")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no checkout root above %s", dir)
		}
		dir = parent
	}
}

func TestEveryPathTheScriptReadsIsTheSamePath(t *testing.T) {
	values := readPythonValues(t)
	cases := map[string]string{
		"page": Page, "latest": Latest, "history": History,
		"baseline": Baseline, "quality_baseline": QualityBaseline,
		"ledger": rfcLedger, "summaries": rfcSummaries,
		"mutation": mutationHistory, "sleep": sleepBaseline,
	}
	if len(cases) != len(values.Paths) {
		t.Errorf("the script names %d path(s) and this package compares %d",
			len(values.Paths), len(cases))
	}
	for name, want := range cases {
		if got := values.Paths[name]; got != want {
			t.Errorf("the %s path is %q in the script and %q here", name, got, want)
		}
	}
	if values.KnownFailures != knownFailures {
		t.Errorf("the known-failure log is %q in the script and %q here",
			values.KnownFailures, knownFailures)
	}
}

func TestEveryTableTheScriptCarriesHasTheSameEntriesInTheSameOrder(t *testing.T) {
	values := readPythonValues(t)

	assertSame(t, "QUALITY_METRICS", values.QualityMetrics, qualityMetrics[:])
	assertSame(t, "TEST_ROOTS", values.TestRoots, testRoots[:])
	assertSame(t, "the annotation kinds", values.Annotations, annotationKinds[:])
	assertSame(t, "the known-failure bookkeeping files", values.NonShards,
		[]string{"README.md", "RESOLVED.md"})

	levels := make([]string, 0, len(gatedLevels))
	for level := range gatedLevels {
		levels = append(levels, level)
	}
	assertSameSet(t, "GATED_LEVELS", values.GatedLevels, levels)

	statuses := make([]string, 0, len(statusValues))
	for status := range statusValues {
		statuses = append(statuses, status)
	}
	assertSameSet(t, "STATUS_VALUES", values.StatusValues, statuses)
}

func TestEveryBoundAndSpellingTheScriptStatesIsTheSame(t *testing.T) {
	values := readPythonValues(t)

	if values.TableHeader != rfcTableHeader {
		t.Errorf("the pinned ledger header is\n  %q in the script\n  %q here",
			values.TableHeader, rfcTableHeader)
	}
	if values.StateEnrolled != rfcStateEnrolled {
		t.Errorf("the enrolled marker is %q in the script and %q here",
			values.StateEnrolled, rfcStateEnrolled)
	}
	if values.MinSamples != minSamples {
		t.Errorf("the trend floor is %d in the script and %d here", values.MinSamples, minSamples)
	}
	if int64(values.Timeout)*int64(time.Second) != int64(subprocessDeadline) {
		t.Errorf("the subprocess bound is %ds in the script and %s here",
			values.Timeout, subprocessDeadline)
	}
	for name, want := range map[string]string{
		"ok": statusOK, "warn": statusWarn, "unknown": statusUnknown,
	} {
		if got := values.Statuses[name]; got != want {
			t.Errorf("the %s status is spelled %q in the script and %q here", name, got, want)
		}
	}
	if values.SparkWidth != sparklineWidth || values.SparkHeight != sparklineHeight {
		t.Errorf("the sparkline box is %dx%d in the script and %dx%d here",
			values.SparkWidth, values.SparkHeight, sparklineWidth, sparklineHeight)
	}
}

func TestThePageHeadingsAndStatusOrderAreTheSame(t *testing.T) {
	values := readPythonValues(t)

	if len(values.Questions) != len(questions) {
		t.Fatalf("the script asks %d question(s) and this package renders %d",
			len(values.Questions), len(questions))
	}
	for _, asked := range questions {
		pair, held := values.Questions[asked.key]
		if !held {
			t.Errorf("the script asks no question keyed %q", asked.key)
			continue
		}
		if len(pair) != 2 || pair[0] != asked.title || pair[1] != asked.prompt {
			t.Errorf("question %s is %v in the script and (%q, %q) here",
				asked.key, pair, asked.title, asked.prompt)
		}
	}
	for status, want := range values.StatusMark {
		if got := statusMark[status]; got != want {
			t.Errorf("the %s marker is %q in the script and %q here", status, want, got)
		}
	}
	for status, want := range values.StatusOrder {
		if got := statusOrder[status]; got != want {
			t.Errorf("the %s sort key is %d in the script and %d here", status, want, got)
		}
	}
	if len(values.StatusMark) != len(statusMark) || len(values.StatusOrder) != len(statusOrder) {
		t.Errorf("the script keys %d marker(s) and %d sort key(s); this package keys %d and %d",
			len(values.StatusMark), len(values.StatusOrder), len(statusMark), len(statusOrder))
	}
}

func TestEveryPatternAcceptsAndRefusesTheSameStrings(t *testing.T) {
	values := readPythonValues(t)

	if len(values.Patterns) != len(goPatterns) {
		t.Fatalf("the script was probed on %d pattern(s) and this package holds %d",
			len(values.Patterns), len(goPatterns))
	}
	for name, verdicts := range values.Patterns {
		pattern, held := goPatterns[name]
		if !held {
			t.Errorf("this package has no counterpart for the script's %s", name)
			continue
		}
		probes := patternProbes[name]
		accepted := 0
		for index, want := range verdicts {
			got := pattern.MatchString(probes[index])
			if got != want {
				t.Errorf("%s over %q: the script says %v and this package says %v",
					name, probes[index], want, got)
			}
			if want {
				accepted++
			}
		}
		// A probe table every pattern refuses would compare equal while proving
		// nothing about what the pattern matches.
		if accepted == 0 || accepted == len(verdicts) {
			t.Errorf("%s accepted %d of %d probes, so the table decides nothing",
				name, accepted, len(verdicts))
		}
	}
}

func TestTheSleepBaselineParserReadsTheSameLedgers(t *testing.T) {
	values := readPythonValues(t)

	if len(values.SleepBaselines) != len(sleepBaselineProbes) {
		t.Fatalf("the script answered %d ledger(s) and %d were asked",
			len(values.SleepBaselines), len(sleepBaselineProbes))
	}
	active := 0
	for index, want := range values.SleepBaselines {
		got, enforced := parseSleepBaseline(sleepBaselineProbes[index])
		if want == nil {
			if enforced {
				t.Errorf("over %q the script leaves the ratchet unenforced and this package "+
					"answers a ceiling of %d", sleepBaselineProbes[index], got)
			}
			continue
		}
		active++
		number, ok := want.(float64)
		if !ok || int(number) != got {
			t.Errorf("over %q the script answers %v and this package answers %d",
				sleepBaselineProbes[index], want, got)
		}
	}
	if active == 0 {
		t.Errorf("no probe produced a ceiling, so the comparison could not have failed")
	}
}

// assertSame fails unless two ordered tables carry the same entries in the same
// order.
func assertSame(t *testing.T, what string, want, got []string) {
	t.Helper()

	if strings.Join(want, "\x00") != strings.Join(got, "\x00") {
		t.Errorf("%s is\n  %v in the script\n  %v here", what, want, got)
	}
}

// assertSameSet fails unless two unordered tables carry the same entries. The
// script's side arrives sorted, so this side is sorted to match.
func assertSameSet(t *testing.T, what string, want, got []string) {
	t.Helper()

	sorted := make([]string, len(got))
	copy(sorted, got)
	sortStrings(sorted)
	assertSame(t, what, want, sorted)
}

// sortStrings orders a table in place.
func sortStrings(values []string) {
	for outer := 1; outer < len(values); outer++ {
		for inner := outer; inner > 0 && values[inner] < values[inner-1]; inner-- {
			values[inner], values[inner-1] = values[inner-1], values[inner]
		}
	}
}
