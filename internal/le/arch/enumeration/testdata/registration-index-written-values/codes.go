package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}

func init() {
	codeTable[0] = diagnostic.CodeMeta{Code: "doctor-local-first"}
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
