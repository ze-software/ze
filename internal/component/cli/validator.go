// Design: docs/architecture/config/yang-config-design.md — config editor

package cli

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
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
	if err := loader.LoadRegistered(); err != nil {
		return nil, fmt.Errorf("failed to load registered YANG: %w", err)
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
	yangErrs := v.validateWithYANG(tree, content)
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
// Uses recursive ValidateTree for systematic validation of all leaves.
// Template inheritance is resolved before validation - inherited values are merged with peer values.
func (v *ConfigValidator) ValidateWithYANG(tree *config.Tree) []ConfigValidationError {
	return v.validateWithYANG(tree, "")
}

// validateWithYANG validates the parsed tree using YANG constraints.
// When content is provided, errors are mapped to source line numbers.
func (v *ConfigValidator) validateWithYANG(tree *config.Tree, content string) []ConfigValidationError {
	if tree == nil || v.yangValidator == nil {
		return nil
	}

	// Get BGP container
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil // No BGP config
	}

	lines := strings.Split(content, "\n")
	var errs []ConfigValidationError

	// Extract templates for inheritance resolution
	templates := v.extractTemplates(tree)

	// Resolve peer inheritance before validation
	peers := bgp.GetList("peer")
	for peerAddr, peerTree := range peers {
		resolved, inheritErr := v.resolveInheritance(peerTree, templates)
		if inheritErr != nil {
			errs = append(errs, ConfigValidationError{
				Line:     findPeerLine(lines, peerAddr),
				Message:  fmt.Sprintf("peer %s: %s", peerAddr, inheritErr.Error()),
				Severity: "error",
			})
			continue
		}

		// Validate resolved peer data using recursive YANG tree walk.
		// ToMap() produces string leaf values; ValidateTree converts them via YANG types.
		// Skip most mandatory-missing errors during editing (config is incomplete).
		// Keep peer-as mandatory check — a peer without AS is never valid.
		peerMap := resolved.ToMap()
		yangErrs := v.yangValidator.ValidateTree("bgp.peer", peerMap)
		for i := range yangErrs {
			if yangErrs[i].Type == yang.ErrTypeMissing && !strings.HasSuffix(yangErrs[i].Path, ".peer-as") {
				continue
			}
			field := yangLeafName(yangErrs[i].Path)
			errs = append(errs, ConfigValidationError{
				Line:     findErrorLine(lines, peerAddr, field, yangErrs[i].Type),
				Message:  formatPeerError(peerAddr, field, yangErrs[i]),
				Severity: "error",
			})
		}
	}

	// Validate BGP-level fields (value correctness only, not mandatory).
	bgpMap := bgp.ToMap()
	// Remove peer sub-map to avoid re-validating (already done above with templates).
	delete(bgpMap, "peer")
	yangErrs := v.yangValidator.ValidateTree("bgp", bgpMap)
	for i := range yangErrs {
		if yangErrs[i].Type == yang.ErrTypeMissing {
			continue
		}
		field := yangLeafName(yangErrs[i].Path)
		errs = append(errs, ConfigValidationError{
			Line:     findFieldLine(lines, "bgp", field),
			Message:  fmt.Sprintf("%s: %s", field, yangErrs[i].Message),
			Severity: "error",
		})
	}

	return errs
}

