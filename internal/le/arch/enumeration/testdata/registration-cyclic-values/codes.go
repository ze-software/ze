package fixture

// These initializer expressions parse even though a Go build rejects their
// cycle. The source walk must terminate and keep judging the rest of the file.
var firstAlias = secondAlias
var secondAlias = firstAlias

var unrelatedCopy = []string{"doctor-probe-first", "doctor-probe-second"}

func init() {
	for _, meta := range firstAlias {
		diagnostic.Register(meta)
	}
}
