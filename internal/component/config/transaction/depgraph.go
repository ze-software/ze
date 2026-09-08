// Design: docs/architecture/config/apply-ordering.md -- operation dependency graph
// Related: operation.go -- operation types and constraint rule registry

package transaction

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
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

// The two derived orderings, named on the edge they produce so an abort
// message says which one holds an operation back.
const (
	edgeProduceBeforeConsume = "derived-produce-before-consume"
	edgeConsumeBeforeDestroy = "derived-consume-before-destroy"
)

// BuildOperationGraph orders operations two ways and returns the dependency
// graph the solver reads.
//
// Most edges are DERIVED: every operation declares the resources it produces
// and the resources it consumes, and one edge falls out of each producer and
// consumer pair over the same resource identity. A root joins the ordering by
// declaring those sets, so no component registers a rule naming another
// component's operation labels (ai/rules/principles.md).
//
// A constraint rule states what no produce and consume pair can state: address
// uniqueness across interfaces, and make-before-break within one interface are
// both facts about two operations over DIFFERENT resources, which the
// derivation has no pair to hang an edge on.
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
				graph.addEdge(before.ID, after.ID, rule.ID, seenEdge)
			}
		}
	}
	graph.addDerivedEdges(ops, seenEdge)
	return graph, nil
}

// addEdge records one dependency edge, ignoring a pair another rule already
// produced. The edge order is what the solver's queue order follows, so this
// is called in operation order and never from a map walk.
func (g *OperationGraph) addEdge(fromID, toID, ruleID string, seenEdge map[string]struct{}) {
	var tb textbuf.Buffer
	key := tb.Str(fromID).Byte(0).Str(toID).String()
	if _, exists := seenEdge[key]; exists {
		return
	}
	seenEdge[key] = struct{}{}
	edge := OperationEdge{FromID: fromID, ToID: toID, RuleID: ruleID}
	g.edges = append(g.edges, edge)
	g.out[fromID] = append(g.out[fromID], edge)
}

// addDerivedEdges adds one edge for every producer and consumer pair sharing a
// resource identity. The verbs decide the direction, and the two directions
// mirror each other:
//
//   - A create makes its resource available, so it runs BEFORE every operation
//     that consumes the resource. A consumer that destroys is excluded: it needs
//     the resource to still exist, never to have just been created, and an edge
//     to it would order a teardown behind an unrelated creation.
//   - A destroy takes the resource away, so every destroy that consumes the
//     resource runs BEFORE it. That keeps an address alive until the last peer
//     bound to it is gone.
//
// A modify neither creates nor destroys, so it produces no edge of its own. It
// still consumes, which is what puts a modification after the create of what it
// binds.
//
// An entry with no identity matches nothing. ValidateOperations refuses one
// before a transaction reaches this point, and the check is repeated here
// because a blank entry that matched every resource is the reachable silently
// wrong value ai/rules/principles.md bans.
func (g *OperationGraph) addDerivedEdges(ops []ConfigOperation, seenEdge map[string]struct{}) {
	producers := indexResourceRefs(ops, func(op *ConfigOperation) []ResourceRef { return op.Produces })
	consumers := indexResourceRefs(ops, func(op *ConfigOperation) []ResourceRef { return op.Consumes })

	for i := range ops {
		from := &ops[i]
		switch from.Verb {
		case VerbCreate:
			for k := range from.Produces {
				identity := resourceIdentity(&from.Produces[k])
				for _, j := range consumers[identity] {
					if j == i || ops[j].Verb == VerbDestroy {
						continue
					}
					g.addEdge(from.ID, ops[j].ID, edgeProduceBeforeConsume, seenEdge)
				}
			}
		case VerbDestroy:
			for k := range from.Consumes {
				identity := resourceIdentity(&from.Consumes[k])
				for _, j := range producers[identity] {
					if j == i || ops[j].Verb != VerbDestroy {
						continue
					}
					g.addEdge(from.ID, ops[j].ID, edgeConsumeBeforeDestroy, seenEdge)
				}
			}
		case VerbModify:
			// A modify makes no resource and takes none away, so it starts no
			// edge. It is reached as the far end of a create's edge, through
			// what it consumes.
		}
	}
}

