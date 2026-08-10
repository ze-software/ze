// Design: docs/architecture/mrt.md — daemon component configuration

package mrt

import "time"

// Config holds MRT dump configuration for the daemon component.
type Config struct {
	UpdatesPath     string        // strftime pattern for UPDATE-only stream
	UpdatesInterval time.Duration // file rotation interval for updates

	AllPath     string        // strftime pattern for all-messages stream
	AllInterval time.Duration // file rotation interval for all messages

	RoutesPath     string        // strftime pattern for periodic RIB snapshots
	RoutesInterval time.Duration // dump interval for TABLE_DUMP_V2

	ExtendedTimestamp bool // use BGP4MP_ET (type 17) instead of BGP4MP (type 16)
	AddPath           bool // force add-path subtypes even when not negotiated

	PeerFilter []string // if non-empty, only dump these peer addresses
	Direction  string   // "received", "sent", or "" for both
}

// hasUpdates reports whether the update stream is configured.
func (c *Config) hasUpdates() bool { return c.UpdatesPath != "" }

// hasAll reports whether the all-messages stream is configured.
func (c *Config) hasAll() bool { return c.AllPath != "" }

// hasRoutes reports whether periodic RIB dumps are configured.
func (c *Config) hasRoutes() bool { return c.RoutesPath != "" && c.RoutesInterval > 0 }

// IsEmpty reports whether no dump streams are configured.
func (c *Config) IsEmpty() bool { return !c.hasUpdates() && !c.hasAll() && !c.hasRoutes() }
