// Related: archive.go -- the trigger keywords this test reads
// Related: scheduler.go, cli/cmd_edit.go -- what each keyword selects

package archive_test

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config/archive"
	configyang "github.com/ze-software/ze/internal/component/config/yang"

	_ "github.com/ze-software/ze/internal/component/config/system/yang" // registers ze-system-conf.yang, which declares the trigger leaf
)

// TestTriggerVocabularyMatchesModel ties the trigger keywords this package acts
// on to the enumeration the archive list declares.
//
// The Go side is the declaration: each keyword selects a firing path (the
// scheduler fires daily and hourly, the editor's commit fires commit, and
// manual is the default nothing fires), which the model cannot hold. A keyword
// the model offered and no path fired would archive nothing with no error, so
// the model is the copy and this test keeps it honest in both directions.
//
// VALIDATES: the enumeration at system/archive/trigger and the keywords the
// package declares are one set.
// PREVENTS: a trigger an operator can commit that never fires, and a keyword
// the scheduler reads that no operator can write.
func TestTriggerVocabularyMatchesModel(t *testing.T) {
	const path = "system/archive/trigger"

	declared, err := configyang.EnumValues(path)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", path, err)
	}
	if len(declared) == 0 {
		t.Fatalf("the model declares no trigger at %s", path)
	}

	known := []string{archive.TriggerCommit, archive.TriggerManual, archive.TriggerDaily, archive.TriggerHourly}
	slices.Sort(known)
	if !slices.Equal(declared, known) {
		t.Errorf("the model declares %v at %s, and this package acts on %v", declared, path, known)
	}
	for _, trigger := range declared {
		if err := archive.ValidateTrigger(trigger); err != nil {
			t.Errorf("the model declares trigger %q and ValidateTrigger refuses it: %v", trigger, err)
		}
	}
}
