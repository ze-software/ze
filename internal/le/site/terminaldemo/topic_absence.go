// Design: docs/contributing/gh-pages.md -- each live topic declares what its commit adds
// Related: register_topics.go -- registerLiveTopic requires the declaration
// Related: validate_topics.go -- requireLiveTopicsAbsent runs every declaration

package siteterminaldemo

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// topicContainersAbsent declares the containers a topic's live commit creates,
// as paths from the root of the configuration. A topic that creates none
// declares what it changes with a function of its own.
func topicContainersAbsent(paths ...string) func(*config.Tree) error {
	if len(paths) == 0 {
		panic("BUG: a live topic declares no container its commit adds")
	}
	return func(tree *config.Tree) error {
		for _, path := range paths {
			if tree.GetContainerPath(path) != nil {
				return fmt.Errorf("showcase base already configures topic %s", path)
			}
		}
		return nil
	}
}

// topicBFDAbsent declares the BFD topic: the bfd container, and the BFD
// binding its commit adds to a BGP peer's connection.
func topicBFDAbsent(tree *config.Tree) error {
	if err := topicContainersAbsent("bfd")(tree); err != nil {
		return err
	}
	if bgp := tree.GetContainer("bgp"); bgp != nil {
		for name, peer := range bgp.GetList("peer") {
			if connection := peer.GetContainer("connection"); connection != nil {
				if _, present := connection.Get("bfd"); present {
					return fmt.Errorf("showcase base peer %s already configures BFD", name)
				}
			}
		}
	}
	return nil
}

// topicIPsecAbsent declares the IPsec topic: the vpn container and the XFRM
// interface its commit creates.
func topicIPsecAbsent(tree *config.Tree) error {
	if err := topicContainersAbsent("vpn")(tree); err != nil {
		return err
	}
	if interfaces := tree.GetContainer("interface"); interfaces != nil {
		if len(interfaces.GetList("xfrm")) != 0 {
			return fmt.Errorf("showcase base already configures an XFRM interface")
		}
	}
	return nil
}

// topicVRRPAbsent declares the VRRP topic: a vrrp container under an address
// family of any unit of any interface.
func topicVRRPAbsent(tree *config.Tree) error {
	interfaces := tree.GetContainer("interface")
	if interfaces == nil {
		return fmt.Errorf("showcase base has no interface section, so the VRRP topic has no link to configure")
	}
	for _, kind := range interfaces.ListNames() {
		for name, iface := range interfaces.GetList(kind) {
			for _, unit := range iface.GetList("unit") {
				for _, family := range []string{"ipv4", "ipv6"} {
					if addresses := unit.GetContainer(family); addresses != nil {
						if addresses.GetContainer("vrrp") != nil {
							return fmt.Errorf("showcase base interface %s already configures VRRP", name)
						}
					}
				}
			}
		}
	}
	return nil
}

// topicCommitConfirmedAbsent declares the confirmed-commit topic. Its commits
// add no container: they change the hostname, so the configuration must still
// hold the hostname the recording starts from.
func topicCommitConfirmedAbsent(tree *config.Tree) error {
	host, err := configHostname(tree)
	if err != nil {
		return err
	}
	if host != commitConfirmedOriginalHost {
		return fmt.Errorf("showcase base hostname is %q, not %q: the confirmed-commit topic already changed it", host, commitConfirmedOriginalHost)
	}
	return nil
}
