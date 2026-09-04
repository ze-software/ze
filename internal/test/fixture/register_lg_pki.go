// Design: docs/architecture/pki/tls-listeners.md -- the looking-glass PKI certificate fixtures

package fixture

// The three looking-glass PKI drivers. Their bodies, and why the assertions
// live in compiled code rather than in the .ci files, are lg_pki_fixture.go.
func init() {
	Register("plugin/lg-pki-certificate", lgPKICertificateServed)
	Register("reload/lg-pki-reference-reload", lgPKIReferenceReload)
	Register("reload/lg-pki-reference-reload-broken", lgPKIReferenceReloadBroken)
}
