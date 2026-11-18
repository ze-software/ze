// Design: docs/architecture/api/process-protocol.md -- accepted redistribution boundary.
// Related: origin_intake.go -- engine-issued original contribution receipts.
package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An immutable accepted snapshot is bound to the actual original Process token.
// It describes the redistribution adapter's input, never an upstream producer
// lease. All reads MUST occur outside issuer, peer and manager leaf locks.
type originAcceptedBinding struct {
	owner    plugin.SelectedOwner
	query    rpc.OriginAdmissionLookupInput
	snapshot rpc.OriginAdmissionSnapshot
}

func originAdmissionPrefix(n nlri.NLRI) (netip.Prefix, error) {
	if n == nil {
		return netip.Prefix{}, errors.New("accepted-origin native route is missing")
	}
	if nlri.HasPathID(n) {
		return netip.Prefix{}, errors.New("accepted redistribution does not carry ADD-PATH identity")
	}
	native := n.Bytes()
	if len(native) == 0 {
		return netip.Prefix{}, errors.New("accepted-origin prefix encoding is empty")
	}
	bits := int(native[0])
	if len(native) != 1+(bits+7)/8 {
		return netip.Prefix{}, errors.New("accepted-origin prefix encoding has another native route or is truncated")
	}
	if n.Family() == family.IPv4Unicast {
		if bits > 32 {
			return netip.Prefix{}, errors.New("accepted-origin IPv4 prefix length exceeds32")
		}
		var bytes [4]byte
		copy(bytes[:], native[1:])
		prefix := netip.PrefixFrom(netip.AddrFrom4(bytes), bits)
		if prefix != prefix.Masked() {
			return netip.Prefix{}, errors.New("accepted-origin IPv4 prefix has host bits")
		}
		return prefix, nil
	}
	if n.Family() != family.IPv6Unicast {
		return netip.Prefix{}, errors.New("accepted redistribution uses only native unicast prefixes")
	}
	if bits > 128 {
		return netip.Prefix{}, errors.New("accepted-origin IPv6 prefix length exceeds128")
	}
	var bytes [16]byte
	copy(bytes[:], native[1:])
	prefix := netip.PrefixFrom(netip.AddrFrom16(bytes), bits)
	if prefix != prefix.Masked() {
		return netip.Prefix{}, errors.New("accepted-origin IPv6 prefix has host bits")
	}
	return prefix, nil
}

func originSourceFromAdmission(ctx context.Context, producer originProducer, id uint64, n nlri.NLRI, build *originBuildValue, peer *Peer, operation rpc.OriginIntakeOperation) (*originSource, error) {
	if producer.kind != originProducerProcess {
		return nil, errors.New("accepted redistribution needs its authenticated original accepting Process")
	}
	prefix, err := originAdmissionPrefix(n)
	if err != nil {
		return nil, err
	}
	input := rpc.OriginAdmissionLookupInput{AdmissionID: id, Family: n.Family().String(), Prefix: prefix.String(), Destination: peer.addrString, PeerOwner: peer.sourceOwner, Operation: operation}
	work, cancel := rpc.OriginAdmissionLookupCallContext(ctx, &input)
	defer cancel()
	output, err := producer.owner.OriginAdmissionLookup(work, input)
	if err != nil {
		return nil, err
	}
	protocol, known := redistevents.ProtocolIDOf(output.Snapshot.Protocol)
	if !known {
		return nil, errors.New("accepted redistribution names an unregistered original protocol")
	}
	if operation == rpc.OriginIntakeAnnounce {
		if err := originAdmissionBuildMatches(build, output.Snapshot); err != nil {
			return nil, err
		}
	}
	// Table and native source instance jointly name the original contribution.
	// Admission ID/generation is a version fence, not another candidate identity.
	instance := uint64(output.Snapshot.Table)<<32 | uint64(output.Snapshot.SourceInstance)
	source, err := newOriginSource(originSourceInput{kind: originSourceRedistributed, protocol: protocol, instance: instance,
		metric: output.Snapshot.SourceMetric, distance: output.Snapshot.DistanceOverride, hasDistance: output.Snapshot.HasDistanceOverride})
	if err != nil {
		return nil, err
	}
	source.accepted = &originAcceptedBinding{owner: producer.owner, query: input, snapshot: output.Snapshot}
	return source, nil
}

