package fixture

var builtinCodes = append([]diagnostic.CodeMeta{}, pluginInfraCodes...)

func RegisterBuiltinCodes() {
	for _, meta := range builtinCodes {
		diagnostic.Register(meta)
	}
}
