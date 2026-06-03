package schema

import _ "embed"

//go:embed ze-pki-conf.yang
var ZePKIConfYANG string

//go:embed ze-pki-api.yang
var ZePKIAPIYANG string

//go:embed ze-pki-cmd.yang
var ZePKICmdYANG string
