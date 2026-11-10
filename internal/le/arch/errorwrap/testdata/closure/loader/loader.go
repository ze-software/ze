// Package loader stands in for the config loader: it prefixes its refusal once.
package loader

import (
	"errors"
	"fmt"
)

var errSyntax = errors.New("unexpected brace")

// Parse refuses every input, prefixed the way the config loader prefixes it.
func Parse() error {
	return fmt.Errorf("parse config: %w", errSyntax)
}
