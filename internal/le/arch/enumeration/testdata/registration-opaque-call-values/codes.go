package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = makeCodeTable(codeFirst, codeSecond)

func makeCodeTable(_, _ string) []diagnostic.CodeMeta {
	return []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
}

func init() {
	diagnostic.RegisterCodes(codeTable)
}
