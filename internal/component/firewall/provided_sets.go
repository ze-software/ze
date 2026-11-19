// Design: docs/architecture/core-design.md — firewall table registry
// Related: config.go -- irrV4Prefix, domainV4Prefix, providedV6Twin: the names another owner fills

package firewall

import "strings"

// providedSetOwners names the plugin that fills each family of set whose name
// carries the prefix, beside the config leaf that asks for it. The prefixes are
// the ones the parser builds those set names from (irrSetMatch, domainSetMatch),
// so the two cannot spell one differently.
var providedSetOwners = [...]struct{ prefix, plugin, leaf string }{
	{irrV4Prefix, "firewall-irr", irrLeaves},
	{irrV6Prefix, "firewall-irr", irrLeaves},
	{domainV4Prefix, "firewall-domain", domainLeaves},
	{domainV6Prefix, "firewall-domain", domainLeaves},
}

const (
	irrLeaves    = "source-asn, source-as-set, destination-asn or destination-as-set"
	domainLeaves = "source-domain-group or destination-domain-group"
)

// ProvidedSetOwner reports whether a set name is one another plugin fills at
// run time rather than one a firewall table declares, and answers that plugin
// and the firewall leaf that uses such a set. A reader outside this package
// that resolves @name against the declared sets uses it to tell the operator
// why the name cannot resolve, instead of telling them to declare it.
func ProvidedSetOwner(name string) (plugin, leaf string, provided bool) {
	for _, owner := range providedSetOwners {
		if strings.HasPrefix(name, owner.prefix) {
			return owner.plugin, owner.leaf, true
		}
	}
	return "", "", false
}
