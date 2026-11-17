package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = append(codeFirst, codeSecond)

func append(_ ...string) []diagnostic.CodeMeta {
	return []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
}

func init() {
	diagnostic.RegisterCodes(codeTable)
}
