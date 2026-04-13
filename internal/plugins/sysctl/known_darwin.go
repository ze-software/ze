//go:build darwin

package sysctl

import sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"

func init() {
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.inet.ip.forwarding", Type: sysctlreg.TypeBool,
		Description: "Enable IPv4 forwarding", Platform: sysctlreg.PlatformDarwin,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.inet6.ip6.forwarding", Type: sysctlreg.TypeBool,
		Description: "Enable IPv6 forwarding", Platform: sysctlreg.PlatformDarwin,
	})
}
