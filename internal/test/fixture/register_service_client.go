// Design: docs/functional-tests.md -- service clients that read what a daemon serves
// Related: service_client_fixture.go -- the bodies of these clients

package fixture

// The clients a .ci runs against a service the daemon serves, so a test
// asserts what a user receives rather than the log line written beside it.
// Their bodies are service_client_fixture.go.
func init() {
	Register("client/dns-query", serviceClientDNSQuery)
	Register("client/http-get", serviceClientHTTPGet)
	Register("client/tftp-get", serviceClientTFTPGet)
}
