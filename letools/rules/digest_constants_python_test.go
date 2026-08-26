// VALIDATES: spec-le-is-a-ze-binary AC-11. Tables and bounds copied from
// rules_index.py, rules_condensed.py, and rules_router.py retain their Python
// values.
// PREVENTS: drift that output comparison cannot see. Both implementations agree
// on the corpus TODAY. A changed stopword, heading, or bound stays invisible
// until a rule reaches that case. Then only one implementation answers it.
//
// This MIGRATION artifact dies with the scripts at step 14 of the spec. It reads
// MODULES instead of source text, so it compares a rewritten table by value.

package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ze-software/ze/letools/lepath"
)

// digestPythonConstants is what the interpreter answers.
type digestPythonConstants struct {
	Skip           []string `json:"skip"`
	Severities     []string `json:"severities"`
	Stopwords      []string `json:"stopwords"`
	DenyFirstWords []string `json:"deny_first_words"`
	TaskSections   []string `json:"task_sections"`
	Artifacts      []string `json:"artifacts"`
	MaxTriggerLine int      `json:"max_trigger_line"`
	TokenBudget    int      `json:"token_budget"`
	BytesPerToken  int      `json:"bytes_per_token"`
	MaxTriggerDF   int      `json:"max_trigger_df"`
	MinHits        int      `json:"min_hits"`
	MaxProse       int      `json:"max_prose"`
	MaxSummary     int      `json:"max_summary"`
	PayloadParts   []string `json:"payload_parts"`
	IndexPointers  []string `json:"index_pointers"`
	Pointers       []string `json:"pointers"`
	EmptyWarning   string   `json:"empty_warning"`
}

// digestPythonDump imports the three modules and prints their tables.
//
// Two lists are not module constants. payload_report builds its file set, and
// compiled patterns contain each pointer vocabulary. The dump reads the first
// by calling that function on an EMPTY directory. Every part then has zero
// characters, but the page names all parts. It reads the second from the
// compiled pattern. Both values come from their owning objects, not copies here.
const digestPythonDump = `
import json, re, sys
sys.path.insert(0, sys.argv[1])
import rules_index as I
import rules_condensed as C
import rules_router as R

import pathlib, tempfile
with tempfile.TemporaryDirectory() as empty:
    page, _, _ = C.payload_report(pathlib.Path(empty))
parts = [line.strip().split(":")[0] for line in page.splitlines()
         if line.strip().startswith("ai")]

def alternatives(pattern):
    inner = re.search(r"\^\(([^)]*)\)", pattern.pattern).group(1)
    return sorted(word.strip().lower() for word in inner.split("|"))

json.dump({
    "skip": sorted(C.SKIP),
    "severities": sorted(C.SEVERITIES),
    "stopwords": sorted(C.STOPWORDS),
    "deny_first_words": sorted(C.DENY_FIRST_WORDS),
    "task_sections": sorted(R.TASK_SECTIONS),
    "artifacts": [name for name, _ in C.ARTIFACTS],
    "max_trigger_line": C.MAX_TRIGGER_LINE,
    "token_budget": C.TOKEN_BUDGET,
    "bytes_per_token": C.BYTES_PER_TOKEN,
    "max_trigger_df": C.MAX_TRIGGER_DF,
    "min_hits": C.MIN_HITS,
    "max_prose": C.MAX_PROSE,
    "max_summary": I.MAX_SUMMARY,
    "payload_parts": sorted(parts),
    "index_pointers": alternatives(I.POINTER_LINE),
    "pointers": alternatives(C.POINTER_LINE),
    "empty_warning": "".join(
        c for c in C.unreachable_blocking.__code__.co_consts
        if isinstance(c, str) and c.startswith("warning: the task corpus")),
}, sys.stdout)
`

// readDigestPythonConstants runs the dump against this checkout's scripts.
func readDigestPythonConstants(t *testing.T) digestPythonConstants {
	t.Helper()

	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("finding the checkout: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", digestPythonDump, filepath.Join(tree, "scripts", "dev"))
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the modules: %v: %s", err, errOut.String())
	}

	var got digestPythonConstants
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decoding the dump: %v: %s", err, out.String())
	}
	return got
}

func TestTheDigestBoundsHoldThePythonValues(t *testing.T) {
	python := readDigestPythonConstants(t)

	cases := []struct {
		name       string
		got, want  int
		whyItHides string
	}{
		{"maxTriggerLine", maxTriggerLine, python.MaxTriggerLine, "a row is cut only when a trigger is long"},
		{"tokenBudget", tokenBudget, python.TokenBudget, "the verdict only moves at the boundary"},
		{"bytesPerToken", bytesPerToken, python.BytesPerToken, "every token count is a division by it"},
		{"maxTriggerDF", maxTriggerDF, python.MaxTriggerDF, "it changes which words route"},
		{"minHits", minHits, python.MinHits, "it changes which rules a task surfaces"},
		{"maxProse", maxProse, python.MaxProse, "a sentence is cut only when it is long"},
		{"maxSummary", maxSummary, python.MaxSummary, "a summary is cut only when it is long"},
	}
	// The empty-corpus sentence is a shared STRING, not a bound. Output
	// comparison cannot reach it because this checkout has a plan/ directory.
	// Thus, neither implementation prints it.
	if want := "warning: " + emptyCorpusWarning; want != python.EmptyWarning {
		t.Errorf("the empty-corpus warning reads\n  go:     %q\n  python: %q", want, python.EmptyWarning)
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, the Python holds %d (%s)", tc.name, tc.got, tc.want, tc.whyItHides)
		}
	}
}

func TestTheDigestTablesHoldThePythonValues(t *testing.T) {
	python := readDigestPythonConstants(t)

	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{"skip", sortedNames(skip), python.Skip},
		{"severities", sortedList(severities[:]), python.Severities},
		{"stopwords", sortedNames(stopwords), python.Stopwords},
		{"denyFirstWords", sortedNames(denyFirstWords), python.DenyFirstWords},
		{"taskSections", sortedList(taskSections[:]), python.TaskSections},
		{"artifacts", []string{triggersFile, coreFile}, python.Artifacts},
		{"payloadParts", sortedList(payloadParts[:]), python.PayloadParts},
		{"indexPointers", sortedList(indexPointers[:]), python.IndexPointers},
		{"condensedPointers", sortedList(condensedPointers[:]), python.Pointers},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.want) == 0 {
				t.Fatalf("the dump read nothing for %s, so this comparison proves nothing", tc.name)
			}
			if len(tc.got) != len(tc.want) {
				t.Fatalf("%s holds %d entries, the Python holds %d\ngo:     %v\npython: %v",
					tc.name, len(tc.got), len(tc.want), tc.got, tc.want)
			}
			for i := range tc.got {
				if tc.got[i] != tc.want[i] {
					t.Errorf("%s[%d] = %q, the Python holds %q", tc.name, i, tc.got[i], tc.want[i])
				}
			}
		})
	}
}

// sortedList answers a copy of the slice, sorted, so a table declared in the
// order a reader wants can still be compared against a set.
func sortedList(items []string) []string {
	out := append([]string{}, items...)
	sort.Strings(out)
	return out
}
