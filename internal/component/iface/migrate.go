// Design: docs/features/interfaces.md — Make-before-break interface migration
// Overview: iface.go — shared types and topic constants
// Detail: migrate_linux.go — Linux implementation of MigrateInterface

package iface

// MigrateConfig describes a make-before-break IP migration.
type MigrateConfig struct {
	// Source: the old interface/unit to migrate FROM.
	OldIface string
	OldUnit  int
	Address  string // CIDR to migrate (e.g., "10.0.0.1/24")

	// Destination: the new interface/unit to migrate TO.
	NewIface     string
	NewUnit      int
	NewIfaceType string // "dummy", "veth", "bridge" (empty = already exists)
}
