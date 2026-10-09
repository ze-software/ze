//go:build linux

// Design: docs/functional-tests.md -- FRR capture completion before SIGINT.
// Related: internal/core/pcap/reassemble.go -- bounded TCP sequence reassembly.
package fixture

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/pcap"
)

// jointSubnetFRRCaptureReady examines an immutable-length view of the growing
// savefile, only when its size changes. Incomplete records/streams remain pending
// under the existing driver deadline; no elapsed quiet period is a success fence.
func jointSubnetFRRCaptureReady(plan *jointSubnetFRRPlan, lastSize *int64) (bool, error) {
	file, err := os.Open(filepath.Join(plan.output, "recipient.pcap"))
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck // read-only descriptor
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() > 1<<20 {
		return false, fmt.Errorf("FRR capture exceeds fixture's 1 MiB bound")
	}
	if info.Size() == *lastSize {
		return false, nil
	}
	*lastSize = info.Size()
	return jointSubnetFRRCaptured(io.NewSectionReader(file, 0, info.Size()), plan.port, plan.scenario)
}

// jointSubnetFRRCaptured requires one contiguous S->P BGP stream from OPEN
// through complete subject and control UPDATEs. It never searches packet bytes
// for marker substrings, concatenates across TCP holes or counts reverse traffic.
func jointSubnetFRRCaptured(source io.Reader, port uint16, scenario jointSubnetFRRScenario) (bool, error) {
	streams, report, err := pcap.Reassemble(source, port)
	if errors.Is(err, io.EOF) {
		return false, nil
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("FRR capture parse: %w", err)
	}
	if report.RecordsSkipped != 0 {
		return false, fmt.Errorf("FRR capture contains %d undecodable records", report.RecordsSkipped)
	}
	if report.RecordsDropped != 0 {
		return false, fmt.Errorf("FRR capture exceeds reassembly flow bound")
	}
	if len(report.Truncated) != 0 {
		return false, fmt.Errorf("FRR capture exceeds reassembly byte bound")
	}
	if len(report.Gaps) != 0 {
		return false, nil // a subsequently captured out-of-order segment may fill it
	}
	var incoming *pcap.Stream
	for i := range streams {
		flow := streams[i].Flow
		if flow.SourceAddr != netip.MustParseAddr("2001:db8:a::1") {
			continue
		}
		if flow.TargetAddr != netip.MustParseAddr("2001:db8:a::2") {
			continue
		}
		if flow.TargetPort != port {
			continue
		}
		if incoming != nil {
			return false, fmt.Errorf("FRR capture contains multiple speaker-to-recipient streams")
		}
		incoming = &streams[i]
	}
	if incoming == nil {
		return false, nil
	}
	data := incoming.Bytes
	var opened, subject, control bool
	for len(data) != 0 {
		if len(data) < 19 {
			return false, nil
		}
		for _, octet := range data[:16] {
			if octet != 0xff {
				return false, fmt.Errorf("FRR capture has invalid BGP framing")
			}
		}
		length := int(binary.BigEndian.Uint16(data[16:18]))
		if length < 19 {
			return false, fmt.Errorf("FRR capture BGP length %d is below its header", length)
		}
		if length > 4096 {
			return false, fmt.Errorf("FRR capture BGP length %d exceeds this session's limit", length)
		}
		if len(data) < length {
			return false, nil
		}
		kind := data[18]
		if !opened {
			if kind != 1 {
				return false, fmt.Errorf("FRR capture misses initial speaker OPEN")
			}
			if length < 29 {
				return false, fmt.Errorf("FRR capture has short OPEN")
			}
			if data[19] != 4 {
				return false, fmt.Errorf("FRR capture has unsupported OPEN version")
			}
			if length != 29+int(data[28]) {
				return false, fmt.Errorf("FRR capture has malformed OPEN optional-parameter length")
			}
			opened = true
		} else {
			switch kind {
			case 2:
				which, err := jointSubnetFRRCapturedUpdate(data[19:length], scenario)
				if err != nil {
					return false, err
				}
				switch which {
				case 1:
					subject = true
				case 2:
					if !subject {
						return false, fmt.Errorf("FRR capture control precedes subject")
					}
					control = true
				}
			case 4:
				if length != 19 {
					return false, fmt.Errorf("FRR capture has malformed KEEPALIVE")
				}
			default:
				return false, fmt.Errorf("FRR capture has unexpected BGP type %d", kind)
			}
		}
		data = data[length:]
	}
	return subject && control, nil
}

