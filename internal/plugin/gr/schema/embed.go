// Package schema provides the YANG schema for the Graceful Restart plugin.
package schema

import _ "embed"

//go:embed ze-graceful-restart.yang
var ZeGracefulRestartYANG string
