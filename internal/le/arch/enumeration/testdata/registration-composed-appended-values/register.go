package fixture

var builtinFamilies = append([]string{}, familyNames...)

func RegisterBuiltinFamilies() {
	for _, name := range builtinFamilies {
		RegisterFamily(name)
	}
}
