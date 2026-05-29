// Package schema provides the YANG schema for RSVP-TE configuration.
package schema

import _ "embed"

//go:embed ze-rsvp-te-conf.yang
var ZeRSVPTEConfYANG string
