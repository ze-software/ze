// Package schema provides the YANG schema for the bgp-cmd-metrics plugin.
package schema

import _ "embed"

//go:embed ze-bgp-cmd-metrics-api.yang
var ZeBgpCmdMetricsAPIYANG string

//go:embed ze-cli-metrics-cmd.yang
var ZeCliMetricsCmdYANG string
