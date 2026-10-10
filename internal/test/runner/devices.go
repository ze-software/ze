// Design: docs/architecture/testing/ci-format.md -- option=needs-linux device declarations
// Related: caps.go -- the caps= vocabulary this one sits beside; record_parse.go -- the needs-linux option

package runner

import (
	"slices"
	"strings"
)

// Values `option=opens-device:name=` accepts, as a comma-separated list, and
// the device node each one names.
//
//   - ppp: /dev/ppp, the PPP channel and unit device. ppp.DevPPPSetup
//     (internal/component/l2tp/ppp/devppp_linux.go) opens it twice for every
//     L2TP session the kernel carries and for every PPPoE session a PADR
//     admits, and the PADS is sent only after it succeeds. A container sees
//     only the device nodes it was given, so without the grant the open fails
//     and the session never reaches the asserted state.
//
// The declaration does not skip a test on a host where the node is absent: the
// daemon's own modprobe at L2TP start can create /dev/ppp on a host /dev, so an
// absence at parse time is no proof the test cannot run, and a gate that
// over-skips deletes coverage. The table has two readers: the parser, which
// refuses a token outside it on every host, and the container grant list,
// ContainerDevicePaths, which grants every node in it whichever tests declare
// one. A test's declaration is checked (TestPPPSessionTestsDeclareDevPPP) but
// decides no grant.
const (
	devicesPPP = "ppp"
)

// requiredDevice is one device a devices= token names: its node, and the
// kernel module whose loading on the Docker host creates that node. A
// container never loads a module, so the module is what a refusal tells the
// operator to load.
type requiredDevice struct {
	path   string
	module string
}

// devicesRequired maps each accepted token to its device.
var devicesRequired = map[string]requiredDevice{
	devicesPPP: {path: "/dev/ppp", module: "ppp_generic"},
}

// ContainerDevicePaths answers, sorted, the node of every device an
// option=opens-device name= token can name. A container given these with `docker run --device` can run
// every `.ci` test that declares a device, which is what the release-candidate
// evidence container needs (spec-lab-containers-least-privilege, site 6), so
// its --device list follows this table rather than a hand list.
func ContainerDevicePaths() []string {
	paths := make([]string, 0, len(devicesRequired))
	for _, device := range devicesRequired {
		paths = append(paths, device.path)
	}
	slices.Sort(paths)
	return paths
}

// DeviceHostModule answers the kernel module that creates the device node at
// path on the Docker host, and false for a path no devices= token names.
func DeviceHostModule(path string) (string, bool) {
	for _, device := range devicesRequired {
		if device.path == path {
			return device.module, true
		}
	}
	return "", false
}

// devicesAccepted lists the accepted tokens for an error message, derived from
// the table so a new device cannot be added without the diagnostic naming it.
func devicesAccepted() string {
	names := make([]string, 0, len(devicesRequired))
	for name := range devicesRequired {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}
