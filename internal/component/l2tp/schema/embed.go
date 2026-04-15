// Package schema provides the YANG schema for the L2TP subsystem.
package schema

import _ "embed"

//go:embed ze-l2tp-conf.yang
var ZeL2TPConfYANG string
