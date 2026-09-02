// Design: docs/architecture/testing/verify-freshness-scope.md -- fixed pre-commit stage population
// Package verifyengine orchestrates the native actions that make up full
// verification. The ordered population is declared here rather than discovered
// at run time.
package verifyengine

import "strings"

// Identity names one native root/action invocation.
type Identity struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Stage is one ordered pre-commit action.
type Stage struct {
	Identity Identity `json:"identity"`
	// Structural says a red here means the tree is BROKEN rather than merely
	// unverified, so a commit is refused instead of taking a debt row.
	//
	// It lives on the stage because the population lives here. The commit gate
	// used to keep its own list of the same eight names, and a stage renamed
	// or re-argumented in this file silently left that set: its red then filed
	// as unattributed and the commit went through. Two lists of one population
	// drift, and this one drifted toward letting commits past a broken tree.
	Structural bool `json:"structural,omitempty"`
}

// Structural answers the stages whose red refuses a commit outright.
func Structural(mode string) map[string]bool {
	named := map[string]bool{}
	for _, one := range StagesForMode(mode) {
		if one.Structural {
			named[one.Identity.Name] = true
		}
	}
	return named
}

// fullStages returns the native verification actions in execution order.
func fullStages() []Stage {
	stages := []Stage{
		structural("verify lint", "run"),
		structural("tier", "check"),
		stage("rfc", "check"),
		structural("iface-resolution"),
		structural("plugin boundary", "check"),
		stage("config coercion", "check"),
		stage("fs-persistence", "check"),
		stage("dash-stdio", "check"),
		stage("port-defaults", "check"),
		stage("config claims"),
		stage("test-sensitivity", "check"),
		stage("test-weakened", "check"),
		structural("staticcheck-feature-matrix", "check"),
		structural("repository tracked-build", "check"),
		stage("platform-vet", "darwin", "freebsd"),
		structural("doc wiring"),
		stage("doc check", "verify"),
		stage("doc check", "links"),
		stage("repository", "tree-check"),
		stage("plugin imports", "check"),
		stage("yang glue", "check"),
		stage("feature-tags", "check"),
		stage("doc check", "templ-output"),
		stage("vendor-web", "check"),
		stage("web-assets", "check"),
		stage("docs-to-code", "index-check"),
		stage("rules", "render-check"),
		stage("rules", "index-check"),
		stage("rules", "condensed-check"),
		stage("rules", "lint"),
		stage("arch-map", "check"),
		stage("discovery-index", "check"),
		stage("test-health", "check"),
		stage("site facts", "check"),
		stage("htmx-upgrade", "check"),
		structural("verify deps", "evidence-vet"),
		stage("hook-check", "unit"),
		stage("verify deps", "vulnerability"),
		stage("verify deps", "unit-cached"),
		stage("verify deps", "unit-race-changed"),
		stage("verify deps", "alloc"),
		stage("functional", "gating"),
		stage("functional", "exabgp-test"),
	}
	return stages
}

// changedStages returns the cheaper per-edit population. Generated-file checks
// stay expanded into their native actions. Full-only evidence and allocation
// passes are omitted, while lint and unit testing use their changed-tree
// identities.
func changedStages() []Stage {
	full := fullStages()
	stages := make([]Stage, 0, len(full)-3)
	for _, current := range full {
		switch current.Identity.Name {
		case "verify deps/evidence-vet", "verify deps/unit-cached", "verify deps/alloc":
			continue
		default:
			stages = append(stages, current)
		}
	}
	return stages
}

// StagesForMode returns a fresh stage population for a supported certificate
// mode. Unknown modes return nil and therefore cannot certify a tree.
func StagesForMode(mode string) []Stage {
	switch mode {
	case Mode:
		return fullStages()
	case ChangedMode:
		return changedStages()
	default:
		return nil
	}
}

func stage(command string, args ...string) Stage {
	return Stage{Identity: identity(command, args)}
}

// structural is stage for an action whose red says the tree is broken.
func structural(command string, args ...string) Stage {
	return Stage{Identity: identity(command, args), Structural: true}
}

func identity(command string, args []string) Identity {
	name := command
	if len(args) > 0 {
		name += "/" + strings.Join(args, "/")
	}
	return Identity{Name: name, Command: command, Args: args}
}
