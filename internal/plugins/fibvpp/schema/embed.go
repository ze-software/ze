// Package schema provides the YANG schema for the fib-vpp plugin.
package schema

import _ "embed"

//go:embed ze-fib-vpp-conf.yang
var ZeFibVppConfYANG string
