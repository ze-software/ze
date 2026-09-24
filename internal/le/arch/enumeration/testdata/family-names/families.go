// A switch over family names the family registry already holds.
package fixture

func label(name string) string {
	switch name {
	case "ipv4/unicast":
		return "v4"
	case "ipv6/multicast":
		return "v6m"
	}
	return ""
}
