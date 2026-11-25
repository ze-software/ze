// Design: docs/architecture/fleet-config.md -- a device name forms a key in the hub store
// Related: envelope.go -- the client's name identifies the authenticated session

package fleet

import "strings"

// ValidDeviceKeyName reports whether a name fits one store key segment and
// can also be used in the client's configuration and held/version key templates.
func ValidDeviceKeyName(name string) bool {
	return name != "" && name != "." && !strings.Contains(name, "/") && !strings.Contains(name, "..")
}
