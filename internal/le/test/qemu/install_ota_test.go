// Design: docs/architecture/testing/qemu-integration.md -- real appliance OTA.

package testqemu

import "testing"

func TestApplianceOTAActionIsNative(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb == "appliance-ota" {
			return
		}
	}
	t.Fatal("missing appliance-ota action: reboot-only evidence does not upload an image")
}
