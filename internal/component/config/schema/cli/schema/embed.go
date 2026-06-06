// Package schema provides the YANG command schema for schema introspection CLI commands.
package schema

import _ "embed"

//go:embed ze-schema-cli-cmd.yang
var ZeSchemaCliCmdYANG string
