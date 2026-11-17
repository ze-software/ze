package fixture

func init() {
	for _, meta := range registeredCodes {
		diagnostic.Register(meta)
	}
}
