// Design: docs/architecture/api/commands.md -- the argument definitions every dispatch route judges
// Related: command.go -- PathToArgDefs, which builds them

package yang

import (
	"sync"

	"github.com/ze-software/ze/internal/component/command"
)

// modelArgDefs holds PathToArgDefs over the default model once it has been
// built, or once a process that already built it seeded it
// (SeedCommandArgDefs). Only a SUCCESS is kept: the shipped model is embedded
// and cannot fail differently on a retry, but a test that registers a broken
// module for its own duration (RegisterModuleForTest) must not leave every
// later caller in the process refused. sync.OnceValues would keep that
// failure forever.
var (
	modelArgDefsMu sync.Mutex
	modelArgDefs   map[string][]command.ArgDef
)

// SeedCommandArgDefs installs defs as the argument definitions every
// dispatch route judges a command against (command.ValidateModelArgs). defs
// MUST be PathToArgDefs over the schema DefaultLoader answered, which is the
// model commandArgDefs would otherwise build the definitions from on first use.
//
// The daemon seeds it at startup because building them on first use is not
// free: before DefaultLoader shared one resolution per process, the model was
// loaded and resolved a second time there, about
// 70 ms on a laptop, paid by the first plugin command, which then reached the
// plugin after the first BGP UPDATE it was meant to precede. Safe for
// concurrent use; the map is shared and MUST NOT be written after the call.
func SeedCommandArgDefs(defs map[string][]command.ArgDef) {
	modelArgDefsMu.Lock()
	defer modelArgDefsMu.Unlock()
	modelArgDefs = defs
}

// ResetCommandArgDefsForTest empties the argument definitions commandArgDefs
// holds, so the next call loads the model, and returns the function that
// restores them. A test in another package uses it to prove a producer
// seeded the definitions rather than inheriting them from an earlier test.
// MUST be paired with the returned restore, typically through t.Cleanup. Not
// safe for use in parallel with another test that validates arguments.
func ResetCommandArgDefsForTest() (restore func()) {
	modelArgDefsMu.Lock()
	saved := modelArgDefs
	modelArgDefs = nil
	modelArgDefsMu.Unlock()
	return func() { SeedCommandArgDefs(saved) }
}

// commandArgDefs is the command.ArgDefSource every dispatch route reads: the
// argument definitions the default model declares at path, nil when it
// declares none. The first call loads the model unless SeedCommandArgDefs ran.
// Safe for concurrent use.
func commandArgDefs(path string) ([]command.ArgDef, error) {
	modelArgDefsMu.Lock()
	defer modelArgDefsMu.Unlock()
	if modelArgDefs == nil {
		schema, err := DefaultLoader()
		if err != nil {
			return nil, err
		}
		modelArgDefs = PathToArgDefs(schema)
	}
	return modelArgDefs[path], nil
}
