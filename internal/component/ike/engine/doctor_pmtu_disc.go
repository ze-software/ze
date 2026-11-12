// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- the per-SA options the kernel holds
// Related: doctor.go -- the sibling ike readiness checks and their registration
// Related: doctor_application_override.go -- procRoot, the /proc root this check reads under
// RFC: rfc/short/rfc4301.md -- Section 8.1, the DF treatment configured for each SA

package engine

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// noPMTUDiscKey is the sysctl, under procRoot, that makes the kernel clear the outer
// DF bit of every IPv4 SA Ze installs.
const noPMTUDiscKey = "sys/net/ipv4/ip_no_pmtu_disc"

// checkIPsecNoPMTUDisc reports a df-bit copy peer whose IPv4 tunnel header the kernel
// writes with DF clear.
//
// RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
// implementation's treatment of the DF bit (set, clear, copy from inner header) for
// each SA." Ze installs df-bit copy as a state without XFRM_STATE_NOPMTUDISC, but
// __xfrm_init_state (net/xfrm/xfrm_state.c) adds that flag to every IPv4 state with no
// direction or an outbound one while net.ipv4.ip_no_pmtu_disc is non-zero:
// `if (family == AF_INET && (!x->dir || x->dir == XFRM_SA_DIR_OUT) &&
// READ_ONCE(xs_net(x)->ipv4.sysctl_ip_no_pmtu_disc))`. Ze sets no direction, so the
// treatment the operator configured is replaced in silence, and only `show vpn ipsec
// dataplane sa` shows it. The check runs only for the Linux XFRM dataplane, whose
// states the sysctl reaches.
func checkIPsecNoPMTUDisc(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		// The check cannot read the configuration, so it cannot answer: it says so
		// rather than read as "no df-bit copy peer affected", the convention of the
		// ddos flowspec orphan-record check.
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorIPsecNoPMTUDisc,
			Severity: severityWarning,
			Message:  "vpn ipsec: this doctor run holds no parsed configuration, so it cannot say whether a df-bit copy peer has its IPv4 tunnel header written with DF clear; the command below prints the running configuration, whose df-bit copy peers are affected when `sysctl net.ipv4.ip_no_pmtu_disc` is not 0",
			Fix:      "show config running",
		}}
	}
	// refuse-by-default: guards ipsec.ParseIPsecConfig -- it answers a nil tree with EmptyConfig, which holds no peer for this check to report, and the runner reports a missing config before this phase
	if tree == nil {
		return nil
	}
	cfg, err := ipsec.ParseIPsecConfig(tree)
	// refuse-by-default: guards checkIPsecInterface -- it already reports an unparseable vpn ipsec section, and a second report would read as two faults
	if err != nil {
		return nil
	}
	// refuse-by-default: guards ipv4CopyPeers -- it reads the config's Peers, and ParseIPsecConfig answers a nil config only beside an error
	if cfg == nil {
		return nil
	}
	if ikeDataplaneName() != "xfrm" {
		return nil
	}
	peers := ipv4CopyPeers(cfg)
	if len(peers) == 0 {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(procRoot, noPMTUDiscKey)) //nolint:gosec // procRoot plus a constant key
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("cannot read net.ipv4.ip_no_pmtu_disc (").Str(err.Error()).
			Str("), so Ze cannot tell whether the kernel writes the IPv4 tunnel header of the df-bit copy peers ").
			Join(peers, ", ").
			Str(" with DF clear. Check that /proc is mounted, then run `sysctl net.ipv4.ip_no_pmtu_disc`: any value but 0 clears DF on those SAs.")
		return []rpc.DoctorCheckDiagnostic{{
			Code:     diagnostic.CodeDoctorIPsecNoPMTUDisc,
			Severity: severityWarning,
			Message:  tb.String(),
		}}
	}
	value := strings.TrimSpace(string(raw))
	if value == "0" {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("net.ipv4.ip_no_pmtu_disc is ").Str(value).
		Str(", so the kernel writes the IPv4 tunnel header of every Child SA with DF clear, and the peers ").
		Join(peers, ", ").
		Str(" configured with df-bit copy get clear instead (RFC 4301 Section 8.1). ").
		Str("Fix: `set sysctl setting net.ipv4.ip_no_pmtu_disc value 0` and commit, or set those peers to `df-bit clear` ").
		Str("if clear is what they need. New Child SAs take the change; rekey or restart the tunnels to apply it to the SAs already up.")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecNoPMTUDisc,
		Severity: severityWarning,
		Message:  tb.String(),
	}}
}

// ipv4CopyPeers names, in order, the peers configured with df-bit copy whose tunnel
// header can be IPv4: every peer whose remote address is not an IPv6 address, since
// a peer with no fixed remote address can still come up over IPv4.
func ipv4CopyPeers(cfg *ipsec.IPsecConfig) []string {
	var names []string
	for name := range cfg.Peers {
		peer := cfg.Peers[name]
		if peer.DFBit != dataplane.DFCopy {
			continue
		}
		if addr, err := netip.ParseAddr(peer.RemoteAddress); err == nil && addr.Unmap().Is6() {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
