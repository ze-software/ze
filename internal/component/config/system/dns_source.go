// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: resolv.go -- the registry that selects with the list read here
// Related: register.go -- installs validateRouterDNSSourceTree as a tree validator

package system

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// routerDNSSourceRefusals are the classes the router's list cannot name.
// The router's resolvers are Ze's own upstreams: self would make Ze ask
// itself, and radius belongs to one PPP subscriber, which the router is not.
var routerDNSSourceRefusals = []dnssource.Refusal{
	{
		Class: dnssource.ClassSelf,
		Why:   "self is Ze's own LAN resolver, and Ze cannot be its own upstream: its lookups would loop back to itself",
		Fix: "delete system dns-source source self, and name the upstream servers with set system name-server <address> " +
			"or take them from the network with set system dns-source source received",
	},
	{
		Class: dnssource.ClassRadius,
		Why:   "radius names the resolvers a RADIUS server gives one PPP subscriber, and the router is not a subscriber",
		Fix:   "delete system dns-source source radius; the router takes configured (system name-server) and received",
	},
}

// RouterDNSSource reads the router's list from the system container sys:
// the written system dns-source, or the built-in default when the container
// is absent (or sys is nil). A written list Ze refuses answers the refusal,
// with the what, why and fix the operator reads.
func RouterDNSSource(sys *config.Tree) (dnssource.List, error) {
	if sys == nil {
		return routerDefault, nil
	}
	container := sys.GetContainer("dns-source")
	if container == nil {
		return routerDefault, nil
	}
	written, err := readWrittenDNSSource(container)
	if err != nil {
		return dnssource.List{}, err
	}
	return dnssource.NewWrittenList(routerDNSSourcePath, dnssource.FamilyAll, written, routerDNSSourceRefusals...)
}

// DefaultRouterDNSSource answers the router's built-in default list, for a
// caller that has no config tree (web-only mode).
func DefaultRouterDNSSource() dnssource.List { return routerDefault }

// readWrittenDNSSource reads one dns-source container as it was written. The
// tree validators run before the YANG walk, so a word the enumerations would
// refuse can reach this reader: it is refused here, in the operator's terms.
func readWrittenDNSSource(container *config.Tree) (*dnssource.Written, error) {
	written := &dnssource.Written{ReceivedInterfaces: container.GetSlice("received-interface")}
	for _, word := range container.GetSlice("source") {
		class, ok := dnssource.ParseClass(word)
		if !ok {
			return nil, unknownDNSSourceClassError(word)
		}
		written.Sources = append(written.Sources, class)
	}
	if word, ok := container.Get("mode"); ok {
		mode, known := dnssource.ParseMode(word)
		if !known {
			return nil, errors.New(routerDNSSourcePath + " mode " + word + ": not a mode, so Ze cannot tell how the sources combine\n" +
				"set " + routerDNSSourcePath + " mode first-usable, or set " + routerDNSSourcePath + " mode merge")
		}
		written.Mode = mode
	}
	return written, nil
}

// unknownDNSSourceClassError refuses a source word that names no class,
// listing the classes there are.
func unknownDNSSourceClassError(word string) error {
	known := dnssource.Classes()
	names := make([]string, len(known))
	for i, class := range known {
		names[i] = string(class)
	}
	return errors.New(routerDNSSourcePath + " source " + word + ": not a source class, so Ze cannot tell where to take resolvers from\n" +
		"delete " + routerDNSSourcePath + " source " + word + ", and name one of " + strings.Join(names, ", "))
}

// validateRouterDNSSourceTree refuses a system dns-source the router cannot
// use. Registered as a tree validator, it runs wherever a config is checked:
// ze config validate, a commit, the daemon's boot load and a reload.
func validateRouterDNSSourceTree(tree *config.Tree) error {
	_, err := RouterDNSSource(tree.GetContainer("system"))
	return err
}
