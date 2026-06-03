// Package schema provides the YANG command schema for the ping feature module
// (show ping, monitor ping, resolve ping) via container merge onto the central
// show, monitor, and resolve verb roots.
package schema

import _ "embed"

//go:embed ze-ping-cmd.yang
var ZePingCmdYANG string
