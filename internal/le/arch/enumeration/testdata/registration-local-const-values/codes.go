package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

func init() {
	const (
		codeFirst  = "doctor-local-first"
		codeSecond = "doctor-local-second"
	)
	codeTable := []diagnostic.CodeMeta{
		{Code: codeFirst},
		{Code: codeSecond},
	}
	firstAlias := codeTable
	secondAlias := firstAlias
	for _, meta := range secondAlias {
		diagnostic.Register(meta)
	}
}
