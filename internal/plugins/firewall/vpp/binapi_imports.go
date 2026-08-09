// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- Vendor pinning

// Anchor file that keeps the GoVPP binapi packages in the module's
// import graph on EVERY platform, so `go mod vendor` retains them under
// `vendor/` regardless of how the rest of the package evolves.

package firewallvpp

import (
	_ "go.fd.io/govpp/binapi/acl"
	_ "go.fd.io/govpp/binapi/acl_types"
	_ "go.fd.io/govpp/binapi/classify"
	_ "go.fd.io/govpp/binapi/nat44_ed"
	_ "go.fd.io/govpp/binapi/nat_types"
)
