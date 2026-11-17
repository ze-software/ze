package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

var codes = []diagnostic.CodeMeta{
	{Code: codeFirst},
	{Code: codeSecond},
}

func init() {
	codes := []diagnostic.CodeMeta{
		{Code: "doctor-local-first"},
		{Code: "doctor-local-second"},
	}
	for _, meta := range codes {
		diagnostic.Register(meta)
	}
}
