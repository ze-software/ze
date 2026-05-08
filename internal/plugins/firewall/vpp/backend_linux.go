// Design: plan/spec-fw-6-firewall-vpp.md -- VPP firewall backend

//go:build linux

package firewallvpp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.fd.io/govpp/api"
	govppacl "go.fd.io/govpp/binapi/acl"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
)

const waitConnectedTimeout = 5 * time.Second
const waitConnectorPoll = 50 * time.Millisecond
const aclTagPrefix = "ze/"

type backend struct {
	mu        sync.Mutex
	connector func() *vppcomp.Connector

	// aclIndexes maps "ze/<table>/<chain>" -> VPP ACL index.
	aclIndexes map[string]uint32

	// ifaceACLs tracks which ACL indexes are bound per interface so
	// reconciliation can rebuild the full ACL list when a chain changes.
	// Each entry holds input ACLs then output ACLs, with nInput recording
	// how many are input.
	ifaceACLs map[interface_types.InterfaceIndex]ifaceACLBinding

	// lastApplied holds the tables from the most recent successful Apply
	// so ListTables can return them without querying VPP.
	lastApplied []firewall.Table
}

// ifaceACLBinding holds the merged input+output ACL vector for one interface.
type ifaceACLBinding struct {
	input  []uint32
	output []uint32
}

func newBackend() (firewall.Backend, error) {
	return &backend{
		connector:  vppcomp.GetActiveConnector,
		aclIndexes: make(map[string]uint32),
		ifaceACLs:  make(map[interface_types.InterfaceIndex]ifaceACLBinding),
	}, nil
}

func (b *backend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx := context.Background()
	connCtx, connCancel := context.WithTimeout(ctx, waitConnectedTimeout)
	conn, err := b.waitConnector(connCtx)
	connCancel()
	if err != nil {
		return fmt.Errorf("firewall-vpp: %w", err)
	}
	if err := conn.WaitConnected(ctx, waitConnectedTimeout); err != nil {
		return fmt.Errorf("firewall-vpp: %w", err)
	}

	ch, err := conn.NewChannel()
	if err != nil {
		return fmt.Errorf("firewall-vpp: new channel: %w", err)
	}
	defer ch.Close()

	return b.applyWithOps(&govppOps{ch: ch}, desired)
}

func (b *backend) waitConnector(ctx context.Context) (*vppcomp.Connector, error) {
	if b.connector == nil {
		return nil, fmt.Errorf("vpp component not initialized")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if conn := b.connector(); conn != nil {
		return conn, nil
	}
	tick := time.NewTicker(waitConnectorPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
			if conn := b.connector(); conn != nil {
				return conn, nil
			}
		}
	}
}

func (b *backend) applyWithOps(ops vppOps, desired []firewall.Table) error {
	nameIndex, err := ops.dumpInterfaces()
	if err != nil {
		return fmt.Errorf("firewall-vpp: %w", err)
	}

	if err := b.cleanupStartupOrphans(ops, desired); err != nil {
		return fmt.Errorf("firewall-vpp: %w", err)
	}

	newACLIndexes := make(map[string]uint32)
	var undo []func()

	applyErr := b.applyAll(ops, desired, newACLIndexes, &undo)
	if applyErr != nil {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		return fmt.Errorf("firewall-vpp: %w", applyErr)
	}

	newIfaceACLs := make(map[interface_types.InterfaceIndex]ifaceACLBinding)
	if err := b.bindAllACLs(ops, nameIndex, desired, newACLIndexes, newIfaceACLs); err != nil {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		return fmt.Errorf("firewall-vpp: %w", err)
	}

	b.reconcileRemovals(ops, newACLIndexes)
	b.aclIndexes = newACLIndexes
	b.ifaceACLs = newIfaceACLs
	cp := make([]firewall.Table, len(desired))
	copy(cp, desired)
	b.lastApplied = cp
	return nil
}

func (b *backend) cleanupStartupOrphans(ops vppOps, desired []firewall.Table) error {
	if len(b.aclIndexes) != 0 {
		return nil
	}
	existing, err := ops.aclDump()
	if err != nil {
		return fmt.Errorf("dump ACLs: %w", err)
	}
	desiredTags := desiredACLTags(desired)
	for _, entry := range existing {
		if !strings.HasPrefix(entry.Tag, aclTagPrefix) {
			continue
		}
		if desiredTags[entry.Tag] {
			continue
		}
		if err := ops.aclDel(entry.Index); err != nil {
			return fmt.Errorf("delete startup orphan ACL %q (index %d): %w", entry.Tag, entry.Index, err)
		}
	}
	return nil
}

func desiredACLTags(desired []firewall.Table) map[string]bool {
	tags := make(map[string]bool)
	for i := range desired {
		for j := range desired[i].Chains {
			if desired[i].Chains[j].IsBase {
				tags[aclTag(desired[i].Name, desired[i].Chains[j].Name)] = true
			}
		}
	}
	return tags
}

