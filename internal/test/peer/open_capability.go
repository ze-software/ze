// Design: docs/architecture/testing/ci-format.md — the capability values ze-peer owns
// Overview: open.go — buildOpen, which reads ze's OPEN and calls into this file
// Related: peer.go — the Config fields a .ci fills for each owned capability
// RFC: rfc/short/rfc5492.md — the Capability Code, Length and Value triple
// RFC: rfc/short/rfc9234.md — the Role capability and its pairing rule
// RFC: rfc/short/rfc7911.md — the ADD-PATH Send/Receive field
// RFC: rfc/short/rfc4724.md — the Graceful Restart Restart Time and family tuples
// RFC: rfc/short/rfc9494.md — the Long-Lived Graceful Restart 7-octet tuple
// RFC: rfc/short/draft-abraitis-idr-addpath-paths-limit.md — the Max Paths field
//
// A capability whose VALUE describes the SENDER is resolved here, once, from the
// test's own configuration. Every other capability ze sent is mirrored octet for
// octet by reconcileParams (open.go), which is what keeps a .ci that says
// nothing about capabilities negotiating whatever ze offers.
//
// The list in ownedCapabilities IS the inventory: a code absent from it is
// mirrored by construction, so there is no second place to keep in step and
// nothing to go stale (ai/rules/principles.md).

package peer

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// ownedCapabilities holds the capability TLVs the builder resolves itself,
// keyed by capability code. Each one replaces the mirrored TLV of the same code,
// in the place the mirror held it, so the parameter order ze sent is the order
// ze-peer answers with.
//
// A code that ze did not send and that ze-peer must nevertheless assert is
// carried here with add set, and appended as its own parameter.
type ownedCapability struct {
	tlv []byte
	add bool
}

// ownedCapabilities resolves every capability whose value describes ze-peer.
func ownedCapabilities(read openParams, id openIdentity) (map[byte]ownedCapability, error) {
	owned := make(map[byte]ownedCapability, 4)

	// RFC 6793 Section 4.1: "The AS number of the BGP speaker MUST be carried in
	// the Capability Value field of the 'support for four-octet AS number
	// capability'." A mirrored capability 65 is therefore ze's AS asserted in
	// ze-peer's name, and it is the carrier a conforming receiver reads first.
	asn4 := &capability.ASN4{ASN: id.as}
	asn4TLV := make([]byte, asn4.Len())
	asn4.WriteTo(asn4TLV, 0)
	// Above 65535 the My Autonomous System field can only carry AS_TRANS, so the
	// capability is the ONLY carrier of the real AS and ze-peer adds it when ze
	// offered none. At or below 65535 the header field carries the AS on its own,
	// and adding a capability ze did not offer would change what the session
	// negotiates.
	owned[byte(capability.CodeASN4)] = ownedCapability{tlv: asn4TLV, add: id.as > 65535}

	for _, param := range read.params {
		for _, tlv := range param.caps {
			code := tlv[0]
			value := tlv[2:]
			switch capability.Code(code) { //nolint:exhaustive // only the codes whose value describes the SENDER are resolved here
			case capability.CodeRole:
				role, err := complementaryRole(value)
				if err != nil {
					return nil, err
				}
				owned[code] = ownedCapability{tlv: role}
			case capability.CodeAddPath:
				addPath, err := invertedAddPath(value)
				if err != nil {
					return nil, err
				}
				owned[code] = ownedCapability{tlv: addPath}
			case capability.CodeFQDN:
				fqdn := &capability.FQDN{Hostname: peerHostname}
				tlv := make([]byte, fqdn.Len())
				fqdn.WriteTo(tlv, 0)
				owned[code] = ownedCapability{tlv: tlv}
			case capability.CodeGracefulRestart:
				owned[code] = ownedCapability{tlv: gracefulRestartTLV(id.gracefulRestart)}
			case capabilityCodeLLGR:
				owned[code] = ownedCapability{tlv: llgrTLV(id.llgr)}
			case capabilityCodeSoftwareVersion:
				owned[code] = ownedCapability{tlv: softwareVersionTLV(peerSoftwareVersion)}
			case capability.CodePathsLimit:
				owned[code] = ownedCapability{tlv: pathsLimitTLV(id.pathsLimit)}
			}
		}
	}

	return owned, nil
}

