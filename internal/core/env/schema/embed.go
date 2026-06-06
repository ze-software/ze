// Package schema provides the YANG command schema for environment variable inspection.
package schema

import _ "embed"

//go:embed ze-env-cmd.yang
var ZeEnvCmdYANG string
