package schema

import _ "embed"

//go:embed ze-config-archive-api.yang
var ZeConfigArchiveAPIYANG string

//go:embed ze-config-archive-cmd.yang
var ZeConfigArchiveCmdYANG string
