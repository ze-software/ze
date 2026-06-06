package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-update-show-cmd.yang", ZeUpdateShowCmdYANG)
	yang.RegisterModule("ze-update-firmware-cmd.yang", ZeUpdateFirmwareCmdYANG)
	yang.RegisterModule("ze-update-api.yang", ZeUpdateAPIYANG)
}
