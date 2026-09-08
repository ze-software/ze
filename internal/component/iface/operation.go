// Design: docs/architecture/config/apply-ordering.md -- iface-owned operation decomposition
// Related: register.go -- SDK operation handlers

package iface

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The operation labels of the `interface` root. They belong here, next to the
// decomposer that emits them and the applier that dispatches on them: a label
// list in a shared package is a central enumeration a new root would edit
// because it looks like the place labels belong (ai/rules/principles.md). The
// engine never compares one. It orders by the verb and the target kind that
// each operation below also carries.
const (
	operationAddInterface    sdk.ConfigOperationType = "add-interface"
	operationRemoveInterface sdk.ConfigOperationType = "remove-interface"
	operationAddAddress      sdk.ConfigOperationType = "add-address"
	operationRemoveAddress   sdk.ConfigOperationType = "remove-address"
)

func init() {
	if err := tx.RegisterOperationDecomposer(configRootInterface, decomposeIfaceOperations); err != nil {
		slog.Error("register iface operation decomposer", "error", err)
		panic("BUG: register iface operation decomposer failed")
	}
	// Two rules survive, and each states a fact about two operations over
	// DIFFERENT resources, which is what no produce and consume pair can
	// carry. One address leaves its old interface before it arrives on the
	// new one, and an interface is never left with no address at all.
	//
	// The five rules that stated a produce and consume fact are gone. Each
	// operation below declares what it owns and what it needs instead, and
	// BuildOperationGraph derives their edges from the pair.
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-remove-address-before-add-same-address",
		Before:   tx.OperationSelector{Type: operationRemoveAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: operationAddAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationSameAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-add-address-before-remove-same-interface",
		Before:   tx.OperationSelector{Type: operationAddAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: operationRemoveAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationSameInterface,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterSettlementRule(tx.SettlementRule{
		ID:           "iface-add-address-settles-addr-added",
		Operation:    tx.OperationSelector{Type: operationAddAddress, ResourceKind: tx.ResourceAddress},
		Readiness:    tx.ConfigOperationReadiness{Namespace: ifaceevents.Namespace, EventType: "addr-added"},
		ResourceFrom: tx.SettlementResourceAddress,
		Timeout:      5 * time.Second,
	}); err != nil {
		slog.Error("register iface settlement rule", "error", err)
	}
	if err := tx.RegisterSettlementRule(tx.SettlementRule{
		ID:           "iface-add-interface-settles-created",
		Operation:    tx.OperationSelector{Type: operationAddInterface, ResourceKind: tx.ResourceInterface},
		Readiness:    tx.ConfigOperationReadiness{Namespace: ifaceevents.Namespace, EventType: "created"},
		ResourceFrom: tx.SettlementResourceInterface,
		Timeout:      5 * time.Second,
	}); err != nil {
		slog.Error("register iface settlement rule", "error", err)
	}
}

func ifaceConfigOperationDecls() []sdk.ConfigOperationDecl {
	return []sdk.ConfigOperationDecl{{
		Root:      configRootInterface,
		Decompose: true,
		Operations: []sdk.ConfigOperationType{
			operationAddInterface,
			operationRemoveInterface,
			operationAddAddress,
			operationRemoveAddress,
		},
	}}
}

func decomposeIfaceOperations(_ context.Context, req tx.DecomposeRequest) ([]tx.ConfigOperation, error) {
	if req.Root != configRootInterface || !ifaceDiffHasDecomposableChanges(req.Diff) {
		return nil, nil
	}
	active, err := parseIfaceSections([]sdk.ConfigSection{{Root: configRootInterface, Data: req.ActiveRoot}})
	if err != nil {
		return nil, fmt.Errorf("iface operation decompose active: %w", err)
	}
	candidate, err := parseIfaceSections([]sdk.ConfigSection{{Root: configRootInterface, Data: req.CandidateRoot}})
	if err != nil {
		return nil, fmt.Errorf("iface operation decompose candidate: %w", err)
	}
	// Resolve both sides' hardware selectors against the devices present now,
	// so every operation names the kernel device it will be applied to. The
	// executor hands that name straight to the backend (applyIfaceOperation),
	// and the settlement rule waits on a monitor event that carries the KERNEL
	// name, so a logical name here would configure the wrong device and then
	// never settle. One listing serves both sides, which keeps the diff between
	// them a diff about config rather than about resolution.
	//
	// Without a listing every selected entry reads as unbound and contributes no
	// operation, which is the same fail-safe direction the apply path takes: skip
	// the entry, never guess a device for it. There is no separate refusal to
	// make here, because this function's caller appends what it returns and
	// cannot tell nil from an empty slice (reload_tx.go decomposeRootOperations).
	// An entry with no selector needs no listing and is unaffected.
	var infos []InterfaceInfo
	if b := GetBackend(); b != nil {
		infos, _ = b.ListInterfaces()
	}
	activeAddrs, activeManaged, _ := active.desiredState(active.bindDevices(infos))
	candidateAddrs, candidateManaged, _ := candidate.desiredState(candidate.bindDevices(infos))

	var ops []tx.ConfigOperation

	for _, ifaceName := range sortedManagedNames(candidateManaged) {
		if !activeManaged[ifaceName] {
			ifType := candidate.ifaceType(ifaceName)
			if !ifaceTypeSupportsOperations(ifType) {
				return nil, nil
			}
			ops = append(ops, ifaceInterfaceOperation(operationAddInterface, ifaceName, ifType))
		}
	}

	for _, ifaceName := range sortedAddressIfaces(candidateAddrs) {
		for _, cidr := range sortedAddressCIDRs(candidateAddrs[ifaceName]) {
			if activeAddrs[ifaceName][cidr] {
				continue
			}
			ops = append(ops, ifaceAddressOperation(operationAddAddress, ifaceName, cidr))
		}
	}
	for _, ifaceName := range sortedAddressIfaces(activeAddrs) {
		for _, cidr := range sortedAddressCIDRs(activeAddrs[ifaceName]) {
			if candidateAddrs[ifaceName][cidr] {
				continue
			}
			ops = append(ops, ifaceAddressOperation(operationRemoveAddress, ifaceName, cidr))
		}
	}

	for _, ifaceName := range sortedManagedNames(activeManaged) {
		if !candidateManaged[ifaceName] {
			ifType := active.ifaceType(ifaceName)
			if !ifaceTypeSupportsOperations(ifType) {
				return nil, nil
			}
			ops = append(ops, ifaceInterfaceOperation(operationRemoveInterface, ifaceName, ifType))
		}
	}

	return ops, nil
}

func ifaceAddressOperation(opType tx.ConfigOperationType, ifaceName, cidr string) tx.ConfigOperation {
	verb := tx.VerbCreate
	word := "add"
	if opType == operationRemoveAddress {
		verb = tx.VerbDestroy
		word = "remove"
	}
	return tx.ConfigOperation{
		ID:    textbuf.Join([]string{componentNameInterface, word, "address", sanitizeOperationID(ifaceName), sanitizeOperationID(cidr)}, "-"),
		Root:  configRootInterface,
		Owner: componentNameInterface,
		Type:  opType,
		Verb:  verb,
		Target: tx.ResourceRef{
			Kind:      tx.ResourceAddress,
			Interface: ifaceName,
			Address:   cidr,
		},
		// The address is what this operation owns, and the interface is what
		// it needs somebody else to have made. Declaring both is what orders
		// this operation against the interface it sits on and against every
		// root that binds the address, with no rule naming either side.
		Produces: []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: cidr}},
		Consumes: []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: ifaceName}},
		Params:   tx.ConfigOperationParams{Interface: ifaceName, CIDR: cidr},
	}
}

