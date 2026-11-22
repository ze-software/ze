// Design: docs/exabgp/exabgp-migration.md -- ExaBGP defaults the migration states
// Overview: migrate.go -- copySimpleFields and migrateCapability write these values

package migration

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
)

// exabgpHoldTimeDefault is the Hold Time ExaBGP offers in OPEN when a neighbor
// states none, the same 180 seconds in every release the migration reads:
//   - 4.2.25 configuration/neighbor/__init__.py: `neighbor.hold_time = local.get('hold-time', HoldTime(180))`
//   - 5.0.1 bgp/neighbor.py, Neighbor.defaults: `'hold-time': HoldTime(180),`
//   - 6.0 bgp/neighbor/settings.py, NeighborSettings: `hold_time: int = 180`
//
// Ze's own receive-hold-time defaults to 90 (ze-bgp-conf.yang, peer > timer),
// so a migrated peer left to that default would offer half the hold time the
// ExaBGP session did. Ze's default stays as it is: the migration states
// ExaBGP's value instead.
const exabgpHoldTimeDefault = "180"

// exabgpGracefulRestartTimeMax is the largest Restart Time the twelve-bit field
// carries. ExaBGP 6.0 caps the hold time it borrows for an unset Restart Time
// at it (bgp/neighbor/neighbor.py, Neighbor.infer:
// `restart_time = min(int(self.hold_time), GRACEFUL_RESTART_MAX_TIME)`).
const exabgpGracefulRestartTimeMax = 4095

// capabilityGracefulRestart is the capability keyword, the same in ExaBGP and ze.
const capabilityGracefulRestart = "graceful-restart"

// exabgpHoldTime answers the Hold Time the ExaBGP neighbor offered: the value
// it states, else ExaBGP's default.
func exabgpHoldTime(neighbor *config.Tree) string {
	if v, ok := neighbor.Get("hold-time"); ok {
		return v
	}
	return exabgpHoldTimeDefault
}

// exabgpGracefulRestartTime answers the Restart Time ExaBGP advertises when its
// config enables Graceful Restart without a time (a bare `graceful-restart;`,
// or `graceful-restart 0;`). ExaBGP then uses the neighbor's hold time, in
// every release: 4.2.25 `capability.get('graceful-restart', 0) or
// int(neighbor.hold_time)`, 5.0.1 `self['capability']['graceful-restart'] =
// int(self['hold-time'])`, 6.0 the min() quoted at exabgpGracefulRestartTimeMax.
// Ze's restart-time defaults to 120 (ze-graceful-restart.yang), so the
// migration states the time. A hold time that does not parse is passed through
// for ze's parser to refuse with its own error.
func exabgpGracefulRestartTime(neighbor *config.Tree) string {
	hold := exabgpHoldTime(neighbor)
	seconds, err := strconv.Atoi(hold)
	if err != nil {
		return hold
	}
	if seconds > exabgpGracefulRestartTimeMax {
		return strconv.Itoa(exabgpGracefulRestartTimeMax)
	}
	return hold
}

// migratedCapabilityValue answers the ze value for a capability ExaBGP states
// in value form. ExaBGP's parser stores a bare keyword as "true", and ze
// writes "enable" for it, except for Graceful Restart: ExaBGP gives a bare
// `graceful-restart;` and a `graceful-restart 0;` the neighbor's hold time, so
// the migration writes that time rather than leave ze's default of 120.
func migratedCapabilityValue(neighbor *config.Tree, field, value string) string {
	if field == capabilityGracefulRestart {
		switch value {
		case "", configTrue, "0":
			return exabgpGracefulRestartTime(neighbor)
		}
		return value
	}
	switch value {
	case "", configTrue:
		return "enable"
	}
	return value
}

// migratedCapabilityContainer copies a capability ExaBGP states in container
// form. A graceful-restart container places restart-time under short-lived (Ze's
// ze-graceful-restart.yang shape) and fills in ExaBGP's hold-time default when
// none was stated, for the reason migratedCapabilityValue gives.
func migratedCapabilityContainer(neighbor *config.Tree, field string, container *config.Tree) *config.Tree {
	migrated := container.Clone()
	if field != capabilityGracefulRestart {
		return migrated
	}
	shortLived := migrated.GetContainer("short-lived")
	if shortLived == nil {
		shortLived = config.NewTree()
		migrated.SetContainer("short-lived", shortLived)
	}
	if rt, ok := migrated.Get("restart-time"); ok {
		migrated.Delete("restart-time")
		if _, hasShortRT := shortLived.Get("restart-time"); !hasShortRT {
			shortLived.Set("restart-time", rt)
		}
		return migrated
	}
	if _, ok := shortLived.Get("restart-time"); ok {
		return migrated
	}
	shortLived.Set("restart-time", exabgpGracefulRestartTime(neighbor))
	return migrated
}
