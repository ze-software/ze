// Design: docs/architecture/cli/plugin-modes.md -- offline subcommand flag inventory

package registry

import "sort"

// Value-hint kinds for FlagSpec.ValueHint. A shell can complete the flag's
// value when the hint names a known value source; "" means the flag is a
// boolean/switch with no value.
const (
	FlagValueNone   = ""
	FlagValueFamily = "family"
	FlagValueFile   = "file"
)

// FlagSpec describes one offline subcommand flag so shell completion can offer
// it by name and, when it carries a value, complete that value. It is the
// registration-over-hardcoding replacement for the per-subcommand flag lists
// the shell generators used to spell inline.
type FlagSpec struct {
	Name        string // full flag token, e.g. "--family"
	Description string // one-line help shown next to the flag
	ValueHint   string // one of the FlagValue* constants
}

var commandFlags = make(map[string][]FlagSpec)

// RegisterCommandFlags records the flags a subcommand path accepts. The path is
// the space-separated command path the operator types after `ze`
// (for example "exabgp plugin"). Registering the same path again replaces the
// prior set. Safe for concurrent use; call it from init()/register.go.
func RegisterCommandFlags(path string, flags []FlagSpec) {
	mu.Lock()
	defer mu.Unlock()
	cp := make([]FlagSpec, len(flags))
	copy(cp, flags)
	commandFlags[path] = cp
}

// CommandFlags returns the flags registered for a command path, sorted by name.
// It returns nil when nothing is registered for the path.
func CommandFlags(path string) []FlagSpec {
	mu.RLock()
	defer mu.RUnlock()
	flags, ok := commandFlags[path]
	if !ok {
		return nil
	}
	out := make([]FlagSpec, len(flags))
	copy(out, flags)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CommandFlagPaths returns every registered flag-bearing command path, sorted.
// Used by tests and introspection.
func CommandFlagPaths() []string {
	mu.RLock()
	defer mu.RUnlock()
	paths := make([]string, 0, len(commandFlags))
	for p := range commandFlags {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
