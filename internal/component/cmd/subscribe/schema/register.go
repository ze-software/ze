package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-bgp-cmd-subscribe-api.yang", ZeBgpCmdSubscribeAPIYANG)
	yang.RegisterModule("ze-cli-subscribe-cmd.yang", ZeCliSubscribeCmdYANG)
}
