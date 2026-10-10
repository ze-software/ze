// Design: docs/architecture/testing/interop.md -- the release-candidate evidence container's device grants
// Related: caps.go -- the caps= vocabulary whose capabilities the same container is granted

package runner

import (
	"slices"
)

// containerDevice is one device node a production path reached by a `.ci`
// test opens, and the kernel module whose loading on the Docker host creates
// that node. A container never loads a module, so the module is what a
// refusal tells the operator to load.
type containerDevice struct {
	path   string
	module string
}

// containerDevices lists every device node a production path reached by a
// `.ci` test opens, so a container given them all can run every test.
//
//   - /dev/ppp, the PPP channel and unit device. ppp.DevPPPSetup
//     (internal/component/l2tp/ppp/devppp_linux.go) opens it twice for every
//     L2TP session the kernel carries and for every PPPoE session a PADR
//     admits, and the PADS is sent only after it succeeds. A container sees
//     only the device nodes it was given, so without the grant the open fails
//     and the session never reaches the asserted state.
//
// A test declares no device: the container is granted the whole table,
// whichever tests open which node, so a per-test declaration would have no
// reader. TestContainerDevicesCoverWhatPPPOpens reads the source that opens
// the node and checks this table holds it.
var containerDevices = []containerDevice{
	{path: "/dev/ppp", module: "ppp_generic"},
}

// ContainerDevicePaths answers, sorted, every node in containerDevices. A
// container given these with `docker run --device` can run every `.ci` test
// that opens a device, which is what the release-candidate evidence container
// needs (spec-lab-containers-least-privilege, site 6), so its --device list
// follows this table rather than a hand list.
func ContainerDevicePaths() []string {
	paths := make([]string, 0, len(containerDevices))
	for _, device := range containerDevices {
		paths = append(paths, device.path)
	}
	slices.Sort(paths)
	return paths
}

// DeviceHostModule answers the kernel module that creates the device node at
// path on the Docker host, and false for a path the table does not hold.
func DeviceHostModule(path string) (string, bool) {
	for _, device := range containerDevices {
		if device.path == path {
			return device.module, true
		}
	}
	return "", false
}
