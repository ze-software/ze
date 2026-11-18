// Design: docs/architecture/api/commands.md -- private artifact import and native selected restore.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-import", Handler: artifactHandler(artifactImport), AuditAction: audit.ActionDataConfigImport},
		pluginserver.RPCRegistration{WireMethod: "ze-config-cli:artifact-restore-config", Handler: artifactResponseHandler(artifactRestoreConfig), AuditAction: audit.ActionDataConfigRestore},
	)
	command.RegisterShape([]string{"request config artifact import", "request config artifact restore config"}, command.ShapeDoc)
}
