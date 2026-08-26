// VALIDATES: spec-le-is-a-ze-binary AC-11. Every word list and bound copied from
// scripts/dev/ste_check.py retains its Python value.
// PREVENTS: drift that output comparison cannot see. Both implementations agree
// on the corpus TODAY. A changed hedge, nominalization, or sentence bound stays
// invisible until a document reaches that case. Then only one implementation
// reports it.
//
// The comparison includes ORDER, not only membership. Equal-length entries use
// dictionary declaration order. Thus, `it seems` and `seems to` determine which
// phrase a finding names.
//
// This file is a MIGRATION artifact and dies with the script at step 14 of the
// spec. It reads the module rather than its source text, so a table rewritten
// as a comprehension is still compared by value.

package ste

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// pythonTimeout bounds the one interpreter this file starts. It imports one
// module and prints a dictionary, which is milliseconds.
const pythonTimeout = 60 * time.Second

// pythonTables is what the interpreter answers: every table this package
// copied, named as the Python names it.
type pythonTables struct {
	PlainWords          [][]string `json:"plain_words"`
	PlainWordIdentifier []string   `json:"plain_word_identifiers"`
	TermSets            [][]any    `json:"term_sets"`
	Hedges              [][]string `json:"hedges"`
	HedgePhrases        [][]string `json:"hedge_phrases"`
	Nominalizations     [][]string `json:"nominalizations"`
	Marketing           []string   `json:"marketing"`
	PhrasalVerbs        [][]string `json:"phrasal_verbs"`
	LatinAbbreviations  [][]string `json:"latin_abbreviations"`
	NotGerund           []string   `json:"not_gerund"`
	Abbreviations       []string   `json:"abbreviations"`
	GoMarkers           []string   `json:"go_markers"`
	GeneratedMarkers    []string   `json:"generated_markers"`
	ExcludeDirs         []string   `json:"exclude_dirs"`
	ExcludeGlobs        []string   `json:"exclude_globs"`
	DefaultGlobs        []string   `json:"default_globs"`
	Surfaces            []string   `json:"surfaces"`
	Habits              []string   `json:"habits"`
	MaxProcedural       int        `json:"max_procedural"`
	MaxDescriptive      int        `json:"max_descriptive"`
	MaxSentences        int        `json:"max_sentences"`
	MaxReport           int        `json:"max_report"`
	ExcerptCut          int        `json:"excerpt_cut"`
	ExitHabitGrew       int        `json:"exit_habit_grew"`
	Held                string     `json:"held"`
	SentenceClosers     string     `json:"sentence_closers"`
	RuleLine            string     `json:"rule_line"`
	HeaderPad           string     `json:"header_pad"`
	MinGoWords          int        `json:"min_go_words"`
	MinYangWords        int        `json:"min_yang_words"`
	HeadLines           int        `json:"head_lines"`
}

