// VALIDATES: spec-le-is-a-ze-binary AC-11 -- every table, bound and path this
// package copied out of rfc_requirements.py still holds the value the Python
// holds.
// PREVENTS: the drift an output comparison cannot see. Both halves agree over
// the corpus TODAY, so a one-character change to a shared table -- a keyword
// dropped from the gated set, an exclusion kind added on one side, the
// correction-quote bound moved from 24 to 12, a test root removed -- changes
// nothing visible until a summary reaches the case it governs, and then only
// one half refuses it.
//
// This file is a MIGRATION artifact and dies with the script at step 14 of the
// spec. It is the one test in this package that reads scripts/dev, and it reads
// the module rather than its source text, so a table rewritten as a
// comprehension is still compared by value.

package rfc

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/functional"
)

// pythonTimeout bounds the one interpreter this file starts. It imports the
// module and prints a dictionary, which is under a second.
const pythonTimeout = 60 * time.Second

// pythonConstants is what the interpreter answers: every table this package
// copied, named as the Python names it.
type pythonConstants struct {
	GatedLevels           []string          `json:"gated_levels"`
	AdvisoryLevels        []string          `json:"advisory_levels"`
	Polarities            []string          `json:"polarities"`
	AnnotationKinds       []string          `json:"annotation_kinds"`
	SupersededKind        string            `json:"superseded_kind"`
	SuccessorDispositions []string          `json:"successor_dispositions"`
	SuccessorTargeted     []string          `json:"successor_targeted"`
	MinCorrectionQuote    int               `json:"min_correction_quote"`
	SHAHexLen             int               `json:"sha_hex_len"`
	NoSection             string            `json:"no_section"`
	TagMarker             string            `json:"tag_marker"`
	TagPunct              string            `json:"tag_punct"`
	TestRoots             []string          `json:"test_roots"`
	DraftPrefix           string            `json:"draft_prefix"`
	DevelopmentToolsPrefix string            `json:"development_tools_prefix"`
	EditorSuite           string            `json:"editor_suite"`
	Tiers                 []string          `json:"tiers"`
	Registers             []string          `json:"registers"`
	RegisterStrength      map[string]int    `json:"register_strength"`
	ExclusionKinds        []string          `json:"exclusion_kinds"`
	RelocatedToSpec       string            `json:"relocated_to_spec"`
	SectionSkipKinds      []string          `json:"section_skip_kinds"`
	SiteDispositions      []string          `json:"site_dispositions"`
	SectionDispositions   []string          `json:"section_dispositions"`
	FrontSection          string            `json:"front_section"`
	SchemaVersion         int               `json:"schema_version"`
	ArtifactKeys          []string          `json:"artifact_keys"`
	SiteKeys              []string          `json:"site_keys"`
	SectionKeys           []string          `json:"section_keys"`
	InteropTrees          [][]string        `json:"interop_trees"`
	CmdWrappers           []string          `json:"cmd_wrappers"`
	MakeFlagsWithArg      []string          `json:"make_flags_with_arg"`
	WorkflowSuffixes      []string          `json:"workflow_suffixes"`
	SpecDirName           string            `json:"spec_dir_name"`
	Dirs                  map[string]string `json:"dirs"`
	FunctionalSuites      []string          `json:"functional_suites"`
}

