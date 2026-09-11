// Design: docs/architecture/config/apply-ordering.md -- operation graph foundation
// Related: types.go -- transaction event payloads

package transaction

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Operation wire/value types are shared with the public plugin RPC package so
// internal and external plugins speak the same operation contract.
type ConfigOperation = rpc.ConfigOperation
type ConfigOperationDecl = rpc.ConfigOperationDecl
type ConfigOperationType = rpc.ConfigOperationType
type ConfigOperationParams = rpc.ConfigOperationParams
type ConfigOperationReadiness = rpc.ConfigOperationReadiness
type OperationVerb = rpc.OperationVerb
type ResourceKind = rpc.ResourceKind
type ResourceRef = rpc.ResourceRef

// The ordering vocabulary. An operation is ordered by its verb and the kind of
// the resource it targets, never by its label, so a config root joins the
// ordering by declaring these and the core keeps no list of roots or labels
// (ai/rules/principles.md).
const (
	VerbCreate  = rpc.VerbCreate
	VerbDestroy = rpc.VerbDestroy
	VerbModify  = rpc.VerbModify
)

// OperationSectionApply labels the coarse node the orchestrator synthesizes for
// a participant that has diffs and owns no operation. The node stands for that
// participant's whole section, so the executor applies it through the section
// apply event and the plugin answers the `config-apply` callback it already
// implements. The five `config-operation-*` callbacks have no SDK default
// (initCallbackDefaults in pkg/plugin/sdk/sdk_callbacks.go), so a plugin that
// registered none of them answers "unknown method" to a per-operation apply.
//
// The label is core-owned: no component emits it, and the planner refuses an
// operation that arrives from a plugin carrying it
// (validateOperationDeclarations in internal/component/plugin/server/reload_tx.go).
const OperationSectionApply ConfigOperationType = "section-apply"

// IsSectionApply reports whether op is a coarse node standing for one
// participant's whole section apply rather than one atomic operation.
func IsSectionApply(op *ConfigOperation) bool {
	if op == nil {
		return false
	}
	return op.Type == OperationSectionApply
}

// ErrOperationNoVerb reports an operation that declared no verb. It is refused
// rather than ordered: with no verb the graph has nothing to order it by, and
// reading the empty value as VerbModify would give a create or a destroy the
// dependencies of a modification.
var ErrOperationNoVerb = errors.New("config operation declares no verb")

// ErrOperationBlankResource reports a Produces or Consumes entry that names no
// resource. It is refused rather than ignored: the entry is a claim about what
// the operation needs, and an operation whose claim cannot be read is ordered
// against nothing at all.
var ErrOperationBlankResource = errors.New("config operation declares a resource with no identity")

// ValidateOperations refuses the first operation the engine cannot order,
// naming the plugin that emitted it, its config root and the operation id. The
// error names no parameter and no resource value: those carry config, keys
// among them.
//
// Two refusals, and both are about a value the engine would otherwise read as
// permission: an operation with no verb, and a Produces or Consumes entry
// naming no resource. An operation arrives from a plugin process over JSON, so
// both are attacker-influenced when a plugin is hostile, and a blank entry that
// matched every resource would order that plugin against the whole transaction.
//
// The coarse section-apply node the orchestrator synthesizes is exempt. It
// stands for a whole participant section rather than one resource, so it has
// no verb to declare and no edge to earn from one.
func ValidateOperations(operations []ConfigOperation) error {
	for i := range operations {
		op := &operations[i]
		if IsSectionApply(op) {
			continue
		}
		if op.Verb == "" {
			return fmt.Errorf("%w: plugin %s, root %s, operation %s", ErrOperationNoVerb, op.Owner, op.Root, op.ID)
		}
		if err := validateResourceRefs(op, op.Produces, "produces"); err != nil {
			return err
		}
		if err := validateResourceRefs(op, op.Consumes, "consumes"); err != nil {
			return err
		}
	}
	return nil
}

// validateResourceRefs refuses the first entry of one declaration whose
// identity is empty, naming the declaration and the position rather than the
// entry's own values.
func validateResourceRefs(op *ConfigOperation, refs []ResourceRef, declaration string) error {
	for i := range refs {
		if resourceIdentity(&refs[i]) != "" {
			continue
		}
		return fmt.Errorf("%w: plugin %s, root %s, operation %s, %s entry %d", ErrOperationBlankResource, op.Owner, op.Root, op.ID, declaration, i)
	}
	return nil
}

const (
	ResourceInterface   = rpc.ResourceInterface
	ResourceAddress     = rpc.ResourceAddress
	ResourcePeer        = rpc.ResourcePeer
	ResourceListener    = rpc.ResourceListener
	ResourceStaticRoute = rpc.ResourceStaticRoute
)

var errOperationRegistryInvalidInput = errors.New("operation registry invalid input")

const maxSettlementTimeout = 60 * time.Second

// DecomposeRequest is passed to component-owned operation decomposers.
type DecomposeRequest struct {
	TransactionID string
	Root          string
	ActiveRoot    string
	CandidateRoot string
	Diff          DiffSection
}

// OperationDecomposer converts a root-level diff plus active/candidate context
// into atomic operations owned by the component for that config root.
type OperationDecomposer func(context.Context, DecomposeRequest) ([]ConfigOperation, error)

// OperationSelector matches an operation in a constraint rule.
type OperationSelector struct {
	Type         ConfigOperationType
	ResourceKind ResourceKind
}

