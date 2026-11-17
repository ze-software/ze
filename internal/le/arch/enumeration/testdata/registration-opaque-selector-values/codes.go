package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var source = struct {
	unused   []diagnostic.CodeMeta
	selected []diagnostic.CodeMeta
}{
	unused:   []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}},
	selected: []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}},
}

var codeTable = source.selected

func init() {
	diagnostic.RegisterCodes(codeTable)
}
