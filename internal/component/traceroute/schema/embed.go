// Package schema provides the YANG command schema for the traceroute feature
// module (show traceroute, show probe-round, monitor traceroute, resolve
// traceroute) via container merge onto the central show, monitor, and resolve
// verb roots.
package schema

import _ "embed"

//go:embed ze-traceroute-cmd.yang
var ZeTracerouteCmdYANG string
