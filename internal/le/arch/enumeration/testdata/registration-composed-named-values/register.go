package fixture

func init() {
	for _, name := range familyNames {
		RegisterFamily(name)
	}
}
