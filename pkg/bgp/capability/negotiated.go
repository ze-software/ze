package capability

// Negotiated holds the result of capability negotiation between two BGP peers.
type Negotiated struct {
	// Peer identification
	LocalASN uint32
	PeerASN  uint32

	// Negotiated features
	ASN4            bool
	ExtendedMessage bool
	RouteRefresh    bool
	HoldTime        uint16

	// Address families (intersection of local and remote)
	families map[Family]bool

	// ADD-PATH modes per family
	addPath map[Family]AddPathMode

	// Graceful Restart state
	GracefulRestart *GracefulRestart

	// Cached family slice
	familySlice []Family
}

// Negotiate performs capability negotiation between local and remote capabilities.
func Negotiate(local, remote []Capability, localASN, peerASN uint32) *Negotiated {
	neg := &Negotiated{
		LocalASN: localASN,
		PeerASN:  peerASN,
		families: make(map[Family]bool),
		addPath:  make(map[Family]AddPathMode),
	}

	// Build sets for efficient lookup
	localFamilies := make(map[Family]bool)
	remoteFamilies := make(map[Family]bool)
	var localAddPath, remoteAddPath *AddPath
	localASN4 := false
	remoteASN4 := false
	localExtMsg := false
	remoteExtMsg := false
	localRR := false
	remoteRR := false

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
		}
	}

	for _, c := range remote {
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
		case *GracefulRestart:
			neg.GracefulRestart = cap
		}
	}

	// Negotiated features (both must support)
	neg.ASN4 = localASN4 && remoteASN4
	neg.ExtendedMessage = localExtMsg && remoteExtMsg
	neg.RouteRefresh = localRR && remoteRR

	// Intersection of address families
	for f := range localFamilies {
		if remoteFamilies[f] {
			neg.families[f] = true
		}
	}

	// ADD-PATH negotiation
	if localAddPath != nil && remoteAddPath != nil {
		localModes := make(map[Family]AddPathMode)
		remoteModes := make(map[Family]AddPathMode)

		for _, f := range localAddPath.Families {
			localModes[Family{AFI: f.AFI, SAFI: f.SAFI}] = f.Mode
		}
		for _, f := range remoteAddPath.Families {
			remoteModes[Family{AFI: f.AFI, SAFI: f.SAFI}] = f.Mode
		}

		// For each family, calculate effective mode
		for f := range neg.families {
			lm := localModes[f]
			rm := remoteModes[f]

			var mode AddPathMode
			// I can send if I want to send AND peer can receive
			canSend := (lm == AddPathSend || lm == AddPathBoth) &&
				(rm == AddPathReceive || rm == AddPathBoth)
			// I can receive if I want to receive AND peer can send
			canReceive := (lm == AddPathReceive || lm == AddPathBoth) &&
				(rm == AddPathSend || rm == AddPathBoth)

			if canSend && canReceive {
				mode = AddPathBoth
			} else if canSend {
				mode = AddPathSend
			} else if canReceive {
				mode = AddPathReceive
			}

			if mode != AddPathNone {
				neg.addPath[f] = mode
			}
		}
	}

	return neg
}

// SupportsFamily returns true if the given family was negotiated.
func (n *Negotiated) SupportsFamily(f Family) bool {
	return n.families[f]
}

// AddPathMode returns the negotiated ADD-PATH mode for a family.
func (n *Negotiated) AddPathMode(f Family) AddPathMode {
	return n.addPath[f]
}

// Families returns a slice of all negotiated families.
func (n *Negotiated) Families() []Family {
	if n.familySlice == nil {
		n.familySlice = make([]Family, 0, len(n.families))
		for f := range n.families {
			n.familySlice = append(n.familySlice, f)
		}
	}
	return n.familySlice
}
