// Design: docs/contributing/gh-pages.md -- live topics use the existing scenario registry
// Related: validate_topics.go -- shared SSH commit and lab lifetime

package siteterminaldemo

import (
	"fmt"
	"io"
)

// registerLiveTopic binds the standalone validator and the showcase check to
// one scenario. await observes a named stage without changing configuration.
func registerLiveTopic(id, fragment string, check func() error, await func(string) error) {
	if check == nil {
		panic("BUG: live topic " + id + " has no check")
	}
	if await == nil {
		panic("BUG: live topic " + id + " has no readiness check")
	}
	if fragment == "" {
		panic("BUG: live topic " + id + " has no authored fragment")
	}
	scenarios.add(id, scenario{
		run: func(action string, args []string, stdout io.Writer) error {
			if action != "await" {
				return runShowcaseLab(action, args, stdout)
			}
			if len(args) != 1 {
				return fmt.Errorf("%s await needs exactly one stage", id)
			}
			if err := await(args[0]); err != nil {
				return err
			}
			return writeLine(stdout, "Topic ready")
		},
		validate: func() error { return validateLiveTopic(check) },
		live:     &liveTopic{fragment: fragment, check: check},
	})
}
