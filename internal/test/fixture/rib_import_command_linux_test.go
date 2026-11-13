//go:build linux

// Design: docs/functional-tests.md -- isolated route-policy command authority.

package fixture

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestImportCommandRetainsOnlyNetworkAdministration(t *testing.T) {
	attributes, err := importCommandAttributes()
	if err != nil {
		t.Fatal(err)
	}
	if len(attributes.AmbientCaps) != 1 {
		t.Fatalf("route-policy ip command ambient grant %v, want only CAP_NET_ADMIN", attributes.AmbientCaps)
	}
	if attributes.AmbientCaps[0] != unix.CAP_NET_ADMIN {
		t.Errorf("route-policy ip command ambient capability %d, want CAP_NET_ADMIN", attributes.AmbientCaps[0])
	}
	if attributes.Credential != nil {
		t.Error("route-policy ip command changed the observer's identity")
	}
}
