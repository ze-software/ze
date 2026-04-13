//go:build darwin

package sysctl

func init() {
	MustRegister(KeyDef{
		Name: "net.inet.ip.forwarding", Type: TypeBool,
		Description: "Enable IPv4 forwarding", Platform: PlatformDarwin,
	})
	MustRegister(KeyDef{
		Name: "net.inet6.ip6.forwarding", Type: TypeBool,
		Description: "Enable IPv6 forwarding", Platform: PlatformDarwin,
	})
}
