// Design: docs/architecture/wire/capabilities.md — capability negotiation
// RFC: rfc/short/rfc5492.md — capability negotiation logic

package capability

import (
	"fmt"
	"maps"
	"sort"
)

// RFC 5492: Capabilities Advertisement with BGP-4
// This file implements the capability negotiation process described in RFC 5492 Section 4.
// When a BGP speaker receives an OPEN message that contains the Capabilities Optional
// Parameter, the speaker MUST use the capabilities it recognizes to determine the
// features supported by both peers.

// Mismatch represents a capability that was not negotiated.
// RFC 5492 Section 3: When a required capability is not supported by peer,
// the speaker MAY send NOTIFICATION and terminate.
type Mismatch struct {
	// Code is the capability code
	Code Code
	// LocalSupported is true if local advertises this capability
	LocalSupported bool
	// PeerSupported is true if peer advertises this capability
	PeerSupported bool
	// Family is set for Multiprotocol mismatches
	Family *Family
}

// String returns a human-readable description of the mismatch.
func (m Mismatch) String() string {
	if m.Family != nil {
		familyStr := fmt.Sprintf("AFI=%d/SAFI=%d", m.Family.AFI, m.Family.SAFI)
		if m.LocalSupported && !m.PeerSupported {
			return "local supports " + familyStr + ", peer does not"
		}
		return "peer supports " + familyStr + ", local does not"
	}
	if m.LocalSupported && !m.PeerSupported {
		return "local supports " + m.Code.String() + ", peer does not"
	}
	return "peer supports " + m.Code.String() + ", local does not"
}

// Negotiated holds the result of capability negotiation between two BGP peers.
// Per RFC 5492 Section 4, a capability is considered negotiated when both peers
// advertise it in their OPEN messages.
//
// This struct uses a composite pattern with sub-components:
//   - Identity: Peer identification (ASNs, Router IDs) - shared with EncodingContexts
//   - Encoding: Wire encoding caps (ASN4, families, ADD-PATH) - shared with EncodingContexts
//   - Session: Session-level caps (ExtendedMessage, GR) - owned by Negotiated only
type Negotiated struct {
	// Composite sub-components (new structure)
	Identity *PeerIdentity // Shared with EncodingContexts
	Encoding *EncodingCaps // Shared with EncodingContexts
	Session  *SessionCaps  // Owned by Negotiated only

	// Backward compatibility fields (delegating to sub-components)
	// TODO: Remove these after all consumers migrate to sub-components

	// Peer identification (delegates to Identity)
	LocalASN uint32
	PeerASN  uint32

	// Negotiated features (delegates to Encoding/Session)
	// RFC 6793: BGP Support for Four-Octet Autonomous System (AS) Number Space
	ASN4 bool
	// RFC 8654: Extended Message Support for BGP
	ExtendedMessage bool
	// RFC 2918: Route Refresh Capability for BGP-4
	RouteRefresh bool
	// RFC 7313: Enhanced Route Refresh Capability for BGP
	EnhancedRouteRefresh bool
	// RFC 4271 Section 4.2: Hold Time is the minimum of the two Hold Time values
	HoldTime uint16

	// RFC 4724: Graceful Restart Mechanism for BGP
	GracefulRestart *GracefulRestart

	// Mismatches contains capabilities that were not negotiated.
	// RFC 5492 Section 3: For logging/reporting purposes.
	Mismatches []Mismatch

	// Internal maps (used by methods, populated from Encoding)
	families        map[Family]bool
	addPath         map[Family]AddPathMode
	extendedNextHop map[Family]AFI

	// peerCodes tracks which capability codes the peer advertised.
	// Used by CheckRefusedCodes to detect refused capabilities in peer's OPEN,
	// which won't appear in the negotiated intersection if we don't advertise them.
	peerCodes map[Code]bool

	// Cached family slice
	familySlice []Family
}

