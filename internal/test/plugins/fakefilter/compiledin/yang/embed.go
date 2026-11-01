// Package yang provides the YANG schema for the fakefilter test plugin.
package yang

import _ "embed"

//go:embed ze-fakefilter-conf.yang
var ZeFakefilterConfYANG string
