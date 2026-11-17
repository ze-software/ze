// Design: docs/architecture/api/process-protocol.md -- engine-owned local origin data.
// Related: origin_receipt.go -- issued scope authority; reactor_api_batch.go -- existing local builder.
package reactor

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// originIdentity owns exactly one native path without ADD-PATH framing. Its
// nil/zero values are invalid. It preserves path-ID presence independently of
// legal identifier zero. Withdrawals need this identity, not announcement data.
// Immutable after construction; safe for concurrent readers.
type originIdentity struct {
	family    family.Family
	key       fwdPathKey
	native    []byte
	pathID    uint32
	hasPathID bool
}

func newOriginIdentity(fam family.Family, native []byte, pathID uint32, hasPathID bool) (*originIdentity, error) {
	if len(native) == 0 {
		return nil, errors.New("origin native path is missing")
	}
	if len(native) > message.ExtMsgLen {
		return nil, errors.New("origin native path exceeds the BGP message limit")
	}
	if !hasPathID {
		if pathID != 0 {
			return nil, errors.New("origin path identifier is present without its framing declaration")
		}
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		return nil, errors.New("origin family has no registered native framing")
	}
	count, err := split(native, false, nil)
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, errors.New("origin identity must contain exactly one native path")
	}
	var key fwdPathKey
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if err := fwdPathKeyFor(&key, fam, 0, native, false, scratch[:]); err != nil {
		return nil, err
	}
	// This copy is retained ownership of caller-borrowed API bytes, not encoding.
	return &originIdentity{family: fam, key: key, native: slices.Clone(native), pathID: pathID, hasPathID: hasPathID}, nil
}

// originBuildInput is raw engine-local input. The existing API must finish its
// semantic attribute and family validation before offering this value. Attribute
// framing and native identity are checked again where ownership is acquired.
type originBuildInput struct {
	family             family.Family
	native             []byte
	attributes         []byte
	attributesExplicit bool
	attributeContext   bgpctx.ContextID // Original Wire.SourceContext; zero/unknown stays unknown.
	nextHop            bgptypes.RouteNextHop
	originAS           uint32
	pathID             uint32
	hasPathID          bool
}

// originBuildValue retains the original local build choices. It holds no peer,
// Session, received-source reference, selection, or policy verdict. Nil/zero is
// invalid. Its byte slices are immutable and MUST NOT be exposed to a callback
// without an owned cross-boundary projection. Safe for concurrent readers.
type originBuildValue struct {
	*originIdentity
	attributes         []byte
	attributesExplicit bool
	attributeContext   bgpctx.ContextID // Original Wire.SourceContext; zero/unknown stays unknown.
	// wire indexes this value's owned immutable attributes once at admission.
	// It borrows that same slice and never outlives its owning origin value.
	wire     *attribute.AttributesWire
	nextHop  bgptypes.RouteNextHop
	originAS uint32
}

func newOriginBuildValue(input originBuildInput) (*originBuildValue, error) {
	identity, err := newOriginIdentity(input.family, input.native, input.pathID, input.hasPathID)
	if err != nil {
		return nil, err
	}
	if len(input.attributes) > message.ExtMsgLen-len(input.native) {
		return nil, errors.New("origin build data exceeds the BGP message limit")
	}
	if !input.attributesExplicit {
		if len(input.attributes) != 0 {
			return nil, errors.New("implicit origin attributes carry an explicit block")
		}
	}
	attributes := slices.Clone(input.attributes)
	var wire *attribute.AttributesWire
	if input.attributesExplicit {
		wire = attribute.NewAttributesWire(attributes, input.attributeContext)
		// Has checks only framing, without decoding ORIGIN or inferring an
		// unknown ASN width. The resulting immutable index is reused by every
		// selected write instead of rebuilt on the per-destination hot path.
		if _, err := wire.Has(attribute.AttrOrigin); err != nil {
			return nil, err
		}
	}
	// Raw build input is open until this constructor validates its policy.
	switch input.nextHop.Policy {
	case bgptypes.NextHopUnset:
		if input.family.NeedsNextHop() {
			return nil, errors.New("origin next-hop policy is missing")
		}
	case bgptypes.NextHopExplicit, bgptypes.NextHopSelf, bgptypes.NextHopDefault:
		if !input.nextHop.IsValid() {
			return nil, errors.New("origin next-hop policy is invalid")
		}
	default:
		return nil, errors.New("origin next-hop policy is unknown")
	}
	return &originBuildValue{originIdentity: identity, attributes: attributes,
		attributesExplicit: input.attributesExplicit, attributeContext: input.attributeContext, wire: wire, nextHop: input.nextHop, originAS: input.originAS}, nil
}
