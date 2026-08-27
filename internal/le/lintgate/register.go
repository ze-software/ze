// Design: docs/architecture/core-design.md -- le's composition, one import per tool
package lintgate

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leroot"
)

func init() {
	leroot.Register(area, Answer, registry.Meta{
		Description: "run golangci-lint over every Go build flavor and prove tracked-file coverage",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subs,
	})
	leroot.RegisterShape(area, command.ShapeDoc)

	// `verify-lint run` is an internal verifier verb rather than a second
	// spelling of ze-lint. It therefore makes no parity denominator claim.
}
