// Related: retired.go -- the rename map these tests walk

package leretired

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: a refusal never names a retired line. For every row of the rename
// map, the old words alone and the old words followed by any tail a caller
// could type (one plain word, or the remainder of any row's own old words, so
// a successor that opens another retired spelling is reached) answer a
// successor that Match does not match.
// PREVENTS: a one-hop successor, where `spec session claim spec plan/x.md`
// answered `spec claim spec plan/x.md`, itself a retired spelling, so the
// caller who followed the refusal was refused a second time.
//
// Method: the tails are derived from the map, not listed, so a new row that
// chains to an old one is walked without editing this test.
func TestSuccessorIsNeverARetiredLine(t *testing.T) {
	tails := [][]string{nil, {"trailing"}}
	for _, row := range Renames() {
		old := row.Old()
		for cut := 1; cut < len(old); cut++ {
			tails = append(tails, append(slices.Clone(old[cut:]), "trailing"))
		}
		tails = append(tails, append(slices.Clone(old), "trailing"))
	}
	chained := 0
	for _, row := range Renames() {
		for _, tail := range tails {
			line := append(row.Old(), tail...)
			matched, ok := Match(line)
			if !ok {
				t.Fatalf("`le %s` names no row, though it opens with the row `le %s`",
					strings.Join(line, " "), strings.Join(row.Old(), " "))
			}
			successor := matched.Successor(line)
			if next, retired := Match(successor); retired {
				t.Errorf("`le %s` is refused naming `le %s`, which is the retired spelling `le %s` in turn",
					strings.Join(line, " "), strings.Join(successor, " "), strings.Join(next.Old(), " "))
				continue
			}
			if _, oneHop := Match(append(matched.New(), line[len(matched.Old()):]...)); oneHop {
				chained++
			}
		}
	}
	// The map holds chains today (`spec session claim spec`), so a walk that
	// met none would no longer prove the fixed point is reached.
	if chained == 0 {
		t.Error("no walked line chains through a second row, so this test no longer exercises the fixed point")
	}
}
