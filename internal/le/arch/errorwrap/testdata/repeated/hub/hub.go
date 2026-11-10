// Package hub stands in for the reload: it prefixes the loader's refusal again.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Reload adds "parse config", which loader.Parse already added.
func Reload() error {
	if err := loader.Parse(); err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}
	return nil
}
