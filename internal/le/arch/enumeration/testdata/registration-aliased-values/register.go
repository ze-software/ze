package fixture

func init() {
	for _, meta := range secondAlias {
		diagnostic.Register(meta)
	}
}