// pythonDump is the program the interpreter runs. It imports the module and
// prints its tables, so a value the Python computes rather than spells is still
// compared.
const pythonDump = `
import inspect, json, re, sys
sys.path.insert(0, sys.argv[1])
import ste_check as S
source = inspect.getsource(S)
def pairs(d):
    return [[k, v] for k, v in d.items()]
def one(pattern, cast=str):
    return cast(re.search(pattern, source).group(1))
def unescaped(pattern):
    return re.search(pattern, source).group(1).encode().decode("unicode_escape")
json.dump({
    "plain_words": pairs(S.PLAIN_WORDS),
    "plain_word_identifiers": sorted(S.PLAIN_WORD_IDENTIFIERS),
    "term_sets": [[c, list(r)] for c, r in S.TERM_SETS],
    "hedges": pairs(S.HEDGES),
    "hedge_phrases": pairs(S.HEDGE_PHRASES),
    "nominalizations": pairs(S.NOMINALIZATIONS),
    "marketing": list(S.MARKETING),
    "phrasal_verbs": pairs(S.PHRASAL_VERBS),
    "latin_abbreviations": pairs(S.LATIN_ABBREVIATIONS),
    "not_gerund": sorted(S.NOT_GERUND),
    "abbreviations": list(S.ABBREVIATIONS),
    "go_markers": list(S.GO_MARKERS),
    "generated_markers": list(S.GENERATED_MARKERS),
    "exclude_dirs": list(S.EXCLUDE_DIRS),
    "exclude_globs": list(S.EXCLUDE_GLOBS),
    "default_globs": list(S.DEFAULT_GLOBS),
    "surfaces": list(S.SURFACES),
    "habits": [S.HABITS[n] for n in sorted(S.HABITS)],
    "max_procedural": S.MAX_PROCEDURAL,
    "max_descriptive": S.MAX_DESCRIPTIVE,
    "max_sentences": S.MAX_SENTENCES_PER_PARAGRAPH,
    "max_report": one(r'"--max-report", type=int, default=(\d+)', int),
    "excerpt_cut": one(r'if excerpt is not None else unit\.text\)\[:(\d+)\]', int),
    "exit_habit_grew": S.EXIT_HABIT_GREW,
    "held": S.HELD,
    "sentence_closers": one(r'SENTENCE_END = re\.compile\(r"\(\?<=\[\.!\?\]\)\[(.*)\]\*'),
    "rule_line": unescaped(r'print\("(\\nRule: [^"]*)"\)'),
    "header_pad": one(r'header = "(  habit +)"'),
    "min_go_words": one(r'if len\(text\.split\(\)\) > (\d+):', int),
    "min_yang_words": one(r'if len\(body\.split\(\)\) < (\d+):', int),
    "head_lines": one(r'head = "\\n"\.join\(text\.splitlines\(\)\[:(\d+)\]\)', int),
}, sys.stdout)
`

// readPythonTables runs the interpreter and decodes what it printed.
func readPythonTables(t *testing.T) pythonTables {
	t.Helper()

	root := checkoutRoot(t)
	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pythonDump, filepath.Join(root, "scripts", "dev"))
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the Python tables: %v: %s", err, errOut.String())
	}

	var tables pythonTables
	if err := json.Unmarshal(out.Bytes(), &tables); err != nil {
		t.Fatalf("decoding the Python tables: %v: %s", err, out.String())
	}
	return tables
}

// checkoutRoot walks up from the test's own directory to the checkout, which is
// where the script lives.
func checkoutRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the checkout: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "feature-gates.txt")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no checkout root above %s", dir)
		}
		dir = parent
	}
}

// pairsOf renders a word list the way the Python dump renders a dictionary.
func pairsOf(list []wordFix) [][]string {
	out := make([][]string, len(list))
	for index, entry := range list {
		out[index] = []string{entry.Word, entry.Fix}
	}
	return out
}

func TestEveryWordListHoldsThePythonValuesInThePythonOrder(t *testing.T) {
	tables := readPythonTables(t)

	for _, pair := range []struct {
		name string
		got  [][]string
		want [][]string
	}{
		{"PLAIN_WORDS", pairsOf(plainWords), tables.PlainWords},
		{"HEDGES", pairsOf(hedges), tables.Hedges},
		{"HEDGE_PHRASES", pairsOf(hedgePhrases), tables.HedgePhrases},
		{"NOMINALIZATIONS", pairsOf(nominalizations), tables.Nominalizations},
		{"PHRASAL_VERBS", pairsOf(phrasalVerbs), tables.PhrasalVerbs},
		{"LATIN_ABBREVIATIONS", pairsOf(latinAbbreviations), tables.LatinAbbreviations},
	} {
		if !reflect.DeepEqual(pair.got, pair.want) {
			t.Errorf("%s: the two halves hold different lists\n go: %v\n py: %v",
				pair.name, pair.got, pair.want)
		}
	}
}

