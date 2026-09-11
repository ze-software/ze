// Design: docs/architecture/config/apply-ordering.md -- operation graph solver
// Related: depgraph.go -- graph construction from constraint rules

package transaction

import "errors"

// ErrOperationCycle reports that graph dependencies contain a cycle.
var ErrOperationCycle = errors.New("operation dependency cycle")

// TopologicalSort returns operations in dependency order. When the graph
// contains a cycle composed entirely of address operations on different
// interfaces (IP swap, three-way rotation), the solver relaxes the
// address-uniqueness constraint by setting AllowDual on the affected address
// creations and removing the cycle-causing edges. Non-address cycles or
// same-interface cycles are rejected.
//
// "Address operation" is a verb and a resource kind, not a label: an operation
// whose verb creates or destroys a resource of kind ResourceAddress. No root
// and no operation label is named here.
//
// The last step places the coarse section-apply nodes, which carry no edge and
// which the graph therefore orders against nothing.
func TopologicalSort(graph *OperationGraph) ([]ConfigOperation, error) {
	if graph == nil {
		return nil, nil
	}
	sorted, remaining := kahnSort(graph, graph.edges)
	if len(remaining) == 0 {
		return placeSectionNodes(sorted), nil
	}
	relaxed, cycleMembers, err := tryRelaxCycle(graph, remaining)
	if err != nil {
		return nil, err
	}
	finalSorted, finalRemaining := kahnSort(graph, relaxed)
	if len(finalRemaining) > 0 {
		return nil, ErrOperationCycle
	}
	return placeSectionNodes(markDualPresence(finalSorted, cycleMembers)), nil
}

// placeSectionNodes moves every coarse section-apply node to the one position
// the core can state for it. That position is immediately after the last
// operation that creates or modifies a resource. A node placed there runs
// after every creation the sort put before it, and before the destructions.
//
// It is the sequence the design states: create the address, update the
// services that bind it, destroy the old address last. A root nobody
// decomposes is one of those services
// (docs/architecture/config/apply-ordering.md).
//
// The position is a placement rather than an edge, and that is the whole
// reason this function exists. An edge from every create and to every destroy
// closes a cycle with iface-remove-address-before-add-same-address, which
// orders a destroy BEFORE a create. An operator moving one address between two
// interfaces, with any diff in an uncovered root, would then meet
// ErrOperationCycle on a reload that works today. A coarse node carries no
// edge at all, so moving it constrains nothing and no other operation moves.
//
// Where a destroy is forced before a create, the two halves cannot both hold
// and the creations win. The section applies the config's END state, so it
// reads the resources as the transaction leaves them.
func placeSectionNodes(sorted []ConfigOperation) []ConfigOperation {
	last := -1
	sections := 0
	for i := range sorted {
		if IsSectionApply(&sorted[i]) {
			sections++
			continue
		}
		switch sorted[i].Verb {
		case VerbCreate, VerbModify:
			last = i
		case VerbDestroy:
			// A destroy is what the coarse node runs BEFORE, so it never
			// moves the insertion point.
		}
	}
	if sections == 0 {
		return sorted
	}

	result := make([]ConfigOperation, 0, len(sorted))
	if last < 0 {
		result = appendSectionNodes(result, sorted)
	}
	for i := range sorted {
		if IsSectionApply(&sorted[i]) {
			continue
		}
		result = append(result, sorted[i])
		if i == last {
			result = appendSectionNodes(result, sorted)
		}
	}
	return result
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

// tryRelaxCycle checks whether the cycle among remainingIDs can be broken
// by relaxing the address-uniqueness constraint (R5). A cycle is relaxable
// when every node is an address operation. Only cross-interface edges
// (same address on different interfaces) are removed; same-interface edges
// (make-before-break ordering) are preserved.
//
// Returns the reduced edge set and the set of cycle member IDs.
func tryRelaxCycle(graph *OperationGraph, remainingIDs []string) ([]OperationEdge, map[string]bool, error) {
	cycleSet := make(map[string]bool, len(remainingIDs))
	for _, id := range remainingIDs {
		cycleSet[id] = true
	}

	for _, id := range remainingIDs {
		op := graph.byID[id]
		if !isAddressOperation(&op) {
			return nil, nil, ErrOperationCycle
		}
	}

	var kept []OperationEdge
	removedCross := false
	for _, edge := range graph.edges {
		if cycleSet[edge.FromID] && cycleSet[edge.ToID] {
			from := graph.byID[edge.FromID]
			to := graph.byID[edge.ToID]
			fromIface := opInterface(&from)
			toIface := opInterface(&to)
			if fromIface == "" || toIface == "" {
				return nil, nil, ErrOperationCycle
			}
			if fromIface != toIface {
				removedCross = true
				continue
			}
		}
		kept = append(kept, edge)
	}
	if !removedCross {
		return nil, nil, ErrOperationCycle
	}
	return kept, cycleSet, nil
}

// isAddressOperation reports whether op creates or destroys an address. It
// reads the verb and the target kind, which is the whole vocabulary the solver
// has: the operation's label belongs to the component that emitted it, and a
// solver that compared labels would relax cycles for the two roots whose
// spellings it happened to know.
func isAddressOperation(op *ConfigOperation) bool {
	if op.Target.Kind != ResourceAddress {
		return false
	}
	switch op.Verb {
	case VerbCreate, VerbDestroy:
		return true
	default:
		return false
	}
}

func opInterface(op *ConfigOperation) string {
	if op.Target.Interface != "" {
		return op.Target.Interface
	}
	return op.Params.Interface
}

// markDualPresence sets AllowDual on the address creations that were part of a
// relaxed cycle. Like isAddressOperation it decides on the verb and the target
// kind, never on the label.
func markDualPresence(sorted []ConfigOperation, cycleMembers map[string]bool) []ConfigOperation {
	if len(cycleMembers) == 0 {
		return sorted
	}
	result := make([]ConfigOperation, len(sorted))
	for i := range sorted {
		result[i] = sorted[i]
		if !cycleMembers[result[i].ID] {
			continue
		}
		if result[i].Verb != VerbCreate || result[i].Target.Kind != ResourceAddress {
			continue
		}
		result[i].Params.AllowDual = true
	}
	return result
}
