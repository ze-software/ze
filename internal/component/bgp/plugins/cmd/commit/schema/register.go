package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-commit-api.yang", ZeBgpCmdCommitAPIYANG)
	yang.RegisterModule("ze-cli-commit-cmd.yang", ZeCliCommitCmdYANG)
}
