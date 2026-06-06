// Package schema provides the YANG schema for NTP client configuration and commands.
package schema

import _ "embed"

//go:embed ze-ntp-conf.yang
var ZeNTPConfYANG string
