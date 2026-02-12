// Package validate provides the ze validate subcommand.
package validate

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/reactor"
)

// Run executes the validate subcommand with the given arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	quiet := fs.Bool("q", false, "quiet mode (exit code only)")
	jsonOut := fs.Bool("json", false, "output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze validate [options] <config-file>

Validate a ze configuration file.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Configuration is valid
  1  Configuration has errors
  2  File not found or unreadable

Examples:
  ze validate config.conf
  ze validate -v config.conf    # verbose output
  ze validate -q config.conf    # quiet mode
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Read file (or stdin if "-")
	var data []byte
	var err error
	if configPath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(configPath) //nolint:gosec // Config path from CLI args
	}
	if err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 2
	}

	// Parse and validate
	result := validateConfig(string(data), configPath)

	// Output
	if *jsonOut {
		return outputJSON(result, *quiet)
	}
	return outputText(result, *verbose, *quiet)
}

// ValidationResult holds validation results.
type ValidationResult struct {
	Valid    bool
	Path     string
	Errors   []ValidationError
	Warnings []ValidationWarning
	Config   *ValidationSummary
}

// ValidationError represents a config error.
type ValidationError struct {
	Line    int
	Message string
}

// ValidationWarning represents a config warning.
type ValidationWarning struct {
	Line    int
	Message string
}

// ValidationSummary shows what was parsed.
type ValidationSummary struct {
	RouterID    string
	LocalAS     uint32
	Listen      string
	Peers       int
	Plugins     int
	PeerDetails []PeerSummary
}

// PeerSummary shows peer details.
type PeerSummary struct {
	Address string
	PeerAS  uint32
	Passive bool
}

func validateConfig(input, path string) *ValidationResult {
	result := &ValidationResult{
		Path:  path,
		Valid: true,
	}

	// Parse with YANG-derived schema
	schema := config.YANGSchema()
	if schema == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: "failed to load YANG schema",
		})
		return result
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Line:    extractLine(err.Error()),
			Message: err.Error(),
		})
		return result
	}

	// Resolve templates and get BGP tree as map.
	bgpTree, err := config.ResolveBGPTree(tree)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: err.Error(),
		})
		return result
	}

	// Build summary from resolved tree.
	result.Config = buildSummary(bgpTree, tree)

	// Semantic validation.
	result.Warnings = semanticValidation(bgpTree)

	// Validate process/capability constraints (without deep route parsing).
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: err.Error(),
		})
		return result
	}
	if err := config.ValidatePeerProcessCaps(peers); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: err.Error(),
		})
		return result
	}

	return result
}

// buildSummary extracts a ValidationSummary from the resolved BGP tree.
func buildSummary(bgpTree map[string]any, tree *config.Tree) *ValidationSummary {
	summary := &ValidationSummary{}

	if rid, ok := bgpTree["router-id"]; ok {
		summary.RouterID = fmt.Sprint(rid)
	}
	summary.LocalAS = treeUint32(bgpTree["local-as"])
	if listen, ok := bgpTree["listen"]; ok {
		summary.Listen = fmt.Sprint(listen)
	}

	if peers, ok := bgpTree["peer"].(map[string]any); ok {
		summary.Peers = len(peers)
		for addr, v := range peers {
			peer, ok := v.(map[string]any)
			if !ok {
				continue
			}
			summary.PeerDetails = append(summary.PeerDetails, PeerSummary{
				Address: addr,
				PeerAS:  treeUint32(peer["peer-as"]),
				Passive: fmt.Sprint(peer["passive"]) == "true",
			})
		}
	}

	if pluginContainer := tree.GetContainer("plugin"); pluginContainer != nil {
		summary.Plugins = len(pluginContainer.GetList("external"))
	}

	return summary
}

// treeUint32 parses a tree value (string) as uint32. Returns 0 on nil or error.
func treeUint32(v any) uint32 {
	if v == nil {
		return 0
	}
	n, err := strconv.ParseUint(fmt.Sprint(v), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n) //nolint:gosec // Validated by ParseUint with bitSize 32
}

