// Design: docs/guide/migrating-from-vyos.md -- the conversion record.
// Related: home_lab_render.go -- the converted configuration the record explains.
// Related: check_conversion.go -- the scenario that runs this check.
package topology

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// recordRow is one row of the conversion record: the source line number and
// text, the Ze form (or "dropped: <reason>") and the mapping class.
type recordRow struct {
	line   int
	source string
	ze     string
	class  string
}

// recordCells is the column count of the record's table (#, VyOS, Ze,
// Class, Note).
const recordCells = 5

// recordDropped opens the Ze cell of a source line with no Ze form.
const recordDropped = "dropped:"

// parseConversionRecord reads the record's table rows: every line that opens
// with "|" and whose first cell is a line number. The header and its
// separator carry no number and are skipped; a numbered row with the wrong
// cell count is refused rather than read short.
func parseConversionRecord(text string) ([]recordRow, error) {
	var rows []recordRow
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		number, err := strconv.Atoi(strings.TrimSpace(cells[0]))
		if err != nil {
			continue
		}
		if len(cells) != recordCells {
			return nil, fmt.Errorf("conversion record row %d holds %d cells, want %d (#, VyOS, Ze, Class, Note): %q", number, len(cells), recordCells, trimmed)
		}
		rows = append(rows, recordRow{line: number,
			source: strings.Trim(strings.TrimSpace(cells[1]), "`"),
			ze:     strings.TrimSpace(cells[2]),
			class:  strings.TrimSpace(cells[3])})
	}
	return rows, nil
}

// recordCoverage proves the record names every source line, in input order,
// with the line's own text, a Ze form or a dropped reason, and a class. It
// refuses a row for a line the source does not hold, so the record cannot
// explain a line that is not there. Every fault is reported, not the first.
func recordCoverage(source string, rows []recordRow) error {
	lines := strings.Split(strings.TrimRight(source, "\n"), "\n")
	var faults []error
	for index, text := range lines {
		number := index + 1
		if index >= len(rows) {
			faults = append(faults, fmt.Errorf("vyos.conf line %d %q has no record row; add it to %s", number, text, cpeRecordFile))
			continue
		}
		if err := rowCovers(rows[index], number, text); err != nil {
			faults = append(faults, err)
		}
	}
	for _, extra := range rows[min(len(rows), len(lines)):] {
		faults = append(faults, fmt.Errorf("record row %d names %q, which vyos.conf does not hold (it has %d lines)", extra.line, extra.source, len(lines)))
	}
	return errors.Join(faults...)
}

// rowCovers judges one row against the source line it must explain.
func rowCovers(row recordRow, number int, text string) error {
	if row.line != number {
		return fmt.Errorf("record row %d stands where vyos.conf line %d %q belongs; keep one row per line in input order", row.line, number, text)
	}
	if row.source != text {
		return fmt.Errorf("record row %d quotes %q, but vyos.conf line %d reads %q; re-run the conversion for the changed line", row.line, row.source, number, text)
	}
	if row.ze == "" {
		return fmt.Errorf("record row %d gives no Ze form for %q; name the Ze lines or write %q with a reason", row.line, text, recordDropped+" <reason>")
	}
	if reason, dropped := strings.CutPrefix(row.ze, recordDropped); dropped {
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("record row %d drops %q with no reason; write the reason after %q", row.line, text, recordDropped)
		}
	}
	if row.class == "" {
		return fmt.Errorf("record row %d gives no class for %q; name the mapping row's class, or - when it has none", row.line, text)
	}
	return nil
}
