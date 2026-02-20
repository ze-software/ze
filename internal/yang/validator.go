// Design: docs/architecture/config/yang-config-design.md — YANG schema handling

package yang

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// ErrorType represents the type of validation error.
type ErrorType int

const (
	ErrTypeUnknown ErrorType = iota
	ErrTypeMissing           // Missing mandatory field
	ErrTypeType              // Wrong type
	ErrTypeRange             // Value outside allowed range
	ErrTypePattern           // String doesn't match pattern
	ErrTypeEnum              // Invalid enum value
	ErrTypeLength            // String length outside allowed range
)

func (e ErrorType) String() string {
	//nolint:exhaustive // default handles unknown
	switch e {
	case ErrTypeMissing:
		return "missing"
	case ErrTypeType:
		return "type"
	case ErrTypeRange:
		return "range"
	case ErrTypePattern:
		return "pattern"
	case ErrTypeEnum:
		return "enum"
	case ErrTypeLength:
		return "length"
	default:
		return "unknown"
	}
}

// ValidationError represents a YANG validation error.
type ValidationError struct {
	Path       string    // Path to the invalid value
	Type       ErrorType // Type of validation error
	Message    string    // Human-readable error message
	Expected   string    // What was expected
	Got        string    // What was provided
	LineNumber int       // Line number in config file (if available)
}

func (e *ValidationError) Error() string {
	if e.LineNumber > 0 {
		return fmt.Sprintf("line %d: %s error at %s: %s", e.LineNumber, e.Type, e.Path, e.Message)
	}
	return fmt.Sprintf("%s error at %s: %s", e.Type, e.Path, e.Message)
}

// Validator validates configuration data against YANG schemas.
type Validator struct {
	loader *Loader
}

// NewValidator creates a new YANG validator.
func NewValidator(loader *Loader) *Validator {
	return &Validator{
		loader: loader,
	}
}

// Validate validates a single value at the given path.
// The path format is "module.container.leaf" or "bgp.peer[address=192.0.2.1].peer-as".
func (v *Validator) Validate(path string, value any) error {
	// Parse the path to find the schema node
	entry, err := v.findSchemaNode(path)
	if err != nil {
		return err
	}

	return v.validateEntry(path, entry, value)
}

// ValidateContainer validates a container with multiple fields.
// Uses the processed entry tree which has mandatory fields properly resolved.
func (v *Validator) ValidateContainer(path string, data map[string]any) error {
	// Parse the path to find the container schema
	entry, err := v.findSchemaNode(path)
	if err != nil {
		return err
	}

	if entry.Dir == nil {
		return &ValidationError{
			Path:    path,
			Type:    ErrTypeType,
			Message: "expected container",
		}
	}

	return v.validateContainerEntry(path, entry, data)
}

// findSchemaNode finds the schema node for the given path.
// It uses the processed entry tree (after Resolve) which has mandatory fields properly set.
func (v *Validator) findSchemaNode(path string) (*yang.Entry, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	// First part should be a module prefix (e.g., "bgp")
	moduleName := v.mapPrefixToModule(parts[0])

	// Get the processed entry tree (has Mandatory properly set)
	entry := v.loader.GetEntry(moduleName)
	if entry == nil {
		entry = v.loader.GetEntry(parts[0])
	}
	if entry == nil {
		return nil, fmt.Errorf("module not found for path: %s", path)
	}

	return v.findInEntry(entry, parts)
}

// findInEntry navigates the entry tree to find a node by path parts.
func (v *Validator) findInEntry(entry *yang.Entry, parts []string) (*yang.Entry, error) {
	// Entry tree has module name as root, look for first part in Dir
	current := entry
	if current.Dir == nil {
		return nil, fmt.Errorf("entry has no children: %s", parts[0])
	}

	// Navigate through each path part
	for _, part := range parts {
		name := v.stripListKey(part)
		child, ok := current.Dir[name]
		if !ok {
			return nil, fmt.Errorf("element not found: %s", name)
		}
		current = child
	}

	return current, nil
}

