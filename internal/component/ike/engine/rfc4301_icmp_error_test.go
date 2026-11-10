// VALIDATES: a Child SA hands the RFC 4301 Section 6.2 ICMP error processing to the
// dataplane on the outbound policy only, by default, and the peer's
// `icmp-error-check false` takes it off.
// PREVENTS: the return-SA mapping set on an inbound or forward policy, where the backend
// refuses it and the Child SA never installs, or left off by default.
// RFC naming: untagged -- RFC4301-6.2-3 is a gap while the owner decides its BYPASS condition, so this unit proves the state Ze installs and carries no tag for it.

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// VALIDATES: a Child SA's outbound policy asks the dataplane for the ICMP error return
// mapping unless the peer turned the processing off, and its inbound and forward
// policies never ask.
func TestChildSAOutboundPolicyMapsICMPErrors(t *testing.T) {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	pair := policyPair{local: local, remote: remote}

	if !childPairPolicy(&ChildSA{}, dataplane.SADirOut, pair).ICMPErrorReturn {
		t.Errorf("the outbound policy of a default Child SA does not ask for the ICMP error return mapping")
	}
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirFwd} {
		if childPairPolicy(&ChildSA{}, dir, pair).ICMPErrorReturn {
			t.Errorf("the direction %d policy asks for the return mapping, which the kernel reads on outbound only", dir)
		}
	}
	off := &ChildSA{SkipICMPErrorCheck: true}
	if childPairPolicy(off, dataplane.SADirOut, pair).ICMPErrorReturn {
		t.Errorf("icmp-error-check false still asks for the ICMP error return mapping")
	}
}
