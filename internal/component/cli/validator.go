// Design: docs/architecture/config/yang-config-design.md — config editor

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// Severity constants for validation issues.
const (
	severityError   = "error"
	severityWarning = "warning"
)

// Config key constants used in validation.
const (
	keyPeer  = "peer"
	keyGroup = "group"
	keyName  = "name"
)

// ConfigValidationError represents a single validation error or warning.
type ConfigValidationError struct {
	Line     int    // 1-based line number (0 if unknown)
	Column   int    // 1-based column (0 if unknown)
	Message  string // Human-readable message
	Severity string // severityError or severityWarning
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
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
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
// Detects the config format (hierarchical vs set/set-meta) and uses
// the appropriate parser. This is necessary because WorkingContent()
// returns set+meta format when a session is active.
func (v *ConfigValidator) Validate(content string) ConfigValidationResult {
	var result ConfigValidationResult

	format := config.DetectFormat(content)

	var tree *config.Tree
	var parseErr error

	switch format {
	case config.FormatSet, config.FormatSetMeta:
		// Set-format content: use SetParser which handles set/delete commands
		// and metadata prefixes (@timestamp, %session, #user, ^previous).
		sp := config.NewSetParser(v.schema)
		tree, _, parseErr = sp.ParseWithMeta(content)
		if parseErr != nil {
			result.Errors = append(result.Errors, v.parseError(parseErr))
			if tree == nil {
				return result
			}
		}
	case config.FormatHierarchical:
		// Hierarchical format: use standard parser.
		parser := config.NewParser(v.schema)
		tree, parseErr = parser.Parse(content)
		if parseErr != nil {
			result.Errors = append(result.Errors, v.parseError(parseErr))
			if tree == nil {
				return result
			}
		}
		// Check parser warnings (unknown fields, etc.)
		for _, warn := range parser.Warnings() {
			result.Warnings = append(result.Warnings, ConfigValidationError{
				Message:  warn,
				Severity: severityWarning,
			})
		}
	}

	// Run YANG validation on the parsed tree
	// This catches RFC-specific constraints from YANG model
	yangErrs, yangWarns := v.validateWithYANG(tree, content)
	result.Errors = append(result.Errors, yangErrs...)
	result.Warnings = append(result.Warnings, yangWarns...)

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
		Severity: severityError,
	}
}

// ValidateWithYANG validates the parsed tree using YANG constraints.
// Uses recursive ValidateTree for systematic validation of all leaves.
// Group peers inherit group-level fields merged with peer-level fields.
// Returns (errors, warnings). Mandatory-missing fields are warnings, value errors are errors.
func (v *ConfigValidator) ValidateWithYANG(tree *config.Tree) ([]ConfigValidationError, []ConfigValidationError) {
	return v.validateWithYANG(tree, "")
}

// validateWithYANG validates the parsed tree using YANG constraints.
// When content is provided, errors are mapped to source line numbers.
func (v *ConfigValidator) validateWithYANG(tree *config.Tree, content string) ([]ConfigValidationError, []ConfigValidationError) {
	if tree == nil || v.yangValidator == nil {
		return nil, nil
	}

	// Get BGP container
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return nil, nil // No BGP config
	}

	lines := strings.Split(content, "\n")
	var errs, warns []ConfigValidationError

	// Validate standalone peers (directly under bgp)
	peers := bgp.GetList(keyPeer)
	for peerAddr, peerTree := range peers {
		v.validatePeer(peerAddr, peerTree, nil, lines, &errs, &warns)
	}

	// Validate group peers (bgp > group > peer) — merge group defaults with peer values
	groups := bgp.GetList(keyGroup)
	for _, groupTree := range groups {
		groupPeers := groupTree.GetList(keyPeer)
		for peerAddr, peerTree := range groupPeers {
			v.validatePeer(peerAddr, peerTree, groupTree, lines, &errs, &warns)
		}
	}

	// Check for duplicate remote > ip across all peers.
	v.checkDuplicateRemoteIPs(bgp, lines, &errs)

	// Validate BGP-level fields — YANG schema defines which are mandatory.
	bgpMap := bgp.ToMap()
	// Remove peer and group sub-maps to avoid re-validating (already done above).
	delete(bgpMap, keyPeer)
	delete(bgpMap, keyGroup)
	yangErrs := v.yangValidator.ValidateTree("bgp", bgpMap)
	for i := range yangErrs {
		field := yangLeafName(yangErrs[i].Path)
		setPath := strings.ReplaceAll(yangErrs[i].Path, ".", " ")
		var msg string
		if yangErrs[i].Type == yang.ErrTypeMissing {
			msg = fmt.Sprintf("missing required field %q (set %s <value>)", field, setPath)
		} else {
			msg = fmt.Sprintf("%s: %s", field, yangErrs[i].Message)
		}
		severity := severityError
		if yangErrs[i].Type == yang.ErrTypeMissing {
			severity = severityWarning
		}
		ve := ConfigValidationError{
			Line:     findFieldLine(lines, "bgp", field),
			Message:  msg,
			Severity: severity,
		}
		if severity == severityWarning {
			warns = append(warns, ve)
		} else {
			errs = append(errs, ve)
		}
	}

	return errs, warns
}

