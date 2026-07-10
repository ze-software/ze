// Design: plan/learned/741-ipsec-8-child-xfrm.md -- dataplane backend registry
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

import "net"

func newNoopBackend() (Dataplane, error) { return noopDataplane{}, nil }

type noopDataplane struct{}

func (noopDataplane) InstallSA(SAParams) error                         { return nil }
func (noopDataplane) RemoveSA(uint32, net.IP, uint8) error             { return nil }
func (noopDataplane) InstallPolicy(SPParams) error                     { return nil }
func (noopDataplane) RemovePolicy(*net.IPNet, *net.IPNet, SADir) error { return nil }
func (noopDataplane) RemovePolicyParams(SPParams) error                { return nil }
func (noopDataplane) ListSAs(uint32) ([]SAInfo, error)                 { return nil, nil }
func (noopDataplane) Close() error                                     { return nil }
