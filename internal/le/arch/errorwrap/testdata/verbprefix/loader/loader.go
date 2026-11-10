// Package loader stands in for the set-format parser: its refusal names the
// line it met.
package loader

import "fmt"

// Parse refuses line 3, naming the line the way the parser does.
func Parse() error {
	return fmt.Errorf("line %d: unexpected brace", 3)
}
