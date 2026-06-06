// Package schema provides the YANG schemas for the update component commands.
package schema

import _ "embed"

//go:embed ze-update-show-cmd.yang
var ZeUpdateShowCmdYANG string

//go:embed ze-update-firmware-cmd.yang
var ZeUpdateFirmwareCmdYANG string

//go:embed ze-update-api.yang
var ZeUpdateAPIYANG string
