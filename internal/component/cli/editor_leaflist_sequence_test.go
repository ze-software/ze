package cli

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// Goal: one session's member edits on a leaf-list mean the same thing in
// every place the editor replays them. Method: run a sequence of operator
// edits (set, delete, deactivate, activate, insert) in a fresh session, then
// compare the view the operator saw with the view after SaveDraft, the draft
// file, the view reloadSessionView rebuilds, the content
// CommitSessionCandidate answers, and the file CommitSession writes. Two list
// kinds run: the bracket `address` leaf-list, where `set` writes the whole
// list, and the ordered plain `bgp filter import` leaf-list, where `set` adds
// one member. A step the editor refuses is part of the sequence: the refusal
// must leave nothing behind either.

// seqList describes one leaf-list kind the sequences drive.
type seqList struct {
	name     string
	parent   []string
	key      string
	universe []string
	ordered  bool
	setWhole bool
	starts   []seqStart
	config   func(seqStart) string
}

// seqStart is the committed list a sequence starts from.
type seqStart struct{ members, inactive string }

type seqOp struct {
	kind string
	a, b string
	pos  string
	set  []string
}

func (o seqOp) String() string {
	switch o.kind {
	case "set":
		return "set [" + strings.Join(o.set, " ") + "]"
	case "insert":
		if o.pos == config.InsertFirst || o.pos == config.InsertLast {
			return "insert " + o.a + " " + o.pos
		}
		return "insert " + o.a + " " + o.pos + " " + o.b
	}
	return o.kind + " " + o.a
}

var seqAddress = seqList{
	name:     "address",
	parent:   ifaceIPv4Path,
	key:      "address",
	universe: []string{"192.0.2.1/24", "192.0.2.2/30", "192.0.2.3/30", "192.0.2.4/30"},
	setWhole: true,
	starts: []seqStart{
		{"192.0.2.1/24 192.0.2.2/30", ""},
		{"192.0.2.1/24 192.0.2.2/30", "192.0.2.2/30"},
		{"192.0.2.1/24", ""},
	},
	config: func(st seqStart) string {
		return validBGPConfig + bracketIfaceWithInactive(st.members, st.inactive)
	},
}

var seqImport = seqList{
	name:     "import",
	parent:   []string{"bgp", "filter"},
	key:      "import",
	universe: []string{"alpha", "bravo", "charlie", "delta"},
	ordered:  true,
	starts: []seqStart{
		{"alpha bravo", ""},
		{"alpha bravo", "bravo"},
		{"alpha", ""},
	},
	config: func(st seqStart) string {
		line := "import [ " + st.members + " ];"
		if st.inactive != "" {
			line += "\n    inactive: import " + st.inactive + ";"
		}
		return strings.Replace(insertConflictConfig, "import [ alpha bravo ];", line, 1)
	},
}

// seqState renders the list under parent in tree, a deactivated member marked
// with "!", sorted unless the list is ordered.
func (l *seqList) seqState(ed *Editor, tree *config.Tree) string {
	if tree == nil {
		return "[]"
	}
	target := walkPath(tree, ed.schema, l.parent)
	if target == nil {
		return "[]"
	}
	var members []string
	for _, s := range target.GetMultiValuesState(l.key) {
		if s.Inactive {
			members = append(members, "!"+s.Value)
			continue
		}
		members = append(members, s.Value)
	}
	if !l.ordered {
		slices.SortFunc(members, func(a, b string) int {
			return strings.Compare(strings.TrimPrefix(a, "!"), strings.TrimPrefix(b, "!"))
		})
	}
	if target.IsLeafInactive(l.key) {
		members = append(members, "(leaf-inactive)")
	}
	return "[" + strings.Join(members, " ") + "]"
}

func (l *seqList) member(m string) []string {
	return append(append(append([]string{}, l.parent...), l.key), m)
}

func (l *seqList) apply(ed *Editor, o seqOp) error {
	switch o.kind {
	case "set":
		return ed.SetValue(l.parent, l.key, strings.Join(o.set, " "))
	case "delete":
		return ed.DeleteByPath(l.member(o.a))
	case "deactivate":
		_, err := ed.ApplyActivation(l.member(o.a), false)
		return err
	case "activate":
		_, err := ed.ApplyActivation(l.member(o.a), true)
		return err
	case "insert":
		ref := o.b
		if o.pos == config.InsertFirst || o.pos == config.InsertLast {
			ref = ""
		}
		return ed.InsertLeafListValue(l.parent, l.key, o.a, o.pos, ref)
	}
	return fmt.Errorf("unknown op %q", o.kind)
}

