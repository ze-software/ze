// Design: docs/architecture/container/network.md -- the translation tables
// Related: tables.go -- the tables under these names

package network

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/plugins/container"
)

//nolint:gochecknoinits // registration: the reservation is made from init() so every process of the binary holds it
func init() {
	// The firewall component accepts a contributed table only under a name
	// its contributor reserved, and refuses that name to every other owner.
	firewall.MustReserveTableNames(container.PluginName, firewall.TableControl{
		Plugin:  "container plugin",
		Decides: "the container networks whose masquerade is true",
		Command: "ze show config container network",
	}, kernelTablePrefix+tableIPv4, kernelTablePrefix+tableIPv6)
}
