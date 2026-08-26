// VALIDATES: spec-le-is-a-ze-binary AC-11. This package keeps every table and
// bound copied from rules_lint.py and rules_points.py equal to its Python value.
// PREVENTS: drift that an output comparison cannot see. Both implementations
// agree on the corpus TODAY. A shared-table change can stay invisible until a
// rule reaches its case. Examples include a dangling word, a slug bound, and an
// RFC 2119 keyword. At that point, only one implementation refuses it.
//
// This MIGRATION artifact dies with the scripts at step 14 of the spec. This
// test reads Python modules instead of their source text.

package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pythonTimeout bounds the one interpreter this file starts. It imports two
// modules and prints a dictionary, which is milliseconds.
const pythonTimeout = 60 * time.Second

// pythonConstants is what the interpreter answers: every table this package
// copied, named as the Python names it.
type pythonConstants struct {
	Skip             []string            `json:"skip"`
	Severities       []string            `json:"severities"`
	Openers          []string            `json:"openers"`
	NotGerund        []string            `json:"not_gerund"`
	TruncatedTail    []string            `json:"truncated_tail"`
	DanglingLastWord []string            `json:"dangling_last_word"`
	CanonKeys        []string            `json:"canon_keys"`
	RFCLevels        []string            `json:"rfc_levels"`
	LevelCanon       map[string]string   `json:"level_canon"`
	LevelRank        []string            `json:"level_rank"`
	LevelTiers       [][]string          `json:"level_tiers"`
	LowerModals      []string            `json:"lower_modals"`
	Kinds            []string            `json:"kinds"`
	Levels           []string            `json:"levels"`
	HeaderKeys       []string            `json:"header_keys"`
	PointKeys        []string            `json:"point_keys"`
	ExceptedBy       string              `json:"excepted_by"`
	SlugMax          int                 `json:"slug_max"`
	Delim            string              `json:"delim"`
	PointIndent      string              `json:"point_indent"`
	TightMark        string              `json:"tight_mark"`
	StructuralKinds  []string            `json:"structural_kinds"`
	NoCheck          string              `json:"no_check"`
	EmptyRef         string              `json:"empty_ref"`
	RetiredFile      string              `json:"retired_file"`
	RetiredTableHead string              `json:"retired_table_head"`
	Settings         string              `json:"settings"`
	HookDir          string              `json:"hook_dir"`
	DispatcherGlob   string              `json:"dispatcher_glob"`
	DocRule          string              `json:"doc_rule"`
	TableHead        string              `json:"table_head"`
	DiffBudgets      []int               `json:"diff_budgets"`
	DiffContext      int                 `json:"diff_context"`
	Extra            map[string][]string `json:"-"`
}

// pythonDump is the program the interpreter runs. It imports the two modules
// and prints their tables, so a value the Python computes rather than spells is
// still compared.
const pythonDump = `
import json, re, sys, difflib, inspect
sys.path.insert(0, sys.argv[1])
import rules_lint as L
import rules_points as P
source = open(sys.argv[1] + "/rules_points.py", encoding="utf-8").read()
json.dump({
    "skip": sorted(L.SKIP),
    "severities": sorted(L.SEVERITIES),
    "openers": list(L.OPENERS),
    "not_gerund": sorted(L.NOT_GERUND),
    "truncated_tail": list(L.TRUNCATED_TAIL),
    "dangling_last_word": sorted(L.DANGLING_LAST_WORD),
    "canon_keys": list(L.CANON_KEYS),
    "rfc_levels": list(L.RFC_LEVELS),
    "level_canon": dict(L.LEVEL_CANON),
    "level_rank": list(L.LEVEL_RANK),
    "level_tiers": [list(t) for t in L.LEVEL_TIERS],
    "lower_modals": re.search(r"\((must[^)]*)\)", L.LOWER_MODAL.pattern).group(1).split("|"),
    "kinds": list(P.KINDS),
    "levels": list(P.LEVELS),
    "header_keys": list(P.HEADER_KEYS),
    "point_keys": list(P.POINT_KEYS),
    "excepted_by": P.EXCEPTED_BY,
    "slug_max": P.SLUG_MAX,
    "delim": P.DELIM,
    "point_indent": P.POINT_INDENT,
    "tight_mark": P.TIGHT_MARK,
    "structural_kinds": list(P.STRUCTURAL_KINDS),
    "no_check": P.NO_CHECK,
    "empty_ref": P.EMPTY_REF,
    "retired_file": P.RETIRED_FILE,
    "retired_table_head": P.RETIRED_TABLE_HEAD,
    "settings": P.SETTINGS,
    "hook_dir": P.HOOK_DIR,
    "dispatcher_glob": P.DISPATCHER_GLOB,
    "doc_rule": P.DOC_RULE,
    "table_head": P.TABLE_HEAD,
    "diff_budgets": [int(n) for n in re.findall(r"\)\[:(\d+)\]", source)],
    "diff_context": inspect.signature(difflib.unified_diff).parameters["n"].default,
}, sys.stdout)
`

