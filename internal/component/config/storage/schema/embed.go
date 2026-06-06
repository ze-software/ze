// Package schema provides the YANG command schema for data storage CLI commands.
package schema

import _ "embed"

//go:embed ze-storage-cli-cmd.yang
var ZeStorageCliCmdYANG string
