package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}
var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}

func changeLocalTable() {
	codeTable := []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
	codeTable[0].Code = "doctor-local-replacement"
}

func init() {
	changeLocalTable()
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