// Negotiate performs capability negotiation between local and remote capabilities.
//
// RFC 5492 Section 4: Capabilities Negotiation
// A BGP speaker determines the features supported by both peers by examining
// the intersection of capabilities advertised in the OPEN messages.
//
// For each capability type:
//   - RFC 4760: Multiprotocol - use intersection of address families
//   - RFC 6793: ASN4 - enabled if both peers advertise
//   - RFC 7911: ADD-PATH - complex mode negotiation per family
//   - RFC 8654: Extended Message - enabled if both peers advertise
//   - RFC 2918: Route Refresh - enabled if both peers advertise
func Negotiate(local, remote []Capability, localASN, peerASN uint32) *Negotiated {
	neg := &Negotiated{
		LocalASN:        localASN,
		PeerASN:         peerASN,
		families:        make(map[Family]bool),
		addPath:         make(map[Family]AddPathMode),
		extendedNextHop: make(map[Family]AFI),
	}

	// Build sets for efficient lookup
	localFamilies := make(map[Family]bool)
	remoteFamilies := make(map[Family]bool)
	var localAddPath, remoteAddPath *AddPath
	var localExtNH, remoteExtNH *ExtendedNextHop
	localASN4 := false
	remoteASN4 := false
	localExtMsg := false
	remoteExtMsg := false
	localRR := false
	remoteRR := false
	localERR := false
	remoteERR := false

	for _, c := range local {
		switch cap := c.(type) {
		case *Multiprotocol:
			localFamilies[Family{AFI: cap.AFI, SAFI: cap.SAFI}] = true
		case *ASN4:
			localASN4 = true
		case *AddPath:
			localAddPath = cap
		case *ExtendedMessage:
			localExtMsg = true
		case *RouteRefresh:
			localRR = true
		case *EnhancedRouteRefresh:
			localERR = true
		case *ExtendedNextHop:
			localExtNH = cap
		}
	}

	// Track peer's raw capability codes for CheckRefusedCodes.
	neg.peerCodes = make(map[Code]bool)
	for _, c := range remote {
		neg.peerCodes[c.Code()] = true
		switch cap := c.(type) {
		case *Multiprotocol:
			remoteFamilies[Family{AFI: cap.AFI, SAFI: cap.SAFI}] = true
		case *ASN4:
			remoteASN4 = true
		case *AddPath:
			remoteAddPath = cap
		case *ExtendedMessage:
			remoteExtMsg = true
		case *RouteRefresh:
			remoteRR = true
		case *EnhancedRouteRefresh:
			remoteERR = true
		case *GracefulRestart:
			neg.GracefulRestart = cap
		case *ExtendedNextHop:
			remoteExtNH = cap
		}
	}

	// RFC 5492 Section 4: Negotiated features require both peers to advertise.
	// RFC 6793 Section 3: ASN4 capability negotiation
	neg.ASN4 = localASN4 && remoteASN4
	// RFC 8654 Section 3: Extended Message capability negotiation
	neg.ExtendedMessage = localExtMsg && remoteExtMsg
	// RFC 2918 Section 3: Route Refresh capability negotiation
	neg.RouteRefresh = localRR && remoteRR
	// RFC 7313 Section 3.1: Enhanced Route Refresh capability negotiation
	neg.EnhancedRouteRefresh = localERR && remoteERR

	// RFC 5492 Section 3: Track mismatches for reporting
	if localASN4 != remoteASN4 {
		neg.Mismatches = append(neg.Mismatches, Mismatch{
			Code:           CodeASN4,
			LocalSupported: localASN4,
			PeerSupported:  remoteASN4,
		})
	}
	if localExtMsg != remoteExtMsg {
		neg.Mismatches = append(neg.Mismatches, Mismatch{
			Code:           CodeExtendedMessage,
			LocalSupported: localExtMsg,
			PeerSupported:  remoteExtMsg,
		})
	}
	if localRR != remoteRR {
		neg.Mismatches = append(neg.Mismatches, Mismatch{
			Code:           CodeRouteRefresh,
			LocalSupported: localRR,
			PeerSupported:  remoteRR,
		})
	}
	if localERR != remoteERR {
		neg.Mismatches = append(neg.Mismatches, Mismatch{
			Code:           CodeEnhancedRouteRefresh,
			LocalSupported: localERR,
			PeerSupported:  remoteERR,
		})
	}

	// RFC 4760 Section 8: Multiprotocol capability negotiation
	// Address families are usable only if both peers advertise support.
	for f := range localFamilies {
		if remoteFamilies[f] {
			neg.families[f] = true
		} else {
			// RFC 5492 Section 3: Track family mismatches
			fCopy := f
			neg.Mismatches = append(neg.Mismatches, Mismatch{
				Code:           CodeMultiprotocol,
				LocalSupported: true,
				PeerSupported:  false,
				Family:         &fCopy,
			})
		}
	}
	// Also track families peer supports but we don't
	for f := range remoteFamilies {
		if !localFamilies[f] {
			fCopy := f
			neg.Mismatches = append(neg.Mismatches, Mismatch{
				Code:           CodeMultiprotocol,
				LocalSupported: false,
				PeerSupported:  true,
				Family:         &fCopy,
			})
		}
	}

	// RFC 7911 Section 4: ADD-PATH Capability Negotiation
	// The negotiation of the ADD-PATH capability is asymmetric:
	// - A BGP speaker can send if it advertises Send/Both AND peer advertises Receive/Both
	// - A BGP speaker can receive if it advertises Receive/Both AND peer advertises Send/Both
	if localAddPath != nil && remoteAddPath != nil {
		localModes := make(map[Family]AddPathMode)
		remoteModes := make(map[Family]AddPathMode)

		for _, f := range localAddPath.Families {
			localModes[Family{AFI: f.AFI, SAFI: f.SAFI}] = f.Mode
		}
		for _, f := range remoteAddPath.Families {
			remoteModes[Family{AFI: f.AFI, SAFI: f.SAFI}] = f.Mode
		}

		// RFC 7911 Section 4: For each family, calculate effective mode
		// based on the intersection of local and remote Send/Receive flags.
		for f := range neg.families {
			lm := localModes[f]
			rm := remoteModes[f]

			var mode AddPathMode
			// RFC 7911 Section 4: I can send if I want to send AND peer can receive
			canSend := (lm == AddPathSend || lm == AddPathBoth) &&
				(rm == AddPathReceive || rm == AddPathBoth)
			// RFC 7911 Section 4: I can receive if I want to receive AND peer can send
			canReceive := (lm == AddPathReceive || lm == AddPathBoth) &&
				(rm == AddPathSend || rm == AddPathBoth)

			switch {
			case canSend && canReceive:
				mode = AddPathBoth
			case canSend:
				mode = AddPathSend
			case canReceive:
				mode = AddPathReceive
			}

			if mode != AddPathNone {
				neg.addPath[f] = mode
			}
		}
	}

	// RFC 8950 Section 4: Extended Next Hop Encoding capability negotiation.
	// A tuple is negotiated only if both peers advertise the same (NLRI AFI, NLRI SAFI, NH AFI).
	if localExtNH != nil && remoteExtNH != nil {
		// Build lookup from local capabilities
		localNHMap := make(map[Family]AFI)
		for _, f := range localExtNH.Families {
			localNHMap[Family{AFI: f.NLRIAFI, SAFI: f.NLRISAFI}] = f.NextHopAFI
		}

		// Find intersection with remote capabilities
		for _, f := range remoteExtNH.Families {
			key := Family{AFI: f.NLRIAFI, SAFI: f.NLRISAFI}
			if localNHMap[key] == f.NextHopAFI {
				neg.extendedNextHop[key] = f.NextHopAFI
			}
		}
	}

	// Build composite sub-components
	neg.buildSubComponents()

	return neg
}