func TestEveryPlainListHoldsThePythonValues(t *testing.T) {
	tables := readPythonTables(t)

	for _, pair := range []struct {
		name string
		got  []string
		want []string
	}{
		{"MARKETING", marketing, tables.Marketing},
		{"PLAIN_WORD_IDENTIFIERS", sortedCopy(plainWordIdentifiers), tables.PlainWordIdentifier},
		{"NOT_GERUND", sortedCopy(notGerund), tables.NotGerund},
		{"ABBREVIATIONS", abbreviations, tables.Abbreviations},
		{"GO_MARKERS", goMarkers, tables.GoMarkers},
		{"GENERATED_MARKERS", generatedMarkers, tables.GeneratedMarkers},
		{"EXCLUDE_DIRS", excludeDirs, tables.ExcludeDirs},
		{"EXCLUDE_GLOBS", excludeGlobs, tables.ExcludeGlobs},
		{"DEFAULT_GLOBS", defaultGlobs, tables.DefaultGlobs},
		{"SURFACES", surfaces, tables.Surfaces},
		{"HABITS", habitNames, tables.Habits},
	} {
		if !reflect.DeepEqual(pair.got, pair.want) {
			t.Errorf("%s: the two halves hold different lists\n go: %v\n py: %v",
				pair.name, pair.got, pair.want)
		}
	}
}

func TestEveryTermSetHoldsThePythonValues(t *testing.T) {
	tables := readPythonTables(t)

	if len(termSets) != len(tables.TermSets) {
		t.Fatalf("the two halves hold %d and %d term sets", len(termSets), len(tables.TermSets))
	}
	for index, set := range termSets {
		row := tables.TermSets[index]
		canonical, _ := row[0].(string)
		if canonical != set.Canonical {
			t.Errorf("term set %d: canonical is %q here and %q there", index, set.Canonical, canonical)
		}
		var rotations []string
		listed, ok := row[1].([]any)
		if !ok {
			t.Errorf("term set %d: the rotations are not a list", index)
			continue
		}
		for _, value := range listed {
			text, _ := value.(string)
			rotations = append(rotations, text)
		}
		if !reflect.DeepEqual(set.Rotations, rotations) {
			t.Errorf("term set %d: rotations are %v here and %v there", index, set.Rotations, rotations)
		}
	}
}

func TestEveryBoundAndSpellingHoldsThePythonValue(t *testing.T) {
	tables := readPythonTables(t)

	for _, pair := range []struct {
		name string
		got  int
		want int
	}{
		{"MAX_PROCEDURAL", maxProcedural, tables.MaxProcedural},
		{"MAX_DESCRIPTIVE", maxDescriptive, tables.MaxDescriptive},
		{"MAX_SENTENCES_PER_PARAGRAPH", maxSentencesPerParagraph, tables.MaxSentences},
		{"--max-report", maxReport, tables.MaxReport},
		{"the excerpt cut", excerptRunes, tables.ExcerptCut},
		{"EXIT_HABIT_GREW", exitHabitGrew, tables.ExitHabitGrew},
		{"the Go comment word floor", 2, tables.MinGoWords},
		{"the YANG description word floor", 3, tables.MinYangWords},
		{"the generated-marker head", 8, tables.HeadLines},
	} {
		if pair.got != pair.want {
			t.Errorf("%s is %d here and %d there", pair.name, pair.got, pair.want)
		}
	}

	if held != tables.Held {
		t.Errorf("the held character is %q here and %q there", held, tables.Held)
	}
	if unescapeClosers(tables.SentenceClosers) != sentenceClosers {
		t.Errorf("the sentence closers are %q here and %q there",
			sentenceClosers, unescapeClosers(tables.SentenceClosers))
	}
	if ruleLine != tables.RuleLine {
		t.Errorf("the rule line is %q here and %q there", ruleLine, tables.RuleLine)
	}
	if reviewHeaderPad != tables.HeaderPad {
		t.Errorf("the header pad is %q here and %q there", reviewHeaderPad, tables.HeaderPad)
	}
}

// unescapeClosers converts the script's character class to the plain characters
// stored here. Only backslash escapes matter, and the class contains one.
func unescapeClosers(class string) string {
	return strings.ReplaceAll(class, `\`, "")
}

// sortedCopy answers a sorted copy and leaves the original unchanged.
func sortedCopy(list []string) []string {
	out := make([]string, len(list))
	copy(out, list)
	sort.Strings(out)
	return out
}