// jointSubnetFRRCapturedUpdate inspects complete UPDATE sections and complete
// MP_REACH values. The fixture has only these two /48 routes and IPv6 EOR.
// The result is the prefix discriminator (1 subject, 2 control); zero means no
// completion prefix. On error the discriminator is zero and MUST NOT be used.
func jointSubnetFRRCapturedUpdate(body []byte, scenario jointSubnetFRRScenario) (int, error) {
	global, err := scenario.nextHop()
	if err != nil {
		return 0, err
	}
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		return 0, fmt.Errorf("FRR capture UPDATE: %w", err)
	}
	if len(sections.Withdrawn(body)) != 0 {
		return 0, fmt.Errorf("FRR capture has unexpected legacy withdrawal")
	}
	if len(sections.NLRI(body)) != 0 {
		return 0, fmt.Errorf("FRR capture has unexpected legacy NLRI")
	}
	var reach, community []byte
	iter := attribute.NewAttrIterator(sections.Attrs(body))
	for code, flags, value, ok := iter.Next(); ok; code, flags, value, ok = iter.Next() {
		//exhaustive:ignore // Only completion fields and withdrawals belong to this oracle.
		switch code {
		case attribute.AttrMPReachNLRI:
			if flags&^attribute.FlagExtLength != attribute.FlagOptional {
				return 0, fmt.Errorf("FRR capture has invalid MP_REACH flags")
			}
			if reach != nil {
				return 0, fmt.Errorf("FRR capture has duplicate MP_REACH")
			}
			reach = value
		case attribute.AttrMPUnreachNLRI:
			if flags&^attribute.FlagExtLength != attribute.FlagOptional {
				return 0, fmt.Errorf("FRR capture has invalid MP_UNREACH flags")
			}
			if !bytes.Equal(value, []byte{0, 2, 1}) {
				return 0, fmt.Errorf("FRR capture has withdrawal or malformed EOR: %x", value)
			}
		case attribute.AttrCommunity:
			if flags&^(attribute.FlagExtLength|attribute.FlagPartial) != attribute.FlagOptional|attribute.FlagTransitive {
				return 0, fmt.Errorf("FRR capture has invalid community flags")
			}
			if community != nil {
				return 0, fmt.Errorf("FRR capture has duplicate community")
			}
			community = value
		}
	}
	if iter.Remaining() != 0 {
		return 0, fmt.Errorf("FRR capture has truncated attribute")
	}
	if reach == nil {
		return 0, nil // initial EOR, not either completion route
	}
	if len(reach) < 5 {
		return 0, fmt.Errorf("FRR capture has short MP_REACH")
	}
	nhLen := int(reach[3])
	if len(reach) != 5+nhLen+7 {
		return 0, fmt.Errorf("FRR capture MP_REACH does not hold exactly one complete /48")
	}
	nlri := reach[5+nhLen:]
	if !bytes.Equal(nlri[:6], []byte{48, 0x20, 1, 0x0d, 0xb8, 0x57}) {
		return 0, fmt.Errorf("FRR capture has unexpected completion NLRI: %x", nlri)
	}
	which := int(nlri[6])
	switch which {
	case 1, 2:
	default:
		return 0, fmt.Errorf("FRR capture has unexpected completion prefix: %x", nlri)
	}
	globalWire := global.As16()
	wantLen := 16
	if scenario == jointSubnetFRRSameLink {
		if which == 1 {
			wantLen = 32
		}
	}
	if nhLen != wantLen {
		return 0, fmt.Errorf("FRR capture next-hop length=%d, want %d", nhLen, wantLen)
	}
	if !bytes.Equal(reach[:3], []byte{0, 2, 1}) {
		return 0, fmt.Errorf("FRR capture MP_REACH family mismatch")
	}
	if reach[4+nhLen] != 0 {
		return 0, fmt.Errorf("FRR capture MP_REACH reserved octet mismatch")
	}
	if !bytes.Equal(reach[4:20], globalWire[:]) {
		return 0, fmt.Errorf("FRR capture global next-hop mismatch")
	}
	if wantLen == 32 {
		linkLocal := netip.MustParseAddr("fe80::9").As16()
		if !bytes.Equal(reach[20:36], linkLocal[:]) {
			return 0, fmt.Errorf("FRR capture link-local next-hop mismatch")
		}
	}
	if which == 2 {
		if !bytes.Equal(community, []byte{0xfd, 0xe9, 0, 7}) {
			return 0, fmt.Errorf("FRR capture control community is not exactly 65001:7")
		}
	}
	return which, nil
}
