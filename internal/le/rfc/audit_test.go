// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11 -- the audit half is called
// as a function, and each of its refusals and each of the four freshness states
// is driven from its own entry point.
// PREVENTS: a re-seal whose rule is proven only by the page it prints over a
// clean corpus. Every verdict in this checkout is fresh, so the three states
// that are not `fresh` have no live example, and the branch that re-stamps has
// nothing to act on.

package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAFingerprintKeyNamesASymbolNeverALocation(t *testing.T) {
	cases := []struct {
		name, key, file, symbol, refusal string
	}{
		{name: "a bare path is the whole file", key: "test/plugin/a.ci", file: "test/plugin/a.ci"},
		{name: "a symbol names one function", key: "a/b_test.go::TestX", file: "a/b_test.go", symbol: "TestX"},
		{name: "an ordinal is validated and dropped", key: "a/b_test.go::TestX#2",
			file: "a/b_test.go", symbol: "TestX"},
		{name: "a two-digit ordinal is legal", key: "a/b_test.go::TestX#12",
			file: "a/b_test.go", symbol: "TestX"},
		{name: "the retired line form is named as retired", key: "a/b_test.go:3",
			refusal: "is the retired '<path>:<line>' form"},
		{name: "an absolute path is outside the tree", key: "/etc/passwd",
			refusal: "names a path outside the repository"},
		{name: "a traversal is outside the tree", key: "../../etc/passwd",
			refusal: "names a path outside the repository"},
		{name: "a home path is outside the tree", key: "~/secrets",
			refusal: "names a path outside the repository"},
		{name: "a retired form outside the tree is judged for the path first",
			key: "/etc/passwd:3", refusal: "names a path outside the repository"},
		{name: "a first ordinal is not a legal spelling", key: "a/b_test.go::TestX#1",
			refusal: "is not '<repo-relative-path>::<FuncName>'"},
		{name: "a symbol that is not an identifier", key: "a/b_test.go::9lives",
			refusal: "is not '<repo-relative-path>::<FuncName>'"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			file, symbol, err := fingerprintKey(one.key, "w")
			if one.refusal != "" {
				if err == nil {
					t.Fatalf("%q was accepted as %q/%q", one.key, file, symbol)
				}
				if !strings.Contains(err.Error(), one.refusal) {
					t.Errorf("the refusal does not say %q: %v", one.refusal, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q was refused: %v", one.key, err)
			}
			if file != one.file || symbol != one.symbol {
				t.Errorf("%q resolved to %q/%q, want %q/%q", one.key, file, symbol,
					one.file, one.symbol)
			}
		})
	}
}

func TestTheFileHalfOfAKeyIsReadWithoutValidation(t *testing.T) {
	for key, want := range map[string]string{
		"a/b_test.go::TestX#3": "a/b_test.go",
		"a/b_test.go::TestX":   "a/b_test.go",
		"a/b_test.go":          "a/b_test.go",
		"a/b_test.go#4":        "a/b_test.go",
		// Unparseable, and it still comes back rather than refusing: the two
		// callers are comparing and reporting, not opening.
		"::": "",
	} {
		if got := keyFile(key); got != want {
			t.Errorf("keyFile(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestAFingerprintIsCheckedForShapeAndNotOnlyForLength(t *testing.T) {
	cases := []struct{ name, value string }{
		{"fifteen characters", "0123456789abcde"},
		{"seventeen characters", "0123456789abcdef0"},
		{"the right length and the wrong charset", "0123456789abcdeZ"},
		{"upper case hex", "0123456789ABCDEF"},
		{"padded to the right length", "0123456789ab    "},
		{"a trailing newline on a valid value", "0123456789abcdef\n"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if err := validateSHA(one.value, "w"); err == nil {
				t.Fatalf("%q was accepted as a fingerprint", one.value)
			}
		})
	}
	if err := validateSHA("0123456789abcdef", "w"); err != nil {
		t.Errorf("a well-formed fingerprint was refused: %v", err)
	}
	if err := validateSHA(16, "w"); err == nil {
		t.Error("a number was accepted as a fingerprint")
	}
}

func TestTwoKeysResolvingToOneUnitDoNotCollapse(t *testing.T) {
	// The COUNT is what makes deleting one of two tags in one function visible.
	// A set would compare equal after the deletion, and the verdict would read
	// fresh over a test that is gone.
	both := unitIdentity(map[string]string{
		"a/b_test.go::TestX":   "0123456789abcdef",
		"a/b_test.go::TestX#2": "0123456789abcdef",
	})
	one := unitIdentity(map[string]string{"a/b_test.go::TestX": "0123456789abcdef"})
	if both[[2]string{"a/b_test.go", "0123456789abcdef"}] != 2 {
		t.Errorf("two keys over one unit counted %d, want 2", both[[2]string{"a/b_test.go", "0123456789abcdef"}])
	}
	if len(both) == len(one) && both[[2]string{"a/b_test.go", "0123456789abcdef"}] ==
		one[[2]string{"a/b_test.go", "0123456789abcdef"}] {
		t.Error("dropping one of two keys over one unit left the identity unchanged")
	}
}

// freshVerdict is a recorded verdict whose three maps the caller spells.
func freshVerdict(reqSHA string, tests, units, code map[string]any) map[string]any {
	out := map[string]any{
		"verdict": verdictEnforced, "note": "n", "requirement_sha": reqSHA,
		"tests": tests,
	}
	if units != nil {
		out["units"] = units
	}
	if code != nil {
		out["code"] = code
	}
	return out
}

func TestEachFreshnessStateIsReachedFromItsOwnEvidence(t *testing.T) {
	const (
		req  = "0000000000000001"
		file = "0000000000000002"
		unit = "0000000000000003"
		gone = "000000000000000f"
	)
	tests := map[string]string{"a/b_test.go::TestX": file}
	units := map[string]string{"a/b_test.go::TestX": unit}

	cases := []struct {
		name    string
		verdict map[string]any
		state   string
		moved   []string
	}{
		{
			name: "everything current",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": file},
				map[string]any{"a/b_test.go::TestX": unit}, nil),
			state: FreshState,
		},
		{
			name: "the obligation's own text moved",
			verdict: freshVerdict(gone, map[string]any{"a/b_test.go::TestX": file},
				map[string]any{"a/b_test.go::TestX": unit}, nil),
			state: StaleRequirementState,
		},
		{
			name: "the unit moved",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": file},
				map[string]any{"a/b_test.go::TestX": gone}, nil),
			state: StaleUnitState,
			moved: []string{"a/b_test.go::TestX"},
		},
		{
			name: "only the file around the unit moved",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": gone},
				map[string]any{"a/b_test.go::TestX": unit}, nil),
			state: ShiftedState,
			moved: []string{"a/b_test.go::TestX"},
		},
		{
			name: "the cited producer moved",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": file},
				map[string]any{"a/b_test.go::TestX": unit},
				map[string]any{"a/prod.go::Send": gone}),
			state: StaleUnitState,
			moved: []string{"a/prod.go::Send"},
		},
		{
			name: "no units recorded, and the files agree",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": file},
				nil, nil),
			state: FreshState,
		},
		{
			name: "no units recorded, and a file moved",
			verdict: freshVerdict(req, map[string]any{"a/b_test.go::TestX": gone},
				nil, nil),
			state: StaleUnitState,
		},
		{
			// An absent map and an empty one are ONE state. The documented way
			// of writing a not-applicable verdict omits the tests map, and
			// reading the raw field made that spelling permanently stale.
			name:    "a verdict that cites nothing",
			verdict: map[string]any{"verdict": verdictNotApplicable, "note": "n", "requirement_sha": req},
			state:   FreshState,
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			var wantTests, wantUnits map[string]string
			if _, held := one.verdict["tests"]; held {
				wantTests, wantUnits = tests, units
			}
			if one.name == "a verdict that cites nothing" {
				wantTests, wantUnits = map[string]string{}, map[string]string{}
			}
			code := map[string]string{"a/prod.go::Send": unit}
			got := verdictFreshness(one.verdict, req, wantTests, wantUnits, code)
			if got.State != one.state {
				t.Fatalf("the state is %q, want %q", got.State, one.state)
			}
			if one.moved != nil && !slices.Equal(got.Moved, one.moved) {
				t.Errorf("the moved keys are %v, want %v", got.Moved, one.moved)
			}
		})
	}
}