func (b *backend) applyAll(
	ops vppOps,
	desired []firewall.Table,
	newACLIndexes map[string]uint32,
	undo *[]func(),
) error {
	for i := range desired {
		tbl := &desired[i]
		for j := range tbl.Chains {
			ch := &tbl.Chains[j]
			if !ch.IsBase {
				continue
			}

			rules, err := chainToACLRules(ch)
			if err != nil {
				return fmt.Errorf("table %q: %w", tbl.Name, err)
			}

			tag := aclTag(tbl.Name, ch.Name)
			prevIndex, isUpdate := b.aclIndexes[tag]
			replaceIndex := uint32(0xffffffff)
			if isUpdate {
				replaceIndex = prevIndex
			}

			req := &govppacl.ACLAddReplace{
				ACLIndex: replaceIndex,
				Tag:      tag,
				Count:    uint32(len(rules)),
				R:        rules,
			}
			aclIdx, err := ops.aclAddReplace(req)
			if err != nil {
				return fmt.Errorf("table %q chain %q: %w", tbl.Name, ch.Name, err)
			}
			newACLIndexes[tag] = aclIdx

			if !isUpdate {
				capturedIdx := aclIdx
				*undo = append(*undo, func() {
					_ = ops.aclDel(capturedIdx)
				})
			}
		}
	}
	return nil
}

// hookIsInput returns true for hooks where ACLs apply as input (ingress)
// filtering, false for output (egress).
func hookIsInput(h firewall.ChainHook) bool {
	switch h {
	case firewall.HookInput, firewall.HookForward, firewall.HookPrerouting, firewall.HookIngress:
		return true
	default:
		return false
	}
}

// bindAllACLs reads the current per-interface ACL bindings from VPP,
// strips ze-owned ACL indexes, merges in the new ze ACLs, and writes
// back. This read-merge-write preserves non-ze ACL bindings that
// another controller may have installed.
//
// VPP's ACLInterfaceSetACLList replaces the entire ACL vector in one
// call: the first nInput entries are input ACLs, the rest are output.
// Both directions are merged into a single call per interface.
//
// Base chains bind to ALL known interfaces in the direction determined
// by the chain's hook point.
func (b *backend) bindAllACLs(
	ops vppOps,
	nameIndex map[string]interface_types.InterfaceIndex,
	desired []firewall.Table,
	aclIndexes map[string]uint32,
	newIfaceACLs map[interface_types.InterfaceIndex]ifaceACLBinding,
) error {
	zeACLSet := b.allZeACLIndexes(aclIndexes)

	zeInput := make(map[interface_types.InterfaceIndex][]uint32)
	zeOutput := make(map[interface_types.InterfaceIndex][]uint32)
	for i := range desired {
		tbl := &desired[i]
		for j := range tbl.Chains {
			ch := &tbl.Chains[j]
			if !ch.IsBase {
				continue
			}
			tag := aclTag(tbl.Name, ch.Name)
			aclIdx, ok := aclIndexes[tag]
			if !ok {
				continue
			}
			isInput := hookIsInput(ch.Hook)
			for _, swIfIndex := range nameIndex {
				if isInput {
					zeInput[swIfIndex] = append(zeInput[swIfIndex], aclIdx)
				} else {
					zeOutput[swIfIndex] = append(zeOutput[swIfIndex], aclIdx)
				}
			}
		}
	}

	touched := make(map[interface_types.InterfaceIndex]bool)
	for swIfIndex := range zeInput {
		touched[swIfIndex] = true
	}
	for swIfIndex := range zeOutput {
		touched[swIfIndex] = true
	}
	for swIfIndex := range b.ifaceACLs {
		touched[swIfIndex] = true
	}

	for swIfIndex := range touched {
		existing, err := ops.aclInterfaceListDump(swIfIndex)
		if err != nil {
			return fmt.Errorf("dump ACL list for interface %d: %w", swIfIndex, err)
		}

		foreignInput, foreignOutput := splitForeign(existing, zeACLSet)

		mergedInput := append(foreignInput, zeInput[swIfIndex]...)
		mergedOutput := append(foreignOutput, zeOutput[swIfIndex]...)

		acls := make([]uint32, 0, len(mergedInput)+len(mergedOutput))
		acls = append(acls, mergedInput...)
		acls = append(acls, mergedOutput...)
		nInput := uint8(len(mergedInput))

		if err := ops.aclInterfaceSetACLList(swIfIndex, nInput, acls); err != nil {
			return fmt.Errorf("bind ACLs to interface %d: %w", swIfIndex, err)
		}
		newIfaceACLs[swIfIndex] = ifaceACLBinding{
			input:  mergedInput,
			output: mergedOutput,
		}
	}
	return nil
}