func TestEveryTableCopiedFromThePythonStillMatchesIt(t *testing.T) {
	want := readPythonConstants(t)

	equal(t, "SKIP", sortedUnique(skip), want.Skip)
	equal(t, "SEVERITIES", severities[:], want.Severities)
	equal(t, "OPENERS", openers[:], want.Openers)
	equal(t, "NOT_GERUND", sortedUnique(notGerund), want.NotGerund)
	equal(t, "TRUNCATED_TAIL", truncatedTail[:], want.TruncatedTail)
	equal(t, "DANGLING_LAST_WORD", sortedUnique(danglingLastWord), want.DanglingLastWord)
	equal(t, "CANON_KEYS", canonKeys[:], want.CanonKeys)
	equal(t, "RFC_LEVELS", rfcLevels[:], want.RFCLevels)
	equal(t, "LEVEL_RANK", levelRank[:], want.LevelRank)
	equal(t, "LOWER_MODAL", lowerModalWords[:], want.LowerModals)
	equal(t, "KINDS", kinds[:], want.Kinds)
	equal(t, "LEVELS", levels[:], want.Levels)
	equal(t, "HEADER_KEYS", headerKeys[:], want.HeaderKeys)
	equal(t, "POINT_KEYS", pointKeys[:], want.PointKeys)
	equal(t, "STRUCTURAL_KINDS", structuralKinds[:], want.StructuralKinds)

	if len(levelTiers) != len(want.LevelTiers) {
		t.Fatalf("LEVEL_TIERS holds %d tiers, the Python holds %d", len(levelTiers), len(want.LevelTiers))
	}
	for i := range levelTiers {
		equal(t, "LEVEL_TIERS", levelTiers[i], want.LevelTiers[i])
	}

	if len(levelCanon) != len(want.LevelCanon) {
		t.Errorf("LEVEL_CANON holds %d keys, the Python holds %d", len(levelCanon), len(want.LevelCanon))
	}
	for keyword, level := range want.LevelCanon {
		if levelCanon[keyword] != level {
			t.Errorf("LEVEL_CANON[%q] is %q, the Python says %q", keyword, levelCanon[keyword], level)
		}
	}

	scalars := []struct {
		name        string
		got, wanted string
	}{
		{"EXCEPTED_BY", exceptedBy, want.ExceptedBy},
		{"DELIM", delim, want.Delim},
		{"POINT_INDENT", pointIndent, want.PointIndent},
		{"TIGHT_MARK", tightMark, want.TightMark},
		{"NO_CHECK", noCheck, want.NoCheck},
		{"EMPTY_REF", emptyRef, want.EmptyRef},
		{"RETIRED_FILE", retiredFile, want.RetiredFile},
		{"RETIRED_TABLE_HEAD", retiredTableHead, want.RetiredTableHead},
		{"SETTINGS", settingsRel, want.Settings},
		{"HOOK_DIR", hookDir, want.HookDir},
		{"DOC_RULE", docRule, want.DocRule},
		{"TABLE_HEAD", tableHead, want.TableHead},
	}
	for _, s := range scalars {
		if s.got != s.wanted {
			t.Errorf("%s is %q, the Python says %q", s.name, s.got, s.wanted)
		}
	}

	// The glob is a pattern in the Python and a prefix here, because Go's
	// os.ReadDir answers names rather than matches. The prefix is what the
	// pattern anchors on, so this is the comparison that keeps them in step.
	if want.DispatcherGlob != dispatcherGlob+"*.py" {
		t.Errorf("DISPATCHER_GLOB is %q, and this package looks for the prefix %q",
			want.DispatcherGlob, dispatcherGlob)
	}

	if slugMax != want.SlugMax {
		t.Errorf("slugMax is %d, the Python says %d", slugMax, want.SlugMax)
	}
	if diffContext != want.DiffContext {
		t.Errorf("diffContext is %d, difflib's default n is %d", diffContext, want.DiffContext)
	}
	if len(want.DiffBudgets) == 0 {
		t.Fatal("the script no longer cuts its diff to a fixed number of lines")
	}
	for _, budget := range want.DiffBudgets {
		if budget != diffBudget {
			t.Errorf("diffBudget is %d, the script cuts at %d", diffBudget, budget)
		}
	}
}

// readPythonConstants runs the interpreter over the two modules and decodes
// what it printed.
func readPythonConstants(t *testing.T) pythonConstants {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	dir := filepath.Join(checkoutRoot(t), "scripts", "dev")
	cmd := exec.CommandContext(ctx, "python3", "-c", pythonDump, dir) // #nosec G204 -- a tracked script directory
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the Python constants: %v: %s", err, errOut.String())
	}
	var answer pythonConstants
	if err := json.Unmarshal(out.Bytes(), &answer); err != nil {
		t.Fatalf("decoding the Python constants: %v: %s", err, out.String())
	}
	return answer
}

// checkoutRoot walks up from the test's working directory to this checkout. It
// does not call lepath.Root, which would answer a fixture tree when one is set.
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

// equal fails unless the two lists hold the same members in the same order.
func equal(t *testing.T, name string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("%s is %v, the Python says %v", name, got, want)
	}
}
