// Design: docs/architecture/config/yang-config-design.md — YANG schema handling
// RFC: rfc/short/rfc7950.md -- Sections 5.1 and 6.3.1, extension prefix resolution
//
// Package yang provides YANG schema loading and validation for ze.
package yang

import (
	"embed"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/openconfig/goyang/pkg/yang"
)

// DefaultLoader loads every embedded and registered module into a new Loader,
// resolves it, and answers the Resolved, or no value when anything failed. Nothing is
// best-effort: a registered module that does not parse, an import nothing
// registers, an undeclared extension, a pattern compilePattern cannot compile
// and a structure checkStructure refuses each come back, joined, so the caller
// cannot serve a schema that silently lacks a module or a restriction.
// LoadRegistered attempts every module, so one broken module does not hide
// the failures of the modules registered after it.
//
// The answer is resolved once per registered module set and shared by every
// caller in the process. Resolving the whole model is the most expensive
// thing a CLI model, an editor session or a daemon component does at
// construction, and every one of them asked for the same module set: 178
// parallel editor tests each resolved it again for every model, session and
// restart. The cache is keyed by the registry generation (modulesGeneration),
// so a test that registers a module for its own duration
// (RegisterModuleForTest) is resolved over that set, and the shipped set is
// resolved again once its restore runs. A failure is shared too: the same
// module set cannot fail differently on a retry. The cache holds one entry,
// the set of the latest generation asked for.
//
// Once the process published the modules its external plugins declared
// (PublishExternalModules), the answer is the compiled-in set joined to them,
// for the rest of the process. A caller that took the Resolved before the
// publish keeps the set it took.
//
// The shared Resolved is read-only: the Loader that produced it is
// unreachable, so no module can be added to it (ErrLoaderResolved guards the
// loader itself), and Resolve converted every module and submodule to its
// entry tree, so a later GetEntry reads goyang's locked entry cache. Callers
// MUST NOT write to a *yang.Entry or *yang.Module it answers. Safe for
// concurrent use; concurrent first callers wait for the one resolution.
func DefaultLoader() (*Resolved, error) {
	defaultSchema.mu.Lock()
	defer defaultSchema.mu.Unlock()
	return moduleSetLocked()
}

// moduleSetLocked answers the process's module set for the registry's
// generation and registered bus participant targets, resolving it when the
// cached entry is of another generation, target set, or holds nothing. The
// caller MUST hold defaultSchema.mu.
func moduleSetLocked() (*Resolved, error) {
	targets := BusParticipantTargets()
	entry := &defaultSchema.entry
	if entry.answers(modulesGeneration, targets) {
		return entry.schema, entry.err
	}
	schema, err := resolveProcessSet()
	*entry = moduleSetEntry{
		resolved:   true,
		generation: modulesGeneration,
		busTargets: targets,
		schema:     schema,
		err:        err,
	}
	return schema, err
}

// defaultSchema is DefaultLoader's state: a one-entry cache and the external
// layer of the process's module set.
var defaultSchema struct {
	mu sync.Mutex
	// entry is the answer for the module set of one registry generation.
	entry moduleSetEntry
	// external is the modules external plugins declared, nil until
	// PublishExternalModules published them. It is written once per process,
	// under mu, and a test's ResetPublishedModulesForTest alone clears it.
	external *publishedModules
}

// moduleSetEntry is DefaultLoader's cached answer for the module set of
// registry generation `generation`. resolved names whether the entry holds an
// answer at all, because generation 0 is a real generation (a binary that
// registers no module) and so cannot mean "empty". After a publish, schema is
// the joined set.
type moduleSetEntry struct {
	resolved   bool
	generation uint64
	busTargets []string
	schema     *Resolved
	err        error
}

// answers reports whether the entry holds the answer for generation and
// bus participant targets.
func (e *moduleSetEntry) answers(generation uint64, targets []string) bool {
	if !e.resolved {
		return false
	}
	if e.generation != generation {
		return false
	}
	return slices.Equal(e.busTargets, targets)
}

// resolveProcessSet resolves the compiled-in modules and joins the published
// external modules to them. A publish swaps the joined set into the cached
// entry itself, so this join runs only when the registry's generation changed
// after a publish, which a test alone does (RegisterModuleForTest). The caller
// MUST hold defaultSchema.mu.
func resolveProcessSet() (*Resolved, error) {
	compiled, err := resolveDefault()
	if err != nil {
		return nil, err
	}
	published := defaultSchema.external
	// refuse-by-default: guards moduleSetLocked -- before any external publication the compiled module set is the complete process schema
	if published == nil {
		return compiled, nil
	}
	return Join(compiled, published.modules, published.validators)
}

