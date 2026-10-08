// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// RFC: rfc/short/rfc7950.md -- Sections 5.1 and 6.3.1, extension prefix resolution
//
// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// DefaultLoader creates a Loader with all embedded and registered modules
// loaded and resolved. Registered module and import resolution errors are
// discarded as best-effort: the command tree only needs the -cmd.yang modules,
// which import ze-extensions (embedded), not the full conf/api module set.
//
// An extension statement that no loaded module declares is NOT best-effort.
// Ze's extension readers match a statement by its keyword, so a misspelled
// `ze:comand` would load and the feature it names would be absent in silence.
// DefaultLoader returns that error, and the errors embedded loading reports.
// A pattern compilePattern cannot compile is not best-effort either: the
// command tree would otherwise hold an argument whose restriction vanished.
func DefaultLoader() (*Loader, error) {
	l := NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("YANG LoadEmbedded: %w", err)
	}
	_ = l.LoadRegistered() // Best-effort: some modules may not be imported in this context
	_ = l.process()        // Best-effort: unresolved modules are skipped by tree walker
	if err := errors.Join(l.checkExtensions(), l.checkPatterns()); err != nil {
		return nil, err
	}
	return l, nil
}

//go:embed modules
var embeddedModules embed.FS

// Loader loads and resolves YANG modules.
type Loader struct {
	modules *yang.Modules
}

// NewLoader creates a new YANG module loader.
func NewLoader() *Loader {
	return &Loader{
		modules: yang.NewModules(),
	}
}

// LoadEmbedded loads the embedded YANG library modules (extensions, types).
// These are true bootstrap modules with no domain content.
// Domain modules (hub-conf, bgp-conf, plugin-conf) are loaded via LoadRegistered().
func (l *Loader) LoadEmbedded() error {
	files := []string{
		"modules/ze-extensions.yang",
		"modules/ze-types.yang",
	}

	for _, path := range files {
		content, err := embeddedModules.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := l.AddModuleFromText(path, string(content)); err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
	}

	return nil
}

// LoadRegistered loads all init()-registered YANG modules into the loader.
// Call after LoadEmbedded() and before Resolve().
func (l *Loader) LoadRegistered() error {
	for _, mod := range modules {
		if err := l.AddModuleFromText(mod.Name, mod.Content); err != nil {
			return err
		}
	}
	return nil
}

// AddModuleFromText adds a YANG module from text content.
func (l *Loader) AddModuleFromText(name, content string) error {
	if err := l.modules.Parse(content, name); err != nil {
		return fmt.Errorf("parse YANG: %w", err)
	}
	return nil
}

// AddModuleFromFile adds a YANG module from a file path.
func (l *Loader) AddModuleFromFile(path string) error {
	if err := l.modules.Read(path); err != nil {
		return fmt.Errorf("read YANG file %s: %w", path, err)
	}
	return nil
}

// Resolve resolves all module dependencies and imports, then refuses every
// extension statement whose prefix names no imported module, or whose keyword
// names no extension the module behind that prefix declares. The error joins
// every failure, and each undeclared extension wraps ErrUndeclaredExtension.
//
// It also refuses every `pattern` statement compilePattern cannot compile,
// each wrapping ErrUncompilablePattern.
func (l *Loader) Resolve() error {
	return errors.Join(l.process(), l.checkExtensions(), l.checkPatterns())
}

// process runs goyang's import and type resolution over every loaded module.
func (l *Loader) process() error {
	errs := l.modules.Process()
	if len(errs) > 0 {
		return fmt.Errorf("resolve YANG modules: %v", errs)
	}
	return nil
}

// ErrUncompilablePattern marks a `pattern` statement that compilePattern, the
// XSD-to-RE2 translation every consumer of a pattern uses, cannot compile. A
// pattern nobody can match against constrains nothing, so the module holding
// it is refused rather than loaded with the restriction dropped.
var ErrUncompilablePattern = errors.New("uncompilable YANG pattern")

// ErrUndeclaredExtension marks an extension statement that its module cannot
// resolve: the prefix names no imported module, or the module behind the
// prefix declares no extension of that keyword.
//
// RFC 7950 Section 6.3.1: "When an imported extension is used, the
// extension's keyword MUST be qualified using the prefix with which the
// extension's module was imported."
var ErrUndeclaredExtension = errors.New("undeclared YANG extension")

// checkExtensions walks the statements of every loaded module and submodule
// and returns one error per undeclared extension statement, joined.
//
// goyang keeps any `prefix:keyword` substatement in Exts without asking
// whether the prefix's module declares it (ast.go, build: "Keyword is not
// known but it has a prefix so it might be an extension"), so this is the
// only place a misspelled Ze extension is refused. The set of allowed keywords
// is derived from the `extension` statements of the module the prefix
// resolves to, never listed here.
func (l *Loader) checkExtensions() error {
	mods := l.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleExtensionErrors(mod)...)
	}
	return errors.Join(errs...)
}

// checkPatterns walks the statements of every loaded module and submodule and
// returns one error per `pattern` statement that compilePattern refuses,
// joined. goyang keeps a pattern without compiling it (types.go: "These
// patterns are not checked because there is no support for W3C regexes by
// Go"), so this is the only place an uncompilable one is refused. Without it
// the config validator reported the pattern on every value and the command
// argument builder dropped it, leaving the argument open to any string.
func (l *Loader) checkPatterns() error {
	mods := l.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, modulePatternErrors(mod)...)
	}
	return errors.Join(errs...)
}