// yangLeafName extracts the leaf name from a YANG path (last dot-separated segment).
func yangLeafName(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// formatPeerError formats a YANG validation error for a peer with clear, non-redundant messaging.
func formatPeerError(peerAddr, field string, yerr yang.ValidationError) string {
	if yerr.Type == yang.ErrTypeMissing {
		return fmt.Sprintf("peer %s: missing required field %q", peerAddr, field)
	}
	if yerr.Type == yang.ErrTypeEnum {
		if yerr.Expected != "" {
			return fmt.Sprintf("peer %s: %q must be one of: %s (got %q)", peerAddr, field, yerr.Expected, yerr.Got)
		}
		return fmt.Sprintf("peer %s: %q has invalid value %q", peerAddr, field, yerr.Got)
	}
	return fmt.Sprintf("peer %s: %q — %s", peerAddr, field, yerr.Message)
}

// findErrorLine returns the source line for a YANG error.
// Missing fields highlight the peer header; value errors highlight the field itself.
func findErrorLine(lines []string, peerAddr, field string, errType yang.ErrorType) int {
	if errType == yang.ErrTypeMissing {
		return findPeerLine(lines, peerAddr)
	}
	if line := findFieldInPeer(lines, peerAddr, field); line > 0 {
		return line
	}
	return findPeerLine(lines, peerAddr)
}

// findPeerLine returns the 1-based line number of "peer <addr>" in the config.
func findPeerLine(lines []string, addr string) int {
	needle := "peer " + addr
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	return 0
}

// findFieldInPeer returns the 1-based line number of a field inside a peer block.
func findFieldInPeer(lines []string, addr, field string) int {
	needle := "peer " + addr
	inPeer := false
	depth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inPeer && strings.Contains(trimmed, needle) {
			inPeer = true
			depth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			continue
		}
		if inPeer {
			depth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if depth <= 0 {
				break
			}
			if strings.HasPrefix(trimmed, field+" ") || strings.HasPrefix(trimmed, field+";") {
				return i + 1
			}
		}
	}
	return 0
}

// findFieldLine returns the 1-based line number of a field inside a named block.
func findFieldLine(lines []string, block, field string) int {
	inBlock := false
	depth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock && strings.HasPrefix(trimmed, block+" ") {
			inBlock = true
			depth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			continue
		}
		if inBlock {
			depth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			if depth <= 0 {
				break
			}
			if strings.HasPrefix(trimmed, field+" ") || strings.HasPrefix(trimmed, field+";") {
				return i + 1
			}
		}
	}
	return 0
}

// extractTemplates extracts named templates from the config tree.
// Supports both new syntax (template.bgp.peer with inherit-name) and legacy (template.group).
func (v *ConfigValidator) extractTemplates(tree *config.Tree) map[string]*config.Tree {
	templates := make(map[string]*config.Tree)

	tmpl := tree.GetContainer("template")
	if tmpl == nil {
		return templates
	}

	// New syntax: template { bgp { peer <pattern> { inherit-name <name>; ... } } }
	if bgpTmpl := tmpl.GetContainer("bgp"); bgpTmpl != nil {
		for _, peerTree := range bgpTmpl.GetList("peer") {
			if inheritName, hasName := peerTree.Get("inherit-name"); hasName {
				templates[inheritName] = peerTree
			}
		}
	}

	// Legacy syntax: template { group <name> { ... } }
	maps.Copy(templates, tmpl.GetList("group"))

	return templates
}

// resolveInheritance merges template values with peer values.
// Template values are applied first, peer values override.
// Returns resolved tree and error if template not found.
func (v *ConfigValidator) resolveInheritance(peerTree *config.Tree, templates map[string]*config.Tree) (*config.Tree, error) {
	// Check if peer uses inheritance
	inheritName, hasInherit := peerTree.Get("inherit")
	if !hasInherit {
		return peerTree, nil // No inheritance, return as-is
	}

	tmpl, found := templates[inheritName]
	if !found {
		return peerTree, fmt.Errorf("template %q not found", inheritName)
	}

	// Create merged tree: start with template, overlay peer values
	merged := config.NewTree()

	// Copy template values first (Values() returns keys, use Get to retrieve values)
	for _, key := range tmpl.Values() {
		if val, ok := tmpl.Get(key); ok {
			merged.Set(key, val)
		}
	}

	// Overlay peer values (these take precedence)
	for _, key := range peerTree.Values() {
		if val, ok := peerTree.Get(key); ok {
			merged.Set(key, val)
		}
	}

	return merged, nil
}

// ValidateSemantic validates semantic constraints on parsed tree.
// Delegates to ValidateWithYANG for YANG-based validation.
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