func TestATagBetweenTwoFunctionsResolvesToTheWholeFile(t *testing.T) {
	// The spans are NOT a partition of the file. A table hoisted between one
	// function's closing brace and the next function's doc comment sits in a
	// gap, and the honest answer there is the whole file: crediting the
	// preceding function would fingerprint text nobody chose.
	const content = `package widget

// TestOne does a thing.
func TestOne(t *testing.T) {
	_ = 1
}

// RFC requirement: RFC9999-2-1 positive -- hoisted above the doc comment
var table = []int{1}

// TestTwo does another.
func TestTwo(t *testing.T) {
	_ = 2
}
`
	index := newScopeIndex()
	// Line 8 is the hoisted tag, between one function's closing brace and the
	// next function's doc comment.
	if name := index.funcNameAt("a_test.go", content, 8); name != "" {
		t.Errorf("a tag in the gap was credited to %q, want file scope", name)
	}
	if name := index.funcNameAt("a_test.go", content, 5); name != "TestOne" {
		t.Errorf("the tag inside the first function resolved to %q", name)
	}
	if name := index.funcNameAt("a_test.go", content, 13); name != "TestTwo" {
		t.Errorf("the tag inside the second function resolved to %q", name)
	}
	if name := index.funcNameAt("a_test.go", content, 1); name != "" {
		t.Errorf("a line above every function resolved to %q, want file scope", name)
	}
	if name := index.funcNameAt("a.ci", content, 5); name != "" {
		t.Errorf("a carrier that is not Go resolved to %q, want file scope", name)
	}
}

