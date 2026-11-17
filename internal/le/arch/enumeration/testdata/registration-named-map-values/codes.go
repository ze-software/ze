package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

type codeMap map[string]int

var builtinCodes = codeMap{codeFirst: 1, codeSecond: 2}
var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}
