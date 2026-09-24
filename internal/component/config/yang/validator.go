// Design: docs/architecture/config/yang-config-design.md — YANG schema handling

package yang

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errEmptyPath = errors.New("empty path")

// ErrorType represents the type of validation error.
type ErrorType int

const (
	ErrTypeUnknown     ErrorType = iota
	ErrTypeMissing               // Missing mandatory field
	ErrTypeType                  // Wrong type
	ErrTypeRange                 // Value outside allowed range
	ErrTypePattern               // String doesn't match pattern
	ErrTypeEnum                  // Invalid enum value
	ErrTypeLength                // String length outside allowed range
	ErrTypeCardinality           // List/leaf-list has too many or too few entries
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
	case ErrTypeCardinality:
		return "cardinality"
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
	var b textbuf.Buffer
	if e.LineNumber > 0 {
		return b.Str("line ").Int(int64(e.LineNumber)).Str(": ").Str(e.Type.String()).Str(" error at ").Str(e.Path).Str(": ").Str(e.Message).String()
	}
	return b.Str(e.Type.String()).Str(" error at ").Str(e.Path).Str(": ").Str(e.Message).String()
}

// Validator validates configuration data against YANG schemas.
type Validator struct {
	loader   *Loader
	registry *ValidatorRegistry
}

// NewValidator creates a new YANG validator.
func NewValidator(loader *Loader) *Validator {
	return &Validator{
		loader: loader,
	}
}

// SetRegistry sets the custom validator registry for ze:validate extensions.
func (v *Validator) SetRegistry(reg *ValidatorRegistry) {
	v.registry = reg
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
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return nil, errEmptyPath
	}

	// The first part names a top-level config section, and the model says which
	// module declares it. A path that names a module directly still resolves.
	entry := v.moduleDeclaring(parts[0])
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

// moduleDeclaring answers the entry tree of the loaded config module that
// declares the named top-level section, or nil when no module declares it.
//
// The model holds the pairing already: a module's entry tree lists the sections
// it declares, so nothing else has to state which module owns "bgp". A
// component or plugin whose YANG registers a new section is therefore resolved
// the moment it registers.
//
// This replaced a switch over twelve prefixes, and the thirteenth is why the
// switch was a defect rather than a shortcut: "pppoe" was never added, so
// ValidateTree("pppoe", ...) resolved no module and checked nothing at all
// (validator_yang_test.go records the workaround it forced).
//
// ConfModuleNames is sorted, so a section several modules contribute to always
// resolves to the same one. ValidateTreeAllModules is the entry point for a
// caller that needs every contributing module rather than one.
func (v *Validator) moduleDeclaring(section string) *yang.Entry {
	for _, module := range v.loader.ConfModuleNames() {
		entry := v.loader.GetEntry(module)
		if entry == nil || entry.Dir == nil {
			continue
		}
		if _, declares := entry.Dir[section]; declares {
			return entry
		}
	}
	return nil
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

// ValidateType validates value against a type of the processed schema. The
// schema builder runs every default statement through it at load, so a
// default that its own type refuses stops the schema build.
func (v *Validator) ValidateType(path string, yangType *yang.YangType, value any) error {
	return v.validateYangType(path, yangType, value)
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
	case yang.Ydecimal64:
		return v.validateDecimal64(path, yangType, value)
	default:
		return nil
	}
}

// decimal64Lexical is the lexical representation of a decimal64 value.
//
// RFC 7950 Section 9.3.1: "A decimal64 value is lexically represented as an
// optional sign ("+" or "-"), followed by a sequence of decimal digits,
// optionally followed by a period ('.') as a decimal indicator and a sequence
// of decimal digits." A sequence holds at least one digit, so RFC 7950
// Section 9.3.2: "there MUST be at least one digit before and after the
// decimal point".
var decimal64Lexical = regexp.MustCompile(`^[+-]?\d+(\.\d+)?$`)

