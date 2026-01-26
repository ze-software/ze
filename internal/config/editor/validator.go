package editor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
	hubschema "codeberg.org/thomas-mangin/ze/internal/hub/schema"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/yang"
)

// ConfigValidationError represents a single validation error or warning.
type ConfigValidationError struct {
	Line     int    // 1-based line number (0 if unknown)
	Column   int    // 1-based column (0 if unknown)
	Message  string // Human-readable message
	Severity string // "error" or "warning"
}

// ConfigValidationResult contains all validation errors and warnings.
type ConfigValidationResult struct {
	Errors   []ConfigValidationError
	Warnings []ConfigValidationError
}

// HasErrors returns true if there are any errors.
func (r *ConfigValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any warnings.
func (r *ConfigValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// ConfigValidator provides configuration text validation.
// Uses YANG-derived schema for parsing and validation.
type ConfigValidator struct {
	schema        *config.Schema
	yangValidator *yang.Validator
}

// NewConfigValidator creates a new config validator.
// Returns error if YANG schema cannot be loaded.
func NewConfigValidator() (*ConfigValidator, error) {
	schema := config.YANGSchema()
	if schema == nil {
		return nil, fmt.Errorf("failed to load YANG schema")
	}

	// Initialize YANG validator
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("failed to load YANG: %w", err)
	}
	// Load module-specific YANG from their packages
	if err := loader.AddModuleFromText("ze-hub.yang", hubschema.ZeHubYANG); err != nil {
		return nil, fmt.Errorf("failed to load hub YANG: %w", err)
	}
	if err := loader.AddModuleFromText("ze-bgp.yang", bgpschema.ZeBGPYANG); err != nil {
		return nil, fmt.Errorf("failed to load BGP YANG: %w", err)
	}
	if err := loader.Resolve(); err != nil {
		return nil, fmt.Errorf("failed to resolve YANG: %w", err)
	}

	return &ConfigValidator{
		schema:        schema,
		yangValidator: yang.NewValidator(loader),
	}, nil
}

// Validate runs all validation levels and returns the result.
func (v *ConfigValidator) Validate(content string) ConfigValidationResult {
	var result ConfigValidationResult

	// Parse with YANG-derived schema - catches syntax and schema errors
	// Including: router-id (TypeIPv4), local-as (TypeUint32),
	// peer-as (TypeUint32), peer address (TypeIP), hold-time (TypeUint16)
	parser := config.NewParser(v.schema)
	tree, err := parser.Parse(content)
	if err != nil {
		// Parser error - extract line number if available
		result.Errors = append(result.Errors, v.parseError(err))
		// Still try semantic validation on partial parse if possible
		if tree == nil {
			return result
		}
	}

	// Check parser warnings (unknown fields, etc.)
	for _, warn := range parser.Warnings() {
		result.Warnings = append(result.Warnings, ConfigValidationError{
			Message:  warn,
			Severity: "warning",
		})
	}

	// Run YANG validation on the parsed tree
	// This catches RFC-specific constraints from YANG model
	yangErrs := v.ValidateWithYANG(tree)
	result.Errors = append(result.Errors, yangErrs...)

	return result
}

// parseError converts a parser error to ConfigValidationError.
func (v *ConfigValidator) parseError(err error) ConfigValidationError {
	msg := err.Error()

	// Extract line number from "line N: message" format
	var line int
	if strings.HasPrefix(msg, "line ") {
		parts := strings.SplitN(msg, ": ", 2)
		if len(parts) == 2 {
			if n, parseErr := strconv.Atoi(strings.TrimPrefix(parts[0], "line ")); parseErr == nil {
				line = n
				msg = parts[1]
			}
		}
	}

	return ConfigValidationError{
		Line:     line,
		Message:  msg,
		Severity: "error",
	}
}

// ValidateWithYANG validates the parsed tree using YANG constraints.
// YANG model defines RFC-compliant constraints like hold-time range "0 | 3..65535".
func (v *ConfigValidator) ValidateWithYANG(tree *config.Tree) []ConfigValidationError {
	if tree == nil || v.yangValidator == nil {
		return nil
	}

	var errs []ConfigValidationError

	// Get BGP container
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil // No BGP config
	}

	// Validate peer-level hold-time using YANG
	// YANG model: range "0 | 3..65535" (RFC 4271 compliant)
	peers := bgp.GetList("peer")
	for peerAddr, peerTree := range peers {
		if holdTimeStr, ok := peerTree.Get("hold-time"); ok {
			holdTime, parseErr := strconv.Atoi(holdTimeStr)
			if parseErr != nil {
				errs = append(errs, ConfigValidationError{
					Message:  fmt.Sprintf("peer %s: invalid hold-time '%s': must be a number", peerAddr, holdTimeStr),
					Severity: "error",
				})
				continue
			}

			// Validate range before conversion to uint16
			if holdTime < 0 || holdTime > 65535 {
				errs = append(errs, ConfigValidationError{
					Message:  fmt.Sprintf("peer %s: invalid hold-time %d: must be 0-65535", peerAddr, holdTime),
					Severity: "error",
				})
				continue
			}

			// Use YANG validator for RFC-compliant range check
			// #nosec G115 -- bounds checked above
			if err := v.yangValidator.Validate("bgp.peer.hold-time", uint16(holdTime)); err != nil {
				var yangErr *yang.ValidationError
				if errors.As(err, &yangErr) {
					errs = append(errs, ConfigValidationError{
						Message:  fmt.Sprintf("peer %s: invalid hold-time %d: %s", peerAddr, holdTime, yangErr.Message),
						Severity: "error",
					})
				} else {
					errs = append(errs, ConfigValidationError{
						Message:  fmt.Sprintf("peer %s: invalid hold-time %d", peerAddr, holdTime),
						Severity: "error",
					})
				}
			}
		}
	}

	return errs
}

// ValidateSemantic is kept for backwards compatibility.
// Now delegates to ValidateWithYANG.
func (v *ConfigValidator) ValidateSemantic(tree *config.Tree) []ConfigValidationError {
	return v.ValidateWithYANG(tree)
}

// ValidateSyntax validates only syntax using YANG-derived schema.
func (v *ConfigValidator) ValidateSyntax(content string) []ConfigValidationError {
	parser := config.NewParser(v.schema)
	_, err := parser.Parse(content)
	if err != nil {
		return []ConfigValidationError{v.parseError(err)}
	}
	return nil
}
