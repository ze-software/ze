// Package schema provides the YANG schema for NTP client configuration.
package schema

import _ "embed"

//go:embed ze-ntp-conf.yang
var ZeNTPConfYANG string
