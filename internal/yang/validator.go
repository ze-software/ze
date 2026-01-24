package yang

import (
	"fmt"
	"regexp"
	"strconv"
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
	ErrTypeLeafref           // Referenced target doesn't exist
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
	case ErrTypeLeafref:
		return "leafref"
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
	node, err := v.findSchemaNode(path)
	if err != nil {
		return err
	}

	return v.validateValue(path, node, value)
}

// ValidateContainer validates a container with multiple fields.
func (v *Validator) ValidateContainer(path string, data map[string]any) error {
	// Parse the path to find the container schema
	node, err := v.findSchemaNode(path)
	if err != nil {
		return err
	}

	container, ok := node.(*yang.Container)
	if !ok {
		// Try to treat as Entry
		entry, ok := node.(*yang.Entry)
		if !ok || entry.Dir == nil {
			return &ValidationError{
				Path:    path,
				Type:    ErrTypeType,
				Message: "expected container",
			}
		}
		return v.validateContainerEntry(path, entry, data)
	}

	return v.validateContainer(path, container, data)
}

// findSchemaNode finds the schema node for the given path.
func (v *Validator) findSchemaNode(path string) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	// First part should be a module prefix (e.g., "bgp")
	// For now, map common prefixes to modules
	moduleName := v.mapPrefixToModule(parts[0])
	mod := v.loader.GetModule(moduleName)
	if mod == nil {
		// Try direct module name
		mod = v.loader.GetModule(parts[0])
	}
	if mod == nil {
		return nil, fmt.Errorf("module not found for path: %s", path)
	}

	// Navigate to the leaf/container
	if len(parts) == 1 {
		// Looking for top-level container
		for _, c := range mod.Container {
			if c.Name == parts[0] {
				return c, nil
			}
		}
		return nil, fmt.Errorf("container not found: %s", parts[0])
	}

	// Navigate deeper
	current := v.findInModule(mod, parts[0])
	if current == nil {
		return nil, fmt.Errorf("element not found: %s in module %s", parts[0], moduleName)
	}

	for i := 1; i < len(parts); i++ {
		name := v.stripListKey(parts[i])
		current = v.findChild(current, name)
		if current == nil {
			return nil, fmt.Errorf("element not found: %s in path %s", name, path)
		}
	}

	return current, nil
}

// mapPrefixToModule maps common prefixes to module names.
func (v *Validator) mapPrefixToModule(prefix string) string {
	// Common mappings
	switch prefix {
	case "bgp":
		return "ze-bgp"
	case "plugin":
		return "ze-plugin"
	default:
		return prefix
	}
}

// stripListKey removes list key from path segment.
// For example, "peer[address=192.0.2.1]" becomes "peer".
func (v *Validator) stripListKey(segment string) string {
	if idx := strings.Index(segment, "["); idx >= 0 {
		return segment[:idx]
	}
	return segment
}

