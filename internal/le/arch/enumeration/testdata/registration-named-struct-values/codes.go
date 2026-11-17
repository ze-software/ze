package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

type codeStruct struct {
	codeFirst  int
	codeSecond int
}

var builtinCodes = codeStruct{codeFirst: 1, codeSecond: 2}
