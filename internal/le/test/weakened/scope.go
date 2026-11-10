// Design: docs/contributing/rfc-implementation-guide.md -- one definition of a Go test unit
// Related: detector.go -- the lexical verdicts applied to these units.
package testweakened

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

type namedUnit struct {
	name       string
	text       string
	incomplete bool
	start      int
	end        int
}

// changedUnitNames compares existing units in the inventory the detector judges.
// This is a text comparison, not a second weakening detector: only a live
// judge approval can make a changed unit without a finding spend a row.
// A new function has no prior test contract to migrate and spends no row.
func changedUnitNames(path, oldText, newText string) ([]string, error) {
	if oldText == newText {
		return nil, nil
	}
	if !strings.HasSuffix(path, ".go") {
		return []string{strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}, nil
	}
	oldUnits := unitsByName(oldText)
	newUnits := unitsByName(newText)
	// A file that does not parse and holds no declaration the parser read is
	// not an empty baseline inventory. It cannot establish which units
	// already existed.
	if len(oldUnits) == 1 {
		if oldUnits[0].name == "" && oldUnits[0].incomplete {
			return nil, fmt.Errorf("cannot judge Go function boundaries in %s; repair the incomplete baseline before preparing the commit", path)
		}
	}
	var changed []string
	for _, unit := range oldUnits {
		if unit.name == "" {
			continue
		}
		known := !unit.incomplete
		if known {
			known = unitBoundaryKnown(newUnits, unit.name)
		}
		if !known {
			return nil, fmt.Errorf("cannot judge Go function boundaries in %s for %s; repair the incomplete declaration before preparing the commit", path, unit.name)
		}
		if unit.text != unitText(newUnits, unit.name) {
			changed = append(changed, unit.name)
		}
	}
	return changed, nil
}

// unitBoundaryKnown distinguishes a removed declaration from one whose boundary
// could not be read. A syntax error after a closed unit cannot invalidate it.
func unitBoundaryKnown(units []namedUnit, name string) bool {
	uncertain := false
	for _, unit := range units {
		if unit.name == name {
			return !unit.incomplete
		}
		if unit.incomplete {
			uncertain = true
		}
	}
	return !uncertain
}

func weakenedUnits(path, oldText, newText string) []namedVerdict {
	oldText = executableTestText(path, oldText)
	newText = executableTestText(path, newText)
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fileVerdict := detect(oldText, newText, path)
	fileDetails := appendVerdict(nil, fileVerdict)
	if !strings.HasSuffix(path, ".go") {
		if len(fileDetails) == 0 {
			return nil
		}
		return []namedVerdict{{name: stem, details: fileDetails, blocking: len(fileVerdict.blocking) != 0}}
	}

	oldUnits := unitsByName(oldText)
	newUnits := unitsByName(newText)
	found := make([]namedVerdict, 0, len(oldUnits)+1)
	namedKinds := make(map[string]bool)
	for _, oldUnit := range oldUnits {
		if oldUnit.incomplete {
			continue
		}
		if !unitBoundaryKnown(newUnits, oldUnit.name) {
			continue
		}
		if strings.TrimSpace(oldUnit.text) == "" {
			continue
		}
		verdict := detect(oldUnit.text, unitText(newUnits, oldUnit.name), path)
		details := appendVerdict(nil, verdict)
		if len(details) == 0 {
			continue
		}
		name := oldUnit.name
		if name == "" {
			name = stem
		}
		found = append(found, namedVerdict{name: name, details: details, blocking: len(verdict.blocking) != 0})
		for _, detail := range details {
			namedKinds[verdictKind(detail)] = true
		}
	}
	// A complete unit cannot account for evidence outside exact declarations.
	// Compare that residual text rather than treating every scanner diagnostic
	// as a finding: malformed unrelated bytes must not duplicate a named verdict.
	unresolved := detect(goUnresolvedText(oldText, newUnits), goUnresolvedText(newText, oldUnits), path)
	// Residual evidence stands on its own: additions in a complete unit can
	// cancel the whole-file count without restoring an unresolved unit's proof.
	residual := appendVerdict(nil, unresolved)
	residualBlocking := len(unresolved.blocking) != 0
	for _, detail := range residual {
		delete(namedKinds, verdictKind(detail))
	}
	for _, detail := range fileDetails {
		if namedKinds[verdictKind(detail)] {
			continue
		}
		if slices.Contains(residual, detail) {
			continue
		}
		residual = append(residual, detail)
		if slices.Contains(fileVerdict.blocking, detail) {
			residualBlocking = true
		}
	}
	if len(residual) != 0 {
		found = append(found, namedVerdict{name: stem, details: residual, blocking: residualBlocking})
	}
	return found
}

type namedVerdict struct {
	name     string
	details  []string
	blocking bool
}

func appendVerdict(target []string, verdict detectorVerdict) []string {
	target = append(target, verdict.blocking...)
	return append(target, verdict.advisory...)
}

func verdictKind(detail string) string {
	at := len(detail)
	if index := strings.IndexByte(detail, '('); index >= 0 && index < at {
		at = index
	}
	if index := strings.IndexByte(detail, ';'); index >= 0 && index < at {
		at = index
	}
	return strings.TrimSpace(detail[:at])
}

func unitsByName(content string) []namedUnit {
	units := goUnits(content)
	grouped := make([]namedUnit, 0, len(units))
	indexes := make(map[string]int, len(units))
	var text textbuf.Buffer
	for _, unit := range units {
		index, found := indexes[unit.name]
		if !found {
			indexes[unit.name] = len(grouped)
			grouped = append(grouped, unit)
			continue
		}
		grouped[index].text = text.Reset().Str(grouped[index].text).Byte('\n').Str(unit.text).String()
		grouped[index].incomplete = grouped[index].incomplete || unit.incomplete
	}
	return grouped
}

func unitText(units []namedUnit, name string) string {
	for _, unit := range units {
		if unit.name == name {
			return unit.text
		}
	}
	return ""
}
