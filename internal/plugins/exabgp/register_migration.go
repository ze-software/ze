// Design: docs/exabgp/exabgp-migration.md -- owner-held migration command registrations.

package exabgp

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	_ "github.com/ze-software/ze/internal/plugins/exabgp/yang"
)

func init() {
	command.MustRegisterLocalData("show exabgp migration config", migrationSupplied(
		"show exabgp migration config", "supplied.conf", migrationConfigBytes), registry.Meta{
		ShortHelp: "Convert supplied ExaBGP configuration bytes to Ze configuration.",
		Mode:      "offline", SuppliedInput: &registry.SuppliedInput{ContentArg: "content", NameArg: "name"},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show exabgp migration environment", migrationSupplied(
		"show exabgp migration environment", "supplied.env", migrationEnvironmentBytes), registry.Meta{
		ShortHelp: "Convert supplied ExaBGP environment bytes to Ze configuration.",
		Mode:      "offline", SuppliedInput: &registry.SuppliedInput{ContentArg: "content", NameArg: "name"},
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{
		"show exabgp migration config", "show exabgp migration environment",
	}, command.ShapeBlob)
}
