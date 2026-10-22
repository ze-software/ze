// Design: docs/architecture/testing/interop.md -- the shared /dev/ppp preflight
// Related: docker.go -- RunOneShot and the commandError a refused container returns
//
// Every lab whose daemons run PPP (the L2TP, PPPoE and RADIUS subscriber labs)
// hands its containers the host's /dev/ppp and refuses a host that lacks it.
// The Docker host kernel check (kernelcheck.go) cannot see a character device,
// so this half of the probe is declared here once. Each lab still runs ONE
// preflight container: it puts PPPDeviceProbe first in its own `sh -c` command,
// adds its own probe lines, parses with ParsePreflight, and refuses through
// PPPDeviceMissing and PPPPreflightRunRefusal.

package interoplab

import (
	"errors"
	"fmt"
	"strings"
)

// PPPDevice is the host's PPP character device, which module ppp_generic
// provides. A lab container never loads that module itself
// (spec-lab-containers-least-privilege, D-7).
const PPPDevice = "/dev/ppp"

// PPPDeviceProbe is the shell line a lab's preflight container runs first: it
// prints DEV_PPP=ok when the container holds the PPP character device, and
// DEV_PPP=missing when it does not.
const PPPDeviceProbe = "echo DEV_PPP=$(test -c /dev/ppp && echo ok || echo missing)"

// PPPProbeImage is the image each PPP lab runs its preflight container on. The
// probe asks the host kernel, so any Alpine userland answers the same; it
// tracks the base the lab images use.
const PPPProbeImage = "alpine:3.24.3"

// pppDeviceKey is the KEY of the line PPPDeviceProbe prints.
const pppDeviceKey = "DEV_PPP"

// PPPDeviceArguments hands a container the host's PPP character device.
func PPPDeviceArguments() []string {
	return []string{"--device", PPPDevice}
}

// ParsePreflight reads the KEY=value lines a preflight container prints. A
// line with no '=' is skipped, so a probe that prints nothing leaves its key
// absent, and the caller's check reads that absence as missing.
func ParsePreflight(output string) map[string]string {
	checks := make(map[string]string, 4)
	for line := range strings.SplitSeq(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			checks[key] = value
		}
	}
	return checks
}

// PPPDeviceMissing answers the refusal entry naming the PPP device and its
// module, and true, when the probe did not print DEV_PPP=ok; it answers false
// when the device is present.
func PPPDeviceMissing(checks map[string]string) (string, bool) {
	if checks[pppDeviceKey] == "ok" {
		return "", false
	}
	return "/dev/ppp (the PPP device, module ppp_generic)", true
}

// PPPPreflightRunRefusal explains a lab's preflight container that did not
// run. It always wraps the Docker error with %w, so the operation, exit code,
// stderr and cause the error carries survive. Docker refuses --device /dev/ppp
// before the probe starts when the host has no PPP device and says so on
// stderr; only then does the refusal add the host command that loads it.
func PPPPreflightRunRefusal(lab string, err error) error {
	stderr := err.Error()
	if refused, ok := errors.AsType[*commandError](err); ok {
		stderr = refused.Stderr
	}
	if !strings.Contains(stderr, PPPDevice) {
		return fmt.Errorf("the %s preflight container did not run: %w", lab, err)
	}
	return fmt.Errorf("the %s preflight container did not run: %w\n"+
		"The Docker host has no PPP device, and lab containers never load kernel modules.\n"+
		"Load it on the Docker host, then run the lab again:\n"+
		"  sudo modprobe ppp_generic", lab, err)
}
