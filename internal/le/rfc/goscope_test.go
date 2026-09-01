package rfc

import (
	"os"
	"path/filepath"
	"sort"
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
	sort.Strings(unresolved)
	sort.Strings(ambiguous)
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
