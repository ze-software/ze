// Design: plan/spec-fw-6-firewall-vpp.md -- Vendor pinning

// Anchor file that keeps the GoVPP ACL binapi packages in the module's
// import graph on EVERY platform, so `go mod vendor` retains them under
// `vendor/` regardless of how the rest of the package evolves.
//
// backend_linux.go carries //go:build linux, so on non-Linux platforms
// its imports are invisible to `go mod vendor`. The unconditional blank
// imports here ensure the ACL packages survive a re-vendor from a
// non-Linux developer machine.

package firewallvpp

import (
	_ "go.fd.io/govpp/binapi/acl"
	_ "go.fd.io/govpp/binapi/acl_types"
)
