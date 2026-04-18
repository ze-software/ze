// Package schema provides the YANG schema for the fib-p4 plugin.
package schema

import _ "embed"

//go:embed ze-fib-p4-conf.yang
var ZeFibP4ConfYANG string
