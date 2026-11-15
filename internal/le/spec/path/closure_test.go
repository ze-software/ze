// Related: closure.go -- ClosedStems, relocatedSpecs, renamedSpecs
//
// The goal is the one rule every reader of a closure applies: a removed spec
// is a closure, a move between buckets and a pure stem rename are not. Each
// test drives ClosedStems over a population and two in-memory sides, so the
// rule is judged without git; the commit gate's tests drive the same rule
// through a fixture repository.
package specpath

import (
	"reflect"
	"strings"
	"testing"
)

// memorySides answers each side from a map: a path the map lacks is absent.
func memorySides(before, after map[string]string) Sides {
	side := func(texts map[string]string) Side {
		return func(path string) (string, bool, string) {
			text, present := texts[path]
			return text, present, ""
		}
	}
	return Sides{Before: side(before), After: side(after)}
}

// TestARemovedSpecIsAClosure: a spec removed and carried nowhere is closed,
// and a commit may close several, which only the commit gate refuses.
func TestARemovedSpecIsAClosure(t *testing.T) {
	removed := []string{"plan/release/fixes/spec-b.md", "plan/post-release/features/spec-a.md", "plan/release/fixes/notes.txt"}
	stems, err := ClosedStems(removed, removed, memorySides(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(stems, want) {
		t.Fatalf("ClosedStems = %v, want %v: two removed specs, and a removed non-spec closes nothing", stems, want)
	}
}

// TestABucketMoveIsNoClosure: the same file name removed from one bucket and
// carried in another is a relocation, in either direction.
func TestABucketMoveIsNoClosure(t *testing.T) {
	paths := []string{"plan/post-release/features/spec-moved.md", "plan/release/fixes/spec-back.md"}
	removed := []string{"plan/release/fixes/spec-moved.md", "plan/post-release/engineering/spec-back.md"}
	stems, err := ClosedStems(append(paths, removed...), removed, memorySides(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(stems) != 0 {
		t.Fatalf("ClosedStems = %v, want none: each removal is a move between buckets", stems)
	}
}

// TestAStemRenameIsNoClosureAndAnEditedOneIs: a new spec whose text is the
// removed spec's with the stem replaced is a rename; one edit more makes the
// removal a closure, because a closure never carries its text verbatim.
func TestAStemRenameIsNoClosureAndAnEditedOneIs(t *testing.T) {
	const gone = "plan/release/fixes/spec-old-name.md"
	const added = "plan/release/fixes/spec-new-name.md"
	oldText := "# Spec: old-name\n\nSee spec-old-name.md.\n"
	renamedText := strings.ReplaceAll(oldText, "old-name", "new-name")
	for name, tc := range map[string]struct {
		after string
		want  []string
	}{
		"pure rename":   {after: renamedText, want: nil},
		"edited rename": {after: renamedText + "One more line.\n", want: []string{"old-name"}},
	} {
		t.Run(name, func(t *testing.T) {
			sides := memorySides(map[string]string{gone: oldText}, map[string]string{added: tc.after})
			stems, err := ClosedStems([]string{added, gone}, []string{gone}, sides)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(stems, tc.want) {
				t.Fatalf("ClosedStems = %v, want %v", stems, tc.want)
			}
		})
	}
}

// TestASideThatCannotBeReadRefuses: a rename cannot be judged without the
// removed spec's text, so the read problem is the answer, never a guess.
func TestASideThatCannotBeReadRefuses(t *testing.T) {
	const gone = "plan/release/fixes/spec-old-name.md"
	const added = "plan/release/fixes/spec-new-name.md"
	broken := func(string) (string, bool, string) { return "", false, "object missing" }
	sides := Sides{Before: broken, After: broken}
	if _, err := ClosedStems([]string{added, gone}, []string{gone}, sides); err == nil {
		t.Fatal("ClosedStems answered over a side it could not read")
	}
}
