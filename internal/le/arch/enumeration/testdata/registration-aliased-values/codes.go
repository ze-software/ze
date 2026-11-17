package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codeTable = []diagnostic.CodeMeta{
	{Code: codeFirst},
	{Code: codeSecond},
}

var firstAlias = codeTable
var secondAlias = firstAlias

var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}