func TestAFunctionTwoSpansDeclareIsRefusedRatherThanPicked(t *testing.T) {
	const content = `package widget

func (a A) Send() {}

func (b B) Send() {}
`
	index := newScopeIndex()
	if found := index.funcTexts(content, "Send"); len(found) != 2 {
		t.Fatalf("two methods sharing a name resolved to %d span(s)", len(found))
	}
	reader := newSourceReader(t.TempDir())
	if err := os.WriteFile(filepath.Join(reader.tree, "a_test.go"), []byte(content), 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	_, err := unitSHAs([]string{"a_test.go::Send"}, reader, index, "w")
	if err == nil {
		t.Fatal("a name two functions declare was fingerprinted anyway")
	}
	if !strings.Contains(err.Error(), "2 top-level functions declare it") {
		t.Errorf("the refusal does not say how many declare it: %v", err)
	}
}

func TestAnEmptyUnitIsRefusedRatherThanHashed(t *testing.T) {
	// Hashing "" would give every unreadable file the same fingerprint, so a
	// deleted test would read as unchanged: a false FRESH, the one
	// catastrophic outcome.
	reader := newSourceReader(t.TempDir())
	index := newScopeIndex()
	if err := os.WriteFile(filepath.Join(reader.tree, "empty_test.go"), nil, 0o600); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	for _, key := range []string{"empty_test.go", "gone_test.go", "gone_test.go::TestX"} {
		if _, err := unitSHAs([]string{key}, reader, index, "w"); err == nil {
			t.Errorf("%q was fingerprinted", key)
		} else if !strings.Contains(err.Error(), "missing or empty") {
			t.Errorf("%q was refused for the wrong reason: %v", key, err)
		}
	}
}

func TestAnUnreadableFileFingerprintsAsUnequalRatherThanRefused(t *testing.T) {
	// The tag-level map is the one place "" is safe: it compares unequal to
	// whatever was recorded, so it degrades to more checking.
	reader := newSourceReader(t.TempDir())
	index := newScopeIndex()
	found := taggedUnitSHAs([]Tag{{RID: "R-1", File: "gone_test.go", Line: 1}}, reader, index)
	if found["gone_test.go"] != "" {
		t.Errorf("an unreadable file fingerprinted as %q", found["gone_test.go"])
	}
}

func TestTwoTagsInOneFunctionMintTwoKeys(t *testing.T) {
	const content = `package widget

// RFC requirement: RFC9999-2-1 positive -- one
// RFC requirement: RFC9999-2-1 negative -- two
func TestBoth(t *testing.T) {}
`
	index := newScopeIndex()
	keys := mintTagKeys([]Tag{
		{RID: "RFC9999-2-1", File: "a_test.go", Line: 3},
		{RID: "RFC9999-2-1", File: "a_test.go", Line: 4},
	}, index, func(string) string { return content })
	want := []string{"a_test.go::TestBoth", "a_test.go::TestBoth#2"}
	if !slices.Equal(keys, want) {
		t.Errorf("the keys are %v, want %v", keys, want)
	}
}

func TestAnAbsentAuditRecordIsAnEmptyAnswerRatherThanARefusal(t *testing.T) {
	tree := t.TempDir()
	found, err := LoadAudit(tree, "rfc9999")
	if err != nil {
		t.Fatalf("an absent record was refused: %v", err)
	}
	if len(found.Verdicts) != 0 {
		t.Errorf("an absent record answered %d verdict(s)", len(found.Verdicts))
	}
}

func TestTheResealPageNamesWhatItDidAndWhatItRefused(t *testing.T) {
	empty := ResealReport{}
	if empty.Text() != "nothing to re-seal: no verdict is in the 'shifted' state (0 refused)\n" {
		t.Errorf("an empty run prints %q", empty.Text())
	}
	busy := ResealReport{
		Resealed: []string{"rfc9999 RFC9999-2-1"},
		Refused:  []string{"rfc9999 RFC9999-2-2: stale-unit, a human must re-read it"},
	}
	page := busy.Text()
	for _, want := range []string{
		"refused rfc9999 RFC9999-2-2: stale-unit, a human must re-read it\n",
		"re-stamped rfc9999 RFC9999-2-1\n",
		"re-sealed 1 shifted verdict(s); 1 refused. The ledger now needs: make ze-rfc-index-update\n",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not carry %q:\n%s", want, page)
		}
	}
}