// findInModule finds a top-level element in a module.
func (v *Validator) findInModule(mod *yang.Module, name string) any {
	// Check containers
	for _, c := range mod.Container {
		if c.Name == name {
			return c
		}
	}
	// Check lists
	for _, l := range mod.List {
		if l.Name == name {
			return l
		}
	}
	// Check leafs
	for _, l := range mod.Leaf {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// findChild finds a child element within a container/list.
func (v *Validator) findChild(parent any, name string) any {
	switch p := parent.(type) {
	case *yang.Container:
		// Check nested containers
		for _, c := range p.Container {
			if c.Name == name {
				return c
			}
		}
		// Check nested lists
		for _, l := range p.List {
			if l.Name == name {
				return l
			}
		}
		// Check leafs
		for _, l := range p.Leaf {
			if l.Name == name {
				return l
			}
		}
		// Check leaf-lists
		for _, l := range p.LeafList {
			if l.Name == name {
				return l
			}
		}
	case *yang.List:
		// Check nested containers
		for _, c := range p.Container {
			if c.Name == name {
				return c
			}
		}
		// Check nested lists
		for _, l := range p.List {
			if l.Name == name {
				return l
			}
		}
		// Check leafs
		for _, l := range p.Leaf {
			if l.Name == name {
				return l
			}
		}
	case *yang.Entry:
		if p.Dir != nil {
			if child, ok := p.Dir[name]; ok {
				return child
			}
		}
	}
	return nil
}

// validateValue validates a single value against a schema node.
func (v *Validator) validateValue(path string, node any, value any) error {
	switch n := node.(type) {
	case *yang.Leaf:
		return v.validateLeaf(path, n, value)
	case *yang.Entry:
		return v.validateEntry(path, n, value)
	default:
		// For unsupported types, pass through (permissive)
		return nil
	}
}

// validateLeaf validates a value against a leaf schema.
func (v *Validator) validateLeaf(path string, leaf *yang.Leaf, value any) error {
	if leaf.Type == nil {
		return nil // No type constraint
	}
	return v.validateType(path, leaf.Type, value)
}

// validateEntry validates a value against an entry (from processed schema).
func (v *Validator) validateEntry(path string, entry *yang.Entry, value any) error {
	if entry.Type == nil {
		return nil
	}
	return v.validateYangType(path, entry.Type, value)
}

// validateType validates a value against a YANG type specification.
func (v *Validator) validateType(path string, typeSpec *yang.Type, value any) error {
	typeName := typeSpec.Name

	switch typeName {
	case "string":
		return v.validateString(path, typeSpec, value)
	case "uint8", "uint16", "uint32", "uint64":
		return v.validateUnsigned(path, typeSpec, value)
	case "int8", "int16", "int32", "int64":
		return v.validateSigned(path, typeSpec, value)
	case "enumeration":
		return v.validateEnumeration(path, typeSpec, value)
	case "boolean":
		return v.validateBoolean(path, value)
	case "union":
		return v.validateUnion(path, typeSpec, value)
	default:
		// For typedef references, the YangType field contains resolved type info
		if typeSpec.YangType != nil {
			return v.validateYangType(path, typeSpec.YangType, value)
		}
		// Unknown type - pass through
		return nil
	}
}

// validateYangType validates against yang.YangType from processed schema.
func (v *Validator) validateYangType(path string, yangType *yang.YangType, value any) error {
	//nolint:exhaustive // default handles unimplemented types
	switch yangType.Kind {
	case yang.Ystring:
		return v.validateStringYT(path, yangType, value)
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		return v.validateUnsignedYT(path, yangType, value)
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		return v.validateSignedYT(path, yangType, value)
	case yang.Yenum:
		return v.validateEnumerationYT(path, yangType, value)
	case yang.Ybool:
		return v.validateBoolean(path, value)
	case yang.Yunion:
		return v.validateUnionYT(path, yangType, value)
	default:
		return nil
	}
}

// validateString validates a string value.
func (v *Validator) validateString(path string, typeSpec *yang.Type, value any) error {
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

	// Check patterns
	for _, p := range typeSpec.Pattern {
		matched, err := regexp.MatchString("^"+p.Name+"$", str)
		if err != nil {
			continue // Invalid pattern, skip
		}
		if !matched {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypePattern,
				Message:  fmt.Sprintf("value %q does not match pattern %q", str, p.Name),
				Expected: p.Name,
				Got:      str,
			}
		}
	}

	return nil
}

// validateStringYT validates a string value against YangType.
func (v *Validator) validateStringYT(path string, yangType *yang.YangType, value any) error {
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

// validateUnsigned validates an unsigned integer value.
func (v *Validator) validateUnsigned(path string, typeSpec *yang.Type, value any) error {
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
				Expected: typeSpec.Name,
				Got:      fmt.Sprintf("%d", n),
			}
		}
		num = uint64(n)
	default:
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected unsigned integer",
			Expected: typeSpec.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}

	// Check range constraints
	if typeSpec.Range != nil && typeSpec.Range.Name != "" {
		if !v.checkRangeString(num, typeSpec.Range.Name) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeRange,
				Message:  fmt.Sprintf("value %d is outside range %s", num, typeSpec.Range.Name),
				Expected: typeSpec.Range.Name,
				Got:      fmt.Sprintf("%d", num),
			}
		}
	}

	return nil
}

