// Design: docs/architecture/traffic/fw-7b-backend-hardening.md -- govppOps production adapter
// Related: ops.go (vppOps interface), backend_linux.go (Apply/reconcile consumers)

//go:build linux

package trafficvpp

import (
	"fmt"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/classify"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"
)

// govppOps is the production adapter that implements vppOps on top of a
// live GoVPP api.Channel. Stateless by design -- the channel is the only
// field, and each method is a direct wrap around a single VPP RPC with
// retval decoding. Tests substitute a fakeOps that records calls.
//
// See `ops.go` for the interface definition.
type govppOps struct {
	ch api.Channel
}

// dumpInterfaces walks SwInterfaceDump and returns a name->sw_if_index
// map for every interface VPP currently knows about. Called once per
// Apply; sw_if_index values change on interface recreate so the lookup
// must not be cached across Applys.
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

// dumpPolicers returns every policer name currently present in VPP. Used only
// by startup orphan cleanup; normal reconciliation uses the in-memory tracker
// because VPP does not expose enough binding state to reconstruct Ze's full
// desired model.
func (g *govppOps) dumpPolicers() ([]string, error) {
	req := &policer.PolicerDump{}
	rctx := g.ch.SendMultiRequest(req)
	var names []string
	for {
		d := &policer.PolicerDetails{}
		last, err := rctx.ReceiveReply(d)
		if err != nil {
			return nil, fmt.Errorf("PolicerDump: %w", err)
		}
		if last {
			break
		}
		names = append(names, d.Name)
	}
	return names, nil
}

// policerAddDel wraps PolicerAddDel with retval checking. Retval != 0 is
// decoded via api.RetvalToVPPApiError so the caller sees VPP's named error
// (e.g. ENOMEM, INVALID_VALUE) instead of a raw integer. Returns the index
// VPP assigned to the policer (required by policerDel).
func (g *govppOps) policerAddDel(req *policer.PolicerAddDel) (uint32, error) {
	reply := &policer.PolicerAddDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("PolicerAddDel: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return 0, fmt.Errorf("PolicerAddDel: %w", apiErr)
	}
	return reply.PolicerIndex, nil
}

// policerDel removes a policer by its VPP-assigned index. Used during
// reconciliation to clean up policers no longer referenced.
func (g *govppOps) policerDel(index uint32) error {
	req := &policer.PolicerDel{PolicerIndex: index}
	reply := &policer.PolicerDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerDel: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerDel: %w", apiErr)
	}
	return nil
}

// policerDeleteByName removes a policer by name via the older add/del API.
// Startup orphan cleanup only has names from PolicerDump, not VPP indexes.
func (g *govppOps) policerDeleteByName(name string) error {
	req := &policer.PolicerAddDel{IsAdd: false, Name: name}
	reply := &policer.PolicerAddDelReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerAddDel(delete): %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerAddDel(delete): %w", apiErr)
	}
	return nil
}

// policerOutput binds (apply=true) or unbinds (apply=false) a policer by
// name to an interface's output. This is the mechanism by which a
// configured class rate actually limits egress traffic.
func (g *govppOps) policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error {
	req := &policer.PolicerOutput{Name: name, SwIfIndex: swIfIndex, Apply: apply}
	reply := &policer.PolicerOutputReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerOutput: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerOutput: %w", apiErr)
	}
	return nil
}

// classifyAddDelTable wraps ClassifyAddDelTable. On create (isAdd=true,
// tableIdx=^uint32(0)) VPP assigns and returns a fresh table index. Match
// vectors count is derived from the mask length; skipNVectors is passed
// through so the policer-classify arc examines the right byte window.
// nextTableIdx chains a miss to a successor table (^uint32(0) = end of chain);
// real VPP v25.10 stores it as the table's NextTbl so a miss on the head falls
// through (validated: chained head shows NextTbl = successor index).
func (g *govppOps) classifyAddDelTable(tableIdx uint32, mask []byte, skipNVectors, nextTableIdx uint32, isAdd bool) (uint32, error) {
	nVectors := uint32(len(mask)) / classifyVectorLen
	if nVectors == 0 {
		nVectors = 1
	}
	req := &classify.ClassifyAddDelTable{
		IsAdd:          isAdd,
		TableIndex:     tableIdx,
		Nbuckets:       2,
		MemorySize:     1 << 20,
		SkipNVectors:   skipNVectors,
		MatchNVectors:  nVectors,
		NextTableIndex: nextTableIdx,
		MissNextIndex:  ^uint32(0),
		MaskLen:        uint32(len(mask)),
		Mask:           mask,
	}
	reply := &classify.ClassifyAddDelTableReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return 0, fmt.Errorf("ClassifyAddDelTable: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return 0, fmt.Errorf("ClassifyAddDelTable: %w", apiErr)
	}
	return reply.NewTableIndex, nil
}

// classifyAddDelSession wraps ClassifyAddDelSession. policerIdx is the policer
// a matching packet is steered to on the policer-classify arc, carried in
// HitNextIndex -- byte-for-byte the session VPP's own CLI `classify session ...
// policer-hit-next <name>` produces (next_index=policer index, action=0),
// verified against VPP v25.10. (An earlier attempt failed INVALID_VALUE (-7);
// the cause was the table's skip/match width, not this field -- the session
// match must span skip+match vectors, so the traffic tables use skip=0 with a
// full-width absolute-offset mask; see translate.go.)
func (g *govppOps) classifyAddDelSession(tableIdx, policerIdx uint32, match []byte, isAdd bool) error {
	req := &classify.ClassifyAddDelSession{
		IsAdd:        isAdd,
		TableIndex:   tableIdx,
		HitNextIndex: policerIdx,
		MatchLen:     uint32(len(match)),
		Match:        match,
	}
	reply := &classify.ClassifyAddDelSessionReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ClassifyAddDelSession: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("ClassifyAddDelSession: %w", apiErr)
	}
	return nil
}

// policerClassifySetInterface binds/unbinds ip4 and ip6 classify tables to
// an interface's policer-classify feature. ^uint32(0) means "no table for
// this family". On unbind (isAdd=false) VPP requires ^uint32(0) for the
// table indices being removed.
func (g *govppOps) policerClassifySetInterface(swIfIndex interface_types.InterfaceIndex, ip4TableIdx, ip6TableIdx uint32, isAdd bool) error {
	if !isAdd {
		ip4TableIdx = ^uint32(0)
		ip6TableIdx = ^uint32(0)
	}
	req := &classify.PolicerClassifySetInterface{
		SwIfIndex:     swIfIndex,
		IP4TableIndex: ip4TableIdx,
		IP6TableIndex: ip6TableIdx,
		L2TableIndex:  ^uint32(0),
		IsAdd:         isAdd,
	}
	reply := &classify.PolicerClassifySetInterfaceReply{}
	if err := g.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("PolicerClassifySetInterface: %w", err)
	}
	if apiErr := api.RetvalToVPPApiError(reply.Retval); apiErr != nil {
		return fmt.Errorf("PolicerClassifySetInterface: %w", apiErr)
	}
	return nil
}
