package iface

// fakeBackend (defined in config_test.go) satisfies the Backend interface's
// LinkSpeedDuplex method here, in a file owned by the flow-export change set, so
// the sFlow if_counters work does not have to touch config_test.go (which
// carries unrelated in-flight test additions). Speed/duplex are best-effort
// enrichment; the fake reports them as unknown.
func (b *fakeBackend) LinkSpeedDuplex(_ string) (int, string) { return 0, "" }
