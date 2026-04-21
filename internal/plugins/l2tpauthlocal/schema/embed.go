// Package schema provides the YANG schema for the l2tp-auth-local plugin.
package schema

import _ "embed"

//go:embed ze-l2tp-auth-local-conf.yang
var ZeL2TPAuthLocalConfYANG string
