// Design: docs/architecture/testing/ci-format.md -- option=needs-linux device declarations
// Related: caps.go -- the caps= vocabulary this one sits beside; record_parse.go -- the needs-linux option

package runner

import (
	"slices"
	"strings"
)

// Values `option=needs-linux:devices=` accepts, as a comma-separated list, and
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
// over-skips deletes coverage. Two readers consume it: the parser, which
// refuses a token outside this table on every host, and the container grant
// list, ContainerDevicePaths.
const (
	devicesPPP = "ppp"
)

// devicesRequired maps each accepted token to its device node.
var devicesRequired = map[string]string{
	devicesPPP: "/dev/ppp",
}

// ContainerDevicePaths answers, sorted, the node of every device a devices=
// token names. A container given these with `docker run --device` can run
// every `.ci` test that declares a device, which is what the release-candidate
// evidence container needs (spec-lab-containers-least-privilege, site 6), so
// its --device list follows this table rather than a hand list.
func ContainerDevicePaths() []string {
	paths := make([]string, 0, len(devicesRequired))
	for _, path := range devicesRequired {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
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
