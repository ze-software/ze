// Design: docs/features/dns-resolver.md -- show system dns drops a consumer the engine removed
// Related: dns_source_withdraw.go -- the observer registered below
// Related: plugin_fixture_13.go -- reloadTrigger13, the SIGHUP trigger registered below

package fixture

func init() {
	// spec-home-cpe-23b review round 2 ISSUE 2: a dns-resolver the reload
	// removes withdraws its selection from show system dns.
	Register("plugin/show-system-dns-withdraw", observer02("show-system-dns-withdraw-test", showSystemDNSWithdraw))
	// The trigger waits for the observer's observer.initial-ok, then reloads.
	Register("plugin/show-system-dns-withdraw-trigger", reloadTrigger13)
}
