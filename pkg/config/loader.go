package config

import (
	"fmt"
	"os"
	"time"

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
	data, err := os.ReadFile(path)
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
	for _, nc := range cfg.Neighbors {
		neighbor := configToNeighbor(&nc, cfg)
		if err := r.AddNeighbor(neighbor); err != nil {
			return nil, fmt.Errorf("add neighbor %s: %w", nc.Address, err)
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

	return n
}
