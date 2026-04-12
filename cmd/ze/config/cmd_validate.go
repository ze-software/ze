// Design: docs/architecture/config/yang-config-design.md — validation CLI
// Overview: main.go — dispatch and exit codes

package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	bgpconfig "codeberg.org/thomas-mangin/ze/internal/component/bgp/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// validationResult holds validation results.
type validationResult struct {
	Valid    bool
	Path     string
	Errors   []validationError
	Warnings []validationWarning
	Config   *validationSummary
}

// validationError represents a config error.
type validationError struct {
	Line    int
	Message string
}

// validationWarning represents a config warning.
type validationWarning struct {
	Line    int
	Message string
}

// validationSummary shows what was parsed.
type validationSummary struct {
	RouterID    string
	LocalAS     uint32
	Listen      string
	Peers       int
	Plugins     int
	PeerDetails []peerSummary
}

// peerSummary shows peer details.
type peerSummary struct {
	Address string
	PeerAS  uint32
	Connect bool // Initiate outbound connections
	Accept  bool // Accept inbound connections
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("config validate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	quiet := fs.Bool("q", false, "quiet mode (exit code only)")
	jsonOut := fs.Bool("json", false, "output as JSON")
	limit := fs.String("limit", "", "limit validation to section (environment)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze config validate",
			Summary: "Validate a ze configuration file",
			Usage:   []string{"ze config validate [options] <config-file>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "-v", Desc: "Verbose output"},
					{Name: "-q", Desc: "Quiet mode (exit code only)"},
					{Name: "--json", Desc: "Output as JSON"},
					{Name: "--limit <section>", Desc: "Limit validation to section (environment)"},
				}},
				{Title: "Limit values", Entries: []helpfmt.HelpEntry{
					{Name: "environment", Desc: "Validate environment variables only (no config file needed)"},
				}},
				{Title: "Exit codes", Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "Configuration is valid"},
					{Name: "1", Desc: "Configuration has errors"},
					{Name: "2", Desc: "File not found or unreadable"},
				}},
			},
			Examples: []string{
				"ze config validate config.conf",
				"ze config validate -v config.conf       # verbose output",
				"ze config validate -q config.conf       # quiet mode",
				"ze config validate --limit environment  # validate env vars only",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Handle --limit environment (no file needed).
	if *limit == "environment" {
		return validateEnvironment(*jsonOut, *quiet)
	}

	if *limit != "" {
		fmt.Fprintf(os.Stderr, "error: unknown --limit value: %s (valid: environment)\n", *limit)
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: missing config file\n")
		fs.Usage()
		return 1
	}

	configPath := fs.Arg(0)

	// Read file (or stdin if "-").
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

	// Parse and validate.
	result := runValidation(string(data), configPath)

	// Output.
	if *jsonOut {
		return outputValidateJSON(result, *quiet)
	}
	return outputValidateText(result, *verbose, *quiet)
}

func validateEnvironment(jsonOutput, quiet bool) int {
	_, err := config.LoadEnvironment()
	if err != nil {
		if quiet {
			return 1
		}
		if jsonOutput {
			fmt.Printf(`{"valid":false,"error":%q}`+"\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: environment validation failed: %v\n", err)
		}
		return 1
	}

	if !quiet {
		if jsonOutput {
			fmt.Println(`{"valid":true}`)
		} else {
			fmt.Println("Environment variables valid")
		}
	}
	return 0
}