// validateDecimal64 validates a decimal64 value: its lexical form, its
// fraction digits against the type's fraction-digits, and its value against
// the type's range.
func (v *Validator) validateDecimal64(path string, yangType *yang.YangType, value any) error {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  "expected decimal64",
			Expected: yangType.Name,
			Got:      fmt.Sprintf("%T", value),
		}
	}
	if !decimal64Lexical.MatchString(str) {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  fmt.Sprintf("expected decimal64, got %q", str),
			Expected: yangType.Name,
			Got:      str,
		}
	}
	// RFC 7950 Section 9.3.4: "The "fraction-digits" statement ... restricts
	// the value space of the type by defining the total number of digits
	// after the decimal point". goyang holds the count on the type, parse
	// refuses a value with more digits than that.
	// #nosec G115 -- goyang bounds FractionDigits to 1..18 at module load
	num, err := yang.ParseDecimal(str, uint8(yangType.FractionDigits))
	if err != nil {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  fmt.Sprintf("expected decimal64 with at most %d fraction digits, got %q", yangType.FractionDigits, str),
			Expected: yangType.Name,
			Got:      str,
		}
	}
	if len(yangType.Range) > 0 && !checkYangRangeDecimal(num, yangType.Range) {
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeRange,
			Message:  fmt.Sprintf("value %s is outside range", str),
			Expected: yangType.Range.String(),
			Got:      str,
		}
	}
	return nil
}

// checkYangRangeDecimal reports whether num sits inside one of the ranges.
// Every Number carries the type's fraction digits, so Less compares them
// directly.
func checkYangRangeDecimal(num yang.Number, ranges yang.YangRange) bool {
	for _, r := range ranges {
		if num.Less(r.Min) {
			continue
		}
		if r.Max.Less(num) {
			continue
		}
		return true
	}
	return false
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
				Message:  textbuf.StrIntStr("string length ", int64(strLen), " is outside allowed range"),
				Expected: yangType.Length.String(),
				Got:      strconv.Itoa(int(strLen)),
			}
		}
	}

	// Check patterns
	for _, p := range yangType.Pattern {
		matched, err := MatchPattern(p, str)
		if err != nil {
			return &ValidationError{
				Path:    path,
				Type:    ErrTypePattern,
				Message: fmt.Sprintf("invalid pattern %q: %v", p, err),
			}
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

// msgExpectedUnsigned is the diagnostic every rejected unsigned value carries.
// One spelling keeps the four branches of validateUnsigned reporting one error.
const msgExpectedUnsigned = "expected unsigned integer"

// validateUnsigned validates unsigned integer against YangType.
// Accepts numeric types and strings (config values often arrive as strings).
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
				Message:  msgExpectedUnsigned,
				Expected: yangType.Name,
				Got:      strconv.Itoa(n),
			}
		}
		num = uint64(n)
	case int64:
		if n < 0 {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  msgExpectedUnsigned,
				Expected: yangType.Name,
				Got:      strconv.Itoa(int(n)),
			}
		}
		num = uint64(n)
	case float64:
		if n < 0 || n != float64(uint64(n)) {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  msgExpectedUnsigned,
				Expected: yangType.Name,
				Got:      fmt.Sprintf("%v", n),
			}
		}
		num = uint64(n)
	case string:
		// Config values arrive as strings — attempt conversion.
		parsed, parseErr := strconv.ParseUint(n, 10, 64)
		if parseErr != nil {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  fmt.Sprintf("expected unsigned integer, got %q", n),
				Expected: yangType.Name,
				Got:      n,
			}
		}
		num = parsed
	default: // reject unhandled types (bool, slice, map, nil)
		return &ValidationError{
			Path:     path,
			Type:     ErrTypeType,
			Message:  msgExpectedUnsigned,
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
				Message:  textbuf.StrUintStr("value ", num, " is outside range"),
				Expected: yangType.Range.String(),
				Got:      textbuf.StringUint(num),
			}
		}
	}

	return nil
}

