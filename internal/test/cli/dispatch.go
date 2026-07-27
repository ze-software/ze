// Design: docs/architecture/testing/ci-format.md -- ze-test root handler helpers

package cli

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

func registerRoot(name string, handler func([]string) int, desc string) {
	registry.MustRegisterRootHandler(name, func(_ *registry.RuntimeContext, args []string) int {
		return handler(args)
	}, registry.Meta{
		Description: desc,
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
}

// registerCIRoot registers one `ze-test <suite>` root handler.
//
// parallel is the DEFAULT for -p, not a ceiling. Zero means "no opinion, use the
// host-derived default" (runner.DefaultSuiteConcurrency), NOT "all at once":
// unset and all-at-once used to be the same value, which is how `ze-test ospf
// --all` came to launch 97 ze daemons simultaneously and kill CI's runner agent.
// An operator who genuinely wants that behavior still asks for it with `-p 0`.
func registerCIRoot(name, testSubdir, description, detail string, parallel int) {
	if parallel == 0 {
		parallel = runner.DefaultSuiteConcurrency()
	}
	cfg := CIRunnerConfig{Name: name, TestSubdir: testSubdir, Description: description, Detail: detail, DefaultParallel: parallel}
	var tb textbuf.Buffer
	registerRoot(name, func(args []string) int { return RunCISubcommand(cfg, args) }, tb.Str("Run ").Str(description).Str(" functional tests").String())
}
