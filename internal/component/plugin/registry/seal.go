// Design: docs/architecture/plugin/plugin-system.md -- registration fields, the seal after start-up
// Related: registry.go -- Register, the registering function the seal guards
//
// seal.go closes the registry once every package initializer has run. Every
// plugin registers from init() in its register.go, and Go runs every init()
// before main, so the cmd/ze main seals before it dispatches. A registration
// after that point comes from code that holds a reference to a registering
// function at run time, which the declared registration walk cannot see; the
// seal turns it into a refusal instead of a silent change to what the binary
// carries.

package registry

import (
	"errors"
	"fmt"
)

// ErrRegistrySealed is the class of every refusal of a registering call made
// after Seal.
var ErrRegistrySealed = errors.New("registry: sealed after start-up")

// sealed is true once Seal ran. Guarded by mu.
var sealed bool

// Seal closes the registry to registration. It MUST be called once, by the
// program's main, after every package initializer has run and before any
// command dispatches; every registering function MUST call refuseSealedLocked
// under mu before it writes. Safe for concurrent use.
func Seal() {
	mu.Lock()
	defer mu.Unlock()
	sealed = true
}

// refuseSealedLocked is the seal guard. It answers ErrRegistrySealed, naming
// what was refused (kind, such as "plugin", and its name), once Seal ran, and
// nil before. The caller MUST hold mu for writing and MUST call it before its
// first write, so a refused call leaves every index untouched.
func refuseSealedLocked(kind, name string) error {
	if !sealed {
		return nil
	}
	return fmt.Errorf("%w: %s %q: registration is allowed only during package initialization; "+
		"move the call into the init() of the plugin's register.go", ErrRegistrySealed, kind, name)
}
