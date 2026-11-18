// Design: docs/architecture/api/commands.md -- caller-held completion and private codec inputs.

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	command.MustRegisterLocalData("show config supplied completion", suppliedHandler("show config supplied completion", suppliedCompletion), registry.Meta{
		ShortHelp: "Complete literal editing input against supplied configuration.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent, Fields: []string{suppliedConfiguration, suppliedInput}},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show secret encode", suppliedSecret("encode", "show secret encode"), registry.Meta{
		ShortHelp: "Encode a supplied private value in the reversible $9$ form.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent},
	}, command.RenderLocalAnswer)
	command.MustRegisterLocalData("show secret decode", suppliedSecret("decode", "show secret decode"), registry.Meta{
		ShortHelp: "Decode a supplied $9$ value to its plaintext.", Mode: modeOffline,
		SuppliedInput: &registry.SuppliedInput{ContentArg: suppliedContent},
	}, command.RenderLocalAnswer)
	command.RegisterShape([]string{"show config supplied completion"}, command.ShapeDoc)
	command.RegisterShape([]string{"show secret encode", "show secret decode"}, command.ShapeBlob)
}
