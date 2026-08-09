//go:build !linux

// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- kernel XFRM readiness probe (non-Linux).
//
// Kernel IPsec (XFRM) is Linux-only. On other platforms the check reports availability so
// config/unit runs do not flag a capability the daemon never uses there (the OSPFv3 raw
// transport is itself Linux-only, mirroring doctor_other.go's raw-socket stub).

package ospf

func xfrmAvailable() bool { return true }