func runValidation(input, path string) *validationResult {
	result := &validationResult{
		Path:  path,
		Valid: true,
	}

	// Parse with YANG-derived schema.
	schema, err := config.YANGSchema()
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, validationError{
			Message: fmt.Sprintf("YANG schema: %v", err),
		})
		return result
	}
	p := config.NewParser(schema)
	tree, err := p.Parse(input)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, validationError{
			Line:    extractLine(err.Error()),
			Message: err.Error(),
		})
		return result
	}

	// Surface parser warnings (e.g., inactive: prefix on leaf nodes).
	for _, w := range p.Warnings() {
		result.Warnings = append(result.Warnings, validationWarning{
			Line:    extractLine(w),
			Message: w,
		})
	}

	// Prune inactive nodes before resolution so the validation summary
	// reflects only active config (inactive peers are not started).
	config.PruneInactive(tree, schema)

	// BGP-specific validation only when bgp {} is present.
	if tree.GetContainer("bgp") != nil {
		// Resolve templates and get BGP tree as map.
		bgpTree, resolveErr := bgpconfig.ResolveBGPTree(tree)
		if resolveErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, validationError{
				Message: resolveErr.Error(),
			})
			return result
		}

		// Build summary from resolved tree.
		result.Config = buildValidationSummary(bgpTree, tree)

		// Semantic validation.
		result.Warnings = semanticValidation(bgpTree)

		// Authorization profile reference validation.
		if authzErr := bgpconfig.ValidateAuthzConfig(tree); authzErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, validationError{
				Message: authzErr.Error(),
			})
			return result
		}

		// Full validation: peer settings, route extraction, and capability constraints.
		if _, peersErr := bgpconfig.PeersFromConfigTree(tree); peersErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, validationError{
				Message: peersErr.Error(),
			})
			return result
		}
	}

	// Hub config validation (secret length, client blocks).
	if tree.GetContainer("plugin") != nil {
		if _, hubErr := config.ExtractHubConfig(tree); hubErr != nil {
			result.Valid = false
			result.Errors = append(result.Errors, validationError{
				Message: hubErr.Error(),
			})
			return result
		}
	}

	// Listener port conflict detection.
	listeners := config.CollectListeners(tree, schema)
	if err := config.ValidateListenerConflicts(listeners); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, validationError{
			Message: err.Error(),
		})
		return result
	}

	return result
}

