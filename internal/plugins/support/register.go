// Design: docs/architecture/core-design.md -- request support archive command registration

package support

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	impl "github.com/ze-software/ze/internal/component/support"
)

func init() {
	command.MustRegisterLocalMeta("request support archive", impl.Run, registry.Meta{
		ShortHelp: "Collect logs, config, state and diagnostics into one archive.",
		Description: "Send the archive to support when you report an issue. Modules can be selected or " +
			"excluded, and a time window narrows what the archive holds.",
		Mode: "offline",
	})
}