// pythonDump is the program the interpreter runs. It imports the module by path
// and prints its tables, so a value the Python computes rather than spells is
// still compared.
const pythonDump = `
import importlib.util, json, os, sys
spec = importlib.util.spec_from_file_location("rr", sys.argv[1])
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
rel = lambda p: os.path.relpath(p, m.PROJECT_DIR).replace(os.sep, "/")
print(json.dumps({
  "gated_levels": sorted(m.GATED_LEVELS),
  "advisory_levels": sorted(m.ADVISORY_LEVELS),
  "polarities": sorted(m.POLARITIES),
  "annotation_kinds": sorted(m.ANNOTATION_KINDS),
  "superseded_kind": m.SUPERSEDED_KIND,
  "successor_dispositions": sorted(m.SUCCESSOR_DISPOSITIONS),
  "successor_targeted": sorted(m._SUCCESSOR_TARGETED),
  "sha_hex_len": m.SHA_HEX_LEN,
  "no_section": m.NO_SECTION,
  "tag_marker": m.TAG_MARKER,
  "tag_punct": m._TAG_PUNCT,
  "test_roots": list(m.TEST_ROOTS),
  "draft_prefix": m.DRAFT_PREFIX,
  "development_tools_prefix": m.DEVELOPMENT_TOOLS_PREFIX,
  "editor_suite": m.EDITOR_SUITE,
  "tiers": [m.TIER_VERIFY, m.TIER_NIGHTLY, m.TIER_UNRUN],
  "registers": list(m.REGISTERS),
  "register_strength": m._REGISTER_STRENGTH,
  "exclusion_kinds": sorted(m.EXCLUSION_KINDS),
  "relocated_to_spec": m.RELOCATED_TO_SPEC,
  "section_skip_kinds": sorted(m.SECTION_SKIP_KINDS),
  "site_dispositions": sorted(m.SITE_DISPOSITIONS),
  "section_dispositions": sorted(m.SECTION_DISPOSITIONS),
  "front_section": m.FRONT_SECTION,
  "schema_version": m.EXTRACTION_SCHEMA_VERSION,
  "artifact_keys": sorted(m._ARTIFACT_KEYS),
  "site_keys": sorted(m._SITE_KEYS),
  "section_keys": sorted(m._SECTION_KEYS),
  "interop_trees": [list(row) for row in m.INTEROP_TREES],
  "cmd_wrappers": list(m._CMD_WRAPPERS),
  "make_flags_with_arg": list(m._MAKE_FLAGS_WITH_ARG),
  "workflow_suffixes": list(m._WORKFLOW_SUFFIXES),
  "spec_dir_name": m._SPEC_DIR_NAME,
  "dirs": {
    "summary": rel(m.SUMMARY_DIR), "enrolled": rel(m.ENROLLED_FILE),
    "extraction": rel(m.EXTRACTION_DIR), "workflows": rel(m.WORKFLOWS_DIR),
    "spec": rel(m.SPEC_DIR),
  },
  "functional_suites": list(m.functional_suites()),
}))
`

// readPythonConstants runs the driver over this checkout's module.
func readPythonConstants(t *testing.T) pythonConstants {
	t.Helper()

	root := checkoutRoot(t)
	ctx, cancel := context.WithTimeout(t.Context(), pythonTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pythonDump, // #nosec G204 -- a tracked script path
		filepath.Join(root, "scripts", "dev", "rfc_requirements.py"))
	cmd.Dir = root
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("reading the Python constants: %v: %s", err, errOut.String())
	}
	var found pythonConstants
	if err := json.Unmarshal(out.Bytes(), &found); err != nil {
		t.Fatalf("the driver answered no JSON: %v: %s", err, out.String())
	}
	return found
}

// checkoutRoot walks up from the test's own working directory. lepath.Root
// would answer a fixture tree when a sibling case has pointed ZE_REPO_ROOT at
// one, and the script lives in this checkout whatever the fixture says.
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

