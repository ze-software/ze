// Design: docs/guide/authentication.md -- the management-listener exposure rule the daemon, `ze config validate` and `ze doctor` share
// Related: listen_error.go -- the other listener refusal this package words for the operator

package network

import (
	"net"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ListenAddrIsLoopback reports whether a listen address ("host:port" or a bare
// host) binds only to a loopback address: 127.0.0.0/8 or ::1.
//
// It is THE loopback rule for every management listener: the boot guard
// (cmd/ze/hub mgmt_guard.go), the offline config checks
// (internal/component/config) and gNMI's own Serve refusal all call it, so
// "reachable from off-box" has one definition.
//
// Fail-closed: a host that does not parse as an IP address (`localhost`, any
// DNS name, an empty host, garbage) and an unspecified address (0.0.0.0, ::)
// answer false, so an operator cannot reach the network past a guard through a
// name or a wildcard bind.
func ListenAddrIsLoopback(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip, err := netip.ParseAddr(host)
	// refuse-by-default: guards Exposure.breaches -- a host that is no IP literal cannot be shown loopback-only, so false makes every guard refuse it
	if err != nil {
		return false
	}
	return ip.IsLoopback()
}

// Exposure is one management listener's inputs to the exposure rule: where it
// binds, whether every request is authenticated, whether TLS is required off
// loopback and configured, and the fix the operator runs for each refusal.
// A fix never carries a secret.
type Exposure struct {
	Service       string   // human name for the refusal, e.g. "gNMI"
	Addrs         []string // resolved listen addresses ("host:port" or bare host)
	Authenticated bool     // a token, users or a real auth mode gates every request
	AuthFix       string   // the command that adds authentication
	RequireTLS    bool     // a non-loopback listener MUST also serve TLS
	TLSConfigured bool     // TLS material is configured for the listener
	TLSFix        string   // the command that configures TLS
}

// ExposureBreach pairs one refused listen address (empty when no address was
// resolved) with the operator error explaining what protection it lacks.
type ExposureBreach struct {
	Addr    string
	Refusal *opererr.Error
}

// ExposurePlan is the per-address verdict of the management-listener exposure
// rule: Allowed holds the addresses that may bind (in their declared order),
// and Breaches holds one entry per missing protection on each refused address
// (or on an empty address list when protection is required).
type ExposurePlan struct {
	Allowed  []string
	Breaches []ExposureBreach
}

// The why texts are shared by every surface that applies the rule, so the boot
// refusal, `ze config validate` and `ze doctor` say the same thing.
const (
	exposureAuthWhy = "a management listener reachable from the network requires authentication, " +
		"or anyone who reaches it can use it; a listener on 127.0.0.1 or ::1 alone needs none"
	exposureTLSWhy = "a management listener reachable from the network requires TLS, " +
		"or its credentials and configuration cross the network in clear text; " +
		"a listener on 127.0.0.1 or ::1 alone needs none"
	exposureNoAddrWhy = "the listener address could not be verified as loopback-only"
	exposureNoAddrFix = "resolve the listener address before declaring it, or bind explicitly to 127.0.0.1 or ::1"
	exposureLoopFix   = "bind the listener to 127.0.0.1 or ::1 only"
)

// Plan applies the management-listener exposure rule per address:
//
//   - when the listener is authenticated and satisfies RequireTLS, every
//     declared address is allowed and Breaches is empty;
//   - otherwise, loopback addresses stay in Allowed in their declared order,
//     each non-loopback address gets one breach per missing protection
//     (authentication first, then TLS), and an empty address list gets one
//     breach per missing protection with Addr == "" (fail-closed,
//     ai/rules/evidence.md).
func (e Exposure) Plan() ExposurePlan {
	missingAuth := !e.Authenticated
	missingTLS := e.RequireTLS && !e.TLSConfigured
	if !missingAuth && !missingTLS {
		return ExposurePlan{Allowed: slices.Clone(e.Addrs)}
	}
	var plan ExposurePlan
	for _, addr := range e.Addrs {
		if ListenAddrIsLoopback(addr) {
			plan.Allowed = append(plan.Allowed, addr)
		}
	}
	if missingAuth {
		plan.Breaches = append(plan.Breaches, e.breaches("authentication", exposureAuthWhy, e.AuthFix)...)
	}
	if missingTLS {
		plan.Breaches = append(plan.Breaches, e.breaches("TLS", exposureTLSWhy, e.TLSFix)...)
	}
	return plan
}

// Refusals applies the management-listener exposure rule and answers one
// operator error per breach, or none when the listener may serve:
//
//   - a non-loopback address without authentication is refused;
//   - when RequireTLS is set, a non-loopback address without TLS is refused;
//   - a listener that must pass either check and carries no resolved address
//     is refused, because it cannot be shown to be loopback-only (an empty set
//     MUST NOT pass by iterating zero times, ai/rules/evidence.md).
//
// A loopback-only listener passes whatever its authentication and TLS.
func (e Exposure) Refusals() []*opererr.Error {
	plan := e.Plan()
	if len(plan.Breaches) == 0 {
		return nil
	}
	refusals := make([]*opererr.Error, len(plan.Breaches))
	for i, breach := range plan.Breaches {
		refusals[i] = breach.Refusal
	}
	return refusals
}

// breaches answers one breach per non-loopback address for one missing
// protection, named by what ("authentication" or "TLS").
func (e Exposure) breaches(what, why, fix string) []ExposureBreach {
	if len(e.Addrs) == 0 {
		var problem textbuf.Buffer
		problem.Str("refusing to start ").Str(e.Service).Str(": declared without ").Str(what).
			Str(" and with no resolved listen address")
		return []ExposureBreach{{
			Refusal: opererr.New(problem.String(), exposureNoAddrWhy, exposureNoAddrFix),
		}}
	}
	if fix == "" {
		fix = exposureLoopFix
	}
	var breaches []ExposureBreach
	for _, addr := range e.Addrs {
		if ListenAddrIsLoopback(addr) {
			continue
		}
		var problem textbuf.Buffer
		problem.Str("refusing to start ").Str(e.Service).Str(" on non-loopback listener ").Quoted(addr).
			Str(" without ").Str(what)
		breaches = append(breaches, ExposureBreach{
			Addr:    addr,
			Refusal: opererr.New(problem.String(), why, fix),
		})
	}
	return breaches
}
