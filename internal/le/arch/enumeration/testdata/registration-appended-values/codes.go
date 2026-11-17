package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var pluginInfraCodes = []diagnostic.CodeMeta{
	{Code: codeFirst},
	{Code: codeSecond},
}

var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}
