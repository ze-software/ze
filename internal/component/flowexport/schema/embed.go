// Package schema provides the YANG schema for flow export configuration.
package schema

import _ "embed"

//go:embed ze-flowexport-conf.yang
var ZeFlowExportConfYANG string

//go:embed ze-flowexport-cmd.yang
var ZeFlowExportCmdYANG string