// generate answers n random ops over the list's universe.
func (l *seqList) generate(r *rand.Rand, n int) []seqOp {
	positions := []string{config.InsertFirst, config.InsertLast, config.InsertBefore, config.InsertAfter}
	pick := func() string { return l.universe[r.Intn(len(l.universe))] }
	ops := make([]seqOp, 0, n)
	for range n {
		switch r.Intn(5) {
		case 0:
			if !l.setWhole {
				ops = append(ops, seqOp{kind: "set", set: []string{pick()}})
				continue
			}
			perm := r.Perm(len(l.universe))
			var set []string
			for _, p := range perm[:1+r.Intn(len(l.universe))] {
				set = append(set, l.universe[p])
			}
			ops = append(ops, seqOp{kind: "set", set: set})
		case 1:
			ops = append(ops, seqOp{kind: "delete", a: pick()})
		case 2:
			ops = append(ops, seqOp{kind: "deactivate", a: pick()})
		case 3:
			ops = append(ops, seqOp{kind: "activate", a: pick()})
		case 4:
			ops = append(ops, seqOp{kind: "insert", a: pick(), b: pick(), pos: positions[r.Intn(len(positions))]})
		}
	}
	return ops
}

// session runs ops in a fresh session over st, answering the editor, its
// config path, and the sequence as text with refused steps marked.
func (l *seqList) session(t *testing.T, st seqStart, ops []seqOp) (*Editor, string, string) {
	t.Helper()
	ed, _, configPath := newLoadSessionEditor(t, l.config(st))
	desc := make([]string, 0, len(ops))
	for _, o := range ops {
		s := o.String()
		if err := l.apply(ed, o); err != nil {
			s += " (refused)"
		}
		desc = append(desc, s)
	}
	return ed, configPath, fmt.Sprintf("%s start=[%s] inactive=[%s]: %s", l.name, st.members, st.inactive, strings.Join(desc, "; "))
}

// check runs ops twice, once to reload, commit as candidate and commit, once
// to save a draft and commit it, and answers one line for each place whose
// list differs from the view.
func (l *seqList) check(t *testing.T, st seqStart, ops []seqOp) []string {
	t.Helper()
	var fails []string
	ed, configPath, desc := l.session(t, st, ops)
	view := l.seqState(ed, ed.tree)
	report := func(where, got string) {
		fails = append(fails, fmt.Sprintf("%s: view %s, %s %s", desc, view, where, got))
	}

	if _, content, err := ed.CommitSessionCandidate(time.Now()); err != nil {
		report("candidate", "error "+err.Error())
	} else if content != "" {
		tree, _, perr := config.NewSetParser(ed.schema).ParseWithMeta(content)
		require.NoError(t, perr)
		if got := l.seqState(ed, tree); got != view {
			report("candidate", got)
		}
	}
	l.compareCommit(t, ed, configPath, view, report)

	guardEd, guardPath, _ := l.session(t, st, ops)
	guard, err := guardEd.store.AcquireLock(guardPath)
	require.NoError(t, err)
	rerr := guardEd.reloadSessionView(guard)
	guard.Release() //nolint:errcheck // Test lock, released on every path.
	if rerr != nil {
		report("reload", "error "+rerr.Error())
	} else if got := l.seqState(guardEd, guardEd.tree); got != view {
		report("reload", got)
	}

	draftEd, draftPath, _ := l.session(t, st, ops)
	if err := draftEd.SaveDraft(); err != nil {
		report("save", "error "+err.Error())
		return fails
	}
	if got := l.seqState(draftEd, draftEd.tree); got != view {
		report("view after save", got)
	}
	// The draft file is read with the ops SaveDraft keeps in the change file,
	// as every reader of this session's draft replays them.
	if data, rerr := draftEd.store.ReadFile(DraftPath(draftPath)); rerr == nil {
		tree, _, perr := config.NewSetParser(draftEd.schema).ParseWithMeta(string(data))
		require.NoError(t, perr)
		var ops []config.StructuralOp
		if change, cerr := draftEd.store.ReadFile(ChangePath(draftPath, draftEd.session.User)); cerr == nil {
			_, _, ops, perr = config.ParseChangeFile(string(change), config.NewSetParser(draftEd.schema))
			require.NoError(t, perr)
		}
		require.NoError(t, applyStructuralOps(tree, draftEd.schema, ops, true))
		if got := l.seqState(draftEd, tree); got != view {
			report("draft file", got)
		}
	}
	l.compareCommit(t, draftEd, draftPath, view, report)
	return fails
}