// validatePeer validates a single peer, optionally merging group defaults.
// When groupTree is non-nil, group-level fields are applied first, then peer-level fields override.
func (v *ConfigValidator) validatePeer(peerAddr string, peerTree, groupTree *config.Tree, lines []string, errs, warns *[]ConfigValidationError) {
	resolved := peerTree
	if groupTree != nil {
		resolved = v.mergeGroupDefaults(peerTree, groupTree)
	}

	// Validate resolved peer data using recursive YANG tree walk.
	// ToMap() produces string leaf values; ValidateTree converts them via YANG types.
	// Mandatory-missing -> warning (don't block editing). Value errors -> error.
	peerMap := resolved.ToMap()
	yangErrs := v.yangValidator.ValidateTree("bgp.peer", peerMap)
	for i := range yangErrs {
		field := yangLeafName(yangErrs[i].Path)
		severity := severityError
		if yangErrs[i].Type == yang.ErrTypeMissing {
			severity = severityWarning
		}
		ve := ConfigValidationError{
			Line:     findErrorLine(lines, peerAddr, field, yangErrs[i].Type),
			Message:  formatPeerError(peerAddr, field, yangErrs[i]),
			Severity: severity,
		}
		if severity == severityWarning {
			*warns = append(*warns, ve)
		} else {
			*errs = append(*errs, ve)
		}
	}

	// Custom check: session > asn > remote is required for peers (not mandatory in YANG because
	// groups can set it at group level, but every peer must have it resolved).
	hasRemoteAS := false
	if sessionContainer := resolved.GetContainer("session"); sessionContainer != nil {
		if asnContainer := sessionContainer.GetContainer("asn"); asnContainer != nil {
			_, hasRemoteAS = asnContainer.Get("remote")
		}
	}
	if !hasRemoteAS {
		*warns = append(*warns, ConfigValidationError{
			Line:     findPeerLine(lines, peerAddr),
			Message:  fmt.Sprintf("peer %s: missing required field \"session asn remote\" (set bgp peer %s session asn remote <value>)", peerAddr, peerAddr),
			Severity: severityWarning,
		})
	}
}

