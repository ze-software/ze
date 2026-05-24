package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

func init() {
	yang.RegisterModule("ze-image-server-conf.yang", ZeImageServerConfYANG)
}
