package fixture

const (
	familyFirst  = "ipv4/unicast"
	familySecond = "ipv6/unicast"
)

func init() {
	RegisterFamilies(familyFirst, familySecond)
}
