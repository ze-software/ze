// Package migration provides config schema versioning and migration.
package migration

// ConfigVersion represents a config schema version.
type ConfigVersion int

const (
	// VersionUnknown indicates version could not be determined.
	VersionUnknown ConfigVersion = 0

	// Version1 is ExaBGP main (2025-12) format.
	// RIB options at neighbor level: group-updates, auto-flush, adj-rib-*.
	// Note: v1 detection not implemented - v1→v2 migration is separate scope.
	Version1 ConfigVersion = 1

	// Version2 is ZeBGP intermediate format.
	// RIB options in rib { } block, uses "neighbor" keyword.
	Version2 ConfigVersion = 2

	// Version3 is ZeBGP target format.
	// Renames neighbor→peer, restructures templates:
	//   - template { group <name> } replaces template { neighbor <name> }
	//   - template { match <glob> } replaces root-level peer <glob>
	//   - peer <IP> replaces neighbor <IP>
	Version3 ConfigVersion = 3

	// VersionCurrent is the latest supported version.
	VersionCurrent = Version3
)

// String returns human-readable version name.
func (v ConfigVersion) String() string {
	switch v {
	case VersionUnknown:
		return "unknown"
	case Version1:
		return "v1 (ExaBGP main 2025-12)"
	case Version2:
		return "v2 (ZeBGP intermediate)"
	case Version3:
		return "v3 (ZeBGP current)"
	}
	return "unknown"
}