// validateSigned validates signed integer against YangType.
// Accepts numeric types and strings (config values often arrive as strings).
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
	case string:
		// Config values arrive as strings — attempt conversion.
		parsed, parseErr := strconv.ParseInt(n, 10, 64)
		if parseErr != nil {
			return &ValidationError{
				Path:     path,
				Type:     ErrTypeType,
				Message:  fmt.Sprintf("expected signed integer, got %q", n),
				Expected: yangType.Name,
				Got:      n,
			}
		}
		num = parsed
	default: // reject unhandled types (bool, slice, map, nil)
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
				Message:  textbuf.StrIntStr("value ", num, " is outside range"),
				Expected: yangType.Range.String(),
				Got:      textbuf.StringInt(num),
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
		expected = textbuf.Join(yangType.Enum.Names(), ", ")
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
		// RFC 7950 Section 9.5.1: "The lexical representation of a boolean value
		// is a string with a value of "true" or "false"". The config spelling enable and
		// disable is rewritten to these by NormalizeLeafValue before the
		// tree stores a value, so it never reaches this validator.
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
	var tb textbuf.Buffer
	for name, child := range entry.Dir {
		if child.Mandatory == yang.TSTrue {
			if _, ok := data[name]; !ok {
				return &ValidationError{
					Path:    tb.Reset().Str(path).Byte('/').Str(name).String(),
					Type:    ErrTypeMissing,
					Message: tb.Reset().Str("mandatory field ").Quoted(name).Str(" is missing").String(),
				}
			}
		}
	}

	// Validate provided values
	for key, value := range data {
		childPath := tb.Reset().Str(path).Byte('/').Str(key).String()
		if child, ok := entry.Dir[key]; ok {
			if err := v.validateEntry(childPath, child, value); err != nil {
				return err
			}
			// Apply ze:validate custom validators. For a leaf-list the delivered
			// value is the single item (scalar) or space-separated items; apply the
			// validator to each item so a leaf-list with ze:validate (e.g. isis
			// `net`) is checked, matching the single-leaf behavior.
			if cverr := v.applyContainerCustomValidators(childPath, child, value); cverr != nil {
				return cverr
			}
		}
	}

	return nil
}

// applyContainerCustomValidators runs the child's ze:validate custom validators
// against value (or each leaf-list item), returning the first failure. It is the
// ValidateContainer-path counterpart of applyCustomValidators (which accumulates
// errors during a full-tree walk).
func (v *Validator) applyContainerCustomValidators(path string, child *yang.Entry, value any) error {
	if v.registry == nil {
		return nil
	}
	if len(SplitValidatorNames(GetValidateExtension(child))) == 0 {
		return nil
	}
	// A leaf-list arrives as a space-separated string (bracket form) or a single
	// scalar; validate each item. A single leaf validates the whole value.
	items := []any{value}
	if child.IsLeafList() {
		if str, ok := value.(string); ok {
			fields := strings.Fields(str)
			items = make([]any, len(fields))
			for i, f := range fields {
				items[i] = f
			}
		}
	}
	var errs []ValidationError
	for _, item := range items {
		v.applyCustomValidators(path, child, item, &errs)
	}
	if len(errs) > 0 {
		return &errs[0]
	}
	return nil
}

// ValidateTree recursively validates a config data tree against YANG schema.
// path is the module prefix (e.g., "bgp"). data is the parsed config map.
// Returns all validation errors found (does not stop at first error).
func (v *Validator) ValidateTree(path string, data map[string]any) []ValidationError {
	entry, err := v.findSchemaNode(path)
	if err != nil {
		return []ValidationError{{Path: path, Type: ErrTypeType, Message: err.Error()}}
	}
	if entry.Dir == nil {
		return nil
	}

	var errs []ValidationError
	v.walkTree(path, entry, data, &errs)
	return errs
}