// mapPrefixToModule maps common prefixes to module names.
func (v *Validator) mapPrefixToModule(prefix string) string {
	return MapPrefixToModule(prefix)
}

// MapPrefixToModule maps common config prefixes to YANG module names.
func MapPrefixToModule(prefix string) string {
	switch prefix {
	case "bgp":
		return "ze-bgp-conf"
	case "plugin":
		return "ze-plugin-conf"
	default: // Pass-through for module names that don't need mapping
		return prefix
	}
}

// stripListKey removes list key from path segment.
// For example, "peer[address=192.0.2.1]" becomes "peer".
func (v *Validator) stripListKey(segment string) string {
	if before, _, ok := strings.Cut(segment, "["); ok {
		return before
	}
	return segment
}

// validateEntry validates a value against an entry (from processed schema).
func (v *Validator) validateEntry(path string, entry *yang.Entry, value any) error {
	if entry.Type == nil {
		return nil
	}
	return v.validateYangType(path, entry.Type, value)
}

// validateYangType validates against yang.YangType from processed schema.
func (v *Validator) validateYangType(path string, yangType *yang.YangType, value any) error {
	//nolint:exhaustive // default handles unimplemented types
	switch yangType.Kind {
	case yang.Ystring:
		return v.validateString(path, yangType, value)
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		return v.validateUnsigned(path, yangType, value)
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		return v.validateSigned(path, yangType, value)
	case yang.Yenum:
		return v.validateEnumeration(path, yangType, value)
	case yang.Ybool:
		return v.validateBoolean(path, value)
	case yang.Yunion:
		return v.validateUnion(path, yangType, value)
	default:
		return nil
	}
}

// validateString validates a string value against YangType.
func (v *Validator) validateString(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected string",
			Expected: "string",
			Got:      fmt.Sprintf("%T", value),
		}
	}

	// Check length constraints
	if len(yangType.Length) > 0 {
		strLen := uint64(len(str))
		if !v.checkYangRange(strLen, yangType.Length) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeLength,
				Message:  fmt.Sprintf("string length %d is outside allowed range", strLen),
				Expected: yangType.Length.String(),
				Got:      fmt.Sprintf("%d", strLen),
			}
		}
	}

	// Check patterns
	for _, p := range yangType.Pattern {
		matched, err := regexp.MatchString("^"+p+"$", str)
		if err != nil {
			continue
		}
		if !matched {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypePattern,
				Message:  fmt.Sprintf("value %q does not match pattern %q", str, p),
				Expected: p,
				Got:      str,
			}
		}
	}

	return nil
}

// validateUnsigned validates unsigned integer against YangType.
func (v *Validator) validateUnsigned(path string, yangType *yang.YangType, value any) error {
	var num uint64
	switch n := value.(type) {
	case uint8:
		num = uint64(n)
	case uint16:
		num = uint64(n)
	case uint32:
		num = uint64(n)
	case uint64:
		num = n
	case int:
		if n < 0 {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  "expected unsigned integer",
				Expected: yangType.Name,
				Got:      fmt.Sprintf("%d", n),
			}
		}
		num = uint64(n)
	case int64:
		if n < 0 {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  "expected unsigned integer",
				Expected: yangType.Name,
				Got:      fmt.Sprintf("%d", n),
			}
		}
		num = uint64(n)
	case float64:
		if n < 0 || n != float64(uint64(n)) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  "expected unsigned integer",
				Expected: yangType.Name,
				Got:      fmt.Sprintf("%v", n),
			}
		}
		num = uint64(n)
	default: // reject unhandled types (string, bool, slice, map, nil)
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected unsigned integer",
			Expected: yangType.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}

	// Check range constraints
	if len(yangType.Range) > 0 {
		if !v.checkYangRange(num, yangType.Range) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeRange,
				Message:  fmt.Sprintf("value %d is outside range", num),
				Expected: yangType.Range.String(),
				Got:      fmt.Sprintf("%d", num),
			}
		}
	}

	return nil
}

