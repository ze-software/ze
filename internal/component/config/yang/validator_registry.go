// Design: docs/architecture/config/yang-config-design.md — custom validation registry
// Related: command.go — ze:command and ze:edit-shortcut extensions (same pattern)

package yang

import (
	"fmt"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// CustomValidator provides validation, completion and per-value help for a
// ze:validate function. ValidateFn checks a value; CompleteFn lists the values
// worth suggesting; DescribeFn says what one suggested value means.
//
// The three are independent, and a validator MAY carry any subset. A validator
// with a nil ValidateFn refuses nothing: it is a SUGGESTION, and every value
// the leaf's YANG type admits stays valid. Completion and refusal are separate
// jobs, so offering the well-known values for a leaf MUST NOT narrow it.
type CustomValidator struct {
	ValidateFn func(path string, value any) error
	CompleteFn func() []string           // nil if no completion support
	DescribeFn func(value string) string // nil if a value needs no help text
}

// ValidatorRegistry stores custom validators registered via init().
// Written during init(), read-only after — no mutex needed.
type ValidatorRegistry struct {
	validators map[string]CustomValidator
}

// globalCompleteFns holds CompleteFn callbacks registered at init() time
// by domain packages (e.g., iface registers MAC address completion).
// This decouples config from domain packages: config defines the validator
// with ValidateFn only, the domain package registers its CompleteFn here,
// and the registry merges them at startup.
// Written during init(), read during MergeGlobalCompletions -- no mutex needed.
var globalCompleteFns = map[string]func() []string{}

// globalSuggestions holds completion-only validators a domain package DECLARES
// for a ze:validate name that config does not own. Each carries a CompleteFn, an
// optional DescribeFn, and never a ValidateFn.
// Written during init(), read during MergeGlobalCompletions -- no mutex needed.
var globalSuggestions = map[string]CustomValidator{}

// RegisterCompleteFn registers a CompleteFn for a named validator.
// Called during init() by domain packages to provide CLI completion
// without requiring the config package to import them.
//
// The name MUST already be a validator in RegisterValidators
// (config/validators_register.go). A name that is not stays absent, so
// CheckAllValidatorsRegistered still reports the leaf that references it. Use
// RegisterSuggestion for a name config does not own.
func RegisterCompleteFn(name string, fn func() []string) {
	globalCompleteFns[name] = fn
}

// RegisterSuggestion declares a completion-only validator for a ze:validate
// name that config does not own, so a plugin can complete its own leaf.
//
// It REFUSES NOTHING. There is no ValidateFn, applyCustomValidators skips the
// entry, and every value the leaf's YANG type admits stays valid. Call it when
// the value set is a SUGGESTION, such as a well-known set that can go stale.
// When a value outside the set is genuinely wrong, the leaf needs a real
// validator in RegisterValidators instead.
//
// describe MAY be nil, and returns the empty string for a value it does not
// know; the completer then prints its generic label.
//
// The two registrations are deliberately separate. This one DECLARES a
// validator, so it satisfies CheckAllValidatorsRegistered; RegisterCompleteFn
// only fills a slot on a validator config already declared, and a name it does
// not match stays missing, which keeps a forgotten ValidateFn loud.
func RegisterSuggestion(name string, values func() []string, describe func(value string) string) {
	globalSuggestions[name] = CustomValidator{CompleteFn: values, DescribeFn: describe}
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

// MergeGlobalCompletions applies both global registrations to this registry.
// Called once after all validators are registered during startup.
//
// A RegisterCompleteFn callback fills the CompleteFn slot of a validator this
// registry already holds, and a name it does not hold is left alone, so the
// startup integrity check still names the leaf that references it.
//
// A RegisterSuggestion entry is ADDED as a validator, because it declares one.
// It never overwrites: a name this registry holds keeps its ValidateFn, and
// gains only the slots it left empty. That is how a plugin completes its own
// leaf, since RegisterValidators (config/validators_register.go) is a central
// list the plugin cannot reach and config cannot write without importing it.
func (r *ValidatorRegistry) MergeGlobalCompletions() {
	for name, fn := range globalCompleteFns {
		cv, ok := r.validators[name]
		if !ok || cv.CompleteFn != nil {
			continue
		}
		cv.CompleteFn = fn
		r.validators[name] = cv
	}
	for name, suggestion := range globalSuggestions {
		cv := r.validators[name]
		if cv.CompleteFn == nil {
			cv.CompleteFn = suggestion.CompleteFn
		}
		if cv.DescribeFn == nil {
			cv.DescribeFn = suggestion.DescribeFn
		}
		r.validators[name] = cv
	}
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
		if strings.HasSuffix(ext.Keyword, ":validate") {
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
		return fmt.Errorf("missing validator registrations: %s", textbuf.Join(missing, ", "))
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
