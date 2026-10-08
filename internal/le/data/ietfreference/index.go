// Design: reference/README.md -- the non-normative IETF reference tree
// Related: refresh.go -- the refresh that reads and writes this format
//
// index.go is the INDEX.tsv format: four comment lines, then one row per
// document, tab-separated, sorted by working group, kind and stem.

package dataietfreference

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// columns is the column count of a row, and the order the header names.
const columns = 8

// row is one document in the index. Every field is a column's text, so two
// rows compare equal exactly when the index line they render is the same.
type row struct {
	wg          string
	kind        string
	stem        string
	category    string
	revision    string
	location    string
	obsoletedBy string
	title       string
}

// key names the row across runs: its working group and stem.
func (r *row) key() string { return r.wg + "/" + r.stem }

// compareRows orders rows by working group, kind, then stem.
func compareRows(a, b row) int {
	return cmp.Or(cmp.Compare(a.wg, b.wg), cmp.Compare(a.kind, b.kind), cmp.Compare(a.stem, b.stem))
}

// readIndex answers the previous index's rows by key. A missing index is a
// first run and answers none; a malformed line is an error, because the
// revision a row records decides whether a draft is downloaded again.
func readIndex(root string) (map[string]row, error) {
	path := filepath.Join(root, filepath.FromSlash(indexRel))
	body, err := os.ReadFile(path) //nolint:gosec // the index path is fixed under the checkout root, never user input
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]row{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", indexRel, err)
	}
	rows := map[string]row{}
	for number, line := range strings.Split(string(body), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != columns {
			return nil, fmt.Errorf("%s line %d: %d columns, want %d: repair the line or remove the index to rebuild it", indexRel, number+1, len(fields), columns)
		}
		r := row{
			wg: fields[0], kind: fields[1], stem: fields[2], category: fields[3],
			revision: fields[4], location: fields[5], obsoletedBy: fields[6], title: fields[7],
		}
		rows[r.key()] = r
	}
	return rows, nil
}

// renderIndex writes the header, naming the working groups, then the rows.
func renderIndex(groups []string, rows []row, today time.Time) []byte {
	names := groupNames(groups)
	var b strings.Builder
	b.WriteString("# Snapshot of the IETF datatracker taken " + today.Format(time.DateOnly) +
		": every published " + names + " RFC\n")
	b.WriteString("# whose IETF category is BCP or Informational, and every active " + names + " draft.\n")
	b.WriteString("# Non-normative. See reference/README.md before citing any of these as a requirement.\n")
	b.WriteString("# columns: wg\tkind\tstem\tcategory (drafts: intended status)\trevision\tlocation\tobsoleted-by\ttitle\n")
	for i := range rows {
		r := &rows[i]
		b.WriteString(strings.Join([]string{
			r.wg, r.kind, r.stem, r.category, r.revision, r.location, r.obsoletedBy, r.title,
		}, "\t"))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// groupNames renders the acronyms as prose: "GROW, IDR and OPSEC".
func groupNames(groups []string) string {
	upper := make([]string, len(groups))
	for i, wg := range groups {
		upper[i] = strings.ToUpper(wg)
	}
	if len(upper) == 1 {
		return upper[0]
	}
	return strings.Join(upper[:len(upper)-1], ", ") + " and " + upper[len(upper)-1]
}
