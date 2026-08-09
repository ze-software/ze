// Design: docs/architecture/config/apply-ordering.md -- iface-owned operation decomposition
// Related: register.go -- SDK operation handlers

package iface

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	if err := tx.RegisterOperationDecomposer(configRootInterface, decomposeIfaceOperations); err != nil {
		slog.Error("register iface operation decomposer", "error", err)
		panic("BUG: register iface operation decomposer failed")
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-add-interface-before-address",
		Before:   tx.OperationSelector{Type: tx.OperationAddInterface, ResourceKind: tx.ResourceInterface},
		After:    tx.OperationSelector{Type: tx.OperationAddAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-remove-address-before-interface",
		Before:   tx.OperationSelector{Type: tx.OperationRemoveAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: tx.OperationRemoveInterface, ResourceKind: tx.ResourceInterface},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-remove-address-before-add-same-address",
		Before:   tx.OperationSelector{Type: tx.OperationRemoveAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: tx.OperationAddAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationSameAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-add-address-before-remove-same-interface",
		Before:   tx.OperationSelector{Type: tx.OperationAddAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: tx.OperationRemoveAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterSettlementRule(tx.SettlementRule{
		ID:           "iface-add-address-settles-addr-added",
		Operation:    tx.OperationSelector{Type: tx.OperationAddAddress, ResourceKind: tx.ResourceAddress},
		Readiness:    tx.ConfigOperationReadiness{Namespace: "interface", EventType: "addr-added"},
		ResourceFrom: tx.SettlementResourceAddress,
		Timeout:      5 * time.Second,
	}); err != nil {
		slog.Error("register iface settlement rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-add-interface-before-tunnel",
		Before:   tx.OperationSelector{Type: tx.OperationAddInterface, ResourceKind: tx.ResourceInterface},
		After:    tx.OperationSelector{Type: tx.OperationAddTunnel, ResourceKind: tx.ResourceTunnel},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-add-interface-before-bridge-member",
		Before:   tx.OperationSelector{Type: tx.OperationAddInterface, ResourceKind: tx.ResourceInterface},
		After:    tx.OperationSelector{Type: tx.OperationAddBridgeMember, ResourceKind: tx.ResourceBridgeMember},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-remove-bridge-member-before-interface",
		Before:   tx.OperationSelector{Type: tx.OperationRemoveBridgeMember, ResourceKind: tx.ResourceBridgeMember},
		After:    tx.OperationSelector{Type: tx.OperationRemoveInterface, ResourceKind: tx.ResourceInterface},
		Relation: tx.ResourceRelationInterfaceAddress,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	if err := tx.RegisterSettlementRule(tx.SettlementRule{
		ID:           "iface-add-interface-settles-created",
		Operation:    tx.OperationSelector{Type: tx.OperationAddInterface, ResourceKind: tx.ResourceInterface},
		Readiness:    tx.ConfigOperationReadiness{Namespace: "interface", EventType: "created"},
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
			sdk.OperationAddInterface,
			sdk.OperationRemoveInterface,
			sdk.OperationAddAddress,
			sdk.OperationRemoveAddress,
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
	activeAddrs, activeManaged, _ := active.desiredState()
	candidateAddrs, candidateManaged, _ := candidate.desiredState()

	var ops []tx.ConfigOperation

	for _, ifaceName := range sortedManagedNames(candidateManaged) {
		if !activeManaged[ifaceName] {
			ifType := candidate.ifaceType(ifaceName)
			if !ifaceTypeSupportsOperations(ifType) {
				return nil, nil
			}
			ops = append(ops, ifaceInterfaceOperation(tx.OperationAddInterface, ifaceName, ifType))
		}
	}

	for _, ifaceName := range sortedAddressIfaces(candidateAddrs) {
		for _, cidr := range sortedAddressCIDRs(candidateAddrs[ifaceName]) {
			if activeAddrs[ifaceName][cidr] {
				continue
			}
			ops = append(ops, ifaceAddressOperation(tx.OperationAddAddress, ifaceName, cidr))
		}
	}
	for _, ifaceName := range sortedAddressIfaces(activeAddrs) {
		for _, cidr := range sortedAddressCIDRs(activeAddrs[ifaceName]) {
			if candidateAddrs[ifaceName][cidr] {
				continue
			}
			ops = append(ops, ifaceAddressOperation(tx.OperationRemoveAddress, ifaceName, cidr))
		}
	}

	for _, ifaceName := range sortedManagedNames(activeManaged) {
		if !candidateManaged[ifaceName] {
			ifType := active.ifaceType(ifaceName)
			if !ifaceTypeSupportsOperations(ifType) {
				return nil, nil
			}
			ops = append(ops, ifaceInterfaceOperation(tx.OperationRemoveInterface, ifaceName, ifType))
		}
	}

	return ops, nil
}

func ifaceAddressOperation(opType tx.ConfigOperationType, ifaceName, cidr string) tx.ConfigOperation {
	verb := "add"
	if opType == tx.OperationRemoveAddress {
		verb = "remove"
	}
	return tx.ConfigOperation{
		ID:    textbuf.Join([]string{"interface", verb, "address", sanitizeOperationID(ifaceName), sanitizeOperationID(cidr)}, "-"),
		Root:  configRootInterface,
		Owner: "interface",
		Type:  opType,
		Target: tx.ResourceRef{
			Kind:      tx.ResourceAddress,
			Interface: ifaceName,
			Address:   cidr,
		},
		Params: tx.ConfigOperationParams{Interface: ifaceName, CIDR: cidr},
	}
}

func ifaceInterfaceOperation(opType tx.ConfigOperationType, ifaceName, ifaceType string) tx.ConfigOperation {
	verb := "add"
	if opType == tx.OperationRemoveInterface {
		verb = "remove"
	}
	return tx.ConfigOperation{
		ID:    textbuf.Join([]string{"interface", verb, sanitizeOperationID(ifaceName)}, "-"),
		Root:  configRootInterface,
		Owner: "interface",
		Type:  opType,
		Target: tx.ResourceRef{
			Kind: tx.ResourceInterface,
			Name: ifaceName,
		},
		Params: tx.ConfigOperationParams{Name: ifaceName, Property: ifaceType},
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
	sort.Strings(names)
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
	sort.Strings(ifaces)
	return ifaces
}

func sortedAddressCIDRs(addrs map[string]bool) []string {
	cidrs := make([]string, 0, len(addrs))
	for cidr := range addrs {
		cidrs = append(cidrs, cidr)
	}
	sort.Strings(cidrs)
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
	case sdk.OperationAddInterface, sdk.OperationRemoveInterface:
		if ifaceOperationName(op) == "" {
			return fmt.Errorf("interface operation %s requires name", op.Type)
		}
	case sdk.OperationAddAddress, sdk.OperationRemoveAddress:
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
	case sdk.OperationAddInterface:
		name := ifaceOperationName(op)
		err = j.Record(
			func() error { return createInterfaceByType(b, name, op.Params.Property) },
			func() error { return b.DeleteInterface(name) },
		)
	case sdk.OperationRemoveInterface:
		name := ifaceOperationName(op)
		err = j.Record(
			func() error { return b.DeleteInterface(name) },
			func() error { return createInterfaceByType(b, name, op.Params.Property) },
		)
	case sdk.OperationAddAddress:
		ifaceName := ifaceOperationInterface(op)
		cidr := ifaceOperationCIDR(op)
		err = j.Record(
			func() error { return b.AddAddress(ifaceName, cidr) },
			func() error { return b.RemoveAddress(ifaceName, cidr) },
		)
	case sdk.OperationRemoveAddress:
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
