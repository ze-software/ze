// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- VPP firewall backend

// Package firewallvpp implements the ze firewall Backend interface on
// top of VPP's binary API via GoVPP. It is registered under the name
// "vpp" alongside the nft backend.
//
// It programs three VPP features. A filter chain becomes ACL rules
// (src/dst prefix, port range, protocol, ICMP type/code, TCP flags,
// permit/deny/reflect). A chain of type nat becomes NAT44-ED state
// (snat, dnat, masquerade). A set-mark or limit action in a filter
// chain becomes a classify table session that sets the session
// metadata or binds a policer. The verifier rejects every other
// expression at commit (ai/rationale/exact-or-reject.md).
package firewallvpp

import (
	"log/slog"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/slogutil"
)

var loggerPtr atomic.Pointer[slog.Logger]

func init() { //nolint:gochecknoinits // logger bootstrap only
	loggerPtr.Store(slogutil.Logger("firewall.vpp"))
}