// gracefulRestartTLV writes ze-peer's own Graceful Restart capability.
//
// RFC 4724 Section 3: the value is "Restart Flags (4 bits)" and "Restart Time in
// seconds (12 bits)", followed by zero or more four-octet <AFI, SAFI, Flags>
// tuples whose Flags octet carries the Forwarding State bit.
//
// The tuples are what make the capability act on a receiver. onSessionDown
// (internal/component/bgp/plugins/gr/gr_state.go) builds its stale-family set
// from them and returns false when the set is empty, so a value carrying the
// time alone dispatches nothing.
func gracefulRestartTLV(decl GracefulRestartDecl) []byte {
	gr := &capability.GracefulRestart{RestartTime: decl.RestartTime}
	gr.Families = make([]capability.GracefulRestartFamily, 0, len(decl.Families))
	for _, fam := range decl.Families {
		gr.Families = append(gr.Families, capability.GracefulRestartFamily{
			AFI:             fam.AFI,
			SAFI:            fam.SAFI,
			ForwardingState: decl.ForwardState,
		})
	}
	tlv := make([]byte, gr.Len())
	gr.WriteTo(tlv, 0)
	return tlv
}

// llgrTLV writes ze-peer's own Long-Lived Graceful Restart capability.
//
// RFC 9494 Section 3 gives the value one 7-octet tuple per family, and no header
// before them: AFI (2 octets), SAFI (1), Flags (1) whose high bit is the
// Long-lived Forwarding State bit, and the Long-Lived Stale Time (3 octets).
// decodeLLGR (internal/component/bgp/plugins/gr/gr_llgr.go) reads exactly that.
//
// The octets are written here rather than through internal/core/bgp/capability
// because that package holds no code-71 type: ze's own value is built by the
// bgp-gr plugin, and importing a plugin into the harness would couple the test
// peer to the subsystem it exists to test.
func llgrTLV(decl LLGRDecl) []byte {
	const tupleLen = 7
	value := make([]byte, tupleLen*len(decl.Families))
	for i, fam := range decl.Families {
		at := i * tupleLen
		binary.BigEndian.PutUint16(value[at:], uint16(fam.AFI))
		value[at+2] = byte(fam.SAFI)
		if decl.ForwardState {
			value[at+3] = 0x80
		}
		value[at+4] = byte(decl.StaleTime >> 16)
		value[at+5] = byte(decl.StaleTime >> 8)
		value[at+6] = byte(decl.StaleTime)
	}
	return capabilityTLV(capabilityCodeLLGR, value)
}

// softwareVersionTLV writes ze-peer's own software version capability.
//
// draft-abraitis-bgp-version-capability: the value is a one-octet length
// followed by that many octets of UTF-8. decodeSoftwareVersion
// (internal/component/bgp/plugins/softver/softver.go) reads that pair, and
// encodeValue in the same file writes ze's own the same way.
//
// The version is a constant rather than an option because no session decision
// turns on it: capability.Parse holds no code-75 arm, and the only reader is
// capabilityToZeJSON for the offline `ze bgp decode`. An option would be a
// declaration nothing could observe.
func softwareVersionTLV(version string) []byte {
	value := make([]byte, 1+len(version))
	value[0] = byte(len(version)) //nolint:gosec // peerSoftwareVersion is a constant well under 255 octets
	copy(value[1:], version)
	return capabilityTLV(capabilityCodeSoftwareVersion, value)
}

// pathsLimitTLV writes ze-peer's own PATHS-LIMIT capability.
//
// draft-abraitis-idr-addpath-paths-limit Section 3 gives each entry five octets:
// AFI (2), SAFI (1) and Max Paths (2). negotiatePathsLimit
// (internal/core/bgp/capability/negotiated.go) reads the entries into
// pathsLimitSend, "Remote's limits (constrains our send)", which
// CommitService.enforcePathsLimit then applies to what ze sends this peer.
func pathsLimitTLV(entries []PathsLimitDecl) []byte {
	limit := &capability.PathsLimit{Entries: make([]capability.PathsLimitEntry, 0, len(entries))}
	for _, entry := range entries {
		limit.Entries = append(limit.Entries, capability.PathsLimitEntry{
			AFI:   entry.Family.AFI,
			SAFI:  entry.Family.SAFI,
			Limit: entry.Limit,
		})
	}
	tlv := make([]byte, limit.Len())
	limit.WriteTo(tlv, 0)
	return tlv
}

// capabilityTLV wraps a capability value in the RFC 5492 Section 4 triple of
// Capability Code, Capability Length and Capability Value.
//
// It is used for the two codes internal/core/bgp/capability holds no type for.
// A value above 255 octets cannot be stated at all, so it is a harness defect
// rather than a wire condition and it stops the process.
func capabilityTLV(code capability.Code, value []byte) []byte {
	if len(value) > 255 {
		panic("BUG: a capability value above the 255 octets RFC 5492 Section 4 gives the Capability Length")
	}
	tlv := make([]byte, 2+len(value))
	tlv[0] = byte(code)
	tlv[1] = byte(len(value)) //nolint:gosec // the guard above bounds it
	copy(tlv[2:], value)
	return tlv
}