func ifaceInterfaceOperation(opType tx.ConfigOperationType, ifaceName, ifaceType string) tx.ConfigOperation {
	verb := tx.VerbCreate
	word := "add"
	if opType == operationRemoveInterface {
		verb = tx.VerbDestroy
		word = "remove"
	}
	return tx.ConfigOperation{
		ID:    textbuf.Join([]string{componentNameInterface, word, sanitizeOperationID(ifaceName)}, "-"),
		Root:  configRootInterface,
		Owner: componentNameInterface,
		Type:  opType,
		Verb:  verb,
		Target: tx.ResourceRef{
			Kind: tx.ResourceInterface,
			Name: ifaceName,
		},
		// The interface is what this operation owns. A create runs before
		// every operation that consumes it, and a destroy runs after every
		// destroy that does.
		Produces: []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: ifaceName}},
		Params:   tx.ConfigOperationParams{Name: ifaceName, Property: ifaceType},
	}
}

func ifaceDiffHasDecomposableChanges(diff tx.DiffSection) bool {
	seen := false
	for _, raw := range []string{diff.Added, diff.Removed, diff.Changed} {
		if raw == "" {
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return false
		}
		for key := range entries {
			if !ifaceKeyDecomposable(key) {
				return false
			}
			seen = true
		}
	}
	return seen
}

func ifaceKeyDecomposable(key string) bool {
	if strings.Contains(key, "/address") {
		return true
	}
	for _, kind := range []string{"/dummy/", "/veth/", "/bridge/"} {
		_, after, ok := strings.Cut(key, kind)
		if !ok {
			continue
		}
		if !strings.Contains(after, "/") {
			return true
		}
	}
	return false
}

