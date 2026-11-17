package fixture

const (
	codeFirst  = "doctor-probe-first"
	codeSecond = "doctor-probe-second"
)

func init() {
	{
		table := []diagnostic.CodeMeta{{Code: codeFirst}, {Code: codeSecond}}
		_ = table
	}
	{
		table := []diagnostic.CodeMeta{{Code: "doctor-local-first"}, {Code: "doctor-local-second"}}
		diagnostic.RegisterCodes(table)
	}
}
