package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

type codeStruct struct {
	codeFirst  int
	codeSecond int
}
type firstStruct = codeStruct
type secondStruct = firstStruct

var builtinCodes = secondStruct{codeFirst: 1, codeSecond: 2}