func sortedManagedNames(managed map[string]bool) []string {
	names := make([]string, 0, len(managed))
	for name := range managed {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (cfg *ifaceConfig) ifaceType(name string) string {
	for i := range cfg.Dummy {
		if cfg.Dummy[i].Name == name {
			return zeTypeDummy
		}
	}
	for i := range cfg.Bridge {
		if cfg.Bridge[i].Name == name {
			return zeTypeBridge
		}
	}
	for i := range cfg.Tunnel {
		if cfg.Tunnel[i].Name == name {
			return zeTypeTunnel
		}
	}
	for i := range cfg.Wireguard {
		if cfg.Wireguard[i].Name == name {
			return zeTypeWireguard
		}
	}
	for i := range cfg.XFRM {
		if cfg.XFRM[i].Name == name {
			return zeTypeXFRM
		}
	}
	for i := range cfg.Veth {
		if cfg.Veth[i].Name == name {
			return zeTypeVeth
		}
	}
	return ""
}

func ifaceTypeSupportsOperations(ifType string) bool {
	switch ifType {
	case zeTypeDummy, zeTypeBridge, zeTypeVeth:
		return true
	default:
		return false
	}
}

func sortedAddressIfaces(addrs map[string]map[string]bool) []string {
	ifaces := make([]string, 0, len(addrs))
	for ifaceName := range addrs {
		ifaces = append(ifaces, ifaceName)
	}
	slices.Sort(ifaces)
	return ifaces
}

func sortedAddressCIDRs(addrs map[string]bool) []string {
	cidrs := make([]string, 0, len(addrs))
	for cidr := range addrs {
		cidrs = append(cidrs, cidr)
	}
	slices.Sort(cidrs)
	return cidrs
}

func sanitizeOperationID(value string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", " ", "_")
	return replacer.Replace(value)
}

func operationJournalKey(txID, opID string) string {
	return txID + "\x00" + opID
}

func verifyIfaceOperation(op *sdk.ConfigOperation) error {
	if op == nil {
		return fmt.Errorf("interface operation is required")
	}
	switch op.Type {
	case operationAddInterface, operationRemoveInterface:
		if ifaceOperationName(op) == "" {
			return fmt.Errorf("interface operation %s requires name", op.Type)
		}
	case operationAddAddress, operationRemoveAddress:
		if ifaceOperationInterface(op) == "" || ifaceOperationCIDR(op) == "" {
			return fmt.Errorf("interface operation %s requires interface and cidr", op.Type)
		}
	default:
		return fmt.Errorf("interface operation %s not supported", op.Type)
	}
	return nil
}

func applyIfaceOperation(op *sdk.ConfigOperation, b Backend) (*sdk.Journal, error) {
	if err := verifyIfaceOperation(op); err != nil {
		return nil, err
	}
	j := sdk.NewJournal()
	var err error
	switch op.Type {
	case operationAddInterface:
		name := ifaceOperationName(op)
		err = j.Record(
			func() error { return createInterfaceByType(b, name, op.Params.Property) },
			func() error { return b.DeleteInterface(name) },
		)
	case operationRemoveInterface:
		name := ifaceOperationName(op)
		err = j.Record(
			func() error { return b.DeleteInterface(name) },
			func() error { return createInterfaceByType(b, name, op.Params.Property) },
		)
	case operationAddAddress:
		ifaceName := ifaceOperationInterface(op)
		cidr := ifaceOperationCIDR(op)
		err = j.Record(
			func() error { return b.AddAddress(ifaceName, cidr) },
			func() error { return b.RemoveAddress(ifaceName, cidr) },
		)
	case operationRemoveAddress:
		ifaceName := ifaceOperationInterface(op)
		cidr := ifaceOperationCIDR(op)
		err = j.Record(
			func() error { return b.RemoveAddress(ifaceName, cidr) },
			func() error { return b.AddAddress(ifaceName, cidr) },
		)
	default:
		return nil, fmt.Errorf("interface operation %s not supported", op.Type)
	}
	if err != nil {
		return nil, err
	}
	return j, nil
}

func createInterfaceByType(b Backend, name, ifType string) error {
	switch ifType {
	case zeTypeDummy:
		return b.CreateDummy(name)
	case zeTypeBridge:
		return b.CreateBridge(name)
	case zeTypeVeth:
		return b.CreateVeth(name, name+"-peer")
	default:
		return fmt.Errorf("interface operation: unsupported type %q for %s", ifType, name)
	}
}

func ifaceOperationName(op *sdk.ConfigOperation) string {
	if op.Params.Name != "" {
		return op.Params.Name
	}
	return op.Target.Name
}

func ifaceOperationInterface(op *sdk.ConfigOperation) string {
	if op.Params.Interface != "" {
		return op.Params.Interface
	}
	return op.Target.Interface
}

func ifaceOperationCIDR(op *sdk.ConfigOperation) string {
	if op.Params.CIDR != "" {
		return op.Params.CIDR
	}
	return op.Target.Address
}