// modulePatternErrors returns one error for each `pattern` statement in mod
// that compilePattern cannot compile. The walk is an explicit stack, as in
// moduleExtensionErrors.
func modulePatternErrors(mod *yang.Module) []error {
	if mod.Source == nil {
		return nil
	}
	var errs []error
	pending := slices.Clone(mod.Source.SubStatements())
	for len(pending) > 0 {
		statement := pending[len(pending)-1]
		pending = append(pending[:len(pending)-1], statement.SubStatements()...)
		if statement.Keyword != "pattern" {
			continue
		}
		if _, err := compilePattern(statement.Argument); err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %w",
				ErrUncompilablePattern, mod.Name, statement.Location(), err))
		}
	}
	return errs
}

// sourceModules answers every loaded module and submodule, sorted by name,
// skipping the revision-qualified duplicate keys goyang also stores.
func (l *Loader) sourceModules() []*yang.Module {
	names := make([]string, 0, len(l.modules.Modules)+len(l.modules.SubModules))
	names = append(names, l.ModuleNames()...)
	for name := range l.modules.SubModules {
		if strings.Contains(name, "@") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	mods := make([]*yang.Module, 0, len(names))
	for _, name := range names {
		mod := l.modules.Modules[name]
		if mod == nil {
			mod = l.modules.SubModules[name]
		}
		mods = append(mods, mod)
	}
	return mods
}

// moduleExtensionErrors returns one error for each extension statement in mod
// that resolves to no declared extension. The walk is an explicit stack rather
// than recursion, so a deep statement tree costs heap slots, not goroutine
// stack. The prefix resolves through mod's own `prefix` (or `belongs-to`) and
// `import` statements; an import whose module is not loaded resolves to none.
func moduleExtensionErrors(mod *yang.Module) []error {
	if mod.Source == nil {
		return nil
	}
	var errs []error
	pending := slices.Clone(mod.Source.SubStatements())
	for len(pending) > 0 {
		statement := pending[len(pending)-1]
		pending = append(pending[:len(pending)-1], statement.SubStatements()...)
		prefix, keyword, isExtension := strings.Cut(statement.Keyword, ":")
		if !isExtension {
			continue
		}
		declaring := yang.FindModuleByPrefix(mod, prefix)
		if declaring == nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %s: prefix %q resolves to no loaded module",
				ErrUndeclaredExtension, mod.Name, statement.Location(), statement.Keyword, prefix))
			continue
		}
		if !declaresExtension(declaring, keyword) {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %s: module %s declares no extension %q",
				ErrUndeclaredExtension, mod.Name, statement.Location(), statement.Keyword, declaring.Name, keyword))
		}
	}
	return errs
}

// declaresExtension reports whether the module that owns mod's prefix carries
// an `extension keyword` statement, in its own body or in a submodule it
// includes. When mod is a submodule, the owner is the module it belongs to.
//
// RFC 7950 Section 5.1: "A submodule can reference any definition in the
// module it belongs to and in all submodules included by the module."
func declaresExtension(mod *yang.Module, keyword string) bool {
	owner := mod
	if mod.BelongsTo != nil {
		if parent := mod.Modules.Modules[mod.BelongsTo.Name]; parent != nil {
			owner = parent
		}
	}
	if moduleDeclaresExtension(owner, keyword) {
		return true
	}
	for _, include := range owner.Include {
		submodule := owner.Modules.FindModule(include)
		if submodule == nil {
			continue
		}
		if moduleDeclaresExtension(submodule, keyword) {
			return true
		}
	}
	return false
}

// moduleDeclaresExtension reports whether mod's own body carries an
// `extension keyword` statement.
func moduleDeclaresExtension(mod *yang.Module, keyword string) bool {
	for _, extension := range mod.Extension {
		if extension.Name == keyword {
			return true
		}
	}
	return false
}

// GetModule returns a loaded module by name.
func (l *Loader) GetModule(name string) *yang.Module {
	return l.modules.Modules[name]
}

// GetEntry returns the processed entry tree for a module.
// The entry tree has all imports resolved and mandatory fields properly set.
func (l *Loader) GetEntry(name string) *yang.Entry {
	mod := l.modules.Modules[name]
	if mod == nil {
		return nil
	}
	return yang.ToEntry(mod)
}

// ModuleNames returns the name of every loaded module, once each.
//
// goyang keys a module that declares a revision under TWO names, its bare name
// and `<name>@<revision>` (vendor/github.com/openconfig/goyang/pkg/yang/
// modules.go, Modules.add), and the bare name always names the most recent
// revision. 205 of Ze's modules declare one, so a caller that walks this map
// raw visits each of them twice and counts what it finds there twice with it.
func (l *Loader) ModuleNames() []string {
	names := make([]string, 0, len(l.modules.Modules))
	for name := range l.modules.Modules {
		if strings.Contains(name, "@") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ConfModuleNames returns sorted names of loaded config modules (suffix "-conf").
func (l *Loader) ConfModuleNames() []string {
	return l.moduleNamesBySuffix("-conf")
}

// APIModuleNames returns sorted names of loaded API modules (suffix "-api").
func (l *Loader) APIModuleNames() []string {
	return l.moduleNamesBySuffix("-api")
}

func (l *Loader) moduleNamesBySuffix(suffix string) []string {
	var names []string
	for _, name := range l.ModuleNames() {
		if strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
