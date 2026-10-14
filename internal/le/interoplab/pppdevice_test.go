// Design: docs/architecture/testing/interop.md -- the shared /dev/ppp preflight
// Related: pppdevice.go -- PPPPreflightRunRefusal, PPPDeviceMissing, ParsePreflight

package interoplab

import (
	"errors"
	"strings"
	"testing"
)

// VALIDATES: R-4 for a host with no /dev/ppp at all. Docker refuses the
// preflight's --device before the probe runs, and the refusal keeps Docker's
// words and the cause, and names the module that provides the device on a line
// of its own.
// PREVENTS: an operator reading only Docker's "error gathering device
// information" with no fix.
func TestPPPPreflightWithoutDevPPPNamesTheHostFix(t *testing.T) {
	stderr := `docker: Error response from daemon: error gathering device information while adding custom device "/dev/ppp": no such file or directory.`
	cause := errors.New("exit status 125")
	refused := &commandError{Operation: "docker run", ExitCode: 125, Stderr: stderr, Cause: cause}

	err := PPPPreflightRunRefusal("PPPoE", refused)
	if !strings.HasSuffix(err.Error(), "\n  sudo modprobe ppp_generic") {
		t.Errorf("refusal %q does not end with the host command on a line of its own", err)
	}
	if !strings.Contains(err.Error(), stderr) {
		t.Errorf("refusal %q drops Docker's own words", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("refusal %q drops the cause", err)
	}
	if !strings.Contains(err.Error(), "the PPPoE preflight container did not run") {
		t.Errorf("refusal %q does not name the lab", err)
	}
}

// VALIDATES: a preflight failure that is not about /dev/ppp keeps Docker's
// words, the exit code and the cause, and names no module (the L2TP case the
// spec's Preflight row requires).
// PREVENTS: a hint that sends the operator to load ppp_generic for a missing
// image, and a refusal that drops the cause, as PPPoE's own refusal did.
func TestPPPPreflightOtherFailureKeepsDockerWords(t *testing.T) {
	cause := errors.New("exit status 125")
	refused := &commandError{Operation: "docker run", ExitCode: 125, Stderr: "pull access denied for alpine", Cause: cause}

	err := PPPPreflightRunRefusal("L2TP", refused)
	if strings.Contains(err.Error(), "modprobe") {
		t.Errorf("a failure unrelated to /dev/ppp answered %q", err)
	}
	if !strings.Contains(err.Error(), "pull access denied for alpine") {
		t.Errorf("refusal %q drops Docker's own words", err)
	}
	if !strings.Contains(err.Error(), "(exit 125)") {
		t.Errorf("refusal %q drops the exit code", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("refusal %q drops the cause", err)
	}
}

// VALIDATES: the shared check reads DEV_PPP=ok as present, and anything else,
// including an absent line, as missing with the module named.
// PREVENTS: a probe that printed nothing passing as a host with the device.
func TestPPPDeviceMissing(t *testing.T) {
	if entry, absent := PPPDeviceMissing(ParsePreflight("DEV_PPP=ok\nOTHER=missing\n")); absent {
		t.Errorf("DEV_PPP=ok answered missing %q", entry)
	}
	for _, output := range []string{"DEV_PPP=missing\n", "", "noise\n"} {
		entry, absent := PPPDeviceMissing(ParsePreflight(output))
		if !absent {
			t.Errorf("output %q answered present", output)
		}
		if entry != "/dev/ppp (the PPP device, module ppp_generic)" {
			t.Errorf("output %q answered entry %q", output, entry)
		}
	}
}

// VALIDATES: the probe fragment prints the DEV_PPP line the check reads, and
// the grant hands the container the device the probe tests.
// PREVENTS: the fragment and the check drifting to two different keys.
func TestPPPDeviceProbeMatchesCheck(t *testing.T) {
	if !strings.HasPrefix(PPPDeviceProbe, "echo "+pppDeviceKey+"=") {
		t.Errorf("probe %q does not print the %s key the check reads", PPPDeviceProbe, pppDeviceKey)
	}
	if !strings.Contains(PPPDeviceProbe, "test -c "+PPPDevice) {
		t.Errorf("probe %q does not test %s", PPPDeviceProbe, PPPDevice)
	}
	arguments := PPPDeviceArguments()
	if len(arguments) != 2 || arguments[0] != "--device" || arguments[1] != PPPDevice {
		t.Errorf("grant %v, want --device %s", arguments, PPPDevice)
	}
}
