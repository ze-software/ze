// Package hub stands in for config validate: it renders the parser's refusal
// under a line prefix the refusal already carries.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Validate adds "line N", which loader.Parse already wrote.
func Validate(line int) error {
	if err := loader.Parse(); err != nil {
		return fmt.Errorf("line %d: parse config: %w", line, err)
	}
	return nil
}

// Open adds the file, which the refusal does not name: no finding.
func Open(path string) error {
	if err := loader.Parse(); err != nil {
		return fmt.Errorf("file %s: %w", path, err)
	}
	return nil
}

// Name leads with a bare value, a segment that holds no word: no wrap the
// check judges, since two bare values match whatever they print.
func Name(name string) error {
	if err := loader.Parse(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