// refuseUnofferedDeclarations refuses a `.ci` that states a fact for a
// capability ze did not offer.
//
// The capability SET ze-peer advertises is a mirror of ze's, so a code ze never
// sent has no TLV for the declared value to replace and the line would change
// nothing. Left to run, the file reads as a test of graceful restart and tests
// the session's establishment instead
// (plan/learned/005-runner-drops-what-it-cannot-honor.md).
func refuseUnofferedDeclarations(owned map[byte]ownedCapability, cfg *Config) error {
	declared := []struct {
		code   capability.Code
		option string
		stated bool
	}{
		{capability.CodeGracefulRestart, optOpenGracefulRestart, cfg.GracefulRestart != nil},
		{capabilityCodeLLGR, optOpenLLGR, cfg.LLGR != nil},
		{capability.CodePathsLimit, optOpenPathsLimit, len(cfg.PathsLimit) > 0},
	}
	for _, fact := range declared {
		if !fact.stated {
			continue
		}
		if _, offered := owned[byte(fact.code)]; offered {
			continue
		}
		return fmt.Errorf(
			"the peer block states option=open:value=%s, and ze's OPEN carries no capability %d, so "+
				"ze-peer has nothing to state it in: the capability SET ze-peer sends mirrors ze's. "+
				"Configure ze to offer capability %d, or drop the option",
			fact.option, fact.code, fact.code)
	}
	return nil
}

// complementaryRole answers ze's Role capability with the role RFC 9234 Section
// 4.2, Table 2 pairs with it.
//
// Section 4.1 defines the capability: code 9, "Length: 1 (octet)", and a value
// from Table 1. Section 4.2 defines which pairs correspond. Both are cited
// below, each at the section that carries what it claims.
//
// RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker MUST
// reject the connection using the Role Mismatch Notification." A mirror sends
// ze's own role back, which corresponds for Peer alone and is refused for the
// other four values. validateOpenRolePair
// (internal/component/bgp/plugins/role/validate.go) holds the same table.
func complementaryRole(value []byte) ([]byte, error) {
	if len(value) != 1 {
		return nil, fmt.Errorf("ze's Role capability carries %d octets, RFC 9234 Section 4.1 defines 1", len(value))
	}
	// RFC 9234 Section 4.1, Table 1 names the values: Provider(0), RS(1),
	// RS-Client(2), Customer(3), Peer(4), with 5 to 255 unassigned. Section 4.2,
	// Table 2 pairs them: Provider with Customer, RS with RS-Client, and Peer
	// with itself.
	complement := map[byte]byte{0: 3, 3: 0, 1: 2, 2: 1, 4: 4}
	pair, known := complement[value[0]]
	if !known {
		return nil, fmt.Errorf("ze's Role capability carries %d, which RFC 9234 Section 4.1 Table 1 leaves unassigned", value[0])
	}
	return []byte{byte(capability.CodeRole), 1, pair}, nil
}

// invertedAddPath answers ze's ADD-PATH capability with the directions that make
// the negotiation non-empty.
//
// RFC 7911 Section 4: the Send/Receive field says whether the SENDER can receive
// (1), send (2) or both (3). Negotiate (internal/core/bgp/capability/negotiated.go)
// intersects the local Send with the remote Receive and the local Receive with
// the remote Send, so a mirrored one-directional capability intersects to
// AddPathNone and the session negotiates ADD-PATH off with no error anywhere.
func invertedAddPath(value []byte) ([]byte, error) {
	parsed, err := capability.Parse(append([]byte{byte(capability.CodeAddPath), byte(len(value))}, value...))
	if err != nil {
		return nil, fmt.Errorf("read ze's ADD-PATH capability: %w", err)
	}
	addPath, ok := parsed[0].(*capability.AddPath)
	if !ok {
		return nil, fmt.Errorf("ze's ADD-PATH capability did not decode as one")
	}
	for i, family := range addPath.Families {
		switch family.Mode {
		case capability.AddPathSend:
			addPath.Families[i].Mode = capability.AddPathReceive
		case capability.AddPathReceive:
			addPath.Families[i].Mode = capability.AddPathSend
		case capability.AddPathBoth, capability.AddPathNone:
			// Both is its own complement, and None is what RFC 7911 Section 4
			// leaves undefined. Neither is changed, so a .ci that drove an
			// invalid direction still drives it.
		}
	}
	tlv := make([]byte, addPath.Len())
	addPath.WriteTo(tlv, 0)
	return tlv, nil
}
