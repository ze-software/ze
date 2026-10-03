package capability

import "testing"

// TestRFC7911AddPathNegotiationIsPerFamily compares matching and crossed AFIs
// and SAFIs, with both local send modes and both remote receive modes.
// RFC 7911 Section 5: "For a BGP speaker to be able to send multiple paths to its
// peer, that BGP speaker MUST advertise the ADD-PATH Capability with the
// Send/Receive field set to either 2 or 3, and MUST receive from its peer the
// ADD-PATH Capability with the Send/Receive field set to either 1 or 3, for the
// corresponding <AFI, SAFI>."
// RFC requirement: RFC7911-5-1 positive -- Send or Both enables sending for the matching family when the peer advertises Receive or Both.
// RFC requirement: RFC7911-5-1 negative -- local send capability for one AFI or SAFI cannot enable sending for another family.
// RFC requirement: RFC7911-5-2 positive -- the peer's Receive or Both capability enables sending for its corresponding family.
// RFC requirement: RFC7911-5-2 negative -- the peer's receive capability for a different AFI or SAFI does not enable sending.
func TestRFC7911AddPathNegotiationIsPerFamily(t *testing.T) {
	families := []Family{{AFI: AFIIPv4, SAFI: SAFIUnicast}, {AFI: AFIIPv6, SAFI: SAFIUnicast}, {AFI: AFIIPv4, SAFI: SAFIMulticast}}
	for _, localMode := range []AddPathMode{AddPathSend, AddPathBoth} {
		for _, remoteMode := range []AddPathMode{AddPathReceive, AddPathBoth} {
			for _, localFamily := range families {
				for _, remoteFamily := range families {
					local := []Capability{&AddPath{Families: []AddPathFamily{{AFI: localFamily.AFI, SAFI: localFamily.SAFI, Mode: localMode}}}}
					remote := []Capability{&AddPath{Families: []AddPathFamily{{AFI: remoteFamily.AFI, SAFI: remoteFamily.SAFI, Mode: remoteMode}}}}
					for _, fam := range families {
						local = append(local, &Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
						remote = append(remote, &Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI})
					}
					neg := Negotiate(local, remote, PeerIdentity{LocalASN: 65001, PeerASN: 65002})
					for _, fam := range families {
						got := neg.AddPathMode(fam)
						wantSend := fam == localFamily && fam == remoteFamily
						canSend := got == AddPathSend || got == AddPathBoth
						if canSend != wantSend {
							t.Fatalf("local %v/%v remote %v/%v: family %v mode %v, want send=%v", localFamily, localMode, remoteFamily, remoteMode, fam, got, wantSend)
						}
					}
				}
			}
		}
	}
}
