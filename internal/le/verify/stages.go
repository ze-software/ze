// Design: docs/architecture/testing/verify-freshness-scope.md -- fixed pre-commit stage population
// Package verify orchestrates the native actions that make up the full
// pre-commit verification gate. It does not discover stages at run time: the
// ordered population is part of the gate's contract.
package verify

// Identity names the historical gate and the exact native root/action that owns
// it. Gate keeps diagnostics and parity stable. Command and Args let an
// injected dispatcher invoke the registered action without deriving a verb or
// carrying a second gate switch.
type Identity struct {
	Gate    string   `json:"gate"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Stage is one ordered pre-commit action.
type Stage struct {
	Identity Identity `json:"identity"`
}

// FullStages returns the current ze-precommit-verify native action population
// in execution order. The generated-files aggregate is expanded at its original
// position so every invoked action has one explicit registered identity. The
// returned slice is independent and may be changed by its caller.
func FullStages() []Stage {
	stages := []Stage{
		stage("ze-lint", "verify-lint", "run"),
		stage("ze-tier-check", "tier", "check"),
		stage("ze-rfc-check", "rfc", "check"),
		stage("ze-iface-resolution-check", "iface-resolution"),
		stage("ze-plugin-boundary-check", "plugin-boundary", "check"),
		stage("ze-config-coercion-check", "config-coercion", "check"),
		stage("ze-fs-persistence-check", "fs-persistence", "check"),
		stage("ze-dash-stdio-check", "dash-stdio", "check"),
		stage("ze-port-defaults-check", "port-defaults", "check"),
		stage("ze-config-claims-check", "config-claims"),
		stage("ze-test-sensitivity-check", "test-sensitivity", "check"),
		stage("ze-test-weakened-check", "test-weakened", "check"),
		stage("ze-staticcheck-feature-matrix-check", "staticcheck-feature-matrix", "check"),
		stage("ze-repository-tracked-build-check", "repository-tracked-build", "check"),
		stage("ze-platform-vet", "platform-vet", "darwin", "freebsd"),
		stage("ze-doc-wiring-check", "doc-wiring"),
		stage("ze-doc-verify", "doc-check", "verify"),
		stage("ze-doc-links-check", "doc-check", "links"),
		stage("ze-repository-tree-check", "repository", "tree-check"),
		stage("ze-plugin-imports-check", "plugin-imports", "check"),
		stage("ze-yang-glue-check", "yang-glue", "check"),
		stage("ze-feature-tags-check", "feature-tags", "check"),
		stage("ze-templ-output-check", "doc-check", "templ-output"),
		stage("ze-vendor-web-check", "vendor-web", "check"),
		stage("ze-web-assets-check", "web-assets", "check"),
		stage("ze-doc-index-check", "docs-to-code", "ze-doc-index-check"),
		stage("ze-rules-render-check", "rules", "render-check"),
		stage("ze-rules-index-check", "rules", "index-check"),
		stage("ze-rules-condensed-check", "rules", "condensed-check"),
		stage("ze-rules-lint", "rules", "lint"),
		stage("ze-arch-map-check", "arch-map", "check"),
		stage("ze-discovery-index-check", "discovery-index", "check"),
		stage("ze-test-health-check", "test-health", "check"),
		stage("ze-site-facts-check", "site-facts", "check"),
		stage("ze-vendor-web-check", "vendor-web", "check"),
		stage("ze-htmx-upgrade-check", "htmx-upgrade", "check"),
		stage("ze-evidence-vet", "verify-deps", "evidence-vet"),
		stage("ze-unit-hook-test", "hook-check", "unit"),
		stage("ze-dependency-vulnerability-check", "verify-deps", "vulnerability"),
		stage("ze-unit-test-cached", "verify-deps", "unit-cached"),
		stage("ze-unit-test-race-changed", "verify-deps", "unit-race-changed"),
		stage("ze-alloc-check", "verify-deps", "alloc"),
		stage("ze-functional-test", "functional"),
		stage("ze-functional-exabgp-test", "functional", "exabgp-test"),
	}
	return stages
}

func stage(gate, command string, args ...string) Stage {
	return Stage{Identity: Identity{Gate: gate, Command: command, Args: args}}
}
