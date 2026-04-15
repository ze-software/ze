// Package schema provides the YANG schema for TACACS+ configuration.
package schema

import _ "embed"

//go:embed ze-tacacs-conf.yang
var ZeTacacsConfYANG string
