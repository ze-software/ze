// Design: docs/architecture/core-design.md -- VPP classify pipeline for SetMark and Limit

//go:build linux

package firewallvpp

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"go.fd.io/govpp/binapi/interface_types"

	"github.com/ze-software/ze/internal/component/firewall"
)

const classifyMatchVectors = 3 // 48 bytes covers IPv4 header (20) + L4 ports (4) + padding

// classifyMaskMatch builds the byte-level mask and match arrays for a
// VPP classify table from ze match expressions. The table operates at
// L3 (skip_n_vectors=0, current_data_flag=0) for IPv4.
//
// IPv4 header layout (offsets from L3 start):
//
//	byte  9:     protocol
//	bytes 12-15: source address
//	bytes 16-19: destination address
//	bytes 20-21: source port (TCP/UDP)
//	bytes 22-23: destination port (TCP/UDP)
func classifyMaskMatch(matches []firewall.Match) (mask, match []byte, err error) {
	size := classifyMatchVectors * 16
	mask = make([]byte, size)
	match = make([]byte, size)

	for _, m := range matches {
		switch v := m.(type) {
		case firewall.MatchProtocol:
			proto, ok := firewall.ProtocolNumber(v.Protocol)
			if !ok {
				return nil, nil, fmt.Errorf("unknown protocol %q", v.Protocol)
			}
			mask[9] = 0xff
			match[9] = proto
		case firewall.MatchSourceAddress:
			applyPrefixMaskMatch(mask, match, v.Prefix, 12)
		case firewall.MatchDestinationAddress:
			applyPrefixMaskMatch(mask, match, v.Prefix, 16)
		case firewall.MatchSourcePort:
			if len(v.Ranges) > 0 && v.Ranges[0].Lo == v.Ranges[0].Hi {
				mask[20] = 0xff
				mask[21] = 0xff
				binary.BigEndian.PutUint16(match[20:22], v.Ranges[0].Lo)
			}
		case firewall.MatchDestinationPort:
			if len(v.Ranges) > 0 && v.Ranges[0].Lo == v.Ranges[0].Hi {
				mask[22] = 0xff
				mask[23] = 0xff
				binary.BigEndian.PutUint16(match[22:24], v.Ranges[0].Lo)
			}
		}
	}
	return mask, match, nil
}

func applyPrefixMaskMatch(mask, match []byte, prefix netip.Prefix, offset int) {
	addr := prefix.Masked().Addr()
	bits := prefix.Bits()
	if !addr.Is4() {
		return
	}
	a4 := addr.As4()
	prefixMask := prefixToMask(bits)
	for i := range 4 {
		mask[offset+i] = prefixMask[i]
		match[offset+i] = a4[i] & prefixMask[i]
	}
}

func prefixToMask(bits int) [4]byte {
	if bits <= 0 {
		return [4]byte{}
	}
	if bits > 32 {
		bits = 32
	}
	m := ^uint32(0) << (32 - bits)
	return [4]byte{byte(m >> 24), byte(m >> 16), byte(m >> 8), byte(m)}
}

// applyClassifyActions processes SetMark and Limit actions in filter
// chain terms. For each term with these actions, it creates a classify
// table + session and either sets metadata (SetMark) or binds to a
// policer (Limit).
func (b *backend) applyClassifyActions(
	ops vppOps,
	desired []firewall.Table,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	for i := range desired {
		tbl := &desired[i]
		for j := range tbl.Chains {
			ch := &tbl.Chains[j]
			if !ch.IsBase || ch.Type == firewall.ChainNAT {
				continue
			}
			for k := range ch.Terms {
				term := &ch.Terms[k]
				if err := b.applyTermClassify(ops, tbl, ch, term, nameIndex); err != nil {
					return fmt.Errorf("firewall-vpp: table %q chain %q term %q: %w",
						tbl.Name, ch.Name, term.Name, err)
				}
			}
		}
	}
	return nil
}

func (b *backend) applyTermClassify(
	ops vppOps,
	tbl *firewall.Table,
	ch *firewall.Chain,
	term *firewall.Term,
	nameIndex map[string]interface_types.InterfaceIndex,
) error {
	var markAction *firewall.SetMark
	var limitAction *firewall.Limit

	for _, a := range term.Actions {
		switch v := a.(type) {
		case firewall.SetMark:
			markAction = &v
		case firewall.Limit:
			limitAction = &v
		}
	}

	if markAction == nil && limitAction == nil {
		return nil
	}

	cmask, cmatch, err := classifyMaskMatch(term.Matches)
	if err != nil {
		return err
	}

	tableIdx, err := ops.classifyAddDelTable(^uint32(0), cmask, true)
	if err != nil {
		return fmt.Errorf("classify table: %w", err)
	}
	undoTable := func() { _, _ = ops.classifyAddDelTable(tableIdx, cmask, false) }

	if markAction != nil {
		if err := ops.classifyAddDelSession(tableIdx, cmatch, markAction.Value, true); err != nil {
			undoTable()
			return fmt.Errorf("classify session (mark): %w", err)
		}
	}

	if limitAction != nil {
		if err := ops.classifyAddDelSession(tableIdx, cmatch, 0, true); err != nil {
			undoTable()
			return fmt.Errorf("classify session (limit): %w", err)
		}
		polName := classifyPolicerName(tbl.Name, ch.Name, term.Name)
		if _, err := b.createLimitPolicer(ops, polName, limitAction); err != nil {
			undoTable()
			return fmt.Errorf("policer: %w", err)
		}
		for _, swIfIndex := range nameIndex {
			if err := ops.policerClassifySetInterface(swIfIndex, tableIdx, true); err != nil {
				undoTable()
				return fmt.Errorf("policer classify bind: %w", err)
			}
		}
	}

	if markAction != nil && limitAction == nil {
		for _, swIfIndex := range nameIndex {
			if err := ops.classifySetInterfaceIPTable(swIfIndex, tableIdx, true); err != nil {
				undoTable()
				return fmt.Errorf("classify interface bind: %w", err)
			}
		}
	}

	return nil
}

func (b *backend) createLimitPolicer(
	ops vppOps,
	name string,
	limit *firewall.Limit,
) (uint32, error) {
	rate, err := limitToRate(limit)
	if err != nil {
		return 0, err
	}
	isPackets := limit.Dimension != firewall.RateDimensionBytes
	return ops.policerAddDel(name, rate, limit.Burst, isPackets, true)
}

func limitToRate(limit *firewall.Limit) (uint32, error) {
	rate := limit.Rate
	switch limit.Unit {
	case "second":
		// rate is already per-second
	case "minute":
		rate /= 60
	case "hour":
		rate /= 3600
	case "day":
		rate /= 86400
	}
	if limit.Dimension == firewall.RateDimensionBytes {
		rate = rate * 8 / 1000
	}
	if rate == 0 {
		rate = 1
	}
	if rate > 0xffffffff {
		return 0, fmt.Errorf("rate %d exceeds VPP policer maximum", rate)
	}
	return uint32(rate), nil
}
