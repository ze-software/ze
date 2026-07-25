// Design: docs/architecture/testing/ci-format.md -- ze-test root handler helpers

package cli

import (
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
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

func registerCIRoot(name, testSubdir, description, detail string, parallel int) {
	cfg := CIRunnerConfig{Name: name, TestSubdir: testSubdir, Description: description, Detail: detail, DefaultParallel: parallel}
	var tb textbuf.Buffer
	registerRoot(name, func(args []string) int { return RunCISubcommand(cfg, args) }, tb.Str("Run ").Str(description).Str(" functional tests").String())
}
