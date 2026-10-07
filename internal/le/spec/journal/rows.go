// Design: docs/contributing/feature-maturity.md -- the Defect review staleness S5 reads
// Related: journal.go -- the HEAD reader and the row parser these rows come from

package specjournal

// Row is one well-formed journal row, as committed at HEAD.
type Row struct {
	Class   string
	Date    string
	Spec    string
	Surface string
	Symptom string
	Fix     string
}

// HeadRows answers every well-formed row of every journal class at HEAD, through
// the same reader Check uses, so a consumer never parses plan/journal itself. A
// malformed row is left out: Check already refuses it by name.
func HeadRows(tree string) ([]Row, error) {
	paths, err := classFilesAtHead(tree)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	classes, err := readClasses(tree, paths)
	if err != nil {
		return nil, err
	}
	var rows []Row
	for _, class := range classes {
		for _, row := range class.rows {
			if row.malformed {
				continue
			}
			rows = append(rows, Row{Class: class.name, Date: row.cells[0], Spec: row.cells[1],
				Surface: row.cells[2], Symptom: row.cells[3], Fix: row.cells[4]})
		}
	}
	return rows, nil
}
