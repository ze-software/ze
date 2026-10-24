// Design: docs/architecture/plugin/plugin-system.md -- a hard dependency the build does not carry
// Related: gated_plugins.go -- the build tags that name what to rebuild with
// Related: registry.go -- ResolveDependencies, the only producer of this error

package registry

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MissingDependencyError is the refusal ResolveDependencies answers when a
// plugin it expands declares a hard dependency (Registration.Dependencies) that
// this binary does not register. The plugin never works without it, so every
// load path refuses rather than starting the plugin alone.
//
// errors.Is(err, ErrMissingDependency) holds for every value, so callers that
// only branch on the class keep working. Tags is empty exactly when the
// dependency is not a gated plugin: no build of this tree registers the name,
// and the message then says so rather than guessing a tag.
type MissingDependencyError struct {
	Plugin     string   // The plugin that declared the dependency.
	Dependency string   // The dependency this build does not register.
	Tags       []string // Build tags that carry Dependency, from GatedPluginTags; empty when ungated.
}

// newMissingDependencyError builds the refusal for plugin's dependency, asking
// the generated gated-plugin table which build tags would carry it.
func newMissingDependencyError(plugin, dependency string) *MissingDependencyError {
	// GatedPluginTags answers no tag for an ungated name, which is what keeps
	// the message from guessing one.
	tags, _ := GatedPluginTags(dependency)

	return &MissingDependencyError{Plugin: plugin, Dependency: dependency, Tags: tags}
}

// Unwrap answers ErrMissingDependency, so errors.Is matches the class.
func (e *MissingDependencyError) Unwrap() error { return ErrMissingDependency }

// Error states the problem, why it stops the plugin, and the fix on its own
// line, in the operator's terms: the plugin and dependency names and the build
// tags exactly as feature-gates.txt spells them.
func (e *MissingDependencyError) Error() string {
	var b textbuf.Buffer
	b.Reset()

	b.Str(`plugin "`).Str(e.Plugin).Str(`" needs plugin "`).Str(e.Dependency).Str(`", `)
	if len(e.Tags) > 0 {
		b.Str("and this build of ze does not include it.\n")
	} else {
		b.Str(`and no build of ze includes a plugin called "`).Str(e.Dependency).
			Str(`": the name is unknown to this build.` + "\n")
	}
	b.Str(`"`).Str(e.Plugin).Str(`" does not work without "`).Str(e.Dependency).
		Str(`", so ze refuses to start it.` + "\n")
	if len(e.Tags) > 0 {
		b.Str("Fix: rebuild ze with the build tags ").Str(strings.Join(e.Tags, " ")).
			Str(` added to -tags, or remove the configuration that loads "`).Str(e.Plugin).Str(`".`)
	} else {
		b.Str(`Fix: remove the configuration that loads "`).Str(e.Plugin).
			Str(`", and report the unknown dependency "`).Str(e.Dependency).Str(`" to the ze developers.`)
	}

	return b.String()
}
