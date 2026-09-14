// Design: manager.go -- weekdayNamed reads the day names off time.Weekday
//
// Goal: prove the day an operator writes at storage/smart/self-test/long/day is
// a day the scheduler matches, so the model cannot offer a word the standard
// library does not name. Method: read the enumeration out of the loaded model
// with configyang.EnumValues, which fails on a leaf that declares no
// enumeration, and hold each value to weekdayNamed in both directions.

package storage

import (
	"slices"
	"strings"
	"testing"
	"time"

	configyang "github.com/ze-software/ze/internal/component/config/yang"

	// The blank import registers ze-storage-conf with the loader, which
	// declares the leaf read below.
	_ "github.com/ze-software/ze/internal/component/storage/yang"
)

const selfTestDayLeaf = "storage/smart/self-test/long/day"

// TestSelfTestDayLeafMatchesWeekdays compares the model's seven words with the
// seven time.Weekday names, spelled the way the leaf spells them.
func TestSelfTestDayLeafMatchesWeekdays(t *testing.T) {
	model, err := configyang.EnumValues(selfTestDayLeaf)
	if err != nil {
		t.Fatalf("read the enumeration at %s: %v", selfTestDayLeaf, err)
	}

	weekdays := make([]string, 0, 7)
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		weekdays = append(weekdays, strings.ToLower(wd.String()))
	}
	slices.Sort(weekdays)

	if !slices.Equal(model, weekdays) {
		t.Errorf("the days disagree: the model at %s holds %v and time.Weekday names %v. "+
			"A word only the model carries matches every day, so a long self-test runs daily instead of weekly",
			selfTestDayLeaf, model, weekdays)
	}
	for _, day := range model {
		if _, named := weekdayNamed(day); !named {
			t.Errorf("%q is offered at %s and weekdayNamed reads it as no day", day, selfTestDayLeaf)
		}
	}
	if _, named := weekdayNamed("someday"); named {
		t.Error("weekdayNamed accepted a word no leaf holds")
	}
}
