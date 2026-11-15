// Design: docs/architecture/firewall/firewall-domain-group.md -- registration of the
// four firewall domain-group functional-test fixtures
// Related: netfilter_fixture_domain_group.go, netfilter_fixture_domain_group_restart.go -- the drivers registered here

package fixture

// The four drivers register here rather than in netfilter_fixture.go's init,
// because three of them serve the `plugin` suite and one the `firewall` suite,
// and one feature's fixtures belong together. The name is the path a `.ci`
// file spells after `le test fixture`.
func init() {
	Register("plugin/firewall-domain-group-update", domainGroupUpdate)
	Register("plugin/firewall-domain-group-clear", domainGroupClear)
	Register("plugin/firewall-domain-group-restart-keeps-answers", domainGroupRestartKeepsAnswers)
	Register("firewall/firewall-cli-domain-group-show", domainGroupShow)
}