// resolveDefault runs the work DefaultLoader caches: every embedded and
// registered module into a new Loader, synthesized bus participant augments,
// then Resolve.
func resolveDefault() (*Resolved, error) {
	l := NewLoader()
	if err := l.LoadEmbedded(); err != nil {
		return nil, fmt.Errorf("YANG LoadEmbedded: %w", err)
	}
	registeredErr := l.LoadRegistered()
	busErr := l.loadBusParticipants()
	schema, resolveErr := l.Resolve()
	if err := errors.Join(registeredErr, busErr, resolveErr); err != nil {
		return nil, err
	}
	return schema, nil
}

//go:embed modules
var embeddedModules embed.FS

// Loader collects YANG modules before resolution. It holds the loading
// operations and one transition, Resolve; every read of the module set is a
// method of Resolved, which only a successful Resolve produces, so no caller
// can read a module set whose checks failed or never ran. Not safe for
// concurrent use.
type Loader struct {
	modules *sourcedModules
}

// ErrLoaderResolved marks a load or a second Resolve on a Loader that Resolve
// already ran on. The Resolved that Resolve returned shares the loader's module
// set, so a module added afterwards would reach readers unchecked.
var ErrLoaderResolved = errors.New("YANG loader already resolved")

// Resolved is a YANG module set that Loader.Resolve resolved and checked:
// every import resolves, every extension is declared, every pattern compiles
// and every structure passes checkStructure. Only a successful Resolve (or
// DefaultLoader) produces one, and the operations that need a checked module
// set (BuildCommandTree, NewValidator, the PathTo maps, ExtractRPCs, ...)
// accept only this type.
//
// The zero value and nil are invalid: only Ze code can build either, and no
// input reaches one, so every accessor ends in a BUG panic on them. The set
// cannot change after resolution, because the Loader refuses every load once
// Resolve ran (ErrLoaderResolved). Safe for concurrent reads: Resolve
// (goyang's Modules.Process) converts every module and submodule to its entry
// tree and caches it, so GetEntry is a read of goyang's lock-guarded entry
// cache (yang.ToEntry). The entries and modules it answers are pointers into
// that shared set, and a reader MUST NOT write to them: DefaultLoader hands one
// Resolved to every caller in the process.
type Resolved struct {
	modules *sourcedModules
	// patterns holds every pattern of the set's modules, compiled by
	// checkPatterns, keyed by the pattern text, so the command lowering reads
	// a checked pattern instead of compiling it again. For a joined set that
	// is the patterns of the set it was joined to and of the external
	// modules.
	patterns compiledPatterns
}

// set answers the checked module set, and panics on a zero or nil Resolved.
func (r *Resolved) set() *sourcedModules {
	if r == nil {
		panic("BUG: a nil yang.Resolved holds no module set; only Loader.Resolve and DefaultLoader produce one")
	}
	if r.modules == nil {
		panic("BUG: a zero yang.Resolved holds no module set; only Loader.Resolve and DefaultLoader produce one")
	}
	return r.modules
}

// sourcedModules is goyang's module set, keeping the text each module was
// parsed from. goyang's Statement records no block when a statement has no
// substatement, so "refine x;" and "refine x {}" parse alike; the text is
// what tells them apart (statementHasBlock). Not safe for concurrent loads,
// as yang.Modules is not; once Resolve has run, readers share it read-only.
type sourcedModules struct {
	*yang.Modules
	// texts is the text of each module and submodule Parse added, keyed by
	// the module goyang built from it. A file name cannot be the key: two
	// texts parsed under one name would share it, and the second would
	// answer for the first. A module goyang read from disk itself, resolving
	// an import, has no entry.
	texts map[*yang.Module]string
	// judged holds the modules that already passed checkExtensions,
	// checkPatterns and checkStructure when another set holding the same text
	// was resolved, and judgedPatterns every pattern that resolve compiled for
	// them. The three checks skip a judged module (sourceModules), and
	// checkPatterns starts from judgedPatterns, so the Resolved still holds
	// every pattern of its modules. Only the join marks modules judged
	// (markJudged), on the loader that reloads a resolved set beside external
	// modules. Empty on a loader NewLoader built: nothing is judged, so every
	// module is checked.
	judged         map[*yang.Module]bool
	judgedPatterns compiledPatterns
	// resolved is set by the first Resolve, successful or not. Every load
	// after it is refused, so a Resolved reading this set never sees a module
	// that the checks did not.
	resolved bool
}

