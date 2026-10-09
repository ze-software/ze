// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// RFC: rfc/short/rfc7950.md -- Sections 5.1 and 6.3.1, extension prefix resolution
//
// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// DefaultLoader creates a Loader with all embedded and registered modules
// loaded and resolved, and answers no Loader when anything failed. Nothing is
// best-effort: a registered module that does not parse, an import nothing
// registers, an undeclared extension, a pattern compilePattern cannot compile
// and a structure checkStructure refuses each come back, joined, so the caller
// cannot serve a schema that silently lacks a module or a restriction.
// LoadRegistered attempts every module, so one broken module does not hide
// the failures of the modules registered after it.
func DefaultLoader() (*Loader, error) {
	l := NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("YANG LoadEmbedded: %w", err)
	}
	if err := errors.Join(l.LoadRegistered(), l.Resolve()); err != nil {
		return nil, err
	}
	return l, nil
}

//go:embed modules
var embeddedModules embed.FS

// Loader loads and resolves YANG modules.
type Loader struct {
	modules *sourcedModules
}

// sourcedModules is goyang's module set, keeping the text each module was
// parsed from. goyang's Statement records no block when a statement has no
// substatement, so "refine x;" and "refine x {}" parse alike; the text is
// what tells them apart (statementHasBlock). Not safe for concurrent use, as
// yang.Modules is not.
type sourcedModules struct {
	*yang.Modules
	// texts is the text of each module and submodule Parse added, keyed by
	// the module goyang built from it. A file name cannot be the key: two
	// texts parsed under one name would share it, and the second would
	// answer for the first. A module goyang read from disk itself, resolving
	// an import, has no entry.
	texts map[*yang.Module]string
}

// Parse parses data into the module set, then binds every module and
// submodule that parse added to data. goyang adds a module before it parses
// the next one in the same text, so a parse that fails part way still binds
// the modules it added.
func (m *sourcedModules) Parse(data, name string) error {
	held := m.held()
	err := m.Modules.Parse(data, name)
	for mod := range m.held() {
		if !held[mod] {
			m.texts[mod] = data
		}
	}
	return err
}

// held answers every module and submodule in the set. goyang stores a
// revisioned module under two keys, so the set is of modules, not of names.
func (m *sourcedModules) held() map[*yang.Module]bool {
	held := make(map[*yang.Module]bool, len(m.Modules.Modules)+len(m.Modules.SubModules))
	for _, mod := range m.Modules.Modules {
		held[mod] = true
	}
	for _, mod := range m.Modules.SubModules {
		held[mod] = true
	}
	return held
}

// source answers mod and the text it was parsed from. Only this method
// builds a moduleSource, from the binding Parse made, so its text is the
// text of its module and of no other parse.
func (m *sourcedModules) source(mod *yang.Module) moduleSource {
	text, parsed := m.texts[mod]
	return moduleSource{module: mod, text: text, parsed: parsed}
}

// moduleSource is one module or submodule and the text it was parsed from.
// Every statement under module.Source was parsed from text, so a position
// goyang reports for one of them is a position in text.
type moduleSource struct {
	module *yang.Module
	text   string
	// parsed is false for a module goyang read from disk itself, resolving
	// an import: the loader never saw its text, so text is empty, and
	// statementHasBlock refuses to answer rather than read it.
	parsed bool
}

