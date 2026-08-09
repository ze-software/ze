// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- dataplane backend registry
//
// The noop backend accepts every install/remove and reports no state. Test
// infrastructure only: unprivileged control-plane .ci tests select it via
// ze.test.ike.dataplane=noop (internal/component/ike/engine/testport.go) so
// two local daemons can complete IKEv2 negotiation without CAP_NET_ADMIN.
// Production always loads "xfrm", where an EPERM install stays fatal by
// design (a daemon that negotiates SAs but cannot program the kernel would
// silently blackhole traffic -- see engine/child.go isXFRMUnsupported).
// Registered in register.go alongside xfrm and vpp.

package dataplane

import (
	"fmt"
	"net"
)

func newNoopBackend() (Dataplane, error) { return noopDataplane{}, nil }

type noopDataplane struct{}

func (noopDataplane) InstallSA(SAParams) error                         { return nil }
func (noopDataplane) RemoveSA(uint32, net.IP, uint8) error             { return nil }
func (noopDataplane) InstallPolicy(SPParams) error                     { return nil }
func (noopDataplane) RemovePolicy(*net.IPNet, *net.IPNet, SADir) error { return nil }
func (noopDataplane) RemovePolicyParams(SPParams) error                { return nil }
func (noopDataplane) Close() error                                     { return nil }

// ListSAs and ListPolicies REFUSE rather than report an empty dataplane.
//
// The write methods above succeed so an unprivileged .ci can complete an IKEv2
// negotiation. The read methods cannot follow them: this backend installs
// nothing, so "no SAs" is true of its own state and false about the machine, and
// an operator who reaches these commands is asking about the machine. Answering
// a question you cannot answer with a confident empty table is the fail-open
// shape ai/rules/evidence.md bans.
func (noopDataplane) ListSAs(uint32) ([]SAInfo, error) {
	return nil, fmt.Errorf("%w: the noop dataplane installs nothing, so it cannot enumerate the SAD", ErrNotSupported)
}

func (noopDataplane) ListPolicies() ([]PolicyInfo, error) {
	return nil, fmt.Errorf("%w: the noop dataplane installs nothing, so it cannot enumerate the SPD", ErrNotSupported)
}
