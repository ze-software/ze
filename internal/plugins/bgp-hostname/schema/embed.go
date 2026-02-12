// Package schema provides the YANG schema for the hostname plugin.
package schema

import _ "embed"

//go:embed ze-hostname.yang
var ZeHostnameYANG string