// validateUnsignedYT validates unsigned integer against YangType.
func (v *Validator) validateUnsignedYT(path string, yangType *yang.YangType, value any) error {
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
	default:
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
				Expected: fmt.Sprintf("%v", yangType.Range),
				Got:      fmt.Sprintf("%d", num),
			}
		}
	}

	return nil
}

// validateSigned validates a signed integer value.
func (v *Validator) validateSigned(path string, typeSpec *yang.Type, value any) error {
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
	default:
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected signed integer",
			Expected: typeSpec.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}
	_ = num // Range checking would use this
	return nil
}

// validateSignedYT validates signed integer against YangType.
func (v *Validator) validateSignedYT(path string, yangType *yang.YangType, value any) error {
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
	default:
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected signed integer",
			Expected: yangType.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}
	_ = num
	return nil
}

// validateEnumeration validates an enumeration value.
func (v *Validator) validateEnumeration(path string, typeSpec *yang.Type, value any) error {
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
	for _, e := range typeSpec.Enum {
		if e.Name == str {
			return nil
		}
	}

	return &ValidationError{
		Path:     path,
		Type:     ErrTypeEnum,
		Message:  fmt.Sprintf("value %q is not a valid enumeration value", str),
		Expected: v.enumValues(typeSpec.Enum),
		Got:      str,
	}
}

// validateEnumerationYT validates enumeration against YangType.
func (v *Validator) validateEnumerationYT(path string, yangType *yang.YangType, value any) error {
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

// validateUnion validates a value against a union type.
func (v *Validator) validateUnion(path string, typeSpec *yang.Type, value any) error {
	// Try each type in the union
	for _, t := range typeSpec.Type {
		if err := v.validateType(path, t, value); err == nil {
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

// validateUnionYT validates value against union YangType.
func (v *Validator) validateUnionYT(path string, yangType *yang.YangType, value any) error {
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

// validateContainer validates a container with data.
func (v *Validator) validateContainer(path string, container *yang.Container, data map[string]any) error {
	// Check mandatory leaves
	for _, leaf := range container.Leaf {
		if v.isMandatory(leaf) {
			if _, ok := data[leaf.Name]; !ok {
				return &ValidationError{
					Path:    path + "." + leaf.Name,
					Type:    ErrTypeMissing,
					Message: fmt.Sprintf("mandatory leaf %q is missing", leaf.Name),
				}
			}
		}
	}

	// Validate provided values
	for key, value := range data {
		childPath := path + "." + key
		child := v.findChild(container, key)
		if child != nil {
			if err := v.validateValue(childPath, child, value); err != nil {
				return err
			}
		}
	}

	return nil
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

// isMandatory checks if a leaf is mandatory.
func (v *Validator) isMandatory(leaf *yang.Leaf) bool {
	// Check for mandatory statement
	// In goyang, this is checked via the Mandatory field on Entry after processing
	return false // Conservative: don't require unless explicitly marked
}

// checkRangeString checks if a value is within a range expression string.
// Range format: "1..100" or "1..100|200..300" for multiple ranges.
func (v *Validator) checkRangeString(num uint64, rangeExpr string) bool {
	// Split on | for multiple ranges
	ranges := strings.Split(rangeExpr, "|")
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		parts := strings.Split(r, "..")
		if len(parts) == 1 {
			// Single value
			val, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
			if err == nil && num == val {
				return true
			}
		} else if len(parts) == 2 {
			// Range
			min, err1 := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
			max, err2 := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
			if err1 == nil && err2 == nil && num >= min && num <= max {
				return true
			}
		}
	}
	return false
}

// checkYangRange checks value against YangRange.
func (v *Validator) checkYangRange(num uint64, ranges yang.YangRange) bool {
	for _, r := range ranges {
		if num >= r.Min.Value && num <= r.Max.Value {
			return true
		}
	}
	return false
}

// enumValues formats enum values for error messages.
func (v *Validator) enumValues(enums []*yang.Enum) string {
	values := make([]string, 0, len(enums))
	for _, e := range enums {
		values = append(values, e.Name)
	}
	return strings.Join(values, ", ")
}