// validateSigned validates signed integer against YangType.
func (v *Validator) validateSigned(path string, yangType *yang.YangType, value any) error {
	var num int64
	switch n := value.(type) {
	case int8:
		num = int64(n)
	case int16:
		num = int64(n)
	case int32:
		num = int64(n)
	case int64:
		num = n
	case int:
		num = int64(n)
	case float64:
		if n != float64(int64(n)) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  "expected signed integer",
				Expected: yangType.Name,
				Got:      fmt.Sprintf("%v", n),
			}
		}
		num = int64(n)
	default:
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected signed integer",
			Expected: yangType.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}

	// Check range constraints
	if len(yangType.Range) > 0 {
		if !v.checkYangRangeSigned(num, yangType.Range) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeRange,
				Message:  fmt.Sprintf("value %d is outside range", num),
				Expected: yangType.Range.String(),
				Got:      fmt.Sprintf("%d", num),
			}
		}
	}

	return nil
}

// validateEnumeration validates enumeration against YangType.
func (v *Validator) validateEnumeration(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected string for enumeration",
			Expected: "string",
			Got:      fmt.Sprintf("%T", value),
		}
	}

	// Check if value is in enum list
	if yangType.Enum != nil && yangType.Enum.IsDefined(str) {
		return nil
	}

	var expected string
	if yangType.Enum != nil {
		expected = strings.Join(yangType.Enum.Names(), ", ")
	}

	return &ValidationError{
		Path:     path,
		Type:     ErrTypeEnum,
		Message:  fmt.Sprintf("value %q is not a valid enumeration value", str),
		Expected: expected,
		Got:      str,
	}
}

// validateBoolean validates a boolean value.
func (v *Validator) validateBoolean(path string, value any) error {
	switch val := value.(type) {
	case bool:
		return nil
	case string:
		if val == "true" || val == "false" {
			return nil
		}
	}
	return &ValidationError{
		Path:     path,
		Type:     ErrTypeType,
		Message:  "expected boolean",
		Expected: "boolean",
		Got:      fmt.Sprintf("%T", value),
	}
}

// validateUnion validates value against union YangType.
func (v *Validator) validateUnion(path string, yangType *yang.YangType, value any) error {
	// Try each type in the union
	for _, t := range yangType.Type {
		if err := v.validateYangType(path, t, value); err == nil {
			return nil
		}
	}
	return &ValidationError{
		Path:    path,
		Type:    ErrTypeType,
		Message: "value does not match any type in union",
		Got:     fmt.Sprintf("%v", value),
	}
}

// validateContainerEntry validates a container entry with data.
func (v *Validator) validateContainerEntry(path string, entry *yang.Entry, data map[string]any) error {
	// Check mandatory children
	for name, child := range entry.Dir {
		if child.Mandatory == yang.TSTrue {
			if _, ok := data[name]; !ok {
				return &ValidationError{
					Path:    path + "." + name,
					Type:    ErrTypeMissing,
					Message: fmt.Sprintf("mandatory field %q is missing", name),
				}
			}
		}
	}

	// Validate provided values
	for key, value := range data {
		childPath := path + "." + key
		if child, ok := entry.Dir[key]; ok {
			if err := v.validateEntry(childPath, child, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkYangRange checks unsigned value against YangRange.
func (v *Validator) checkYangRange(num uint64, ranges yang.YangRange) bool {
	for _, r := range ranges {
		if num >= r.Min.Value && num <= r.Max.Value {
			return true
		}
	}
	return false
}

// checkYangRangeSigned checks signed value against YangRange.
func (v *Validator) checkYangRangeSigned(num int64, ranges yang.YangRange) bool {
	for _, r := range ranges {
		// YangRange stores values as uint64 bit patterns.
		// For signed types, reinterpret as int64 (two's complement).
		// #nosec G115 -- intentional bit reinterpretation for signed range check
		min := int64(r.Min.Value)
		// #nosec G115 -- intentional bit reinterpretation for signed range check
		max := int64(r.Max.Value)
		if num >= min && num <= max {
			return true
		}
	}
	return false
}
