// Design: docs/architecture/plugin/plugin-system.md -- a hard dependency the build does not carry
// Related: gated_plugins.go -- the build tags that name what to rebuild with
// Related: registry.go -- ResolveDependencies, the producer for a registered plugin's dependency

package registry

import (
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// MissingDependencyError is the refusal answered when a plugin needs another
// plugin that is not there, so the plugin never works and every load path
// refuses rather than starting it alone. Two producers build it:
// ResolveDependencies, for a hard dependency (Registration.Dependencies) this
// binary does not register, and UnloadedDependencyError, for a dependency a
// plugin declared at Stage 1 that nothing configured or loaded.
//
// errors.Is(err, ErrMissingDependency) holds for every value, so callers that
// only branch on the class keep working, and errors.As finds the
// *opererr.Error that carries the three parts. Tags is empty exactly when the
// dependency is not a gated plugin: the message then names no tag rather than
// guessing one.
//
// Build it with newMissingDependencyError or UnloadedDependencyError: a value
// built any other way carries no absence, and its message says so.
type MissingDependencyError struct {
	Plugin     string   // The plugin that needs the dependency.
	Dependency string   // The dependency that is not there.
	Tags       []string // Build tags that carry Dependency, from GatedPluginTags; empty when ungated.
	absence    dependencyAbsence
}

// dependencyAbsence says why the dependency is not there, which decides the
// fix the refusal names. Only the two constructors in this file set it.
type dependencyAbsence uint8

const (
	// absenceUnspecified is a value no constructor built: a ze defect.
	absenceUnspecified dependencyAbsence = iota
	// absenceCompiledOut: a gated plugin this build was compiled without.
	absenceCompiledOut
	// absenceUnknown: a registered plugin's dependency that no build of this
	// tree registers and no gate covers, a defect in that plugin's registration.
	absenceUnknown
	// absenceNotLoaded: a plugin this build carries that the config does not
	// load, so only a Stage 1 declaration can name it.
	absenceNotLoaded
	// absenceNotConfigured: a name no build carries, declared at Stage 1, which
	// only an external program the operator configures can provide.
	absenceNotConfigured
)

// newMissingDependencyError builds the refusal for plugin's hard dependency,
// which the registry does not hold, asking the generated gated-plugin table
// which build tags would carry it. It runs under ResolveDependencies' read lock,
// so it reads the generated table only, never the registry.
func newMissingDependencyError(plugin, dependency string) *MissingDependencyError {
	// GatedPluginTags answers no tag for an ungated name, which is what keeps
	// the message from guessing one.
	tags, gated := GatedPluginTags(dependency)
	absence := absenceUnknown
	if gated {
		absence = absenceCompiledOut
	}

	return &MissingDependencyError{Plugin: plugin, Dependency: dependency, Tags: tags, absence: absence}
}

// UnloadedDependencyError builds the refusal for a dependency plugin declared
// at Stage 1 that no `plugin { }` block configures and no startup phase loaded.
// Nobody resolves a declared dependency for the operator, so the fix names what
// to configure: the plugin this build carries, or an external program for a
// name no build carries. A dependency this build was compiled without names
// its build tags instead, as ResolveDependencies does.
//
// It takes the registry's read lock, so a caller MUST NOT hold the registry
// lock.
func UnloadedDependencyError(plugin, dependency string) *MissingDependencyError {
	if Has(dependency) {
		return &MissingDependencyError{Plugin: plugin, Dependency: dependency, absence: absenceNotLoaded}
	}
	tags, gated := GatedPluginTags(dependency)
	absence := absenceNotConfigured
	if gated {
		absence = absenceCompiledOut
	}

	return &MissingDependencyError{Plugin: plugin, Dependency: dependency, Tags: tags, absence: absence}
}

// Unwrap answers ErrMissingDependency, so errors.Is matches the class, and the
// operator error, so errors.As reaches the three parts.
func (e *MissingDependencyError) Unwrap() []error {
	return []error{ErrMissingDependency, e.refusal()}
}

// Error renders the operator error: what is missing, why the plugin cannot
// start, and the fix on its own line, in the operator's terms.
func (e *MissingDependencyError) Error() string {
	return e.refusal().Error()
}

// refusal writes the three parts for the absence. Every fix keeps the other
// way out, removing the configuration that loads the plugin, so an operator
// who does not want the dependency is never sent to rebuild ze.
func (e *MissingDependencyError) refusal() *opererr.Error {
	needs := `plugin "` + e.Plugin + `" needs plugin "` + e.Dependency + `"`
	why := `"` + e.Plugin + `" does not work without "` + e.Dependency + `", so ze refuses to start it`
	remove := `remove the configuration that loads "` + e.Plugin + `"`

	switch e.absence {
	case absenceCompiledOut:
		return opererr.New(needs+", and this build of ze was compiled without it", why,
			// The tags stay space-separated, exactly as they go into -tags, so
			// the operator copies them; BuildTagsPhrase's "a and b" is prose.
			"rebuild ze with "+strings.Join(e.Tags, " ")+" added to its -tags list, or "+remove)
	case absenceUnknown:
		return opererr.New(needs+`, and no build of ze includes a plugin called "`+e.Dependency+`"`, why,
			remove+`, and report the unknown dependency "`+e.Dependency+`" to the ze developers`)
	case absenceNotLoaded:
		return opererr.New(needs+", and the configuration does not load it", why,
			`add 'plugin { internal `+e.Dependency+` { use `+e.Dependency+`; } }' to the config, or `+remove)
	case absenceNotConfigured:
		return opererr.New(needs+`, and no plugin called "`+e.Dependency+`" is configured`,
			why+`; ze does not include a plugin of that name, so only an external program can provide it`,
			`add 'plugin { external `+e.Dependency+` { run "<the command that starts `+e.Dependency+
				`>"; } }' to the config, or `+remove)
	case absenceUnspecified:
	}

	return opererr.New(needs+", and ze did not record why it is missing", why,
		"report this message to the ze developers: the refusal was built without its constructor")
}