// markJudged marks every module and submodule the set holds now as judged,
// with patterns the compiled form of every pattern they declare. The caller
// MUST hold the proof that each of them passed the per-module checks: it was
// parsed from the text of a module of a Resolved, and patterns is that
// Resolved's. A module added afterwards is not judged, so the checks judge
// it.
func (m *sourcedModules) markJudged(patterns compiledPatterns) {
	m.judged = m.held()
	m.judgedPatterns = patterns
}

// refuseLoad answers ErrLoaderResolved once Resolve has run.
func (m *sourcedModules) refuseLoad() error {
	if m.resolved {
		return ErrLoaderResolved
	}
	return nil
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
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
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
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
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
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
	if err := l.modules.Parse(content, name); err != nil {
		return fmt.Errorf("parse YANG: %w", err)
	}
	return nil
}

// AddModuleFromFile adds a YANG module from a file path. The file is read
// here rather than by goyang's Modules.Read, so its text is recorded as every
// parsed module's is.
func (l *Loader) AddModuleFromFile(path string) error {
	if err := l.modules.refuseLoad(); err != nil {
		return err
	}
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
// and RFC 7950 forbids (checkStructure): an include of a submodule that
// belongs to another module (ErrIncludeBelongsTo), length parts that overlap or descend
// (ErrLengthOrder), an enum restriction that departs from its base type
// (ErrEnumRestriction), a non-YANG statement under an extension
// (ErrExtensionSubstatement), and a leafref whose path names no leaf, whose
// require-instance config leaf refers to state, or whose chain of leafrefs
// loops (ErrLeafrefPath, checkReferences).
//
// goyang's processing and checkReferences judge the whole set. The extension,
// pattern and structure checks judge each module that is not judged already
// (sourceModules): on a loader NewLoader built, every module; on the loader a
// join builds, the external modules alone (markJudged).
//
// It answers the Resolved only when every check passed, and no value
// otherwise. It runs once: afterwards, successful or not, every load and every
// further Resolve on l returns ErrLoaderResolved.
func (l *Loader) Resolve() (*Resolved, error) {
	if err := l.modules.refuseLoad(); err != nil {
		return nil, err
	}
	l.modules.resolved = true
	processErr := l.process()
	extensionErr := l.checkExtensions()
	patterns, patternErr := l.checkPatterns()
	structureErr := l.checkStructure()
	if err := errors.Join(processErr, extensionErr, patternErr, structureErr); err != nil {
		return nil, err
	}
	resolved := &Resolved{modules: l.modules, patterns: patterns}
	if err := checkReferences(resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

// process runs goyang's import and type resolution over every loaded module.
func (l *Loader) process() error {
	errs := l.modules.Process()
	if len(errs) == 0 {
		errs = l.modules.augmentErrors()
	}
	if len(errs) > 0 {
		return &processError{text: fmt.Errorf("resolve YANG modules: %v", errs), errs: errs}
	}
	return nil
}

// processError is the errors goyang answered for one Process, kept one by one
// so a join can read each (Join names the module an error begins in). Its text
// is the one process always wrote, "resolve YANG modules: [<each error>]".
type processError struct {
	text error
	errs []error
}

func (e *processError) Error() string { return e.text.Error() }

// Unwrap answers each of goyang's errors.
func (e *processError) Unwrap() []error { return e.errs }

// augmentErrors answers the errors goyang recorded on an entry while it
// applied the augments, which Modules.Process does not report: Entry.merge
// records a node an augment adds that its target already holds on the
// target's entry, and Process collects the entry errors only of a module that
// still holds an augment it could not apply, so the added node was dropped in
// silence. Called after a Process that reported nothing, when every entry tree
// is built. Sorted by text and without repeats, as a module's errors are
// reached through each of its keys.
func (m *sourcedModules) augmentErrors() []error {
	held := m.held()
	perModule := make([][]error, 0, len(held))
	for mod := range held {
		perModule = append(perModule, yang.ToEntry(mod).GetErrors())
	}
	errs := slices.Concat(perModule...)
	slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	return slices.CompactFunc(errs, func(a, b error) bool { return a.Error() == b.Error() })
}

// moduleCheckErrors answers what checkExtensions, checkPatterns and
// checkStructure, the three per-module checks Resolve runs, refuse in mod
// alone, over l's set as goyang processed it. A join reads it to name the
// module a failed resolve's check refused without reading the errors' text.
// l is not changed.
func (l *Loader) moduleCheckErrors(mod *yang.Module) error {
	judged := l.modules.held()
	delete(judged, mod)
	alone := &Loader{modules: &sourcedModules{
		Modules: l.modules.Modules, texts: l.modules.texts, judged: judged, resolved: true,
	}}
	extensionErr := alone.checkExtensions()
	_, patternErr := alone.checkPatterns()
	structureErr := alone.checkStructure()
	return errors.Join(extensionErr, patternErr, structureErr)
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
	mods := l.modules.sourceModules()
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, moduleExtensionErrors(mod)...)
	}
	return errors.Join(errs...)
}

// checkPatterns walks the statements of every loaded module and submodule and
// returns one error per `pattern` statement that compilePattern refuses,
// joined, and every pattern it compiled, keyed by its text. goyang keeps a pattern without compiling it (types.go: "These
// patterns are not checked because there is no support for W3C regexes by
// Go"), so this is the only place an uncompilable one is refused. Without it
// the config validator reported the pattern on every value and the command
// argument builder dropped it, leaving the argument open to any string.
//
// The answer also holds the patterns of the judged modules (markJudged), which
// the walk skips: a Resolved holds every pattern of its modules.
func (l *Loader) checkPatterns() (compiledPatterns, error) {
	mods := l.modules.sourceModules()
	compiled := make(compiledPatterns, len(l.modules.judgedPatterns))
	maps.Copy(compiled, l.modules.judgedPatterns)
	errs := make([]error, 0, len(mods))
	for _, mod := range mods {
		errs = append(errs, modulePatternErrors(mod, compiled)...)
	}
	return compiled, errors.Join(errs...)
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
// that compilePattern cannot compile, and records each one it compiled in
// compiled. Each error names the module, the leaf,
// leaf-list, typedef or deviation the pattern restricts, the source location, and the
// reason compilePattern gave, which quotes the pattern. The walk is an
// explicit stack, as in moduleExtensionErrors.
func modulePatternErrors(mod *yang.Module, compiled compiledPatterns) []error {
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
		re, err := compilePattern(entry.statement.Argument)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: module %s: %s: %s: %w",
				ErrUncompilablePattern, mod.Name, patternOwnerName(owner),
				entry.statement.Location(), err))
			continue
		}
		compiled[entry.statement.Argument] = re
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

