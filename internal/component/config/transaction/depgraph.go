// Design: plan/spec-config-apply-ordering.md -- operation dependency graph
// Related: operation.go -- operation types and constraint rule registry

package transaction

import (
	"fmt"
	"strings"
)

// OperationEdge is one dependency edge in the operation graph.
type OperationEdge struct {
	FromID string
	ToID   string
	RuleID string
}

// OperationGraph is a dependency graph over config operations.
type OperationGraph struct {
	operations []ConfigOperation
	byID       map[string]ConfigOperation
	edges      []OperationEdge
	out        map[string][]OperationEdge
}

// BuildOperationGraph applies data constraint rules to operations and returns
// the dependency graph used by the solver.
func BuildOperationGraph(ops []ConfigOperation, rules []ConstraintRule) (*OperationGraph, error) {
	graph := &OperationGraph{
		operations: make([]ConfigOperation, 0, len(ops)),
		byID:       make(map[string]ConfigOperation, len(ops)),
		out:        make(map[string][]OperationEdge, len(ops)),
	}
	for i := range ops {
		op := &ops[i]
		if op.ID == "" {
			return nil, fmt.Errorf("operation id is required")
		}
		if _, exists := graph.byID[op.ID]; exists {
			return nil, fmt.Errorf("duplicate operation id %q", op.ID)
		}
		graph.operations = append(graph.operations, *op)
		graph.byID[op.ID] = *op
	}

	seenEdge := make(map[string]struct{})
	for _, rule := range rules {
		for i := range ops {
			before := &ops[i]
			if !matchesSelector(before, rule.Before) {
				continue
			}
			for j := range ops {
				after := &ops[j]
				if before.ID == after.ID || !matchesSelector(after, rule.After) {
					continue
				}
				if !operationsRelated(before, after, rule.Relation) {
					continue
				}
				key := before.ID + "\x00" + after.ID
				if _, exists := seenEdge[key]; exists {
					continue
				}
				seenEdge[key] = struct{}{}
				edge := OperationEdge{FromID: before.ID, ToID: after.ID, RuleID: rule.ID}
				graph.edges = append(graph.edges, edge)
				graph.out[before.ID] = append(graph.out[before.ID], edge)
			}
		}
	}
	return graph, nil
}

// HasEdge reports whether the graph contains a dependency edge from -> to.
func (g *OperationGraph) HasEdge(fromID, toID string) bool {
	if g == nil {
		return false
	}
	for _, edge := range g.out[fromID] {
		if edge.ToID == toID {
			return true
		}
	}
	return false
}

func matchesSelector(op *ConfigOperation, selector OperationSelector) bool {
	if selector.Type != "" && op.Type != selector.Type {
		return false
	}
	if selector.ResourceKind != "" && op.Target.Kind != selector.ResourceKind {
		return false
	}
	return true
}

func operationsRelated(before, after *ConfigOperation, relation ResourceRelation) bool {
	switch relation {
	case ResourceRelationAny:
		return true
	case ResourceRelationSameResource:
		left := resourceKey(before)
		right := resourceKey(after)
		return left != "" && left == right
	case ResourceRelationInterfaceAddress:
		iface := opIfaceName(before)
		addrIface := opAddrIface(after)
		return iface != "" && iface == addrIface
	case ResourceRelationAddressUsedBy:
		return addrMatchesUse(before, after)
	case ResourceRelationSameAddress:
		return opAddr(before) != "" && opAddr(before) == opAddr(after)
	default:
		return false
	}
}

func addrMatchesUse(left, right *ConfigOperation) bool {
	if left.Target.Kind == ResourceAddress {
		addr := opAddr(left)
		used := usedAddr(right)
		return addr != "" && addr == used
	}
	if right.Target.Kind == ResourceAddress {
		addr := opAddr(right)
		used := usedAddr(left)
		return addr != "" && addr == used
	}
	leftAddr := usedAddr(left)
	rightAddr := usedAddr(right)
	return leftAddr != "" && leftAddr == rightAddr
}

func resourceKey(op *ConfigOperation) string {
	switch op.Target.Kind {
	case ResourceInterface:
		return string(ResourceInterface) + ":" + opIfaceName(op)
	case ResourceAddress:
		return string(ResourceAddress) + ":" + opAddrIface(op) + ":" + opAddr(op)
	case ResourcePeer:
		return string(ResourcePeer) + ":" + firstNonEmpty(op.Target.Peer, op.Params.Peer)
	case ResourceListener:
		return string(ResourceListener) + ":" + normalizeAddress(firstNonEmpty(op.Target.Address, op.Params.Address)) + ":" + fmt.Sprint(firstNonZeroUint16(op.Target.Port, op.Params.Port))
	case ResourceStaticRoute:
		return string(ResourceStaticRoute) + ":" + firstNonEmpty(op.Target.Prefix, op.Params.Prefix) + ":" + normalizeAddress(firstNonEmpty(op.Target.NextHop, op.Params.NextHop))
	default:
		return string(op.Target.Kind) + ":" + firstNonEmpty(op.Target.Name, op.Params.Name, op.Target.Address, op.Params.Address)
	}
}

func opIfaceName(op *ConfigOperation) string {
	return firstNonEmpty(op.Target.Name, op.Target.Interface, op.Params.Name, op.Params.Interface)
}

func opAddrIface(op *ConfigOperation) string {
	return firstNonEmpty(op.Target.Interface, op.Params.Interface)
}

func opAddr(op *ConfigOperation) string {
	return normalizeAddress(firstNonEmpty(op.Target.Address, op.Params.CIDR, op.Params.Address))
}

func usedAddr(op *ConfigOperation) string {
	return normalizeAddress(firstNonEmpty(op.Params.Address, op.Target.Address, op.Params.CIDR))
}

func normalizeAddress(value string) string {
	before, _, _ := strings.Cut(value, "/")
	return before
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZeroUint16(values ...uint16) uint16 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
