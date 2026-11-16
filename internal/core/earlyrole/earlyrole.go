// Design: docs/architecture/system-architecture.md -- a role selected by an environment key before command dispatch
// Related: ../seedinit/register.go -- the session seed role, which checks arguments and descriptors this leaf's roles have none of
//
// Package earlyrole holds the roles a ze process takes in place of its command
// line. A role is selected by an environment key its owner registered, and it
// runs before crash logging, version stamping and command dispatch.
//
// The leaf names no feature: the package that owns a role registers the key
// and the function from its own init(), so a build without that package holds
// no role and Dispatch answers "not handled" for every environment.
package earlyrole

import (
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Role is the whole run of a process selected by its key. Its result is the
// process exit code. A role that replaces the process image never returns.
type Role func() int

type entry struct {
	key  string
	role Role
}

// roles is written only by Register, from package init functions, and read
// only by Dispatch, from main before any goroutine of ze starts. Neither is
// safe for concurrent use, and neither needs to be.
var roles []entry

// Register installs role as the run of every process whose environment holds
// key. It MUST be called from a package init function. key is the name of an
// environment variable only that role's parent sets.
//
// An empty key, a nil role and a key registered twice are Ze defects and
// panic: no input reaches this call.
func Register(key string, role Role) {
	if key == "" {
		panic("BUG: early role registered with an empty environment key")
	}
	if role == nil {
		panic("BUG: early role " + key + " registered with no function")
	}
	for _, registered := range roles {
		if registered.key == key {
			panic("BUG: early role " + key + " registered twice")
		}
	}
	roles = append(roles, entry{key: key, role: role})
}

// Dispatch runs the role whose key the environment holds and answers its exit
// code with handled true. With no registered key in the environment it
// answers handled false, and ordinary startup goes on.
//
// A key is selected by its presence, whatever its value: the role's own
// start-up judges the value, so a damaged value is refused there and never
// read as an ordinary command line. Two registered keys in one environment
// name two roles for one process, which is refused before either runs.
func Dispatch() (handled bool, code int) {
	var selected *entry
	for i := range roles {
		// The key is the variable of another program's protocol, never a Ze
		// setting, so it is read from the process environment directly.
		if _, present := os.LookupEnv(roles[i].key); !present {
			continue
		}
		if selected != nil {
			var tb textbuf.Buffer
			tb.Str("ze: the environment selects two start-up roles, ").Str(selected.key).
				Str(" and ").Str(roles[i].key).Str(", and a process runs one\n").
				StdErr() //nolint:errcheck // Startup refusal: stderr is the only channel.
			return true, 2
		}
		selected = &roles[i]
	}
	if selected == nil {
		return false, 0 // No role was requested: ordinary startup.
	}
	return true, selected.role()
}

// Snapshot answers a function that puts the registered roles back as they are
// now. A test that registers a role MUST take it first and call the result in
// its cleanup, so the role does not reach the next test.
func Snapshot() (restore func()) {
	saved := append([]entry(nil), roles...)
	return func() { roles = saved }
}
