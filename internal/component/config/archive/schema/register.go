package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-config-archive-api.yang", ZeConfigArchiveAPIYANG)
	yang.RegisterModule("ze-config-archive-cmd.yang", ZeConfigArchiveCmdYANG)
}
