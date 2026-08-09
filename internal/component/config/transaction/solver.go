// Design: docs/architecture/config/apply-ordering.md -- operation graph solver
// Related: depgraph.go -- graph construction from constraint rules

package transaction

import "errors"

// ErrOperationCycle reports that graph dependencies contain a cycle.
var ErrOperationCycle = errors.New("operation dependency cycle")

// TopologicalSort returns operations in dependency order. When the graph
// contains a cycle composed entirely of address operations on different
// interfaces (IP swap, three-way rotation), the solver relaxes the
// address-uniqueness constraint by setting AllowDual on the affected
// ADD_ADDRESS operations and removing the cycle-causing edges.
// Non-address cycles or same-interface cycles are rejected.
func TopologicalSort(graph *OperationGraph) ([]ConfigOperation, error) {
	if graph == nil {
		return nil, nil
	}
	sorted, remaining := kahnSort(graph, graph.edges)
	if len(remaining) == 0 {
		return sorted, nil
	}
	relaxed, cycleMembers, err := tryRelaxCycle(graph, remaining)
	if err != nil {
		return nil, err
	}
	finalSorted, finalRemaining := kahnSort(graph, relaxed)
	if len(finalRemaining) > 0 {
		return nil, ErrOperationCycle
	}
	return markDualPresence(finalSorted, cycleMembers), nil
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

func isAddressOperation(op *ConfigOperation) bool {
	switch op.Type {
	case OperationAddAddress, OperationRemoveAddress:
		return op.Target.Kind == ResourceAddress
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

// markDualPresence sets AllowDual on ADD_ADDRESS operations that were part
// of a relaxed cycle.
func markDualPresence(sorted []ConfigOperation, cycleMembers map[string]bool) []ConfigOperation {
	if len(cycleMembers) == 0 {
		return sorted
	}
	result := make([]ConfigOperation, len(sorted))
	for i := range sorted {
		result[i] = sorted[i]
		if cycleMembers[result[i].ID] && result[i].Type == OperationAddAddress {
			result[i].Params.AllowDual = true
		}
	}
	return result
}
