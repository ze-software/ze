package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/reactor"
)

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	_, r, err := LoadReactorWithConfig(input)
	return r, err
}

// LoadReactorWithConfig parses config and returns both Config and Reactor.
func LoadReactorWithConfig(input string) (*BGPConfig, *reactor.Reactor, error) {
	// Parse input
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	if err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	// Convert to typed config
	cfg, err := TreeToConfig(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("convert config: %w", err)
	}

	// Create reactor
	r, err := CreateReactor(cfg)
	if err != nil {
		return nil, nil, err
	}

	return cfg, r, nil
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(path string) (*reactor.Reactor, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Config file path from user
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return LoadReactor(string(data))
}

// CreateReactor creates a Reactor from typed BGPConfig.
func CreateReactor(cfg *BGPConfig) (*reactor.Reactor, error) {
	// Build reactor config
	reactorCfg := &reactor.Config{
		ListenAddr: cfg.Listen,
		RouterID:   cfg.RouterID,
		LocalAS:    cfg.LocalAS,
	}

	r := reactor.New(reactorCfg)

	// Add neighbors
	for i := range cfg.Neighbors {
		neighbor := configToNeighbor(&cfg.Neighbors[i], cfg)
		if err := r.AddNeighbor(neighbor); err != nil {
			return nil, fmt.Errorf("add neighbor %s: %w", cfg.Neighbors[i].Address, err)
		}
	}

	return r, nil
}

// configToNeighbor converts NeighborConfig to reactor.Neighbor.
func configToNeighbor(nc *NeighborConfig, global *BGPConfig) *reactor.Neighbor {
	// Determine local AS (inherit from global if not set)
	localAS := nc.LocalAS
	if localAS == 0 {
		localAS = global.LocalAS
	}

	// Determine router ID (inherit from global if not set)
	routerID := nc.RouterID
	if routerID == 0 {
		routerID = global.RouterID
	}

	// Determine hold time (default 90s)
	holdTime := time.Duration(nc.HoldTime) * time.Second
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}

	n := reactor.NewNeighbor(nc.Address, localAS, nc.PeerAS, routerID)
	n.HoldTime = holdTime
	n.Passive = nc.Passive

	// Build capabilities.
	// Add Multiprotocol capabilities from configured families.
	for _, family := range nc.Families {
		switch family {
		case "ipv4 unicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv4,
				SAFI: capability.SAFIUnicast,
			})
		case "ipv6 unicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv6,
				SAFI: capability.SAFIUnicast,
			})
		case "ipv4 multicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv4,
				SAFI: capability.SAFIMulticast,
			})
		case "ipv6 multicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv6,
				SAFI: capability.SAFIMulticast,
			})
		}
	}

	// Add FQDN capability if hostname or domain is set.
	if nc.Hostname != "" || nc.DomainName != "" {
		n.Capabilities = append(n.Capabilities, &capability.FQDN{
			Hostname:   nc.Hostname,
			DomainName: nc.DomainName,
		})
	}
	if nc.Capabilities.SoftwareVersion {
		n.Capabilities = append(n.Capabilities, &capability.SoftwareVersion{
			Version: "ExaBGP/5.0.0-0+test",
		})
	}

	// Override port from environment (for testing).
	if p := os.Getenv("exabgp_tcp_port"); p != "" {
		if v, err := strconv.ParseUint(p, 10, 16); err == nil {
			n.Port = uint16(v) //nolint:gosec // Validated above
		}
	}

	// Convert static routes.
	for _, sr := range nc.StaticRoutes {
		route := reactor.StaticRoute{
			Prefix:          sr.Prefix,
			LocalPreference: sr.LocalPreference,
			MED:             sr.MED,
		}

		// Parse next-hop (can be "self" or an IP address).
		if sr.NextHop != "" && sr.NextHop != "self" {
			if ip, err := netip.ParseAddr(sr.NextHop); err == nil {
				route.NextHop = ip
			}
		}

		n.StaticRoutes = append(n.StaticRoutes, route)
	}

	return n
}
