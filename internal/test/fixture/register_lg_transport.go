// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass listener serves HTTPS or HTTP

package fixture

// The looking-glass transport driver: one daemon serving HTTP and HTTPS on two
// server entries, and a reload flipping one. Its body, and why the assertions
// live in compiled code rather than in the .ci file, are lg_transport_fixture.go.
//
// Related: lg_transport_fixture.go -- the scenario body
// Related: test/reload/lg-transport-mixed.ci -- the run, once promoted from test/draft/reload/
func init() {
	Register("reload/lg-transport-mixed", lgTransportMixed)
}
