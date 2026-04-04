// Package schema provides the YANG schema for the fib-kernel plugin.
package schema

import _ "embed"

//go:embed ze-fib-conf.yang
var ZeFibConfYANG string
