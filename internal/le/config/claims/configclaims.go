// Design: docs/architecture/config/yang-config-design.md -- claim completeness gate
// Related: internal/component/config/claims -- the claim semantics and the audit
// Related: internal/le/yangleafmentions -- the advisory leaf-level companion
//
// Package configclaims fails when a config subtree an operator can write is
// delivered to nobody, and when a declared config root names no schema node.
//
// Delivery is claimed per path. Server.reloadConfig selects plugins whose
// Registration.WantsConfigRoots match the changed paths, and Hub.RouteCommand
// resolves a path to a subsystem through SchemaRegistry.FindHandler. A path
// matched by neither is accepted, stored, and delivered nowhere: reloadConfig
// logs Info "config reload: no affected plugins, updating config" and calls
// SetConfigTree. Nothing else rejects it, because validateContainerEntry
// validates only the keys it already knows.
//
// Both inventories are read LIVE. The blank import of
// internal/component/plugin/all links every registration, so the claim side is
// the registry the daemon uses and the schema side is the loader the daemon
// uses. Neither is written down twice.
//
// Build le with the full feature tag set. A reduced set compiles modules out
// and shrinks the surface this gate can see.

package configclaims

import (
	"fmt"
	"os"

	// Blank import: links every plugin registration and every YANG module glue
	// init(), which is what makes both inventories live rather than listed.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	"github.com/ze-software/ze/internal/component/config/claims"
	schemacli "github.com/ze-software/ze/internal/component/config/schema/cli"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leroot"
)

// name is the word this command is typed as, and the prefix its own messages
// carry. The retired Make target used the ze-config-claims-check spelling.
const name = "config claims"

// Floors, not counts. An enumeration that broke would otherwise report a clean
// tree: 36 top-level config roots and 72 claims on 2026-08-03.
const (
	minRoots  = 25
	minClaims = 50
)

// Audit reads both inventories out of the live registry and answers what the
// claim audit found. The error is a loader or schema failure, which is a broken
// build rather than a finding about the tree.
func Audit() (Report, error) {
	var report Report

	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return report, fmt.Errorf("load embedded YANG modules: %w", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		return report, fmt.Errorf("load registered YANG modules: %w", err)
	}
	if err := loader.Resolve(); err != nil {
		return report, fmt.Errorf("resolve YANG modules: %w", err)
	}

	root, err := claims.SchemaTree(loader)
	if err != nil {
		return report, err
	}

	claimed := claims.FromConfigRoots(pluginregistry.ConfigRootsMap())
	handlers, err := schemacli.ConfigHandlerPaths()
	if err != nil {
		return report, fmt.Errorf("build schema handler paths: %w", err)
	}
	claimed = append(claimed, claims.FromHubHandlers(handlers)...)

	allow, err := claims.Allowlist()
	if err != nil {
		return report, err
	}

	audit := claims.Audit(root, claimed, allow)

	report.Roots = len(root.Children)
	report.Claims = len(claimed)
	report.Allowlisted = audit.Allowlisted
	for _, finding := range audit.Findings {
		report.Findings = append(report.Findings, finding.String())
	}

	report.Findings = append(report.Findings, floorFindings(report.Roots, report.Claims)...)
	return report, nil
}

// floorFindings answers the non-vacuity findings for an inventory that came
// back too small to have checked anything.
//
// Audit reports an empty tree and an empty claim set, but a surface that shrank
// without emptying would pass while guarding little.
func floorFindings(roots, claimed int) []string {
	var findings []string
	var tb textbuf.Buffer
	if roots < minRoots {
		findings = append(findings, tb.Reset().
			Str("unclassifiable: only ").Int(int64(roots)).
			Str(" top-level config roots enumerated (floor ").Int(minRoots).
			Str("): the schema walk is broken, so this gate checked almost nothing").String())
	}
	if claimed < minClaims {
		findings = append(findings, tb.Reset().
			Str("unclassifiable: only ").Int(int64(claimed)).
			Str(" claims enumerated (floor ").Int(minClaims).
			Str("): the plugin registry is not populated, so every root would look unclaimed").String())
	}
	return findings
}

// Answer is the `le config-claims` command. The tree it judges is the linked
// registry rather than a directory, so the command takes no argument at all.
func Answer(args []string) (any, int) {
	if len(args) > 0 {
		return nil, leroot.RefuseArgument(name, args[0])
	}

	report, err := Audit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-claims: %v\n", err) //nolint:errcheck // CLI output
		return nil, 1
	}

	if code := verdict(report); code != 0 {
		// The script printed this verdict on stderr in BOTH renderings, so it
		// is not part of the answer and `| json` still sees it.
		fmt.Fprintln(os.Stderr, "config-claims: FAILED") //nolint:errcheck // CLI output
		return report, code
	}
	return report, 0
}

// verdict answers the exit code a report earns. One finding is enough: a config
// subtree delivered to nobody is accepted and stored in silence by the daemon,
// so the gate is the only thing that says so.
func verdict(report Report) int {
	if len(report.Findings) > 0 {
		return 1
	}
	return 0
}