// allZeACLIndexes returns the union of currently-tracked and
// newly-created ze ACL indexes so splitForeign can identify which
// indexes to strip from existing bindings.
func (b *backend) allZeACLIndexes(newIndexes map[string]uint32) map[uint32]bool {
	s := make(map[uint32]bool, len(b.aclIndexes)+len(newIndexes))
	for _, idx := range b.aclIndexes {
		s[idx] = true
	}
	for _, idx := range newIndexes {
		s[idx] = true
	}
	return s
}

// splitForeign separates an interface's existing ACL list into
// foreign (non-ze) input and output slices, preserving order.
func splitForeign(existing ifaceACLList, zeACLs map[uint32]bool) (foreignInput, foreignOutput []uint32) {
	nInput := int(existing.nInput)
	for i, idx := range existing.acls {
		if zeACLs[idx] {
			continue
		}
		if i < nInput {
			foreignInput = append(foreignInput, idx)
		} else {
			foreignOutput = append(foreignOutput, idx)
		}
	}
	return foreignInput, foreignOutput
}

func (b *backend) reconcileRemovals(ops vppOps, newACLIndexes map[string]uint32) {
	lg := logger()
	for tag, idx := range b.aclIndexes {
		if _, keep := newACLIndexes[tag]; keep {
			continue
		}
		if err := ops.aclDel(idx); err != nil {
			lg.Warn("firewall-vpp: delete stale ACL failed (treating as already gone)",
				"tag", tag, "idx", idx, "err", err)
		}
	}
}

func (b *backend) ListTables() ([]firewall.Table, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lastApplied == nil {
		return nil, nil
	}
	return b.lastApplied, nil
}

func (b *backend) GetCounters(tableName string) ([]firewall.ChainCounters, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// VPP ACL has per-ACL counters but they require ACLStatsIntfCountersEnable
	// and a stats segment connection. Return empty for now; operators use
	// `vppctl show acl-plugin acl` for VPP-native counter inspection.
	return nil, nil
}

func (b *backend) Close() error { return nil }

// govppOps is the production adapter implementing vppOps on top of a
// live GoVPP api.Channel.
type govppOps struct {
	ch api.Channel
}

func (g *govppOps) dumpInterfaces() (map[string]interface_types.InterfaceIndex, error) {
	req := &interfaces.SwInterfaceDump{SwIfIndex: ^interface_types.InterfaceIndex(0)}
	rctx := g.ch.SendMultiRequest(req)
	out := make(map[string]interface_types.InterfaceIndex)
	for {
		d := &interfaces.SwInterfaceDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return nil, fmt.Errorf("SwInterfaceDump: %w", err)
		}
		if last {
			break
		}
		out[d.InterfaceName] = d.SwIfIndex
	}
	return out, nil
}

func (g *govppOps) aclAddReplace(req *govppacl.ACLAddReplace) (uint32, error) {
	reply := &govppacl.ACLAddReplaceReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("ACLAddReplace: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return 0, fmt.Errorf("ACLAddReplace: %w", apiErr)
	}
	return reply.ACLIndex, nil
}

func (g *govppOps) aclDel(aclIndex uint32) error {
	req := &govppacl.ACLDel{ACLIndex: aclIndex}
	reply := &govppacl.ACLDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ACLDel: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("ACLDel: %w", apiErr)
	}
	return nil
}

func (g *govppOps) aclDump() ([]aclDumpEntry, error) {
	req := &govppacl.ACLDump{ACLIndex: ^uint32(0)}
	rctx := g.ch.SendMultiRequest(req)
	var entries []aclDumpEntry
	for {
		d := &govppacl.ACLDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return nil, fmt.Errorf("ACLDump: %w", err)
		}
		if last {
			break
		}
		entries = append(entries, aclDumpEntry{
			Index: d.ACLIndex,
			Tag:   d.Tag,
			Rules: d.R,
		})
	}
	return entries, nil
}

func (g *govppOps) aclInterfaceListDump(swIfIndex interface_types.InterfaceIndex) (ifaceACLList, error) {
	req := &govppacl.ACLInterfaceListDump{SwIfIndex: swIfIndex}
	rctx := g.ch.SendMultiRequest(req)
	var result ifaceACLList
	for {
		d := &govppacl.ACLInterfaceListDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return ifaceACLList{}, fmt.Errorf("ACLInterfaceListDump: %w", err)
		}
		if last {
			break
		}
		if d.SwIfIndex == swIfIndex {
			result.nInput = d.NInput
			result.acls = d.Acls
		}
	}
	return result, nil
}

func (g *govppOps) aclInterfaceSetACLList(swIfIndex interface_types.InterfaceIndex, nInput uint8, acls []uint32) error {
	req := &govppacl.ACLInterfaceSetACLList{
		SwIfIndex: swIfIndex,
		Count:     uint8(len(acls)),
		NInput:    nInput,
		Acls:      acls,
	}
	reply := &govppacl.ACLInterfaceSetACLListReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ACLInterfaceSetACLList: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("ACLInterfaceSetACLList: %w", apiErr)
	}
	return nil
}