// mergeGroupDefaults creates a merged tree with group defaults applied first,
// then peer values overriding. Merges both leaf values and containers (deep merge).
// Skips "peer" and "name" sub-entries which are group structural elements.
func (v *ConfigValidator) mergeGroupDefaults(peerTree, groupTree *config.Tree) *config.Tree {
	merged := config.NewTree()

	// Copy group-level leaf values first (skip "peer" and "name" sub-entries).
	for _, key := range groupTree.Values() {
		if key == keyPeer || key == keyName {
			continue
		}
		if val, ok := groupTree.Get(key); ok {
			merged.Set(key, val)
		}
	}

	// Copy group-level containers (e.g., capability, family, rib).
	for _, name := range groupTree.ContainerNames() {
		if name == keyPeer {
			continue
		}
		if c := groupTree.GetContainer(name); c != nil {
			merged.SetContainer(name, c)
		}
	}

	// Overlay peer leaf values (these take precedence).
	for _, key := range peerTree.Values() {
		if val, ok := peerTree.Get(key); ok {
			merged.Set(key, val)
		}
	}

	// Overlay peer containers: deep merge when both group and peer have the
	// same container (e.g., group sets capability.GR, peer sets capability.hostname).
	// If only the peer has the container, it replaces any group container.
	for _, name := range peerTree.ContainerNames() {
		peerContainer := peerTree.GetContainer(name)
		if peerContainer == nil {
			continue
		}
		groupContainer := merged.GetContainer(name)
		if groupContainer == nil {
			// No group container to merge with -- just set the peer's.
			merged.SetContainer(name, peerContainer)
			continue
		}
		// Both exist -- deep merge: start with group, overlay peer values.
		mergedContainer := config.NewTree()
		// Copy group container values.
		for _, key := range groupContainer.Values() {
			if val, ok := groupContainer.Get(key); ok {
				mergedContainer.Set(key, val)
			}
		}
		for _, cname := range groupContainer.ContainerNames() {
			if c := groupContainer.GetContainer(cname); c != nil {
				mergedContainer.SetContainer(cname, c)
			}
		}
		// Overlay peer container values.
		for _, key := range peerContainer.Values() {
			if val, ok := peerContainer.Get(key); ok {
				mergedContainer.Set(key, val)
			}
		}
		for _, cname := range peerContainer.ContainerNames() {
			if c := peerContainer.GetContainer(cname); c != nil {
				mergedContainer.SetContainer(cname, c)
			}
		}
		merged.SetContainer(name, mergedContainer)
	}

	return merged
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
		setPath := strings.ReplaceAll(yerr.Path, ".", " ")
		setPath = strings.Replace(setPath, "bgp peer", "bgp peer "+peerAddr, 1)
		return fmt.Sprintf("peer %s: missing required field %q (set %s <value>)", peerAddr, field, setPath)
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
// Only matches direct children (depth 1), not fields nested inside sub-containers.
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
			// Only match at depth 1 (direct children of the block)
			if depth == 1 && (strings.HasPrefix(trimmed, field+" ") || strings.HasPrefix(trimmed, field+";")) {
				return i + 1
			}
		}
	}
	return 0
}

// checkDuplicateRemoteIPs checks that no two peers share the same remote > ip value.
// Collects IPs from both standalone and grouped peers and reports duplicates as errors.
func (v *ConfigValidator) checkDuplicateRemoteIPs(bgp *config.Tree, lines []string, errs *[]ConfigValidationError) {
	seen := make(map[string]string) // remote IP -> first peer name

	checkPeer := func(peerName string, peerTree *config.Tree) {
		connContainer := peerTree.GetContainer("connection")
		if connContainer == nil {
			return
		}
		remoteContainer := connContainer.GetContainer("remote")
		if remoteContainer == nil {
			return
		}
		ip, ok := remoteContainer.Get("ip")
		if !ok || ip == "" {
			return
		}
		if firstPeer, exists := seen[ip]; exists {
			*errs = append(*errs, ConfigValidationError{
				Line:     findPeerLine(lines, peerName),
				Message:  fmt.Sprintf("duplicate remote IP %s in peer %s (already used by peer %s)", ip, peerName, firstPeer),
				Severity: severityError,
			})
			return
		}
		seen[ip] = peerName
	}

	// Standalone peers.
	for peerName, peerTree := range bgp.GetList(keyPeer) {
		checkPeer(peerName, peerTree)
	}

	// Group peers.
	for _, groupTree := range bgp.GetList(keyGroup) {
		for peerName, peerTree := range groupTree.GetList(keyPeer) {
			checkPeer(peerName, peerTree)
		}
	}
}

// ValidateSemantic validates semantic constraints on parsed tree.
// Delegates to ValidateWithYANG for YANG-based validation.
func (v *ConfigValidator) ValidateSemantic(tree *config.Tree) ([]ConfigValidationError, []ConfigValidationError) {
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