// buildSubComponents creates Identity, Encoding, and Session from negotiated data.
func (n *Negotiated) buildSubComponents() {
	// Build sorted family slice for Encoding
	families := make([]Family, 0, len(n.families))
	for f := range n.families {
		families = append(families, f)
	}
	sort.Slice(families, func(i, j int) bool {
		if families[i].AFI != families[j].AFI {
			return families[i].AFI < families[j].AFI
		}
		return families[i].SAFI < families[j].SAFI
	})

	// Create Identity
	n.Identity = &PeerIdentity{
		LocalASN: n.LocalASN,
		PeerASN:  n.PeerASN,
		// Router IDs will be set separately when available
	}

	// Create Encoding (copy maps to avoid aliasing)
	addPathCopy := make(map[Family]AddPathMode, len(n.addPath))
	maps.Copy(addPathCopy, n.addPath)
	extNHCopy := make(map[Family]AFI, len(n.extendedNextHop))
	maps.Copy(extNHCopy, n.extendedNextHop)
	n.Encoding = &EncodingCaps{
		ASN4:            n.ASN4,
		ExtendedMessage: n.ExtendedMessage, // RFC 8654: affects wire encoding (max message size)
		Families:        families,
		AddPathMode:     addPathCopy,
		ExtendedNextHop: extNHCopy,
	}

	// Create Session
	n.Session = &SessionCaps{
		RouteRefresh:         n.RouteRefresh,
		EnhancedRouteRefresh: n.EnhancedRouteRefresh,
		HoldTime:             n.HoldTime,
		GracefulRestart:      n.GracefulRestart,
		Mismatches:           n.Mismatches,
	}
}

