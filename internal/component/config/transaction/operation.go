// Design: plan/learned/1055-config-apply-ordering.md -- operation graph foundation
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
type ResourceKind = rpc.ResourceKind
type ResourceRef = rpc.ResourceRef

const (
	OperationAddInterface       = rpc.OperationAddInterface
	OperationRemoveInterface    = rpc.OperationRemoveInterface
	OperationAddAddress         = rpc.OperationAddAddress
	OperationRemoveAddress      = rpc.OperationRemoveAddress
	OperationSetProperty        = rpc.OperationSetProperty
	OperationAddBridgeMember    = rpc.OperationAddBridgeMember
	OperationRemoveBridgeMember = rpc.OperationRemoveBridgeMember
	OperationAddPeer            = rpc.OperationAddPeer
	OperationRemovePeer         = rpc.OperationRemovePeer
	OperationModifyPeer         = rpc.OperationModifyPeer
	OperationAddListener        = rpc.OperationAddListener
	OperationRemoveListener     = rpc.OperationRemoveListener
	OperationAddStaticRoute     = rpc.OperationAddStaticRoute
	OperationRemoveStaticRoute  = rpc.OperationRemoveStaticRoute
	OperationSetAdminDistance   = rpc.OperationSetAdminDistance
	OperationSetSysctl          = rpc.OperationSetSysctl
	OperationStartDHCP          = rpc.OperationStartDHCP
	OperationStopDHCP           = rpc.OperationStopDHCP
	OperationAddTunnel          = rpc.OperationAddTunnel
	OperationRemoveTunnel       = rpc.OperationRemoveTunnel
)

const (
	ResourceInterface    = rpc.ResourceInterface
	ResourceAddress      = rpc.ResourceAddress
	ResourcePeer         = rpc.ResourcePeer
	ResourceListener     = rpc.ResourceListener
	ResourceBridgeMember = rpc.ResourceBridgeMember
	ResourceStaticRoute  = rpc.ResourceStaticRoute
	ResourceSysctl       = rpc.ResourceSysctl
	ResourceDHCP         = rpc.ResourceDHCP
	ResourceTunnel       = rpc.ResourceTunnel
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
type ResourceRelation string

const (
	ResourceRelationAny              ResourceRelation = ""
	ResourceRelationSameResource     ResourceRelation = "same-resource"
	ResourceRelationInterfaceAddress ResourceRelation = "interface-address"
	ResourceRelationAddressUsedBy    ResourceRelation = "address-used-by"
	ResourceRelationSameAddress      ResourceRelation = "same-address"
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
