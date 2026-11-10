// Package hub wraps an error that gained "parse config" two calls down.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Reload adds "parse config", which loader.Load passes up from parse.
func Reload() error {
	if err := loader.Load(); err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}
	return nil
}
