// Package schema provides the YANG schema for DNS resolver configuration.
package schema

import _ "embed"

//go:embed ze-dns-conf.yang
var ZeDNSConfYANG string
