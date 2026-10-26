// Related: retired.go -- the rename map these tests walk

package leretired

import (
	"slices"
	"strconv"
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

// VALIDATES: a chain one hop longer than the map's row count, which reuses a
// row on its way, is followed to its end: `a` answers `z` through
// a->b, b->a y, a y->b y (row a again), b y->z.
// PREVENTS: the cycle panic firing on the line after the last hop the loop
// allowed, which it never checked, so a legitimate chain of that length
// panicked the le every session runs.
func TestSuccessorFollowsAChainOneHopLongerThanTheMap(t *testing.T) {
	chain := newRenameTable([]Rename{
		{Retired: "a", Command: "b"},
		{Retired: "b", Command: "a y"},
		{Retired: "b y", Command: "z"},
	})
	got := chain.successor(chain.rows[0], []string{"a"})
	if !slices.Equal(got, []string{"z"}) {
		t.Errorf("the successor of `a` is %q, want [z]", got)
	}
}

// VALIDATES: a rename map that holds a cycle still panics with the BUG
// message naming the line it reached.
// PREVENTS: a bound fix that turned a cycle into an endless loop or into a
// silent answer that is itself a retired line.
func TestSuccessorPanicsOnACycle(t *testing.T) {
	cycle := newRenameTable([]Rename{
		{Retired: "a", Command: "b"},
		{Retired: "b", Command: "a"},
	})
	defer func() {
		message, isString := recover().(string)
		if !isString || !strings.HasPrefix(message, "BUG: leretired: the rename map holds a cycle through `") {
			t.Errorf("a cyclic map answered the panic %q, want the cycle BUG", message)
		}
	}()
	got := cycle.successor(cycle.rows[0], []string{"a"})
	t.Errorf("a cyclic map answered %q instead of panicking", got)
}

// VALIDATES: a chain that reuses rows while its trailing words grow, and so
// takes more hops than the map has rows, is followed to its end: `a` answers
// `z` through a->b, b->a q, a q->b q (row a again), b q->a q q (row b again),
// a q q->b q q (row a a third time), b q q->z. Six hops over three rows.
// PREVENTS: a hop count above the row count taken for a cycle, which panicked
// on a chain that ends, though no line on it repeats.
func TestSuccessorFollowsAChainThatReusesRowsAsItGrows(t *testing.T) {
	chain := newRenameTable([]Rename{
		{Retired: "a", Command: "b"},
		{Retired: "b", Command: "a q"},
		{Retired: "b q q", Command: "z"},
	})
	got := chain.successor(chain.rows[0], []string{"a"})
	if !slices.Equal(got, []string{"z"}) {
		t.Errorf("the successor of `a` is %q, want [z]", got)
	}
}

// VALIDATES: a rename map whose row re-opens its own old words, so the line
// grows on every hop and never repeats, panics with the BUG message naming the
// hop bound rather than looping.
// PREVENTS: a repeat check alone, which never fires on a growing line, turning
// such a declaration into an le that hangs every session.
func TestSuccessorPanicsOnAChainThatNeverSettles(t *testing.T) {
	growing := newRenameTable([]Rename{
		{Retired: "a", Command: "a q"},
	})
	defer func() {
		message, isString := recover().(string)
		want := "BUG: leretired: the rename of `a` did not settle within " +
			strconv.Itoa(successorHopsMax) + " hops, at `a "
		if !isString || !strings.HasPrefix(message, want) {
			t.Errorf("a growing map answered the panic %q, want the hop-bound BUG", message)
		}
	}()
	got := growing.successor(growing.rows[0], []string{"a"})
	t.Errorf("a growing map answered %q instead of panicking", got)
}

// VALIDATES: a real row whose successor re-opens its own old words, typed with
// many more of them than the hop bound, is followed to its end: every hop
// shortens the line by one word, so the chain is as long as the caller made
// it. `test harness` then 100 `harness` answers `test`, `rollback newest` then
// 100 `newest` answers `rollback`, and `spec claim spec` then 100 `spec`
// answers `spec claim`.
// PREVENTS: a fixed hop bound reached by a line the user typed, which panicked
// the le with a BUG that blames the declaration for the caller's input.
func TestSuccessorFollowsAShorteningChainLongerThanTheBound(t *testing.T) {
	cases := []struct {
		typed  []string
		repeat string
		want   []string
	}{
		{typed: []string{"test", "harness"}, repeat: "harness", want: []string{"test"}},
		{typed: []string{"rollback", "newest"}, repeat: "newest", want: []string{"rollback"}},
		{typed: []string{"spec", "claim", "spec"}, repeat: "spec", want: []string{"spec", "claim"}},
	}
	for _, tc := range cases {
		line := slices.Clone(tc.typed)
		for range 100 {
			line = append(line, tc.repeat)
		}
		row, ok := Match(line)
		if !ok {
			t.Fatalf("`le %s ...` names no row of the real map", strings.Join(tc.typed, " "))
		}
		if got := row.Successor(line); !slices.Equal(got, tc.want) {
			t.Errorf("the successor of `le %s` then 100 `%s` is %q, want %q",
				strings.Join(tc.typed, " "), tc.repeat, got, tc.want)
		}
	}
}
