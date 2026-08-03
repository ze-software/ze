// Design: docs/architecture/config/yang-config-design.md -- claim completeness gate
// Related: internal/component/config/claims -- the claim semantics and the audit
// Related: yang_leaf_mentions.go -- the advisory leaf-level companion
//
// config_claims fails when a config subtree an operator can write is delivered
// to nobody, and when a declared config root names no schema node.
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
// Run it through `make ze-config-claims-check`, never a bare `go run`: the make
// variable carries the full feature tag set, and a reduced one compiles modules
// out and shrinks the surface this checks.
//
// Usage:   go run scripts/checks/config_claims.go [--json]
// Called by: make ze-config-claims-check (a verify stage)
//
//go:build ignore

package main

import (
	"encoding/json"
	"flag"
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
)

// Floors, not counts. An enumeration that broke would otherwise report a clean
// tree: 36 top-level config roots and 72 claims on 2026-08-03.
const (
	minRoots  = 25
	minClaims = 50
)

type output struct {
	Roots       int      `json:"roots"`
	Claims      int      `json:"claims"`
	Allowlisted []string `json:"allowlisted"`
	Findings    []string `json:"findings"`
}

func main() {
	jsonOut := flag.Bool("json", false, "emit the report as JSON")
	flag.Parse()

	out, err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-claims: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(out); encErr != nil {
			fmt.Fprintf(os.Stderr, "config-claims: encode: %v\n", encErr)
			os.Exit(1)
		}
	} else {
		fmt.Fprint(os.Stdout, "# Config Claim Completeness Gate\n\n")
		fmt.Fprintf(os.Stdout, "Top-level config roots: %d\nClaims: %d\nAllowlisted: %d\n\n",
			out.Roots, out.Claims, len(out.Allowlisted))
		for _, p := range out.Allowlisted {
			fmt.Fprintf(os.Stdout, "  allowlisted %s\n", p)
		}
		if len(out.Findings) > 0 {
			fmt.Fprintf(os.Stdout, "\n## Findings (%d)\n\n", len(out.Findings))
			for _, f := range out.Findings {
				fmt.Fprintf(os.Stdout, "  %s\n", f)
			}
		}
	}

	if len(out.Findings) > 0 {
		fmt.Fprintln(os.Stderr, "config-claims: FAILED")
		os.Exit(1)
	}
	if !*jsonOut {
		fmt.Fprintln(os.Stdout, "config-claims: OK")
	}
}

func run() (output, error) {
	var out output

	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return out, fmt.Errorf("load embedded YANG modules: %w", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		return out, fmt.Errorf("load registered YANG modules: %w", err)
	}
	if err := loader.Resolve(); err != nil {
		return out, fmt.Errorf("resolve YANG modules: %w", err)
	}

	root, err := claims.SchemaTree(loader)
	if err != nil {
		return out, err
	}

	cs := claims.FromConfigRoots(pluginregistry.ConfigRootsMap())
	handlers, err := schemacli.ConfigHandlerPaths()
	if err != nil {
		return out, fmt.Errorf("build schema handler paths: %w", err)
	}
	cs = append(cs, claims.FromHubHandlers(handlers)...)

	allow, err := claims.Allowlist()
	if err != nil {
		return out, err
	}

	report := claims.Audit(root, cs, allow)

	out.Roots = len(root.Children)
	out.Claims = len(cs)
	out.Allowlisted = report.Allowlisted
	for _, f := range report.Findings {
		out.Findings = append(out.Findings, f.String())
	}

	// Non-vacuity. Audit reports an empty tree and an empty claim set, but a
	// surface that shrank without emptying would pass while guarding little.
	var tb textbuf.Buffer
	if out.Roots < minRoots {
		out.Findings = append(out.Findings, tb.Reset().
			Str("unclassifiable: only ").Int(int64(out.Roots)).
			Str(" top-level config roots enumerated (floor ").Int(minRoots).
			Str("): the schema walk is broken, so this gate checked almost nothing").String())
	}
	if out.Claims < minClaims {
		out.Findings = append(out.Findings, tb.Reset().
			Str("unclassifiable: only ").Int(int64(out.Claims)).
			Str(" claims enumerated (floor ").Int(minClaims).
			Str("): the plugin registry is not populated, so every root would look unclaimed").String())
	}
	return out, nil
}
