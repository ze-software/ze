package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}

func init() {
	codeTable = []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
