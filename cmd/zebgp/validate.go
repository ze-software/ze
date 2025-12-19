package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/exa-networks/zebgp/pkg/config"
)

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output")
	quiet := fs.Bool("q", false, "quiet mode (exit code only)")
	json := fs.Bool("json", false, "output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: zebgp validate [options] <config-file>

Validate a ZeBGP configuration file.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Exit codes:
  0  Configuration is valid
  1  Configuration has errors
  2  File not found or unreadable

Examples:
  zebgp validate config.conf
  zebgp validate -v config.conf    # verbose output
  zebgp validate -q config.conf    # quiet mode
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

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return 2
	}

	// Parse and validate
	result := validateConfig(string(data), configPath)

	// Output
	if *json {
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
	RouterID        string
	LocalAS         uint32
	Listen          string
	Neighbors       int
	Processes       int
	NeighborDetails []NeighborSummary
}

// NeighborSummary shows neighbor details.
type NeighborSummary struct {
	Address string
	PeerAS  uint32
	Passive bool
}

func validateConfig(input, path string) *ValidationResult {
	result := &ValidationResult{
		Path:  path,
		Valid: true,
	}

	// Parse with schema
	p := config.NewParser(config.BGPSchema())
	tree, err := p.Parse(input)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Line:    extractLine(err.Error()),
			Message: err.Error(),
		})
		return result
	}

	// Convert to typed config
	cfg, err := config.TreeToConfig(tree)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: err.Error(),
		})
		return result
	}

	// Build summary
	result.Config = &ValidationSummary{
		LocalAS:   cfg.LocalAS,
		Listen:    cfg.Listen,
		Neighbors: len(cfg.Neighbors),
		Processes: len(cfg.Processes),
	}

	if cfg.RouterID != 0 {
		result.Config.RouterID = uint32ToIP(cfg.RouterID)
	}

	for _, n := range cfg.Neighbors {
		result.Config.NeighborDetails = append(result.Config.NeighborDetails, NeighborSummary{
			Address: n.Address.String(),
			PeerAS:  n.PeerAS,
			Passive: n.Passive,
		})
	}

	// Semantic validation
	result.Warnings = semanticValidation(cfg)

	return result
}

func semanticValidation(cfg *config.BGPConfig) []ValidationWarning {
	var warnings []ValidationWarning

	// Check for missing router-id
	if cfg.RouterID == 0 {
		warnings = append(warnings, ValidationWarning{
			Message: "router-id not configured (will use system default)",
		})
	}

	// Check for missing local-as
	if cfg.LocalAS == 0 {
		warnings = append(warnings, ValidationWarning{
			Message: "local-as not configured globally",
		})
	}

	// Check each neighbor
	for _, n := range cfg.Neighbors {
		if n.LocalAS == 0 && cfg.LocalAS == 0 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("neighbor %s: local-as not configured", n.Address),
			})
		}
		if n.PeerAS == 0 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("neighbor %s: peer-as not configured", n.Address),
			})
		}
		if n.HoldTime > 0 && n.HoldTime < 3 {
			warnings = append(warnings, ValidationWarning{
				Message: fmt.Sprintf("neighbor %s: hold-time %d too low (minimum 3)", n.Address, n.HoldTime),
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
			fmt.Printf("  neighbors: %d\n", result.Config.Neighbors)
			fmt.Printf("  processes: %d\n", result.Config.Processes)

			if len(result.Config.NeighborDetails) > 0 {
				fmt.Println()
				fmt.Println("Neighbors:")
				for _, n := range result.Config.NeighborDetails {
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
		fmt.Printf(`,"config":{"router_id":%q,"local_as":%d,"neighbors":%d}`,
			result.Config.RouterID, result.Config.LocalAS, result.Config.Neighbors)
	}

	fmt.Println("}")

	if result.Valid {
		return 0
	}
	return 1
}

func extractLine(errMsg string) int {
	// Extract line number from "line N:" format
	if idx := strings.Index(errMsg, "line "); idx >= 0 {
		var line int
		fmt.Sscanf(errMsg[idx:], "line %d", &line)
		return line
	}
	return 0
}

func uint32ToIP(n uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		(n>>24)&0xFF, (n>>16)&0xFF, (n>>8)&0xFF, n&0xFF)
}
