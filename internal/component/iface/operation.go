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
//
// The first four each apply one resource. `configure-interfaces` applies the
// interface configuration as a whole, which is how this root covers a key it
// has no primitive for: an MTU, a mirror, a tunnel spec, a DHCP client. It is
// what makes the decomposition TOTAL, and the contract every decomposer owes
// is exactly that (participantsWithoutOperations in
// internal/component/config/transaction/orchestrator.go).
const (
	operationAddInterface    sdk.ConfigOperationType = "add-interface"
	operationRemoveInterface sdk.ConfigOperationType = "remove-interface"
	operationAddAddress      sdk.ConfigOperationType = "add-address"
	operationRemoveAddress   sdk.ConfigOperationType = "remove-address"
	operationConfigureIfaces sdk.ConfigOperationType = "configure-interfaces"
)

func init() {
	if err := tx.RegisterOperationDecomposer(configRootInterface, decomposeIfaceOperations); err != nil {
		slog.Error("register iface operation decomposer", "error", err)
		panic("BUG: register iface operation decomposer failed")
	}
	// One rule survives, and it states a fact about two operations over
	// DIFFERENT resources, which is what no produce and consume pair can
	// carry: every address this commit removes leaves the host before any
	// address this commit adds arrives. That is phases 3 and 4 of the
	// requirement, in the order the requirement gives them
	// (docs/architecture/config/apply-ordering.md).
	//
	// The five rules that stated a produce and consume fact are gone. Each
	// operation below declares what it owns and what it needs instead, and
	// BuildOperationGraph derives their edges from the pair.
	//
	// Two rules became this one. The first ordered a removal before the
	// addition of the SAME address, which is one address living on one
	// interface. The second held the new address on an interface until the
	// old one left, so that no interface was ever bare: that was
	// make-before-break, which the requirement does not ask for. Removing
	// only the second would have left a renumber unordered, because the old
	// address and the new one are two different addresses and no rule related
	// them, and the planner emits its additions first.
	if err := tx.RegisterConstraintRule(tx.ConstraintRule{
		ID:       "iface-remove-address-before-add-address",
		Before:   tx.OperationSelector{Type: operationRemoveAddress, ResourceKind: tx.ResourceAddress},
		After:    tx.OperationSelector{Type: operationAddAddress, ResourceKind: tx.ResourceAddress},
		Relation: tx.ResourceRelationAny,
	}); err != nil {
		slog.Error("register iface constraint rule", "error", err)
	}
	// The configure operation applies the END state of the whole root, so it
	// is safe only once every operation that moves one resource has run. One
	// rule for each operation this root emits says that. They are placement
	// rather than a produce and consume fact: the configure operation targets
	// no resource, and what it declares in Produces orders what BINDS the
	// addressing it creates, never the four operations below.
	//
	// Run it before a destroy and it takes an address off the host at a
	// position the graph chose for something else, which is the disturbance
	// the ordering exists to place. Run it before a create and it adds the
	// address the create is about to add. Run it last and every resource it
	// reads is already where the plan left it, so its own pass over them
	// changes nothing and only the keys no operation carries are applied.
	//
	// It orders nothing after itself, which is what keeps it out of every
	// cycle the solver would have to reject.
	for _, before := range []tx.OperationSelector{
		{Type: operationAddAddress, ResourceKind: tx.ResourceAddress},
		{Type: operationRemoveAddress, ResourceKind: tx.ResourceAddress},
		{Type: operationAddInterface, ResourceKind: tx.ResourceInterface},
		{Type: operationRemoveInterface, ResourceKind: tx.ResourceInterface},
	} {
		if err := tx.RegisterConstraintRule(tx.ConstraintRule{
			ID:       textbuf.Join([]string{componentNameInterface, string(before.Type), "before-configure"}, "-"),
			Before:   before,
			After:    tx.OperationSelector{Type: operationConfigureIfaces},
			Relation: tx.ResourceRelationAny,
		}); err != nil {
			slog.Error("register iface constraint rule", "error", err)
		}
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
			operationConfigureIfaces,
		},
	}}
}

