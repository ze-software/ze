// Design: docs/architecture/firewall/firewall-domain-group.md -- registration of the
// three firewall domain-group functional-test fixtures
// Related: netfilter_fixture_domain_group.go -- the drivers registered here

package fixture

// The three drivers register here rather than in netfilter_fixture.go's init,
// because two of them serve the `plugin` suite and one the `firewall` suite,
// and one feature's fixtures belong together. The name is the path a `.ci`
// file spells after `ze-test fixture`.
func init() {
	Register("plugin/firewall-domain-group-update", domainGroupUpdate)
	Register("plugin/firewall-domain-group-clear", domainGroupClear)
	Register("firewall/firewall-cli-domain-group-show", domainGroupShow)
}
