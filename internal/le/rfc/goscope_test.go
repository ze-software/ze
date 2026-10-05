package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// VALIDATES: A-3 -- every tag on a carrier resolves to a tagged unit, and the key minted
// from that unit resolves BACK to exactly one unit in the same file.
// METHOD: the real checkout. UnitAt answers the unit at each tag's line, the key is minted
// the way a discrimination record mints it, and the key is then resolved again.
// PREVENTS: a proof route that cannot name what it proved. A tag whose unit does not
// resolve gets no record, and a key that resolves to two functions fingerprints text
// nobody chose (`funcTexts` refuses that, so the record would die on its first re-check).
func TestUnitAtResolvesEveryInScopeTag(t *testing.T) {
	root := checkoutRoot(t)
	tags, err := ScanTree(root)
	if err != nil {
		t.Fatalf("ScanTree: %v", err)
	}
	found, err := carriers(root)
	if err != nil {
		t.Fatalf("carriers: %v", err)
	}

	contents := map[string]string{}
	index := newScopeIndex()
	var inScope, funcScoped, fileScoped int
	var unresolved, ambiguous []string
	for _, tag := range tags {
		if _, held := CarrierFor(tag.File, found); !held {
			continue
		}
		inScope++

		content, cached := contents[tag.File]
		if !cached {
			raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(tag.File))) // #nosec G304 -- a tracked path
			if readErr != nil {
				t.Fatalf("reading %s: %v", tag.File, readErr)
			}
			content = string(raw)
			contents[tag.File] = content
		}

		unit := UnitAt(tag.File, content, tag.Line)
		var where textbuf.Buffer
		where.Str(tag.File).Byte(':').Int(int64(tag.Line)).Str(" (").Str(tag.RID).Byte(')')
		if unit.Text == "" {
			unresolved = append(unresolved, where.String())
			continue
		}
		if unit.Scope == ScopeFile {
			fileScoped++
			continue
		}
		funcScoped++

		// The key a record would carry, resolved again the way check re-verifies
		// one. A name two functions declare is refused rather than guessed.
		name := funcNameIn(unit.Text)
		if name == "" {
			unresolved = append(unresolved, where.Str(" resolved to a function with no name").String())
			continue
		}
		if again := index.funcTexts(content, name); len(again) != 1 {
			ambiguous = append(ambiguous, where.Str(" key names ").Int(int64(len(again))).
				Str(" function(s)").String())
		}
	}

	if inScope < 3000 {
		t.Fatalf("only %d in-scope tag(s) were walked; this checkout carries thousands", inScope)
	}
	slices.Sort(unresolved)
	slices.Sort(ambiguous)
	if len(unresolved) > 0 {
		t.Errorf("%d of %d in-scope tag(s) resolve to no unit, so no record can name what they prove:\n%s",
			len(unresolved), inScope, joinLimited(unresolved))
	}
	if len(ambiguous) > 0 {
		t.Errorf("%d of %d in-scope tag(s) mint a key that names other than one function:\n%s",
			len(ambiguous), inScope, joinLimited(ambiguous))
	}
	t.Logf("A-3: %d in-scope tag(s); %d resolve to a function and %d to a whole file",
		inScope, funcScoped, fileScoped)
}

// joinLimited renders at most ten offenders, which is enough to act on.
func joinLimited(items []string) string {
	var tb textbuf.Buffer
	for index, item := range items {
		if index == 10 {
			tb.Str("  ... and ").Int(int64(len(items) - 10)).Str(" more\n")
			break
		}
		tb.Str("  ").Str(item).Byte('\n')
	}
	return tb.String()
}

// TestFunctionSpansPreserveLexicalBoundaries checks the line walk's exact
// boundaries, including incomplete Go and the ASCII word boundary the previous
// regexp used. Scope is conservative source text, not a successful Go parse.
func TestFunctionSpansPreserveLexicalBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"empty", "", nil},
		{"not a keyword", "function()\nfunc_name()\nfunc1()\n", nil},
		{"indented declaration", " func Hidden() {\n}\n", nil},
		{"incomplete keyword", "func", []string{"func"}},
		{"unicode boundary", "funcé() {\n}\n", []string{"funcé() {\n}\n"}},
		{"brace at EOF", "func Last() {\n}", []string{"func Last() {\n}"}},
		{"CRLF", "func Windows() {\r\n}\r\n", []string{"func Windows() {\r\n}\r"}},
		{"nested brace", "func Outer() {\n\tif true {\n\t}\n}\n",
			[]string{"func Outer() {\n\tif true {\n\t}\n}\n"}},
		{"doc comments and gap", "// First.\nfunc First() {\n}\n\n// Gap.\n\n// Second.\nfunc Second() {\n}\n",
			[]string{"// First.\nfunc First() {\n}\n", "// Second.\nfunc Second() {\n}\n"}},
		{"one line cap", "func First() {}\n\n// Second.\nfunc Second() {}\n",
			[]string{"func First() {}\n\n", "// Second.\nfunc Second() {}\n"}},
		{"unclosed cap", "func First() {\n// Second.\nfunc Second() {\n}\n",
			[]string{"func First() {\n", "// Second.\nfunc Second() {\n}\n"}},
		{"earlier braces", "}\n}\nfunc First() {\n}\nfunc Second() {\n}\n",
			[]string{"func First() {\n}\n", "func Second() {\n}\n"}},
		{"one line then multiline", "func First() {}\n// Second.\nfunc Second() {\n}\n",
			[]string{"func First() {}\n", "// Second.\nfunc Second() {\n}\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, one := range goFuncSpans(tc.content) {
				got = append(got, tc.content[one.begin:one.end])
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("function texts = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestScopeIndexKeepsAmbiguityAndEditedContent resolves repeated names and
// changed bytes through one operation's index. Reusing source structure must
// neither pick a duplicate method nor give edited content an earlier answer.
func TestScopeIndexKeepsAmbiguityAndEditedContent(t *testing.T) {
	const first = "// First.\nfunc (First) Send() {\n}\n"
	const second = "// Second.\nfunc (Second) Send() {\n}\n"
	const generic = "func Other[T any]() {\n}\n"
	content := first + second + generic
	index := newScopeIndex()
	renamed := strings.ReplaceAll(content, "Send", "Receive")
	changed := strings.ReplaceAll(content, "First", "Changed")
	cases := []struct {
		content string
		name    string
		want    []string
	}{
		{content, "Send", []string{first, second}},
		{content, "Other", []string{generic}},
		{content, "Missing", nil},
		{renamed, "Send", nil},
		{renamed, "Receive", []string{
			strings.ReplaceAll(first, "Send", "Receive"),
			strings.ReplaceAll(second, "Send", "Receive"),
		}},
		{changed, "Send", []string{strings.ReplaceAll(first, "First", "Changed"), second}},
		{generic, "Send", nil},
		{content, "Send", []string{first, second}},
	}
	for _, tc := range cases {
		if got := index.funcTexts(tc.content, tc.name); !slices.Equal(got, tc.want) {
			t.Errorf("%s in %q = %q, want %q", tc.name, tc.content, got, tc.want)
		}
	}
}