func TestTheAuthoredKeyOrderDecidesWhichDefectIsNamedFirst(t *testing.T) {
	// Two malformed keys in one map. The module walks a dict in insertion
	// order and reports the first, so the port reads the document order rather
	// than a Go map's.
	// The two keys are chosen so document order and sorted order DISAGREE: the
	// retired form is written first and sorts last. A reader that fell back to
	// sorting would name the traversal instead, and both halves would still
	// refuse -- which is why the assertion is on the message rather than on the
	// exit code.
	raw := []byte(`{"tests": {"z/a_test.go:1": "0123456789abcdef", "/etc/passwd": "0123456789abcdef"}}`)
	document, order, err := decodeOrdered(raw)
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	data, _ := document.(map[string]any)
	err = validateSHAMap(data, order, "tests", "w")
	if err == nil {
		t.Fatal("two malformed keys were accepted")
	}
	if !strings.Contains(err.Error(), "is the retired '<path>:<line>' form") {
		t.Errorf("the first defect in document order was not the one reported: %v", err)
	}
	keys := order.child("tests").orderOf(map[string]any{"/etc/passwd": "y", "z/a_test.go:1": "x"})
	if !slices.Equal(keys, []string{"z/a_test.go:1", "/etc/passwd"}) {
		t.Errorf("the recorded order is %v", keys)
	}
}

func TestARenamedFunctionWhoseTextDidNotChangeIsNotAUnitChange(t *testing.T) {
	// The unit identity is a multiset of (file, unit sha), so a key whose
	// SYMBOL moved while its text did not compares equal. That is deliberate:
	// the module treats a pure rename as a file-level shift, which the tests
	// map then reports, and it stays right when a function is renamed AND its
	// text edited, because then the pair moves.
	const (
		req   = "0000000000000001"
		file  = "0000000000000002"
		unit  = "0000000000000003"
		moved = "0000000000000004"
	)
	verdict := freshVerdict(req,
		map[string]any{"a/b_test.go::Old": file},
		map[string]any{"a/b_test.go::Old": unit}, nil)

	renamed := verdictFreshness(verdict, req,
		map[string]string{"a/b_test.go::New": moved},
		map[string]string{"a/b_test.go::New": unit}, nil)
	if renamed.State != ShiftedState {
		t.Errorf("a rename with identical text is %q, want %q", renamed.State, ShiftedState)
	}

	edited := verdictFreshness(verdict, req,
		map[string]string{"a/b_test.go::New": moved},
		map[string]string{"a/b_test.go::New": moved}, nil)
	if edited.State != StaleUnitState {
		t.Errorf("a rename WITH an edit is %q, want %q", edited.State, StaleUnitState)
	}
}

func TestTheFunctionSpansOfAFileNeverOverlap(t *testing.T) {
	// funcNameAt answers file scope for a line inside MORE THAN ONE span, and
	// this is why that branch cannot be reached: a span ends at or before the
	// next function's doc comment, which is where the next span begins. The
	// property is asserted rather than reasoned about, because the guard it
	// justifies is the difference between crediting a tag to one function and
	// crediting it to two.
	root := checkoutRoot(t)
	tags, err := ScanTree(root)
	if err != nil {
		t.Fatalf("ScanTree: %v", err)
	}
	seen := map[string]bool{}
	checked := 0
	for _, tag := range tags {
		if seen[tag.File] || !goScoped(tag.File) {
			continue
		}
		seen[tag.File] = true
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tag.File))) // #nosec G304 -- a tracked path
		if err != nil {
			t.Fatalf("reading %s: %v", tag.File, err)
		}
		spans := goFuncSpans(string(body))
		for i := 1; i < len(spans); i++ {
			if spans[i].begin < spans[i-1].end {
				t.Errorf("%s: span %d [%d,%d) overlaps span %d [%d,%d)", tag.File,
					i, spans[i].begin, spans[i].end, i-1, spans[i-1].begin, spans[i-1].end)
			}
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("only %d tagged Go file(s) were walked; this checkout has more", checked)
	}
}
