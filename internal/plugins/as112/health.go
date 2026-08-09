// Design: docs/architecture/dns/as112.md -- `ze ... request as112 healthcheck` command (finding M4)
// RFC: rfc/short/rfc7534.md Section 3.3 -- healthcheck ordering: DNS readiness before BGP announcement

package as112

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// errHealthUsage is returned for any args shape other than none or the
// documented "target <ip>" keyword form (yang/ze-as112-cmd.yang's usage
// string).
var errHealthUsage = errors.New("request as112 healthcheck: usage is 'request as112 healthcheck [target <ip>]'")

// healthCheckTimeout bounds the one-shot query the health command and probe
// issue, so a wedged/unreachable target never hangs the caller.
const healthCheckTimeout = 3 * time.Second

// healthCheckQuery is the fixed authoritative query the health command
// issues: an SOA query for a Direct-Delegation zone, which any correctly
// running as112 node answers with NOERROR and the zone SOA in Authority.
const healthCheckQuery = "10.in-addr.arpa."

// runHealthQuery issues one authoritative DNS query against addr and
// reports 0 iff the response is the expected AS112 answer (NOERROR, SOA
// present), non-zero otherwise (unreachable, timeout, wrong/missing
// answer). This is the exit-code contract child 3's healthcheck probe
// depends on (finding M4): dig is not on the gokrazy appliance and `ze
// resolve dns` cannot target a specific server.
func runHealthQuery(ctx context.Context, addr string, timeout time.Duration) int {
	c := &dns.Client{Net: "udp", Timeout: timeout}
	m := new(dns.Msg)
	m.SetQuestion(healthCheckQuery, dns.TypeSOA)

	resp, _, err := c.ExchangeContext(ctx, m, addr)
	if err != nil {
		return 1
	}
	if resp.Rcode != dns.RcodeSuccess {
		return 1
	}
	for _, rr := range resp.Ns {
		if _, ok := rr.(*dns.SOA); ok {
			return 0
		}
	}
	for _, rr := range resp.Answer {
		if _, ok := rr.(*dns.SOA); ok {
			return 0
		}
	}
	return 1
}

// defaultHealthTarget picks the on-box loopback target to query when the
// caller supplies no explicit address. An ipv6-only node never binds
// 127.0.0.1 (serverEndpoints, register.go, only adds it when the family
// isn't ipv6-only), so defaulting to it there would report a healthy node
// as unreachable; ::1 is used instead. Every other family (including "no
// state yet") defaults to 127.0.0.1, matching prior behavior.
func defaultHealthTarget() string {
	if s := loadState(); s != nil && s.cfg.AddressFamily == addressFamilyIPv6Only {
		return net.JoinHostPort("::1", "53")
	}
	return net.JoinHostPort("127.0.0.1", "53")
}

// parseHealthArgs extracts the optional target IP from the dispatcher's raw
// args. The CLI dispatcher does not strip keyword tokens before invoking a
// plugin RPC handler -- "request as112 healthcheck target 1.2.3.4" reaches the handler
// as args=["target","1.2.3.4"], matching every other multi-leaf command
// handler's convention (e.g. diag's tcp-check parses "source"/"timeout"
// keywords itself; internal/component/plugin/server/command_test.go's
// TestDispatcherKeywordExtraction pins this for the "count <value>" case).
// Returns "" (no error) when no target was given, so the caller falls back
// to defaultHealthTarget().
func parseHealthArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) != 2 || args[0] != "target" || args[1] == "" {
		return "", errHealthUsage
	}
	return args[1], nil
}

// handleAS112Health answers the `request as112 healthcheck` RPC: the optional "target
// <ip>" arg names the anycast address to query on port 53 (the
// address-family-appropriate loopback is used when omitted). Returns a
// non-error Response whose Data carries the same exit-code semantics
// runHealthQuery uses, so the CLI dispatcher's process exit code matches
// (finding M4's "shell-friendly exit code" requirement).
func handleAS112Health(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	targetIP, err := parseHealthArgs(args)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, nil //nolint:nilerr // operational error in Response, not a Go error
	}
	target := defaultHealthTarget()
	if targetIP != "" {
		target = net.JoinHostPort(targetIP, "53")
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	code := runHealthQuery(ctx, target, healthCheckTimeout)
	status := plugin.StatusDone
	if code != 0 {
		status = plugin.StatusError
	}
	return &plugin.Response{Status: status, Data: plugin.Map{"healthy": code == 0, "target": target}}, nil
}
