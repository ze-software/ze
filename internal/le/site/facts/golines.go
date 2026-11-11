// Design: docs/architecture/site-facts.md -- one compact precision for Go line facts.
package sitefacts

import (
	"strconv"
)

// The Go line keys are shared by derivation, staleness and the site snapshot.
const (
	GoSourceLinesKey = "repo.go_source_lines"
	GoTestLinesKey   = "repo.go_test_lines"
)

// FormatGoLines states a line total to the nearest hundredth of a thousand or
// million. Smaller counts stay exact. Integer arithmetic preserves the visible
// rounding boundary and cannot overflow for an int-sized source inventory.
func FormatGoLines(count int) string {
	for _, unit := range []struct {
		value  int
		suffix string
	}{
		{1_000_000, "M"},
		{1_000, "K"},
	} {
		// Promote the unit when rounding the smaller one would reach it.
		if count < unit.value-unit.value/200_000 {
			continue
		}
		hundredths := count/unit.value*100 + (count%unit.value*100+unit.value/2)/unit.value
		fraction := strconv.Itoa(hundredths % 100)
		if len(fraction) == 1 {
			fraction = "0" + fraction
		}
		return strconv.Itoa(hundredths/100) + "." + fraction + unit.suffix
	}
	return strconv.Itoa(count)
}

// displayedFact selects the published precision for the fact being compared.
// Existing counts keep their established formatter; only Go line totals use
// the new compact display.
func displayedFact(name string, count int) string {
	switch name {
	case GoSourceLinesKey, GoTestLinesKey:
		return FormatGoLines(count)
	default:
		// Fact names are an open JSON vocabulary; old and unknown counts retain
		// the general repository-count representation.
		return render(count)
	}
}
