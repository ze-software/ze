// Package loader passes its parser's refusal up through one more function.
package loader

import (
	"errors"
	"fmt"
)

var errSyntax = errors.New("unexpected brace")

// Load hands back what parse refused, unwrapped.
func Load() error {
	if err := parse(); err != nil {
		return err
	}
	return nil
}

// parse prefixes its refusal once.
func parse() error {
	return fmt.Errorf("parse config: %w", errSyntax)
}
