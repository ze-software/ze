// Design: docs/architecture/config/apply-ordering.md -- operation graph solver
// Related: depgraph.go -- graph construction from constraint rules

package transaction

import "errors"

// ErrOperationCycle reports that graph dependencies contain a cycle.
var ErrOperationCycle = errors.New("operation dependency cycle")

// TopologicalSort returns operations in dependency order, and rejects a graph
// that holds a cycle.
//
// Nothing relaxes a cycle. The solver used to break an address swap by
// removing its cross-interface edges, which left both addresses on the host
// while they traded places. That window was make-before-break, and the
// requirement asks for the opposite order: remove the disturbed addresses with
// the binder stopped, then add them back. The rule that closed the cycle is
// gone with the policy, so a swap now carries one destroy and one create for
// each address with a single edge between them, and no cycle to break
// (docs/architecture/config/apply-ordering.md).
//
// The last step places the coarse section-apply nodes, which carry no edge and
// which the graph therefore orders against nothing.
func TopologicalSort(graph *OperationGraph) ([]ConfigOperation, error) {
	if graph == nil {
		return nil, nil
	}
	sorted, remaining := kahnSort(graph, graph.edges)
	if len(remaining) > 0 {
		return nil, ErrOperationCycle
	}
	return placeSectionNodes(sorted), nil
}

// placeSectionNodes moves the coarse section-apply nodes into the gap between
// phase 4 and phase 5 of the requirement: after the addresses and the
// interfaces this commit adds, and before the first operation that starts
// something which binds them
// (docs/architecture/config/apply-ordering.md, "The five phases").
//
// The core knows nothing about a root nobody decomposes, so the fail-safe
// default reads that participant as a binder, and a binder belongs in phase 5.
// Inside phase 5 it runs BEFORE the decomposed starts, because a binder has to
// find the subsystems it uses already configured: the `bgp` root's peers start
// after the plugins that configure the RIB, the graceful-restart state and the
// filters have applied their own sections.
//
// That order is derived from the verb and the resource kind every operation
// already declares. It names no root, no participant and no operation label,
// and it replaced a sort that moved the participant called "bgp" to the tail of
// the slice (ai/rules/principles.md).
//
// The position is a placement rather than an edge, and that is the whole reason
// this function exists. A coarse node stands for a whole participant section,
// so it carries no resource identity, and nothing in the graph can state where
// it goes. An edge invented for it would join it to cycles the operator never
// wrote.
func placeSectionNodes(sorted []ConfigOperation) []ConfigOperation {
	sections := 0
	for i := range sorted {
		if IsSectionApply(&sorted[i]) {
			sections++
		}
	}
	if sections == 0 {
		return sorted
	}

	ordered := make([]ConfigOperation, 0, len(sorted)-sections)
	for i := range sorted {
		if IsSectionApply(&sorted[i]) {
			continue
		}
		ordered = append(ordered, sorted[i])
	}

	at := sectionNodePosition(ordered)
	result := make([]ConfigOperation, 0, len(sorted))
	result = append(result, ordered[:at]...)
	result = appendSectionNodes(result, sorted)
	return append(result, ordered[at:]...)
}

// sectionNodePosition returns the index in ops at which the coarse nodes are
// inserted. It is the first binder start that follows the last addressing
// addition, and the end of the slice when no binder starts after it.
//
// The two bounds cross when a binder start sorts ahead of an address this
// commit adds. No edge can produce that for a binder which declares the address
// it binds, so the operations are independent where it happens, and the
// addressing bound wins: a participant started before its address is the
// failure the requirement exists to prevent, while one started late costs a
// session restart (docs/architecture/config/apply-ordering.md, "The fail-safe
// default").
func sectionNodePosition(ops []ConfigOperation) int {
	addressingDone := 0
	for i := range ops {
		if addsAddressing(&ops[i]) {
			addressingDone = i + 1
		}
	}
	for i := addressingDone; i < len(ops); i++ {
		if startsABinder(&ops[i]) {
			return i
		}
	}
	return len(ops)
}

// addsAddressing reports whether op is one of the requirement's phase 4
// additions: an address this commit adds or moves, or the interface that
// carries one.
func addsAddressing(op *ConfigOperation) bool {
	if op.Verb != VerbCreate {
		return false
	}
	return isAddressingKind(op.Target.Kind)
}

// startsABinder reports whether op is one of the requirement's phase 5 starts:
// it brings up or changes something that BINDS the addressing rather than
// providing the addressing itself.
//
// A destroy is never one. The stops are phases 1 and 2, and they run before the
// removals whatever they touch.
func startsABinder(op *ConfigOperation) bool {
	switch op.Verb {
	case VerbCreate, VerbModify:
		return !isAddressingKind(op.Target.Kind)
	default:
		return false
	}
}

// isAddressingKind reports whether kind names a resource of the ADDRESSING
// layer: the local addresses a commit moves, and the interfaces that carry
// them. It is the line the requirement's phases draw. Phases 3 and 4 remove and
// add the addressing, and phase 5 starts what binds it.
//
// This is the only place the engine reads a resource kind for anything but
// identity. A root that PROVIDES addressing declares one of these two kinds,
// which is what `interface` does today. A root that BINDS addressing declares
// its own kind and lands on the phase 5 side with no edit here, which is what
// `bgp` does with ResourcePeer.
func isAddressingKind(kind ResourceKind) bool {
	switch kind {
	case ResourceAddress, ResourceInterface:
		return true
	default:
		return false
	}
}

// appendSectionNodes appends the coarse nodes of sorted, in the order the sort
// left them, so two uncovered participants keep the order the planner gave
// them.
func appendSectionNodes(result, sorted []ConfigOperation) []ConfigOperation {
	for i := range sorted {
		if !IsSectionApply(&sorted[i]) {
			continue
		}
		result = append(result, sorted[i])
	}
	return result
}

func kahnSort(graph *OperationGraph, edges []OperationEdge) (sorted []ConfigOperation, remainingIDs []string) {
	out := make(map[string][]OperationEdge, len(graph.operations))
	for _, edge := range edges {
		out[edge.FromID] = append(out[edge.FromID], edge)
	}
	indegree := make(map[string]int, len(graph.operations))
	for i := range graph.operations {
		indegree[graph.operations[i].ID] = 0
	}
	for _, edge := range edges {
		indegree[edge.ToID]++
	}

	queue := make([]string, 0, len(graph.operations))
	for i := range graph.operations {
		op := &graph.operations[i]
		if indegree[op.ID] == 0 {
			queue = append(queue, op.ID)
		}
	}

	sorted = make([]ConfigOperation, 0, len(graph.operations))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, graph.byID[id])
		for _, edge := range out[id] {
			indegree[edge.ToID]--
			if indegree[edge.ToID] == 0 {
				queue = append(queue, edge.ToID)
			}
		}
	}

	if len(sorted) == len(graph.operations) {
		return sorted, nil
	}

	sortedSet := make(map[string]bool, len(sorted))
	for i := range sorted {
		sortedSet[sorted[i].ID] = true
	}
	remainingIDs = make([]string, 0, len(graph.operations)-len(sorted))
	for i := range graph.operations {
		if !sortedSet[graph.operations[i].ID] {
			remainingIDs = append(remainingIDs, graph.operations[i].ID)
		}
	}
	return sorted, remainingIDs
}
