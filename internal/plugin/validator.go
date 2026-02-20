// Design: docs/architecture/api/process-protocol.md — plugin process management

package plugin

// ValueValidator validates individual values against a schema (e.g., YANG).
// When set via SetYANGValidator, attribute values are validated against YANG types.
type ValueValidator interface {
	Validate(path string, value any) error
}

// yangValidator is the package-level YANG validator for attribute values.
// Nil by default; set during engine startup via SetYANGValidator.
var yangValidator ValueValidator //nolint:gochecknoglobals // follows logger pattern

// SetYANGValidator sets the YANG validator for attribute value validation.
// Pass nil to clear the validator.
func SetYANGValidator(v ValueValidator) {
	yangValidator = v
}

// YANGValidator returns the current YANG validator.
// Returns nil if no validator has been set.
func YANGValidator() ValueValidator {
	return yangValidator
}
