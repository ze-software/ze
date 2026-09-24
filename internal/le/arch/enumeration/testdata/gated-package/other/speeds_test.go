// A second declaration of the same test name, in a package that holds no
// gated table. It is what bare/rates.go must NOT resolve to, and what
// spelled/modes.go names by package.
package other

import "testing"

func TestSpeedsMatchTheModel(t *testing.T) {}