// indexResourceRefs groups operation indexes by the identity of the resources
// refs(op) names. Indexes stay in operation order, so the edges a walk of this
// index produces are ordered by the operations rather than by a map.
//
// An entry with no identity never enters the index, so a lookup for one finds
// nothing. That is where the blank entry is stopped from matching everything.
func indexResourceRefs(ops []ConfigOperation, refs func(*ConfigOperation) []ResourceRef) map[string][]int {
	index := make(map[string][]int, len(ops))
	for i := range ops {
		list := refs(&ops[i])
		for k := range list {
			identity := resourceIdentity(&list[k])
			if identity == "" {
				continue
			}
			index[identity] = append(index[identity], i)
		}
	}
	return index
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
	case ResourceRelationSameInterface:
		iface := opInterface(before)
		return iface != "" && iface == opInterface(after)
	case ResourceRelationSameAddress:
		return opAddr(before) != "" && opAddr(before) == opAddr(after)
	default:
		return false
	}
}

// resourceIdentity is the key one operation's Produces entry and another's
// Consumes entry are matched on. Two entries name one resource when their
// identities are equal.
//
// An entry that carries no identifying value has an EMPTY identity, and an
// empty identity matches nothing. An operation crosses a JSON boundary from a
// plugin process, so a blank entry that matched every resource would let a
// hostile plugin order itself against the whole transaction
// (ai/rules/principles.md).
//
// An address is identified by its IP alone. The prefix length is a property of
// the address rather than part of its name, and the interface is where the
// address lives rather than what it is: a peer that binds 192.0.2.1 says so
// without knowing which interface carries it, and the box holds that address
// once, which is what the surviving uniqueness rule says.
//
// A kind this package does not name still gets an identity, through the
// default branch, so a root can declare a resource nothing here has heard of.
func resourceIdentity(ref *ResourceRef) string {
	var tb textbuf.Buffer
	switch ref.Kind {
	case ResourceInterface:
		return identityKey(&tb, ref.Kind, firstNonEmpty(ref.Name, ref.Interface))
	case ResourceAddress:
		return identityKey(&tb, ref.Kind, normalizeAddress(ref.Address))
	case ResourcePeer:
		return identityKey(&tb, ref.Kind, ref.Peer)
	case ResourceListener:
		if ref.Address == "" {
			return ""
		}
		return tb.Str(string(ref.Kind)).Byte(':').Str(normalizeAddress(ref.Address)).Byte(':').Uint16(ref.Port).String()
	case ResourceStaticRoute:
		if ref.Prefix == "" {
			return ""
		}
		return tb.Str(string(ref.Kind)).Byte(':').Str(ref.Prefix).Byte(':').Str(normalizeAddress(ref.NextHop)).String()
	default:
		return identityKey(&tb, ref.Kind, firstNonEmpty(ref.Name, ref.Interface, ref.Address, ref.Peer, ref.Prefix))
	}
}

// identityKey joins a kind and the value that names one resource of that kind.
// It answers "" for a ref with no kind or no value, which is how an
// unidentified entry stops matching.
func identityKey(tb *textbuf.Buffer, kind ResourceKind, value string) string {
	if kind == "" || value == "" {
		return ""
	}
	return tb.Str(string(kind)).Byte(':').Str(value).String()
}

func resourceKey(op *ConfigOperation) string {
	var tb textbuf.Buffer
	switch op.Target.Kind {
	case ResourceInterface:
		return tb.Str(string(ResourceInterface)).Byte(':').Str(opIfaceName(op)).String()
	case ResourceAddress:
		return tb.Str(string(ResourceAddress)).Byte(':').Str(opAddrIface(op)).Byte(':').Str(opAddr(op)).String()
	case ResourcePeer:
		return tb.Str(string(ResourcePeer)).Byte(':').Str(firstNonEmpty(op.Target.Peer, op.Params.Peer)).String()
	case ResourceListener:
		return tb.Str(string(ResourceListener)).Byte(':').Str(normalizeAddress(firstNonEmpty(op.Target.Address, op.Params.Address))).Byte(':').Uint16(firstNonZeroUint16(op.Target.Port, op.Params.Port)).String()
	case ResourceStaticRoute:
		return tb.Str(string(ResourceStaticRoute)).Byte(':').Str(firstNonEmpty(op.Target.Prefix, op.Params.Prefix)).Byte(':').Str(normalizeAddress(firstNonEmpty(op.Target.NextHop, op.Params.NextHop))).String()
	default:
		return tb.Str(string(op.Target.Kind)).Byte(':').Str(firstNonEmpty(op.Target.Name, op.Params.Name, op.Target.Address, op.Params.Address)).String()
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
