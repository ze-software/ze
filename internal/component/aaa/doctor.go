// Design: docs/architecture/aaa-tacacs.md -- the chain, and what a dropped backend costs
// Detail: types.go -- backendRegistry.Build, which drops a backend and composes the rest
// Overview: register.go -- registers this check
// Related: ../radius/doctor.go -- the reachability check for one backend

// The aaa component owns the CHAIN, so it owns the readiness question no single
// backend can answer: whether anything is left to authenticate against when a
// remote backend stops working (ai/rules/repo-maintenance.md, "Components that
// are not plugins").
package aaa

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorCodeNoLocalFallback names the one diagnostic this check emits: a remote
// AAA backend decides who logs in, and no local account can answer when it
// stops working.
const doctorCodeNoLocalFallback = "doctor-aaa-no-local-fallback"

// aaaLocalFallbackDoctorCheck runs after the config is read, because it reads
// it. Order 730 puts it just after the RADIUS reachability check at 720, so an
// operator who sees both reads the specific one first.
var aaaLocalFallbackDoctorCheck = diagnostic.DoctorCheck{
	Name:      "aaa-no-local-fallback",
	Phase:     diagnostic.DoctorPhasePostConfig,
	Order:     730,
	Component: "aaa",
	// The dependency this check guards is the REMOTE authentication server, on
	// whichever backend the config declares. Losing it is what makes the local
	// account load-bearing, and this check is about there being one.
	Dependencies: []string{"remote-auth-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{doctorCodeNoLocalFallback},
	Check:        checkAAALocalFallback,
}

// checkAAALocalFallback warns when a remote AAA backend is configured and no
// local user is, so the operator learns on their own box what the chain does
// when that backend stops working.
//
// The chain asks the remote backend first and reaches the local one only when
// the remote fails to ANSWER. That fallback is an ACCOUNT: with no local user
// declared there is nothing to fall back to, and the box has no login at all
// once the remote server is unreachable or its config will not build.
//
// It is a no-op when no remote backend is configured, because a box with local
// accounts alone has nothing to fall back FROM.
func checkAAALocalFallback(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return nil
	}

	remote := configuredRemoteBackends(auth)
	if len(remote) == 0 {
		return nil
	}
	if len(auth.GetList("user")) > 0 {
		return nil
	}

	var tb textbuf.Buffer
	tb.Str("no local user is configured, so ")
	for i, name := range remote {
		if i > 0 {
			tb.Str(" and ")
		}
		tb.Str(name)
	}
	tb.Str(" is the only way in: declare a system.authentication.user for the chain to fall back to")

	return []diagnostic.Diagnostic{{
		Code:     doctorCodeNoLocalFallback,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
	}}
}

// configuredRemoteBackends names every remote authentication backend the config
// declares, in the chain's own priority order.
//
// The names come from the containers the operator wrote, not from the backend
// registry: `ze doctor` runs before the daemon, so a registry answer would
// report what this BINARY carries rather than what this CONFIG asks for.
func configuredRemoteBackends(auth *config.Tree) []string {
	backends := []struct {
		container string
		label     string
	}{
		{container: "radius", label: "RADIUS"},
		{container: "tacacs", label: "TACACS+"},
	}

	var names []string
	for _, backend := range backends {
		sub := auth.GetContainer(backend.container)
		if sub == nil {
			continue
		}
		if len(sub.GetList("server")) == 0 {
			continue
		}
		names = append(names, backend.label)
	}
	return names
}