// ResourceRelation describes when two selector-matched operations are related
// enough for a constraint rule to produce an edge.
//
// No relation here states that one operation produces what the other consumes.
// That fact is DECLARED by the operations themselves, in Produces and Consumes,
// and the graph derives its edge from the pair (BuildOperationGraph). A rule
// carries what a pair cannot: a fact about two operations over DIFFERENT
// resources. That is why the two relations left are the two the surviving
// iface rules select.
type ResourceRelation string

const (
	ResourceRelationAny           ResourceRelation = ""
	ResourceRelationSameInterface ResourceRelation = "same-interface"
	ResourceRelationSameAddress   ResourceRelation = "same-address"
)

// ConstraintRule is a data rule that produces an ordering edge when both
// selectors match operations in the graph.
type ConstraintRule struct {
	ID          string
	Description string
	Before      OperationSelector
	After       OperationSelector
	Relation    ResourceRelation
}

// SettlementResourceSource selects which operation field supplies the resource
// value matched against the readiness event payload.
type SettlementResourceSource string

const (
	SettlementResourceNone      SettlementResourceSource = ""
	SettlementResourceAddress   SettlementResourceSource = "address"
	SettlementResourceInterface SettlementResourceSource = "interface"
	SettlementResourcePeer      SettlementResourceSource = "peer"
)

// SettlementRule declares an async readiness event required after an operation.
// Rules are data so component packages can register their own side effects.
type SettlementRule struct {
	ID           string
	Description  string
	Operation    OperationSelector
	Readiness    ConfigOperationReadiness
	ResourceFrom SettlementResourceSource
	Timeout      time.Duration
}

var operationRegistry = struct {
	sync.RWMutex
	decomposers map[string]OperationDecomposer
	rules       map[string]ConstraintRule
	settlement  map[string]SettlementRule
}{
	decomposers: make(map[string]OperationDecomposer),
	rules:       make(map[string]ConstraintRule),
	settlement:  make(map[string]SettlementRule),
}

// RegisterOperationDecomposer registers the semantic decomposer for one config root.
func RegisterOperationDecomposer(root string, fn OperationDecomposer) error {
	if root == "" || fn == nil {
		return fmt.Errorf("%w: root and decomposer are required", errOperationRegistryInvalidInput)
	}
	operationRegistry.Lock()
	defer operationRegistry.Unlock()
	if _, exists := operationRegistry.decomposers[root]; exists {
		return fmt.Errorf("operation decomposer for root %q already registered", root)
	}
	operationRegistry.decomposers[root] = fn
	return nil
}

// OperationDecomposerFor returns the registered decomposer for root.
func OperationDecomposerFor(root string) (OperationDecomposer, bool) {
	operationRegistry.RLock()
	defer operationRegistry.RUnlock()
	fn, ok := operationRegistry.decomposers[root]
	return fn, ok
}

// RegisterConstraintRule registers a data-driven ordering rule.
func RegisterConstraintRule(rule ConstraintRule) error {
	if rule.ID == "" || rule.Before.Type == "" || rule.After.Type == "" {
		return fmt.Errorf("%w: rule id, before type, and after type are required", errOperationRegistryInvalidInput)
	}
	operationRegistry.Lock()
	defer operationRegistry.Unlock()
	if _, exists := operationRegistry.rules[rule.ID]; exists {
		return fmt.Errorf("constraint rule %q already registered", rule.ID)
	}
	operationRegistry.rules[rule.ID] = rule
	return nil
}

// ConstraintRules returns registered rules sorted by ID for deterministic graph building.
func ConstraintRules() []ConstraintRule {
	operationRegistry.RLock()
	defer operationRegistry.RUnlock()
	rules := make([]ConstraintRule, 0, len(operationRegistry.rules))
	for _, rule := range operationRegistry.rules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules
}

// RegisterSettlementRule registers one operation readiness rule.
func RegisterSettlementRule(rule SettlementRule) error {
	if rule.ID == "" || rule.Operation.Type == "" || rule.Readiness.Namespace == "" || rule.Readiness.EventType == "" {
		return fmt.Errorf("%w: rule id, operation type, readiness namespace, and readiness event type are required", errOperationRegistryInvalidInput)
	}
	if rule.Timeout <= 0 {
		return fmt.Errorf("%w: settlement timeout must be positive", errOperationRegistryInvalidInput)
	}
	if rule.Timeout > maxSettlementTimeout {
		rule.Timeout = maxSettlementTimeout
	}
	operationRegistry.Lock()
	defer operationRegistry.Unlock()
	if _, exists := operationRegistry.settlement[rule.ID]; exists {
		return fmt.Errorf("settlement rule %q already registered", rule.ID)
	}
	operationRegistry.settlement[rule.ID] = rule
	return nil
}

// SettlementRules returns registered settlement rules sorted by ID.
func SettlementRules() []SettlementRule {
	operationRegistry.RLock()
	defer operationRegistry.RUnlock()
	rules := make([]SettlementRule, 0, len(operationRegistry.settlement))
	for _, rule := range operationRegistry.settlement {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules
}

// SettlementRulesFor returns settlement rules matching op.
func SettlementRulesFor(op *ConfigOperation) []SettlementRule {
	if op == nil {
		return nil
	}
	all := SettlementRules()
	rules := make([]SettlementRule, 0, len(all))
	for _, rule := range all {
		if matchesSelector(op, rule.Operation) {
			rules = append(rules, rule)
		}
	}
	return rules
}