// sortedOf answers a copy in sorted order, for a Python tuple this package
// keeps as a set.
func sortedOf(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

func TestTheScriptAndTheCommandShareTheSameTables(t *testing.T) {
	python := readPythonConstants(t)

	strengths := maps.Clone(registerStrength)
	trees := [][]string{}
	for _, one := range interopTrees {
		trees = append(trees, []string{one.name, one.prefix, one.target})
	}

	// `ordered` says the table's ORDER is part of its meaning: the carrier
	// tiers, the register ranking, the run list and the two argv tables are
	// each read in sequence, so comparing them as sets would pass a table
	// somebody reordered.
	lists := []struct {
		name        string
		python, got []string
		ordered     bool
	}{
		{name: "GATED_LEVELS", python: python.GatedLevels, got: GatedLevels()},
		{name: "ADVISORY_LEVELS", python: python.AdvisoryLevels, got: AdvisoryLevels()},
		{name: "POLARITIES", python: python.Polarities, got: Polarities()},
		{name: "ANNOTATION_KINDS", python: python.AnnotationKinds, got: AnnotationKinds()},
		{name: "SUCCESSOR_DISPOSITIONS", python: python.SuccessorDispositions, got: SuccessorDispositions()},
		{name: "_SUCCESSOR_TARGETED", python: python.SuccessorTargeted, got: sortedKeys(successorTargeted)},
		{name: "EXCLUSION_KINDS", python: python.ExclusionKinds, got: ExclusionKinds()},
		{name: "SECTION_SKIP_KINDS", python: python.SectionSkipKinds, got: SectionSkipKinds()},
		{name: "SITE_DISPOSITIONS", python: python.SiteDispositions, got: SiteDispositions()},
		{name: "SECTION_DISPOSITIONS", python: python.SectionDispositions, got: SectionDispositions()},
		{name: "_ARTIFACT_KEYS", python: python.ArtifactKeys, got: sortedKeys(artifactKeys)},
		{name: "_SITE_KEYS", python: python.SiteKeys, got: sortedKeys(siteKeys)},
		{name: "_SECTION_KEYS", python: python.SectionKeys, got: sortedKeys(sectionKeys)},
		{name: "_CMD_WRAPPERS", python: sortedOf(python.CmdWrappers), got: sortedKeys(cmdWrappers)},
		{name: "_MAKE_FLAGS_WITH_ARG", python: sortedOf(python.MakeFlagsWithArg), got: sortedKeys(makeFlagsWithArg)},
		{name: "TEST_ROOTS", python: python.TestRoots, got: testRoots[:], ordered: true},
		{name: "TIER_*", python: python.Tiers, got: []string{tierVerify, tierNightly, tierUnrun}, ordered: true},
		{name: "REGISTERS", python: python.Registers, got: Registers(), ordered: true},
		{name: "_WORKFLOW_SUFFIXES", python: python.WorkflowSuffixes, got: workflowSuffixes[:], ordered: true},
		{name: "functional_suites()", python: python.FunctionalSuites, got: FunctionalSuites(), ordered: true},
		{name: "functional.Gating", python: python.FunctionalSuites, got: functional.Gating, ordered: true},
	}
	for _, one := range lists {
		want, got := slices.Clone(one.python), slices.Clone(one.got)
		if !one.ordered {
			slices.Sort(want)
			slices.Sort(got)
		}
		if !slices.Equal(want, got) {
			t.Errorf("%s: the script holds %v and the command holds %v", one.name, want, got)
		}
	}

	words := []struct {
		name        string
		python, got string
	}{
		{"SUPERSEDED_KIND", python.SupersededKind, supersededKind},
		{"NO_SECTION", python.NoSection, noSection},
		{"TAG_MARKER", python.TagMarker, tagMarker},
		{"_TAG_PUNCT", python.TagPunct, tagPunct},
		{"DRAFT_PREFIX", python.DraftPrefix, draftPrefix},
		{"DEVELOPMENT_TOOLS_PREFIX", python.DevelopmentToolsPrefix, developmentToolsPrefix},
		{"EDITOR_SUITE", python.EditorSuite, editorSuite},
		{"RELOCATED_TO_SPEC", python.RelocatedToSpec, relocatedToSpec},
		{"FRONT_SECTION", python.FrontSection, frontSection},
		{"_SPEC_DIR_NAME", python.SpecDirName, specDirName},
		{"SUMMARY_DIR", python.Dirs["summary"], summaryRel},
		{"ENROLLED_FILE", python.Dirs["enrolled"], enrolledRel},
		{"EXTRACTION_DIR", python.Dirs["extraction"], extractionRel},
		{"WORKFLOWS_DIR", python.Dirs["workflows"], workflowsRel},
		{"SPEC_DIR", python.Dirs["spec"], specDirName},
	}
	for _, one := range words {
		if one.python != one.got {
			t.Errorf("%s: the script holds %q and the command holds %q", one.name, one.python, one.got)
		}
	}

	numbers := []struct {
		name        string
		python, got int
	}{
		{"SHA_HEX_LEN", python.SHAHexLen, shaHexLen},
		{"EXTRACTION_SCHEMA_VERSION", python.SchemaVersion, extractionSchemaVersion},
	}
	for _, one := range numbers {
		if one.python != one.got {
			t.Errorf("%s: the script holds %d and the command holds %d", one.name, one.python, one.got)
		}
	}

	if len(python.RegisterStrength) != len(strengths) {
		t.Errorf("_REGISTER_STRENGTH: the script holds %d entries and the command holds %d",
			len(python.RegisterStrength), len(strengths))
	}
	for name, rank := range python.RegisterStrength {
		if strengths[name] != rank {
			t.Errorf("_REGISTER_STRENGTH[%q]: the script holds %d and the command holds %d",
				name, rank, strengths[name])
		}
	}

	if len(python.InteropTrees) != len(trees) {
		t.Fatalf("INTEROP_TREES: the script holds %d rows and the command holds %d",
			len(python.InteropTrees), len(trees))
	}
	for i, row := range python.InteropTrees {
		if !slices.Equal(row, trees[i]) {
			t.Errorf("INTEROP_TREES[%d]: the script holds %v and the command holds %v", i, row, trees[i])
		}
	}
}
