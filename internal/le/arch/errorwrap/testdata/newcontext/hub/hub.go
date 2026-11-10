// Package hub stands in for the reload: it adds only the context it holds.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Reload adds "reload", which loader.Parse does not carry.
func Reload() error {
	if err := loader.Parse(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}
	return nil
}
