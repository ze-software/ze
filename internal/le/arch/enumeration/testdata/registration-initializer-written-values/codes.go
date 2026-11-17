package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}

var initializerRan = func() bool {
	codeTable = []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
	return true
}()

func init() {
	for _, meta := range codeTable {
		diagnostic.Register(meta)
	}
}
