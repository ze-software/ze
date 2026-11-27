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
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/loginservice"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// doctorCodeNoLocalFallback names the diagnostic emitted when a remote AAA
	// backend decides who logs in and no local account can answer when it stops
	// working.
	doctorCodeNoLocalFallback = "doctor-aaa-no-local-fallback"

	// CodeDoctorAAAAllAccountsExcluded names the diagnostic emitted when every
	// configured account's login-service set excludes every registered login
	// service.
	CodeDoctorAAAAllAccountsExcluded = "doctor-aaa-all-accounts-excluded"

	configKeyLoginService = "login-service"
	configKeyServer       = "server"
)

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

// aaaAllAccountsExcludedDoctorCheck runs after the config is read to warn when
// every configured local user and remote AAA mapping is excluded from every
// registered login service.
var aaaAllAccountsExcludedDoctorCheck = diagnostic.DoctorCheck{
	Name:         "aaa-all-accounts-excluded",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        735,
	Component:    "aaa",
	Dependencies: []string{"config-loaded"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{CodeDoctorAAAAllAccountsExcluded},
	Check:        checkAAAAllAccountsExcluded,
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
		if len(sub.GetList(configKeyServer)) == 0 {
			continue
		}
		names = append(names, backend.label)
	}
	return names
}

// checkAAAAllAccountsExcluded warns when at least one local user or remote AAA
// mapping is configured and every configured account's login-service set
// excludes every login service registered in this build.
func checkAAAAllAccountsExcluded(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	// refuse-by-default: guards aaaAllAccountsExcludedDoctorCheck -- no parsed *config.Tree means no accounts to inspect.
	if !ok {
		return nil
	}
	if tree == nil {
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

	loginSets := extractLoginSetsFromTree(sys)
	configuredCount := 0
	firstExcludedSet := ""
	recordExcludedSet := func(setName string) {
		if setName == "" {
			return
		}
		if firstExcludedSet == "" || setName < firstExcludedSet {
			firstExcludedSet = setName
		}
	}

	for _, userTree := range auth.GetList("user") {
		configuredCount++
		setName, _ := userTree.Get(configKeyLoginService)
		if loginSetAllowsAnyDoor(setName, loginSets) {
			return nil
		}
		recordExcludedSet(setName)
	}

	if radiusTree := auth.GetContainer("radius"); radiusTree != nil {
		if len(radiusTree.GetList(configKeyServer)) > 0 {
			configuredCount++
			setName, _ := radiusTree.Get(configKeyLoginService)
			if loginSetAllowsAnyDoor(setName, loginSets) {
				return nil
			}
			recordExcludedSet(setName)
		}
	}

	if tacacsTree := auth.GetContainer("tacacs"); tacacsTree != nil {
		if len(tacacsTree.GetList(configKeyServer)) > 0 {
			for _, profileTree := range auth.GetList("tacacs-profile") {
				if len(profileTree.GetSlice("profile")) == 0 {
					continue
				}
				configuredCount++
				setName, _ := profileTree.Get(configKeyLoginService)
				if loginSetAllowsAnyDoor(setName, loginSets) {
					return nil
				}
				recordExcludedSet(setName)
			}
		}
	}

	if configuredCount == 0 {
		return nil
	}

	return []diagnostic.Diagnostic{{
		Code:     CodeDoctorAAAAllAccountsExcluded,
		Severity: diagnostic.SeverityWarning,
		Message:  "every configured account is excluded from every registered login service by system.authorization.login-service, so no configured operator can log in (only an unshadowed stored recovery account or the serial console remains)",
		Fix:      allAccountsExcludedFix(firstExcludedSet),
	}}
}

// extractLoginSetsFromTree reads system/authorization/login-service entries
// directly from the parsed config tree without importing config/infra.
func extractLoginSetsFromTree(sys *config.Tree) map[string][]string {
	authzTree := sys.GetContainer("authorization")
	if authzTree == nil {
		return nil
	}
	entries := authzTree.GetList(configKeyLoginService)
	if len(entries) == 0 {
		return nil
	}
	sets := make(map[string][]string, len(entries))
	for name, entryTree := range entries {
		sets[name] = entryTree.GetSlice("service")
	}
	return sets
}

// loginSetAllowsAnyDoor reports whether setName admits at least one login
// service registered in this build. An absent setName ("") admits every door
// (OD-3 option a). An empty set ([]string{}) or a set containing no registered
// service admits no door.
func loginSetAllowsAnyDoor(setName string, loginSets map[string][]string) bool {
	if setName == "" {
		return true
	}
	allowed, ok := loginSets[setName]
	if !ok {
		return false
	}
	return slices.ContainsFunc(allowed, loginservice.Registered)
}

// allAccountsExcludedFix builds the concrete editor command that restores
// login access for an excluded login-service set.
func allAccountsExcludedFix(setName string) string {
	door := "ssh"
	if !loginservice.Registered(door) {
		if registered := loginservice.Names(); len(registered) > 0 {
			door = registered[0]
		}
	}
	if setName == "" {
		setName = "<set>"
	}
	var tb textbuf.Buffer
	tb.Str("set system authorization login-service ").Str(setName).Str(" service ").Str(door)
	return tb.String()
}
