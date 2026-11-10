// Package hub stands in for the reload as the daemon wires it: the loader
// arrives as a function value.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Reload wraps whatever load refused; which function load is, only a caller knows.
func Reload(load func() error) error {
	if err := load(); err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}
	return nil
}

// Run hands Reload the loader that already adds "parse config".
func Run() error {
	return Reload(loader.Parse)
}
