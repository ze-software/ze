// Package schema provides the YANG schema for SSH server configuration.
package schema

import _ "embed"

//go:embed ze-ssh-conf.yang
var ZeSSHConfYANG string
