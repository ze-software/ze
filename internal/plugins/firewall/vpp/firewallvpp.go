// Design: docs/architecture/core-design.md -- VPP firewall backend

// Package firewallvpp implements the ze firewall Backend interface on
// top of VPP's ACL plugin binary API via GoVPP. It is registered under
// the name "vpp" alongside the nft backend.
//
// Current scope: ACL rules translating ze Match/Action types to VPP
// ACLRule fields (src/dst prefix, port range, protocol, ICMP type/code,
// TCP flags, permit/deny/reflect). Expression types without a faithful
// VPP ACL representation are rejected at commit by the verifier per
// rules/exact-or-reject.md. NAT, classifier, policer, and
// packet-modification actions are deferred to follow-up specs.
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