// decomposeIfaceOperations turns one interface diff into the operations the
// graph orders. It covers the WHOLE root on every call: an address change and
// an interface of a type this package can create become one operation each,
// and everything else rides the configure operation appended at the end.
//
// It used to answer nothing at all when any key in the diff was one it had no
// primitive for. A commit that edited an MTU and moved an address therefore
// emitted no address operation, the core read the plan and found no address
// disturbed, and every peer bound to that address stayed up while the coarse
// section apply moved the address underneath it. That is the failure phase 2
// of the requirement exists to prevent, and an MTU edit on its own took the
// identical path and was correct there
// (docs/architecture/config/apply-ordering.md).
//
// So the address question is answered for the keys this package can read,
// whatever else the diff carries, and no key is classified: a diff with any
// key at all produces the configure operation, so nothing in the root can be
// dropped by misreading what a key means. Where a key DOES disturb an address
// the address operations say so, and where it does not they are absent, which
// is the same answer the two cases gave before.
func decomposeIfaceOperations(_ context.Context, req tx.DecomposeRequest) ([]tx.ConfigOperation, error) {
	if req.Root != configRootInterface {
		return nil, nil
	}
	changed, err := ifaceDiffHasChanges(req.Diff)
	if err != nil {
		return nil, fmt.Errorf("iface operation decompose diff: %w", err)
	}
	if !changed {
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
	// operation of its own, and the configure operation applies it instead. That
	// is the same fail-safe direction the apply path takes: never guess a device
	// for an entry. An entry with no selector needs no listing and is
	// unaffected.
	var infos []InterfaceInfo
	if b := GetBackend(); b != nil {
		infos, _ = b.ListInterfaces()
	}
	activeAddrs, activeManaged, _ := active.desiredState(active.bindDevices(infos))
	candidateAddrs, candidateManaged, _ := candidate.desiredState(candidate.bindDevices(infos))

	var ops []tx.ConfigOperation

	// configureCreates holds the interfaces the configure operation brings up,
	// because this package has no create primitive for their type. An address
	// on one of them waits for the same operation: an add-address operation
	// would name a device that does not exist yet, and it earns no edge to
	// the create because there is no create operation to earn it from.
	//
	// configureProduces is what the configure operation declares for them. A
	// binder of one of those addresses is ordered after it by the derived
	// edge that declaration earns, which is the ordering an operation naming
	// no resource could never have (ifaceConfigureOperation).
	configureCreates := make(map[string]bool)
	var configureProduces []tx.ResourceRef
	for _, ifaceName := range sortedManagedNames(candidateManaged) {
		if activeManaged[ifaceName] {
			continue
		}
		ifType := candidate.ifaceType(ifaceName)
		if !ifaceTypeSupportsOperations(ifType) {
			configureCreates[ifaceName] = true
			configureProduces = append(configureProduces, tx.ResourceRef{Kind: tx.ResourceInterface, Name: ifaceName})
			continue
		}
		ops = append(ops, ifaceInterfaceOperation(operationAddInterface, ifaceName, ifType))
	}

	for _, ifaceName := range sortedAddressIfaces(candidateAddrs) {
		for _, cidr := range sortedAddressCIDRs(candidateAddrs[ifaceName]) {
			if activeAddrs[ifaceName][cidr] {
				continue
			}
			if configureCreates[ifaceName] {
				configureProduces = append(configureProduces, tx.ResourceRef{Kind: tx.ResourceAddress, Address: cidr})
				continue
			}
			ops = append(ops, ifaceAddressOperation(operationAddAddress, ifaceName, cidr))
		}
	}
	// Every address leaving the host is named here, including one leaving on
	// an interface the configure operation deletes. That operation is what the
	// core reads the disturbed set out of, so an address removed without one
	// is an address no binder is ever told about (DisturbedAddresses in
	// internal/component/config/transaction/operation.go).
	for _, ifaceName := range sortedAddressIfaces(activeAddrs) {
		for _, cidr := range sortedAddressCIDRs(activeAddrs[ifaceName]) {
			if candidateAddrs[ifaceName][cidr] {
				continue
			}
			ops = append(ops, ifaceAddressOperation(operationRemoveAddress, ifaceName, cidr))
		}
	}

	for _, ifaceName := range sortedManagedNames(activeManaged) {
		if candidateManaged[ifaceName] {
			continue
		}
		ifType := active.ifaceType(ifaceName)
		if !ifaceTypeSupportsOperations(ifType) {
			continue
		}
		ops = append(ops, ifaceInterfaceOperation(operationRemoveInterface, ifaceName, ifType))
	}

	return append(ops, ifaceConfigureOperation(configureProduces)), nil
}

// ifaceConfigureOperation is the operation that applies the interface
// configuration as a whole. It is what carries every key the four resource
// operations above have no primitive for, so this root leaves nothing for a
// coarse section apply to pick up.
//
// It targets no resource, because it applies a section rather than one thing.
// It DECLARES the addressing it creates all the same: the devices this package
// has no create primitive for, a tunnel, a wireguard device or an xfrm device,
// and the addresses that arrive on one of them. An operation that declared
// nothing was an operation nothing could be ordered against, and a peer bound
// to such an address sorted ahead of the operation that creates it, so the
// bind failed on an address the host did not have yet and the transaction
// rolled back (docs/architecture/config/apply-ordering.md, "The five phases").
//
// It declares no Consumes, and it declares nothing for the devices it DELETES.
// A modify that named a resource in Produces would say it makes that resource
// available, which is the wrong direction for a deletion; the four placement
// rules registered in init() are what keep it behind every destroy.
func ifaceConfigureOperation(produces []tx.ResourceRef) tx.ConfigOperation {
	return tx.ConfigOperation{
		ID:       textbuf.Join([]string{componentNameInterface, "configure"}, "-"),
		Root:     configRootInterface,
		Owner:    componentNameInterface,
		Type:     operationConfigureIfaces,
		Verb:     tx.VerbModify,
		Produces: produces,
	}
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

// ifaceDiffHasChanges reports whether this diff names any key at all, and
// answers the parse error rather than a verdict when a section will not
// unmarshal.
//
// A root asked with no diff is the binder case: the planner asks every root
// that decomposes a second time once an address is disturbed, and this one is
// asked then even when its own config did not change (bindingRoots in
// internal/component/plugin/server/reload_tx.go). Answering with operations
// there would apply a section nothing changed.
//
// It replaced a predicate that asked whether EVERY key was one this package
// had a primitive for, and refused the whole root otherwise. No key is
// classified now, because the configure operation applies them all.
//
// A section that will not parse is answered with the error. "No change" is the
// answer that drops every address operation this root owns, so the core reads
// the commit as quiet and every binder stays up while the section apply moves
// the address under it: a value that is silently wrong must not be reachable
// (ai/rules/principles.md). The first-party producer marshals a map and cannot
// emit one (buildDiffSections, internal/component/plugin/server/reload_tx.go),
// so this aborts the transaction rather than guessing on behalf of a plugin
// that sent something else.
func ifaceDiffHasChanges(diff tx.DiffSection) (bool, error) {
	seen := false
	for _, raw := range []string{diff.Added, diff.Removed, diff.Changed} {
		if raw == "" {
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return false, err
		}
		if len(entries) > 0 {
			seen = true
		}
	}
	return seen, nil
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
	case operationConfigureIfaces:
		// It names no resource, so there is nothing to require. What it
		// applies is the config the verify phase already validated, and the
		// applier refuses it when that config is absent.
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
	case operationConfigureIfaces:
		// The configure operation applies the pending config, which lives in
		// the plugin closure beside the verify that produced it, so the
		// callback in register.go applies it and never reaches here. Reaching
		// here means that dispatch was lost, and a silent success would apply
		// none of the keys this operation carries (ai/rules/principles.md).
		return nil, fmt.Errorf("interface operation %s is applied by the pending-config applier", op.Type)
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