// ValidateTreeAllModules validates a config section against every conf module
// that defines a top-level container matching the section name. This handles
// sections like l2tp where multiple modules contribute children under the
// same top-level container. Each module is validated independently; unknown
// fields from other modules are silently skipped.
func (v *Validator) ValidateTreeAllModules(section string, data map[string]any) []ValidationError {
	var errs []ValidationError
	for _, modName := range v.loader.ConfModuleNames() {
		entry := v.loader.GetEntry(modName)
		if entry == nil || entry.Dir == nil {
			continue
		}
		sectionEntry, ok := entry.Dir[section]
		if !ok {
			continue
		}
		v.walkTree(section, sectionEntry, data, &errs)
	}
	return errs
}

// walkTree recursively validates a container/list against its YANG entry.
func (v *Validator) walkTree(path string, entry *yang.Entry, data map[string]any, errs *[]ValidationError) {
	var tb textbuf.Buffer
	// Check mandatory children at this level.
	for name, child := range entry.Dir {
		if child.Mandatory == yang.TSTrue {
			if _, ok := data[name]; !ok {
				*errs = append(*errs, ValidationError{
					Path:    tb.Reset().Str(path).Byte('/').Str(name).String(),
					Type:    ErrTypeMissing,
					Message: tb.Reset().Str("mandatory field ").Quoted(name).Str(" is missing").String(),
				})
			}
			continue
		}
		// An ABSENT leaf-list with min-elements > 0 violates its bound just as
		// surely as an empty one, and NOTHING else catches it.
		//
		// The obvious assumption -- "absence is the mandatory check's business"
		// above -- is FALSE for a leaf-list, and provably so at the producer:
		// goyang's `case *LeafList:` (vendor/.../goyang/pkg/yang/entry.go)
		// synthesizes a Leaf WITHOUT copying Mandatory, and *LeafList has no
		// Mandatory field to copy (yang.go: it carries MaxElements/MinElements
		// but no mandatory). So Entry.Mandatory is TSUnset forever for a
		// leaf-list and the loop above can never fire on one.
		//
		// Omitting the leaf entirely is also the shape an operator is most
		// likely to write, so checking only the explicitly-empty `foo [ ]` form
		// would leave the common case inert -- which is the defect this spec is
		// named for.
		if child.IsLeafList() && child.ListAttr != nil && child.ListAttr.MinElements > 0 {
			if _, ok := data[name]; !ok {
				checkCardinality(tb.Reset().Str(path).Byte('/').Str(name).String(), child, 0, errs)
			}
		}
	}

	// Validate each provided value.
	for key, value := range data {
		child, ok := entry.Dir[key]
		if !ok {
			continue // unknown field — handled elsewhere by config reader
		}
		childPath := tb.Reset().Str(path).Byte('/').Str(key).String()

		// Map values are containers or list entries — recurse.
		subMap, isMap := value.(map[string]any)
		if isMap {
			if child.IsList() {
				// List: each map entry is keyed by list key → sub-map.
				for listKey, listVal := range subMap {
					entryMap, entryOK := listVal.(map[string]any)
					if !entryOK {
						continue
					}
					entryPath := tb.Reset().Str(childPath).Byte('[').Str(listKey).Byte(']').String()
					// Validate the list key value itself. walkTree recurses into
					// the entry's children, but the key is only the map key — not a
					// child — so without this a ze:validate on a list key leaf
					// (e.g. redistribute `import` source) never runs.
					v.validateListKey(entryPath, child, listKey, entryMap, errs)
					v.walkTree(entryPath, child, entryMap, errs)
				}
				// Check list cardinality against YANG max-elements.
				checkCardinality(childPath, child, uint64(len(subMap)), errs)
			} else if child.Dir != nil {
				// Container: recurse.
				v.walkTree(childPath, child, subMap, errs)
			}
			continue
		}

		// Leaf-list: validate cardinality and each item individually. A leaf-list
		// reaches us in one of three shapes (see leafListItems): a bare string for
		// a single active member, a []string for several, or a space-separated
		// string. Handling only the string shape silently skipped cardinality and
		// per-item checks for any multi-member leaf-list -- exactly the case a
		// max-elements bound must catch.
		if child.IsLeafList() {
			items := leafListItems(value)
			// Cardinality is checked for a PRESENT leaf-list even when it is
			// EMPTY. This call used to sit inside `if len(items) > 0`, which
			// made min-elements inert: zero is precisely the count that violates
			// a min bound, and it was the one count that skipped the check.
			// max-elements was unaffected (an over-count is necessarily
			// non-empty), which is why the max-side coverage passed while the
			// min side enforced nothing.
			//
			// An ABSENT leaf-list does not reach this loop (it ranges over the
			// data actually supplied); absence is handled by the min-elements
			// scan over entry.Dir at the top of walkTree. Do NOT assume the
			// mandatory check covers it -- it cannot, see the note there.
			//
			// Note this counts an UNINTERPRETABLE value as 0, because
			// leafListItems returns nil for a shape it cannot read. That is
			// currently unreachable: Tree.ToMap only ever emits string or
			// []string for a leaf (config/tree.go), and both are interpreted.
			// If another producer ever feeds walkTree a differently-shaped
			// value, this would misreport it as "too few entries" rather than
			// as a type error -- distinguish the two here if that day comes.
			checkCardinality(childPath, child, uint64(len(items)), errs)
			for _, item := range items {
				if leafErr := v.validateEntry(childPath, child, item); leafErr != nil {
					if valErr, ok := errors.AsType[*ValidationError](leafErr); ok {
						*errs = append(*errs, *valErr)
					} else {
						*errs = append(*errs, ValidationError{
							Path:    childPath,
							Type:    ErrTypeType,
							Message: leafErr.Error(),
						})
					}
				}
				// Apply ze:validate custom validators per leaf-list item, the
				// same way a single leaf is custom-validated below. Without this
				// a ze:validate extension on a leaf-list (e.g. isis `net`) would
				// never run.
				v.applyCustomValidators(childPath, child, item, errs)
			}
			continue
		}

		// Non-map values are leaves — validate against YANG type.
		if leafErr := v.validateEntry(childPath, child, value); leafErr != nil {
			if valErr, ok := errors.AsType[*ValidationError](leafErr); ok {
				*errs = append(*errs, *valErr)
			} else {
				*errs = append(*errs, ValidationError{
					Path:    childPath,
					Type:    ErrTypeType,
					Message: leafErr.Error(),
				})
			}
		}

		// Check ze:validate extension for custom validation.
		v.applyCustomValidators(childPath, child, value, errs)
	}
}