// buildValidationSummary extracts a validationSummary from the resolved BGP tree.
func buildValidationSummary(bgpTree map[string]any, tree *config.Tree) *validationSummary {
	summary := &validationSummary{}

	if rid, ok := bgpTree["router-id"]; ok {
		summary.RouterID = fmt.Sprint(rid)
	}
	if localMap, ok := bgpTree["local"].(map[string]any); ok {
		summary.LocalAS = treeUint32(localMap["as"])
	}
	if listen, ok := bgpTree["listen"]; ok {
		summary.Listen = fmt.Sprint(listen)
	}

	if peers, ok := bgpTree["peer"].(map[string]any); ok {
		summary.Peers = len(peers)
		for name, v := range peers {
			peer, ok := v.(map[string]any)
			if !ok {
				continue
			}
			connect := true
			if localMap, ok := peer["local"].(map[string]any); ok {
				if v, ok := localMap["connect"]; ok {
					if b, err := config.ParseBoolStrict(fmt.Sprint(v)); err == nil {
						connect = b
					}
				}
			}
			accept := true
			if remoteMap, ok := peer["remote"].(map[string]any); ok {
				if v, ok := remoteMap["accept"]; ok {
					if b, err := config.ParseBoolStrict(fmt.Sprint(v)); err == nil {
						accept = b
					}
				}
			}
			var peerAS uint32
			var addr string
			if remoteMap, ok := peer["remote"].(map[string]any); ok {
				peerAS = treeUint32(remoteMap["as"])
				if ip, ok := remoteMap["ip"]; ok {
					addr = fmt.Sprint(ip)
				}
			}
			_ = name // peer name is the map key
			summary.PeerDetails = append(summary.PeerDetails, peerSummary{
				Address: addr,
				PeerAS:  peerAS,
				Connect: connect,
				Accept:  accept,
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

func semanticValidation(bgpTree map[string]any) []validationWarning {
	var warnings []validationWarning

	// Check for missing router-id.
	if _, ok := bgpTree["router-id"]; !ok {
		warnings = append(warnings, validationWarning{
			Message: "router-id not configured (will use system default)",
		})
	}

	// Check for missing local-as.
	var globalLocalAS uint32
	if localMap, ok := bgpTree["local"].(map[string]any); ok {
		globalLocalAS = treeUint32(localMap["as"])
	}
	if globalLocalAS == 0 {
		warnings = append(warnings, validationWarning{
			Message: "local-as not configured globally",
		})
	}

	// Check each peer.
	peers, _ := bgpTree["peer"].(map[string]any)
	for name, v := range peers {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		var peerLocalAS uint32
		if localMap, ok := peer["local"].(map[string]any); ok {
			peerLocalAS = treeUint32(localMap["as"])
		}
		if peerLocalAS == 0 && globalLocalAS == 0 {
			warnings = append(warnings, validationWarning{
				Message: fmt.Sprintf("peer %s: local-as not configured", name),
			})
		}
		var peerAS uint32
		if remoteMap, ok := peer["remote"].(map[string]any); ok {
			peerAS = treeUint32(remoteMap["as"])
		}
		if peerAS == 0 {
			warnings = append(warnings, validationWarning{
				Message: fmt.Sprintf("peer %s: remote as not configured", name),
			})
		}
		if timerMap, ok := peer["timer"].(map[string]any); ok {
			holdTime := treeUint32(timerMap["receive-hold-time"])
			if holdTime > 0 && holdTime < 3 {
				warnings = append(warnings, validationWarning{
					Message: fmt.Sprintf("peer %s: receive-hold-time %d too low (minimum 3)", name, holdTime),
				})
			}
			sendHoldTime := treeUint32(timerMap["send-hold-time"])
			if sendHoldTime > 0 && sendHoldTime < 480 {
				warnings = append(warnings, validationWarning{
					Message: fmt.Sprintf("peer %s: send-hold-time %d too low (RFC 9687 minimum 480)", name, sendHoldTime),
				})
			}
		}
	}

	return warnings
}

func outputValidateText(result *validationResult, verbose, quiet bool) int {
	if quiet {
		if result.Valid {
			return 0
		}
		return 1
	}

	if result.Valid {
		fmt.Printf("configuration valid: %s\n", result.Path)

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
					var flags []string
					if !n.Connect {
						flags = append(flags, "connect disabled")
					}
					if !n.Accept {
						flags = append(flags, "accept disabled")
					}
					if len(flags) == 0 {
						fmt.Printf("  - %s AS%d\n", n.Address, n.PeerAS)
					} else {
						fmt.Printf("  - %s AS%d (%s)\n", n.Address, n.PeerAS, strings.Join(flags, ", "))
					}
				}
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Println()
			fmt.Println("Warnings:")
			for _, w := range result.Warnings {
				fmt.Printf("  warning: %s\n", w.Message)
			}
		}

		return 0
	}

	fmt.Printf("configuration invalid: %s\n", result.Path)
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

func outputValidateJSON(result *validationResult, quiet bool) int {
	if quiet {
		if result.Valid {
			return 0
		}
		return 1
	}

	// Simple JSON output without encoding/json to keep it minimal.
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
		fmt.Printf(`,"config":{"router-id":%q,"local-as":%d,"peers":%d}`,
			result.Config.RouterID, result.Config.LocalAS, result.Config.Peers)
	}

	fmt.Println("}")

	if result.Valid {
		return 0
	}
	return 1
}

func extractLine(errMsg string) int {
	// Extract line number from "line N:" format.
	idx := strings.Index(errMsg, "line ")
	if idx < 0 {
		return 0
	}
	var line int
	// Best effort extraction - if it fails, return 0.
	if n, err := fmt.Sscanf(errMsg[idx:], "line %d", &line); n != 1 || err != nil {
		return 0
	}
	return line
}