// NewLoader creates a new YANG module loader.
func NewLoader() *Loader {
	return &Loader{
		modules: &sourcedModules{Modules: yang.NewModules(), texts: map[*yang.Module]string{}},
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
// Call after LoadEmbedded() and before Resolve(). It attempts every module and
// joins every parse error, each naming its module, so a module registered
// after a broken one is still loaded and its own failure still reported.
func (l *Loader) LoadRegistered() error {
	var errs []error
	for _, mod := range modules {
		if err := l.AddModuleFromText(mod.Name, mod.Content); err != nil {
			errs = append(errs, fmt.Errorf("registered YANG module %s: %w", mod.Name, err))
		}
	}
	return errors.Join(errs...)
}

// AddModuleFromText adds a YANG module from text content.
func (l *Loader) AddModuleFromText(name, content string) error {
	if err := l.modules.Parse(content, name); err != nil {
		return fmt.Errorf("parse YANG: %w", err)
	}
	return nil
}

// AddModuleFromFile adds a YANG module from a file path. The file is read
// here rather than by goyang's Modules.Read, so its text is recorded as every
// parsed module's is.
func (l *Loader) AddModuleFromFile(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // the caller names the module file to load
	if err != nil {
		return fmt.Errorf("read YANG file %s: %w", path, err)
	}
	if err := l.modules.Parse(string(data), path); err != nil {
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
// each wrapping ErrUncompilablePattern, and every structure goyang accepts
// and RFC 7950 forbids (checkStructure): length parts that overlap or descend
// (ErrLengthOrder), an enum restriction that departs from its base type
// (ErrEnumRestriction), and a non-YANG statement under an extension
// (ErrExtensionSubstatement).
func (l *Loader) Resolve() error {
	return errors.Join(l.process(), l.checkExtensions(), l.checkPatterns(), l.checkStructure())
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
// extension's module was imported." A prefix that resolves to no declaring
// module breaks that requirement.
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

// patternOwnerKeywords are the statements whose name an operator recognizes
// as the owner of a `pattern`: the node or the named type the restriction
// constrains. The pattern error names the nearest one enclosing it.
var patternOwnerKeywords = []string{"leaf", "leaf-list", "typedef", "deviation"}

// pendingPattern is one statement on the modulePatternErrors walk, carrying
// the nearest enclosing owner statement (nil at the module's top level).
type pendingPattern struct {
	statement *yang.Statement
	owner     *yang.Statement
}

// modulePatternErrors returns one error for each `pattern` statement in mod
// that compilePattern cannot compile. Each error names the module, the leaf,
// leaf-list, typedef or deviation the pattern restricts, the source location, and the
// reason compilePattern gave, which quotes the pattern. The walk is an
// explicit stack, as in moduleExtensionErrors.
func modulePatternErrors(mod *yang.Module) []error {
	if mod.Source == nil {
		return nil
	}
	var errs []error
	var pending []pendingPattern
	for _, statement := range mod.Source.SubStatements() {
		pending = append(pending, pendingPattern{statement: statement})
	}
	for len(pending) > 0 {
		entry := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		owner := entry.owner
		if slices.Contains(patternOwnerKeywords, entry.statement.Keyword) {
			owner = entry.statement
		}
		for _, child := range entry.statement.SubStatements() {
			pending = append(pending, pendingPattern{statement: child, owner: owner})
		}
		if entry.statement.Keyword != "pattern" {
			continue
		}
		if _, err := compilePattern(entry.statement.Argument); err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %s: %w",
				ErrUncompilablePattern, mod.Name, patternOwnerName(owner),
				entry.statement.Location(), err))
		}
	}
	return errs
}

// patternOwnerName answers the owner statement as "<keyword> <name>", the form
// the operator wrote it in, or "module level" for a pattern no owner encloses.
func patternOwnerName(owner *yang.Statement) string {
	if owner == nil {
		return "module level"
	}
	return owner.Keyword + " " + owner.Argument
}

// sourceModules answers every loaded module and submodule, sorted by name,
// skipping the revision-qualified duplicate keys goyang also stores.
func (l *Loader) sourceModules() []*yang.Module {
	names := make([]string, 0, len(l.modules.Modules.Modules)+len(l.modules.Modules.SubModules))
	names = append(names, l.ModuleNames()...)
	for name := range l.modules.Modules.SubModules {
		if strings.Contains(name, "@") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	mods := make([]*yang.Module, 0, len(names))
	for _, name := range names {
		mod := l.modules.Modules.Modules[name]
		if mod == nil {
			mod = l.modules.Modules.SubModules[name]
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
// module it belongs to and in all submodules included by the module." So the
// owner's body and every submodule it includes are searched.
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
	return l.modules.Modules.Modules[name]
}

// GetEntry returns the processed entry tree for a module.
// The entry tree has all imports resolved and mandatory fields properly set.
func (l *Loader) GetEntry(name string) *yang.Entry {
	mod := l.modules.Modules.Modules[name]
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
	names := make([]string, 0, len(l.modules.Modules.Modules))
	for name := range l.modules.Modules.Modules {
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