func semanticValidation(bgpTree map[string]any) []ValidationWarning {
	var warnings []ValidationWarning

	// Check for missing router-id.
	if _, ok := bgpTree["router-id"]; !ok {
		warnings = append(warnings, ValidationWarning{
			Message: "router-id not configured (will use system default)",
		})
	}

	// Check for missing local-as.
	globalLocalAS := treeUint32(bgpTree["local-as"])
	if globalLocalAS == 0 {
		warnings = append(warnings, ValidationWarning{
			Message: "local-as not configured globally",
		})
	}

	// Check each peer.
	peers, _ := bgpTree["peer"].(map[string]any)
	for addr, v := range peers {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if treeUint32(peer["local-as"]) == 0 && globalLocalAS == 0 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("peer %s: local-as not configured", addr),
			})
		}
		if treeUint32(peer["peer-as"]) == 0 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("peer %s: peer-as not configured", addr),
			})
		}
		holdTime := treeUint32(peer["hold-time"])
		if holdTime > 0 && holdTime < 3 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("peer %s: hold-time %d too low (minimum 3)", addr, holdTime),
			})
		}
	}

	return warnings
}

func outputText(result *ValidationResult, verbose, quiet bool) int {
	if quiet {
		if result.Valid {
			return 0
		}
		return 1
	}

	if result.Valid {
		fmt.Printf("✓ %s: configuration valid\n", result.Path)

		if verbose && result.Config != nil {
			fmt.Println()
			fmt.Println("Configuration summary:")
			if result.Config.RouterID != "" {
				fmt.Printf("  router-id: %s\n", result.Config.RouterID)
			}
			if result.Config.LocalAS != 0 {
				fmt.Printf("  local-as:  %d\n", result.Config.LocalAS)
			}
			if result.Config.Listen != "" {
				fmt.Printf("  listen:    %s\n", result.Config.Listen)
			}
			fmt.Printf("  peers: %d\n", result.Config.Peers)
			fmt.Printf("  plugins: %d\n", result.Config.Plugins)

			if len(result.Config.PeerDetails) > 0 {
				fmt.Println()
				fmt.Println("Peers:")
				for _, n := range result.Config.PeerDetails {
					mode := "active"
					if n.Passive {
						mode = "passive"
					}
					fmt.Printf("  - %s AS%d (%s)\n", n.Address, n.PeerAS, mode)
				}
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Println()
			fmt.Println("Warnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  ⚠ %s\n", w.Message)
			}
		}

		return 0
	}

	fmt.Printf("✗ %s: configuration invalid\n", result.Path)
	fmt.Println()
	fmt.Println("Errors:")
	for _, e := range result.Errors {
		if e.Line > 0 {
			fmt.Printf("  line %d: %s\n", e.Line, e.Message)
		} else {
			fmt.Printf("  %s\n", e.Message)
		}
	}
	return 1
}

func outputJSON(result *ValidationResult, quiet bool) int {
	if quiet {
		if result.Valid {
			return 0
		}
		return 1
	}

	// Simple JSON output without encoding/json to keep it minimal
	fmt.Printf(`{"valid":%t,"path":%q`, result.Valid, result.Path)

	if len(result.Errors) > 0 {
		fmt.Printf(`,"errors":[`)
		for i, e := range result.Errors {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf(`{"line":%d,"message":%q}`, e.Line, e.Message)
		}
		fmt.Print("]")
	}

	if len(result.Warnings) > 0 {
		fmt.Printf(`,"warnings":[`)
		for i, w := range result.Warnings {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf(`{"message":%q}`, w.Message)
		}
		fmt.Print("]")
	}

	if result.Config != nil {
		fmt.Printf(`,"config":{"router_id":%q,"local_as":%d,"peers":%d}`,
			result.Config.RouterID, result.Config.LocalAS, result.Config.Peers)
	}

	fmt.Println("}")

	if result.Valid {
		return 0
	}
	return 1
}

func extractLine(errMsg string) int {
	// Extract line number from "line N:" format
	idx := strings.Index(errMsg, "line ")
	if idx < 0 {
		return 0
	}
	var line int
	// Best effort extraction - if it fails, return 0
	if n, err := fmt.Sscanf(errMsg[idx:], "line %d", &line); n != 1 || err != nil {
		return 0
	}
	return line
}
