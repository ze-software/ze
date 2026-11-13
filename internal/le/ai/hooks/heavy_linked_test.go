// Design: docs/contributing/running-commands.md -- the Bash guard reads what each area declared
// Related: parity_catalog.go -- the piped-command rows that name these areas
// Related: expensive_test.go -- the same linking need, for this package's own action
//
// The piped-command guard holds no list of areas: it answers from what each
// area declared where its verb registers (leroot.HeavyLine). The parity rows
// judge `./le verify current` and `./le test unit all` piped through a
// filter, so the test binary links those areas, as the le that runs the hook
// selftest links every one. None of them imports this package.

package aihooks

import (
	_ "github.com/ze-software/ze/internal/le/go/lint"
	_ "github.com/ze-software/ze/internal/le/test/unit"
	_ "github.com/ze-software/ze/internal/le/verify"
	_ "github.com/ze-software/ze/internal/le/verify/deps"
)