func (l *seqList) compareCommit(t *testing.T, ed *Editor, configPath, view string, report func(string, string)) {
	t.Helper()
	res, err := ed.CommitSession()
	if err != nil {
		report("commit", "error "+err.Error())
		return
	}
	if len(res.Conflicts) > 0 {
		report("commit", fmt.Sprintf("conflicts %+v", res.Conflicts))
		return
	}
	if got := l.seqState(ed, committedLoadTree(t, ed, configPath)); got != view {
		report("commit", got)
	}
}

// TestLeafListSequencesAgree runs the review's named sequences, then a seeded,
// bounded random set of sequences, over both list kinds.
// VALIDATES: round-5 BLOCKER 1 (insert, then deactivate the inserted member),
// BLOCKER 2 (a member edit after a whole set), BLOCKER 3 (delete, then insert
// the committed member again), ISSUE 1 (an insert after an inserted reference,
// and before a deactivated one), and two siblings a wider sweep found on the
// plain list: `set` of a member the session deleted, and an insert after a
// `set` that appended a member.
// PREVENTS: a view the save, the reload or either commit path disagrees with.
func TestLeafListSequencesAgree(t *testing.T) {
	a := seqAddress.universe
	named := []struct {
		list *seqList
		st   seqStart
		ops  []seqOp
	}{
		{&seqAddress, seqAddress.starts[0], []seqOp{{kind: "insert", a: a[3], pos: config.InsertBefore, b: a[0]}, {kind: "deactivate", a: a[3]}}},
		{&seqAddress, seqAddress.starts[2], []seqOp{{kind: "insert", a: a[3], pos: config.InsertLast}, {kind: "deactivate", a: a[3]}, {kind: "activate", a: a[3]}}},
		{&seqAddress, seqAddress.starts[2], []seqOp{{kind: "set", set: []string{a[2]}}, {kind: "insert", a: a[3], pos: config.InsertFirst}}},
		{&seqAddress, seqAddress.starts[0], []seqOp{{kind: "set", set: []string{a[1]}}, {kind: "delete", a: a[1]}}},
		{&seqAddress, seqAddress.starts[0], []seqOp{{kind: "delete", a: a[0]}, {kind: "insert", a: a[0], pos: config.InsertBefore, b: a[1]}}},
		{&seqAddress, seqAddress.starts[2], []seqOp{{kind: "insert", a: a[2], pos: config.InsertFirst}, {kind: "insert", a: a[3], pos: config.InsertAfter, b: a[2]}}},
		{&seqAddress, seqAddress.starts[1], []seqOp{{kind: "insert", a: a[2], pos: config.InsertBefore, b: a[1]}}},
		{&seqImport, seqImport.starts[0], []seqOp{{kind: "insert", a: "charlie", pos: config.InsertLast}, {kind: "deactivate", a: "charlie"}}},
		{&seqImport, seqImport.starts[0], []seqOp{{kind: "delete", a: "alpha"}, {kind: "insert", a: "alpha", pos: config.InsertAfter, b: "bravo"}}},
		{&seqImport, seqImport.starts[2], []seqOp{{kind: "insert", a: "charlie", pos: config.InsertFirst}, {kind: "insert", a: "delta", pos: config.InsertAfter, b: "charlie"}}},
		{&seqImport, seqImport.starts[1], []seqOp{{kind: "insert", a: "charlie", pos: config.InsertBefore, b: "bravo"}}},
		{&seqImport, seqImport.starts[1], []seqOp{{kind: "delete", a: "bravo"}, {kind: "insert", a: "delta", pos: config.InsertLast}, {kind: "set", set: []string{"bravo"}}}},
		{&seqImport, seqImport.starts[2], []seqOp{{kind: "set", set: []string{"delta"}}, {kind: "insert", a: "bravo", pos: config.InsertLast}}},
		{&seqImport, seqImport.starts[2], []seqOp{{kind: "delete", a: "alpha"}, {kind: "set", set: []string{"alpha"}}, {kind: "set", set: []string{"charlie"}}}},
	}
	var fails []string
	for _, tc := range named {
		fails = append(fails, tc.list.check(t, tc.st, tc.ops)...)
	}

	// The bound keeps the race run short; the seed keeps every run the same.
	const sequences = 40
	for _, list := range []*seqList{&seqAddress, &seqImport} {
		r := rand.New(rand.NewSource(5)) //nolint:gosec // Deterministic test sequences, not security.
		for range sequences {
			st := list.starts[r.Intn(len(list.starts))]
			fails = append(fails, list.check(t, st, list.generate(r, 1+r.Intn(4)))...)
		}
	}
	for _, f := range fails {
		t.Log(f)
	}
	require.Empty(t, fails)
}
