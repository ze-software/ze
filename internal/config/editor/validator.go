package editor

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
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
// Uses the existing config parser for syntax/schema validation,
// and adds semantic validation for BGP-specific rules.
type ConfigValidator struct {
	schema *config.Schema
}

// NewConfigValidator creates a new config validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		schema: config.BGPSchema(),
	}
}

// Validate runs all validation levels and returns the result.
func (v *ConfigValidator) Validate(content string) ConfigValidationResult {
	var result ConfigValidationResult

	// Parse with schema - this catches syntax and schema errors
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

	// Run semantic validation on the parsed tree
	// This catches cross-field rules that the parser can't check
	semanticErrs := v.ValidateSemantic(tree)
	result.Errors = append(result.Errors, semanticErrs...)

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

// ValidateSemantic checks semantic rules on the parsed tree.
// These are cross-field validations that the parser can't catch.
//
// Note: Many validations are handled by the parser's schema validation:
// - router-id: TypeIPv4 validates format
// - local-as, peer-as: TypeUint32 validates format
// - peer address: TypeIP validates format
// - Duplicate peers: Parser renames with #N suffix
//
// Semantic validation adds RFC-specific rules beyond type checking.
func (v *ConfigValidator) ValidateSemantic(tree *config.Tree) []ConfigValidationError {
	if tree == nil {
		return nil
	}

	var errs []ConfigValidationError

	// Get BGP container
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil // No BGP config
	}

	// Validate peer-level hold-time per RFC 4271
	// Schema validates TypeUint16 (0-65535), but RFC requires 0 or >= 3
	peers := bgp.GetList("peer")
	for peerAddr, peerTree := range peers {
		if holdTimeStr, ok := peerTree.Get("hold-time"); ok {
			if err := v.validateHoldTime(holdTimeStr); err != nil {
				errs = append(errs, ConfigValidationError{
					Message:  fmt.Sprintf("peer %s: %s", peerAddr, err.Error()),
					Severity: "error",
				})
			}
		}
	}

	return errs
}

// validateHoldTime checks hold-time per RFC 4271.
//
// Per RFC 4271 Section 4.2, the Hold Time MUST be either zero or at
// least three seconds. Values 1 and 2 are invalid.
//
// Note: Schema TypeUint16 already validates range 0-65535 and rejects
// negative values. This function adds the RFC-specific constraint.
func (v *ConfigValidator) validateHoldTime(holdTimeStr string) error {
	holdTime, err := strconv.Atoi(holdTimeStr)
	if err != nil {
		return fmt.Errorf("invalid hold-time '%s': must be a number", holdTimeStr)
	}

	// RFC 4271 Section 4.2: hold time must be 0 or >= 3
	if holdTime == 1 || holdTime == 2 {
		return fmt.Errorf("invalid hold-time %d: must be 0 or >= 3 (RFC 4271)", holdTime)
	}

	return nil
}

// ValidateSyntax is kept for backwards compatibility with existing tests.
// It now uses the parser for validation.
func (v *ConfigValidator) ValidateSyntax(content string) []ConfigValidationError {
	parser := config.NewParser(v.schema)
	_, err := parser.Parse(content)
	if err != nil {
		return []ConfigValidationError{v.parseError(err)}
	}
	return nil
}
