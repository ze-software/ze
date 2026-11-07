// Design: docs/architecture/plugin/feature-gates.md -- one analysis answers each reflectedValue query once per source, scope and depth.
// Related: externalreflection.go -- reflectedValue asks this table before it walks.
package repoinventory

// externalReflectionMemoSlots bounds the answers one analysis keeps. A query
// past the bound is answered by its walk, as before the table existed.
const externalReflectionMemoSlots = 1 << 18

// externalReflectionMemoKey is the part of a reflectedValue query that names
// what it walks: the type expression and the substitution scope it is read in.
// The scope is compared by identity; a fresh scope chain is a new key, which
// only costs a walk.
type externalReflectionMemoKey struct {
	source sourceExpr
	scope  *externalTypeScope
}

type externalReflectionMemoSlot struct {
	depth  int
	answer externalReflection
}

// externalReflectionMemo keeps reflectedValue answers for the whole analysis.
//
// A reflectedValue answer is a function of its source, scope and incoming
// depth alone. Past its ambient check, the query reads the captured source,
// the type identity index, the method censuses (built once per package from
// that source), the standard-library entry sets and the analysis-wide frame
// table, and nothing an analysis step writes: the ambient pool, the bindings
// and the settled type values are never read by a reflection walk. The frame
// table only holds each exact frame's final contribution, the same from every
// query (externalReflectionWalk). So a second query with the same source,
// scope and depth answers what the first did. The depth is matched exactly:
// the walk's entry guard and the value-root chain (interfaceRoot) read it.
//
// The ambient check stays in front of the table: once the pool holds an
// unknown, every query answers that unknown, as before.
//
// Answers are shared: a caller MUST only read one (every caller joins it,
// which copies). Not safe for concurrent use; one analysis runs on one
// goroutine.
type externalReflectionMemo struct {
	answers map[externalReflectionMemoKey][]externalReflectionMemoSlot
	slots   int
}

func (m *externalReflectionMemo) answer(key externalReflectionMemoKey, depth int) (externalReflection, bool) {
	for _, slot := range m.answers[key] {
		if slot.depth == depth {
			return slot.answer, true
		}
	}
	return externalReflection{}, false
}

// keep stores an answer while the table has room.
func (m *externalReflectionMemo) keep(key externalReflectionMemoKey, depth int, answer externalReflection) {
	if m.slots >= externalReflectionMemoSlots {
		return
	}
	if m.answers == nil {
		m.answers = make(map[externalReflectionMemoKey][]externalReflectionMemoSlot)
	}
	m.answers[key] = append(m.answers[key], externalReflectionMemoSlot{depth: depth, answer: answer})
	m.slots++
}