// validateListKey validates a list entry's key value against the list's key-leaf
// schema, running the key leaf's YANG type check and any ze:validate custom
// validators. Without it, a ze:validate on a list key (e.g. the redistribute
// `import` source) is dead code: walkTree only validates an entry's children,
// and the key is the map key, not a child. Skips composite keys, keys with no
// resolvable leaf, and keys the entry also stores as a child (validated by the
// normal child walk, so we avoid a duplicate error).
func (v *Validator) validateListKey(path string, list *yang.Entry, keyVal string, entryMap map[string]any, errs *[]ValidationError) {
	key := list.Key
	if key == "" || strings.Contains(key, " ") {
		return // no key, or composite key — not a single validated leaf
	}
	if _, dup := entryMap[key]; dup {
		return // key is also stored as a child; the child walk validates it
	}
	keyLeaf, ok := list.Dir[key]
	if !ok {
		return
	}
	if leafErr := v.validateEntry(path, keyLeaf, keyVal); leafErr != nil {
		if valErr, ok := errors.AsType[*ValidationError](leafErr); ok {
			*errs = append(*errs, *valErr)
		} else {
			*errs = append(*errs, ValidationError{
				Path:    path,
				Type:    ErrTypeType,
				Message: leafErr.Error(),
			})
		}
	}
	v.applyCustomValidators(path, keyLeaf, keyVal, errs)
}

