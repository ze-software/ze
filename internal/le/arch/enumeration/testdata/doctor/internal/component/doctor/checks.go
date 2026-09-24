package doctor

func checkDisk() []diagnostic.Diagnostic { return nil }

func checkClock() []diagnostic.Diagnostic { return nil }

// checkRegistered is reached through the registry, so it is not hand-called.
func checkRegistered() []diagnostic.Diagnostic { return nil }

var registration = doctorCheck{
	Name:  "registered",
	Check: checkRegistered,
}
