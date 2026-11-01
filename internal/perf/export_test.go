package perf

// Export test helpers for use by external test package (perf_test).

// NewTestForwarder is the exported alias of newTestForwarder for external tests.
var NewTestForwarder = newTestForwarder

// WithPassiveListenPorts returns cfg with the sender and receiver listening on
// the given ports instead of 179 in passive-listen mode, so a test runs the
// passive path unprivileged.
func WithPassiveListenPorts(cfg BenchmarkConfig, sender, receiver int) BenchmarkConfig {
	cfg.senderListenPort = sender
	cfg.receiverListenPort = receiver

	return cfg
}
