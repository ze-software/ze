// VALIDATES: the hook words the name rules leave alone are exactly the
// `le ai hooks <name>` calls .claude/settings.json makes (AC-52, D-P4-frozen of
// spec-le-cli-usability).
// PREVENTS: the frozen exemption growing to cover a word no outside caller
// spells, which would let a compound or verb-bearing name skip the vocabulary,
// and a hook settings calls losing its exemption and being renamed under it.
package aihooks

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// hookCall is how settings.json spells one hook: the launcher, then the area,
// then the word.
var hookCall = regexp.MustCompile(`le ai hooks ([a-z][a-z0-9-]*)`)

func TestFrozenHookWordsAreExactlyTheSettingsCalls(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	settings, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read .claude/settings.json: %v", err)
	}
	called := map[string]bool{}
	for _, match := range hookCall.FindAllSubmatch(settings, -1) {
		called[string(match[1])] = true
	}
	if len(called) == 0 {
		t.Fatal(".claude/settings.json holds no `le ai hooks <name>` call, so this test would prove nothing")
	}

	frozen := map[string]bool{}
	for _, row := range Actions().Actions {
		if row.Frozen {
			frozen[row.Verb] = true
		}
	}
	want := slices.Sorted(maps.Keys(called))
	got := slices.Sorted(maps.Keys(frozen))
	if !slices.Equal(got, want) {
		t.Errorf("frozen hook words %v, settings.json calls %v: the two sets must be equal", got, want)
	}
}
