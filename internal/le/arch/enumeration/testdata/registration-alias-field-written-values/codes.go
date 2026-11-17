package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}
var alias = codeTable

func init() {
	alias[0].Code = "doctor-local-first"
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
