// Design: docs/architecture/core-design.md -- YANG schema embedding

package schema

import _ "embed"

//go:embed ze-connected-conf.yang
var ZeConnectedConfYANG string
