package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}
var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}

func init() {
	_ = len(codeTable)
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
