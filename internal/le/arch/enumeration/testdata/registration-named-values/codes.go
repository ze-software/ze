package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var registeredCodes = []diagnostic.CodeMeta{
	{Code: codeFirst},
	{Code: codeSecond},
	{Code: registeredExtra},
}