// SupportsFamily returns true if the given family was negotiated.
// RFC 4760 Section 8: A family is supported only if both peers advertise it.
func (n *Negotiated) SupportsFamily(f Family) bool {
	return n.families[f]
}

// AddPathMode returns the negotiated ADD-PATH mode for a family.
// RFC 7911 Section 4: Returns the effective mode after asymmetric negotiation.
func (n *Negotiated) AddPathMode(f Family) AddPathMode {
	return n.addPath[f]
}

// ExtendedNextHopAFI returns the negotiated next-hop AFI for a family.
// RFC 8950 Section 4: Returns the next-hop AFI if extended next-hop is negotiated.
// Returns 0 if extended next-hop is not negotiated for this family.
// When non-zero, the family can use next-hops of the returned AFI via MP_REACH_NLRI.
func (n *Negotiated) ExtendedNextHopAFI(f Family) AFI {
	return n.extendedNextHop[f]
}

// Families returns a slice of all negotiated families.
// RFC 4760: Returns the intersection of local and remote multiprotocol capabilities.
func (n *Negotiated) Families() []Family {
	if n.familySlice == nil {
		n.familySlice = make([]Family, 0, len(n.families))
		for f := range n.families {
			n.familySlice = append(n.familySlice, f)
		}
	}
	return n.familySlice
}

// CheckRequiredCodes returns non-family capability codes that were required but not negotiated.
// Returns nil if all required codes are present in the negotiated result.
// RFC 5492 Section 3: Required capabilities must be supported by both peers.
func (n *Negotiated) CheckRequiredCodes(required []Code) []Code {
	if len(required) == 0 {
		return nil
	}

	// Map negotiated boolean flags to capability codes.
	// Maintenance: when adding a new negotiated capability, add an entry here.
	// Codes absent from this map default to false (fail-closed: reported as missing).
	negotiated := map[Code]bool{
		CodeASN4:            n.ASN4,
		CodeExtendedMessage: n.ExtendedMessage,
		CodeRouteRefresh:    n.RouteRefresh,
		CodeAddPath:         len(n.addPath) > 0,
		CodeExtendedNextHop: len(n.extendedNextHop) > 0,
		CodeGracefulRestart: n.GracefulRestart != nil,
	}

	var missing []Code
	for _, code := range required {
		if !negotiated[code] {
			missing = append(missing, code)
		}
	}
	return missing
}

// CheckRefusedCodes returns capability codes that were refused but present in peer's OPEN.
// Unlike CheckRequiredCodes, this checks against the peer's raw advertised capabilities,
// not the negotiated intersection — because if we refuse and don't advertise, the
// intersection won't contain it even when the peer has it.
// Returns nil if no refused codes were found in peer's capabilities.
func (n *Negotiated) CheckRefusedCodes(refused []Code) []Code {
	if len(refused) == 0 {
		return nil
	}

	var present []Code
	for _, code := range refused {
		if n.peerCodes[code] {
			present = append(present, code)
		}
	}
	return present
}

// CheckRequired returns families that were required but not negotiated.
// Returns nil if all required families were successfully negotiated.
func (n *Negotiated) CheckRequired(required []Family) []Family {
	if len(required) == 0 {
		return nil
	}

	var missing []Family
	for _, f := range required {
		if !n.families[f] {
			missing = append(missing, f)
		}
	}
	return missing
}
