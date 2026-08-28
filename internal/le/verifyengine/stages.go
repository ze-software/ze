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
}

// fullStages returns the native verification actions in execution order.
func fullStages() []Stage {
	stages := []Stage{
		stage("verify-lint", "run"),
		stage("tier", "check"),
		stage("rfc", "check"),
		stage("iface-resolution"),
		stage("plugin-boundary", "check"),
		stage("config-coercion", "check"),
		stage("fs-persistence", "check"),
		stage("dash-stdio", "check"),
		stage("port-defaults", "check"),
		stage("config-claims"),
		stage("test-sensitivity", "check"),
		stage("test-weakened", "check"),
		stage("staticcheck-feature-matrix", "check"),
		stage("repository-tracked-build", "check"),
		stage("platform-vet", "darwin", "freebsd"),
		stage("doc-wiring"),
		stage("doc-check", "verify"),
		stage("doc-check", "links"),
		stage("repository", "tree-check"),
		stage("plugin-imports", "check"),
		stage("yang-glue", "check"),
		stage("feature-tags", "check"),
		stage("doc-check", "templ-output"),
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
		stage("site-facts", "check"),
		stage("htmx-upgrade", "check"),
		stage("verify-deps", "evidence-vet"),
		stage("hook-check", "unit"),
		stage("verify-deps", "vulnerability"),
		stage("verify-deps", "unit-cached"),
		stage("verify-deps", "unit-race-changed"),
		stage("verify-deps", "alloc"),
		stage("functional"),
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
		case "verify-deps/evidence-vet", "verify-deps/unit-cached", "verify-deps/alloc":
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
	name := command
	if len(args) > 0 {
		name += "/" + strings.Join(args, "/")
	}
	return Stage{Identity: Identity{Name: name, Command: command, Args: args}}
}