// sourceModules answers the modules and submodules the per-module checks
// (checkExtensions, checkPatterns, checkStructure) judge, sorted by name: every
// loaded one except those markJudged marked, skipping the revision-qualified
// duplicate keys goyang also stores. Each of those checks reads one module's
// statements and the modules it imports or includes, never another module of
// the set, so a judged module's verdict is the one it already had.
func (m *sourcedModules) sourceModules() []*yang.Module {
	names := make([]string, 0, len(m.Modules.Modules)+len(m.Modules.SubModules))
	names = append(names, m.moduleNames()...)
	for name := range m.Modules.SubModules {
		if strings.Contains(name, "@") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	mods := make([]*yang.Module, 0, len(names))
	for _, name := range names {
		mod := m.Modules.Modules[name]
		if mod == nil {
			mod = m.Modules.SubModules[name]
		}
		if m.judged[mod] {
			continue
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

// GetModule returns a resolved module by name.
func (r *Resolved) GetModule(name string) *yang.Module {
	return r.set().Modules.Modules[name]
}

// GetEntry returns the processed entry tree for a module.
// The entry tree has all imports resolved and mandatory fields properly set.
func (r *Resolved) GetEntry(name string) *yang.Entry {
	mod := r.set().Modules.Modules[name]
	if mod == nil {
		return nil
	}
	return yang.ToEntry(mod)
}

// ModuleNames returns the name of every resolved module, once each.
func (r *Resolved) ModuleNames() []string {
	return r.set().moduleNames()
}

// moduleNames returns the name of every loaded module, once each.
//
// goyang keys a module that declares a revision under TWO names, its bare name
// and `<name>@<revision>` (vendor/github.com/openconfig/goyang/pkg/yang/
// modules.go, Modules.add), and the bare name always names the most recent
// revision. 205 of Ze's modules declare one, so a caller that walks this map
// raw visits each of them twice and counts what it finds there twice with it.
func (m *sourcedModules) moduleNames() []string {
	names := make([]string, 0, len(m.Modules.Modules))
	for name := range m.Modules.Modules {
		if strings.Contains(name, "@") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ConfModuleNames returns sorted names of resolved config modules (suffix "-conf").
func (r *Resolved) ConfModuleNames() []string {
	return r.moduleNamesBySuffix("-conf")
}

// APIModuleNames returns sorted names of resolved API modules (suffix "-api").
func (r *Resolved) APIModuleNames() []string {
	return r.moduleNamesBySuffix("-api")
}

func (r *Resolved) moduleNamesBySuffix(suffix string) []string {
	var names []string
	for _, name := range r.ModuleNames() {
		if strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