// applyCustomValidators runs the child's ze:validate custom validators against
// value. Multiple validators joined with "|" use OR semantics: the value is
// valid if ANY validator accepts it. Called for both a single leaf value and
// each leaf-list item so a ze:validate extension applies uniformly.
func (v *Validator) applyCustomValidators(path string, child *yang.Entry, value any, errs *[]ValidationError) {
	if v.registry == nil {
		return
	}
	names := SplitValidatorNames(GetValidateExtension(child))
	if len(names) == 0 {
		return
	}
	var lastErr error
	for _, validatorName := range names {
		cv := v.registry.Get(validatorName)
		if cv == nil {
			continue
		}
		// A nil ValidateFn is a completion-only validator: it says which values
		// are worth suggesting and refuses none of them. Skipping it leaves the
		// leaf's YANG type as the only constraint, which is what a suggestion
		// means (validator_registry.go, MergeGlobalCompletions).
		if cv.ValidateFn == nil {
			continue
		}
		cvErr := cv.ValidateFn(path, value)
		if cvErr == nil {
			lastErr = nil
			break
		}
		lastErr = cvErr
	}
	if lastErr != nil {
		*errs = append(*errs, ValidationError{
			Path:    path,
			Type:    ErrTypeType,
			Message: lastErr.Error(),
		})
	}
}

// leafListItems flattens the shapes a leaf-list takes in the config tree map
// into individual members. Tree.ToMap stores a single active member as a bare
// string and two-or-more members as a []string; a raw parsed value can also be
// a space-separated string or a []any. Returning nil for anything else lets the
// caller skip a leaf-list it cannot interpret rather than miscount it.
func leafListItems(value any) []string {
	switch vv := value.(type) {
	case string:
		if vv == "" {
			return nil
		}
		return strings.Fields(vv)
	case []string:
		return vv
	case []any:
		items := make([]string, 0, len(vv))
		for _, e := range vv {
			if s, ok := e.(string); ok {
				items = append(items, s)
			} else {
				items = append(items, fmt.Sprint(e))
			}
		}
		return items
	default:
		return nil
	}
}

// checkCardinality validates list/leaf-list entry count against YANG
// min-elements and max-elements constraints. Appends errors if violated.
func checkCardinality(path string, entry *yang.Entry, count uint64, errs *[]ValidationError) {
	if entry.ListAttr == nil {
		return
	}
	if entry.ListAttr.MinElements > 0 && count < entry.ListAttr.MinElements {
		var bMsg textbuf.Buffer
		*errs = append(*errs, ValidationError{
			Path:     path,
			Type:     ErrTypeCardinality,
			Message:  bMsg.Reset().Str("too few entries: ").Uint(count).Str(" (minimum ").Uint(entry.ListAttr.MinElements).Byte(')').String(),
			Expected: textbuf.StrUint(">=", entry.ListAttr.MinElements),
			Got:      textbuf.StringUint(count),
		})
	}
	if entry.ListAttr.MaxElements > 0 && count > entry.ListAttr.MaxElements {
		var bMsg textbuf.Buffer
		*errs = append(*errs, ValidationError{
			Path:     path,
			Type:     ErrTypeCardinality,
			Message:  bMsg.Reset().Str("too many entries: ").Uint(count).Str(" (maximum ").Uint(entry.ListAttr.MaxElements).Byte(')').String(),
			Expected: textbuf.StrUint("<=", entry.ListAttr.MaxElements),
			Got:      textbuf.StringUint(count),
		})
	}
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
		if num >= signedBound(r.Min) && num <= signedBound(r.Max) {
			return true
		}
	}
	return false
}

// signedBound returns a range bound as int64. goyang stores a Number as a
// magnitude and a Negative flag (vendor/github.com/openconfig/goyang/pkg/
// yang/types_builtin.go, Number), never as a two's complement pattern, so
// the flag decides the sign. The magnitude of the smallest int64 wraps to
// itself under negation, which is the value it names.
func signedBound(n yang.Number) int64 {
	// #nosec G115 -- goyang bounds a signed magnitude to int64 at module load
	magnitude := int64(n.Value)
	if n.Negative {
		return -magnitude
	}
	return magnitude
}
