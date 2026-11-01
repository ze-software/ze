// Design: docs/architecture/api/process-protocol.md -- Filter Declaration (Stage 1)
// Related: registry.go -- the other runtime seams the plugin server fills

package registry

import (
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DeclaredFilter is one Stage 1 filter declaration in the form the subsystem
// that runs filters judges: the name it answers for, exactly or by pattern, and
// the route fields it reads and sets. It is a value: nothing in it points into
// the plugin's declaration.
type DeclaredFilter struct {
	Name        string         // exact filter name; empty when Pattern is set
	Pattern     *regexp.Regexp // anchored to the whole name; nil when Name is set
	PatternText string         // the expression as the plugin declared it
	Reads       []string       // route fields read, rpc.FilterFieldRaw included
	Sets        []string       // route fields the filter may set
}

// Covers reports whether the declaration answers for filter name.
func (d *DeclaredFilter) Covers(name string) bool {
	if d.Pattern != nil {
		return d.Pattern.MatchString(name)
	}
	return d.Name == name
}

// Label names the declaration in an operator message: its name, or its
// pattern.
func (d *DeclaredFilter) Label() string {
	if d.Pattern != nil {
		return "pattern " + d.PatternText
	}
	return d.Name
}

// ReadsRaw reports whether the filter declared the raw UPDATE body read.
func (d *DeclaredFilter) ReadsRaw() bool {
	for _, read := range d.Reads {
		if read == rpc.FilterFieldRaw {
			return true
		}
	}
	return false
}

// FindDeclaredFilter answers the declaration that serves filter name: the one
// declaring it exactly, else the first pattern that covers it.
func FindDeclaredFilter(declared []DeclaredFilter, name string) (*DeclaredFilter, bool) {
	var covering *DeclaredFilter
	for i := range declared {
		d := &declared[i]
		if d.Pattern == nil && d.Name == name {
			return d, true
		}
		if covering == nil && d.Pattern != nil && d.Pattern.MatchString(name) {
			covering = d
		}
	}
	return covering, covering != nil
}

// LabelDeclaredFilters joins every declaration's label, for a refusal that
// lists what a plugin offers.
func LabelDeclaredFilters(declared []DeclaredFilter) string {
	labels := make([]string, len(declared))
	for i := range declared {
		labels[i] = declared[i].Label()
	}
	return strings.Join(labels, ", ")
}

// CompileFilterDeclarations turns a plugin's Stage 1 filters into declared
// filters, refusing an entry with both a name and a pattern, with neither, or
// with a pattern that does not compile. The name "*" was the text era's
// wildcard; a plugin covering every name declares the pattern ".+".
func CompileFilterDeclarations(pluginName string, filters []rpc.FilterDecl) ([]DeclaredFilter, error) {
	declared := make([]DeclaredFilter, 0, len(filters))
	for i := range filters {
		d, err := compileFilterDeclaration(pluginName, &filters[i])
		if err != nil {
			return nil, err
		}
		declared = append(declared, d)
	}
	return declared, nil
}

func compileFilterDeclaration(pluginName string, f *rpc.FilterDecl) (DeclaredFilter, error) {
	var tb textbuf.Buffer
	tb.Str("plugin ").Quoted(pluginName).Str(" declares a filter ")
	if f.Name != "" && f.NamePattern != "" {
		return DeclaredFilter{}, errors.New(tb.Str("with both name ").Quoted(f.Name).Str(" and name-pattern ").
			Quoted(f.NamePattern).Str("; declare one of them (docs/plugin-development/protocol.md)").String())
	}
	if f.Name == "" && f.NamePattern == "" {
		return DeclaredFilter{}, errors.New(tb.Str("with neither name nor name-pattern; declare one of them " +
			"(docs/plugin-development/protocol.md)").String())
	}
	if f.Name == "*" {
		return DeclaredFilter{}, errors.New(tb.Str(`named "*", the wildcard of the text era; declare ` +
			`"name-pattern": ".+" to answer for every name (docs/plugin-development/protocol.md)`).String())
	}
	d := DeclaredFilter{Name: f.Name, Reads: append([]string(nil), f.Reads...), Sets: append([]string(nil), f.Sets...)}
	if f.NamePattern == "" {
		return d, nil
	}
	pattern, err := regexp.Compile("^(?:" + f.NamePattern + ")$")
	if err != nil {
		return DeclaredFilter{}, errors.New(tb.Str("whose name-pattern ").Quoted(f.NamePattern).
			Str(" is not an RE2 expression: ").Err(err).String())
	}
	d.Pattern = pattern
	d.PatternText = f.NamePattern
	return d, nil
}

// filterDeclarations holds the two seams of filter declarations: the judge,
// filled from init() by the subsystem that runs filters, and the runtime
// answer, filled by the plugin server, which owns the running processes.
//
//nolint:gochecknoglobals // Package-level seams, same pattern as runtimeSessionReady.
var filterDeclarations struct {
	mu       sync.RWMutex
	check    func(pluginName string, declared []DeclaredFilter) error
	declared func(process string) ([]DeclaredFilter, bool)
}

// SetFilterDeclarationCheck installs the judge of Stage 1 filter
// declarations: the vocabulary of what a filter reads and sets belongs to the
// subsystem that runs the filters, which the plugin server cannot import.
// Called from init() by that subsystem; nil removes it. It answers the judge
// it replaced, so a test that installs its own restores the one before it.
func SetFilterDeclarationCheck(fn func(pluginName string, declared []DeclaredFilter) error) func(string, []DeclaredFilter) error {
	filterDeclarations.mu.Lock()
	defer filterDeclarations.mu.Unlock()
	previous := filterDeclarations.check
	filterDeclarations.check = fn
	return previous
}

// ErrLoadedChainRefused marks a Stage 1 refusal of a chain reference that a
// loaded config already holds, as distinct from a refusal of the declaration
// alone. The config holding that chain cannot be applied, so the plugin
// server fails daemon startup on it, whatever the plugin's failure policy,
// the way a config reload refuses the same chain at load.
var ErrLoadedChainRefused = errors.New("a chain in the loaded config names a filter its plugin's declaration refuses")

// RefuseLoadedChain marks err, the judge's refusal of a chain reference a
// loaded config holds, as ErrLoadedChainRefused. The message stays the
// judge's own.
func RefuseLoadedChain(err error) error { return loadedChainRefusal{err: err} }

// loadedChainRefusal is the error RefuseLoadedChain answers: the judge's
// message, matching both the judge's error and ErrLoadedChainRefused.
type loadedChainRefusal struct{ err error }

func (r loadedChainRefusal) Error() string   { return r.err.Error() }
func (r loadedChainRefusal) Unwrap() []error { return []error{r.err, ErrLoadedChainRefused} }

// CheckFilterDeclarations compiles a plugin's Stage 1 filters and hands them
// to the installed judge. A build with no judge links no subsystem that runs
// filters, so a plugin declaring one is refused rather than accepted unjudged.
func CheckFilterDeclarations(pluginName string, filters []rpc.FilterDecl) error {
	if len(filters) == 0 {
		return nil
	}
	declared, err := CompileFilterDeclarations(pluginName, filters)
	if err != nil {
		return err
	}
	filterDeclarations.mu.RLock()
	check := filterDeclarations.check
	filterDeclarations.mu.RUnlock()
	if check == nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("plugin ").Quoted(pluginName).Str(" declares route filters, and this build of ze " +
			"links no subsystem that runs them; build ze with BGP (the ze_bgp tag) or remove the filters").String())
	}
	return check(pluginName, declared)
}

// SetRuntimeFilterDeclarations installs the seam that answers a running
// process's Stage 1 filter declarations, so a config reload judges a chain
// naming an external filter against what that plugin declared. Called once at
// plugin server startup; nil removes it.
func SetRuntimeFilterDeclarations(fn func(process string) ([]DeclaredFilter, bool)) {
	filterDeclarations.mu.Lock()
	defer filterDeclarations.mu.Unlock()
	filterDeclarations.declared = fn
}

// RuntimeFilterDeclarations answers the filters a running process declared at
// Stage 1. False means the process has not declared yet (it is not running, or
// it is still before Stage 1), never that it declared no filter.
func RuntimeFilterDeclarations(process string) ([]DeclaredFilter, bool) {
	filterDeclarations.mu.RLock()
	declared := filterDeclarations.declared
	filterDeclarations.mu.RUnlock()
	if declared == nil {
		return nil, false
	}
	return declared(process)
}
