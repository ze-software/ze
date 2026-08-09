//go:build !linux

// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- kernel XFRM drop stats (non-Linux stub).
//
// Kernel IPsec (XFRM) is Linux-only, so there are no drop counters to read on other
// platforms; the OSPFv3 IPsec metric stays at zero there.

package ospf

func readXfrmDropsPlatform() (map[string]uint64, error) { return map[string]uint64{}, nil }