// current MUST run outside issuer/manager/election locks. The lookup worker is
// independent of the source's command awaiting this selected result. Exact
// retiring data can withdraw; it can never authorize another announcement.
func (source *originSource) acceptedCurrent(ctx context.Context, peer *Peer) error {
	if source.accepted == nil {
		return nil
	}
	binding := source.accepted
	input := binding.query
	input.Operation = rpc.OriginIntakeAnnounce
	input.Destination = peer.addrString
	input.PeerOwner = peer.sourceOwner
	input.DeadlineMS = 0
	work, cancel := rpc.OriginAdmissionLookupCallContext(ctx, &input)
	defer cancel()
	output, err := binding.owner.OriginAdmissionLookup(work, input)
	if err != nil {
		return err
	}
	if !binding.snapshot.Equal(output.Snapshot) {
		return errors.New("accepted redistribution original was replaced; reconcile the current contribution")
	}
	return nil
}

func originAdmissionBuildMatches(build *originBuildValue, snapshot rpc.OriginAdmissionSnapshot) error {
	if build == nil {
		return errors.New("accepted redistribution announce has no owned native build")
	}
	if build.originAS != snapshot.OriginASN {
		return errors.New("accepted redistribution origin AS differs from its original build")
	}
	if snapshot.NextHop == "" {
		if !build.nextHop.IsSelf() {
			return errors.New("accepted redistribution self next hop differs from its build")
		}
	} else {
		nextHop, err := netip.ParseAddr(snapshot.NextHop)
		if err != nil {
			return err
		}
		if build.nextHop.Policy != bgptypes.NextHopDefault {
			return errors.New("accepted redistribution next-hop policy differs from its default build")
		}
		if build.nextHop.Addr != nextHop {
			return errors.New("accepted redistribution next hop differs from its original build")
		}
	}
	origin, med, communities := false, false, false
	raw := build.attributes
	// Each iteration consumes one complete already-framed original attribute.
	for len(raw) > 0 {
		_, code, length, header, err := attribute.ParseHeader(raw)
		if err != nil {
			return err
		}
		if int(length) > len(raw)-header {
			return errors.New("accepted redistribution attributes are truncated")
		}
		value := raw[header : header+int(length)]
		// This adapter emits exactly ORIGIN, optional MED and communities.
		// Explicit API and other native-family rails never enter this branch.
		switch code {
		case attribute.AttrOrigin:
			if origin {
				return errors.New("accepted redistribution duplicates ORIGIN")
			}
			origin = true
			if len(value) != 1 {
				return errors.New("accepted redistribution ORIGIN is malformed")
			}
			want := byte(2)
			if snapshot.OriginASN != 0 {
				want = 0
			}
			if value[0] != want {
				return errors.New("accepted redistribution ORIGIN differs from its original source form")
			}
		case attribute.AttrMED:
			if med {
				return errors.New("accepted redistribution duplicates MED")
			}
			med = true
			if !snapshot.HasMetric {
				return errors.New("accepted redistribution has an unaccepted wire MED")
			}
			if len(value) != 4 {
				return errors.New("accepted redistribution wire MED is malformed")
			}
			if binary.BigEndian.Uint32(value) != snapshot.Metric {
				return errors.New("accepted redistribution wire MED differs from its accepted transform")
			}
		case attribute.AttrCommunity:
			if communities {
				return errors.New("accepted redistribution duplicates communities")
			}
			communities = true
			if len(value) != 4*len(snapshot.Community) {
				return errors.New("accepted redistribution communities differ from its accepted input")
			}
			for i, want := range snapshot.Community {
				if binary.BigEndian.Uint32(value[i*4:]) != want {
					return errors.New("accepted redistribution community differs from its accepted input")
				}
			}
		default:
			return errors.New("accepted redistribution build contains an attribute its accepted source did not provide")
		}
		raw = raw[header+int(length):]
	}
	if !origin {
		return errors.New("accepted redistribution ORIGIN is absent")
	}
	if med != snapshot.HasMetric {
		return errors.New("accepted redistribution optional MED presence differs")
	}
	if len(snapshot.Community) > 0 {
		if !communities {
			return errors.New("accepted redistribution community attribute is absent")
		}
	}
	return nil
}
