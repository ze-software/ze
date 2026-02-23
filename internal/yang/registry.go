// Design: docs/architecture/config/yang-config-design.md — custom validation registry

package yang

import (
	"fmt"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"
)

// CustomValidator provides both validation and completion for a ze:validate function.
// ValidateFn checks a value; CompleteFn returns valid options for CLI completion.
type CustomValidator struct {
	ValidateFn func(path string, value any) error
	CompleteFn func() []string // nil if no completion support
}

// ValidatorRegistry stores custom validators registered via init().
// Written during init(), read-only after — no mutex needed.
type ValidatorRegistry struct {
	validators map[string]CustomValidator
}

// NewValidatorRegistry creates an empty registry.
func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{
		validators: make(map[string]CustomValidator),
	}
}

// Register adds a custom validator by name.
func (r *ValidatorRegistry) Register(name string, cv CustomValidator) {
	r.validators[name] = cv
}

// Get returns the custom validator for name, or nil if not registered.
func (r *ValidatorRegistry) Get(name string) *CustomValidator {
	cv, ok := r.validators[name]
	if !ok {
		return nil
	}
	return &cv
}

// Names returns all registered validator names (sorted).
func (r *ValidatorRegistry) Names() []string {
	names := make([]string, 0, len(r.validators))
	for name := range r.validators {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetValidateExtension reads the ze:validate extension from a YANG entry.
// Returns empty string if no validate extension is present.
func GetValidateExtension(entry *gyang.Entry) string {
	if entry == nil {
		return ""
	}
	for _, ext := range entry.Exts {
		if ext.Keyword == "ze:validate" || strings.HasSuffix(ext.Keyword, ":validate") {
			return ext.Argument
		}
	}
	return ""
}

// SplitValidatorNames splits a ze:validate argument that may contain multiple
// pipe-separated validator names. Returns nil for empty input.
func SplitValidatorNames(arg string) []string {
	if arg == "" {
		return nil
	}
	var names []string
	for part := range strings.SplitSeq(arg, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			names = append(names, part)
		}
	}
	return names
}

// CheckAllValidatorsRegistered walks the YANG tree and verifies every ze:validate
// reference has a corresponding registered function. Returns error listing all missing.
func CheckAllValidatorsRegistered(loader *Loader, reg *ValidatorRegistry) error {
	seen := make(map[string]bool)

	for _, moduleName := range loader.ModuleNames() {
		entry := loader.GetEntry(moduleName)
		if entry == nil {
			continue
		}
		collectMissingValidators(entry, reg, seen)
	}

	missing := make([]string, 0, len(seen))
	for name := range seen {
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing validator registrations: %s", strings.Join(missing, ", "))
	}
	return nil
}

// collectMissingValidators recursively checks entries for ze:validate extensions.
func collectMissingValidators(entry *gyang.Entry, reg *ValidatorRegistry, missing map[string]bool) {
	for _, name := range SplitValidatorNames(GetValidateExtension(entry)) {
		if reg.Get(name) == nil {
			missing[name] = true
		}
	}

	for _, child := range entry.Dir {
		collectMissingValidators(child, reg, missing)
	}
}
