// Design: docs/features/interfaces.md -- the readiness checks this plugin owns
// Overview: register.go -- the init() that installs the registrations below
// Related: doctor_linux.go, doctor_other.go -- the CAP_SYS_TIME probe, per platform
// Related: ntp.go -- the client that queries the servers this check probes
//
// Both checks lived in internal/component/doctor and the runner reached each
// one by writing its name out. Each reads `environment { ntp }`, the block this
// plugin's YANG declares (ai/patterns/registration.md, "Doctor Check
// Registry"), so they are owned here now and dropping this plugin drops them
// with it:
//
//   - client: an enabled client whose servers all fail an SNTP probe cannot
//     synchronize. A DISABLED client is a finding too, on the platforms where
//     nothing else keeps the clock: gokrazy has no other clock source, so it
//     is an error there, and a warning on a Linux host whose external
//     synchronization the operator has to verify. Without this plugin the
//     advice to enable environment/ntp cannot be followed, which is why the
//     disabled verdict lives here and not with the doctor runner.
//   - clock privilege: setClock and slewClock need CAP_SYS_TIME, which root
//     always holds and an unprivileged daemon has to be granted.

package ntp

import (
	"context"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The codes the checks emit. internal/core/diagnostic/codes.go declares each
// one, so `ze explain <code>` answers. Each is its own declaration: one code
// is one fact, and a block of them reads as a second list of the code
// registry (./le enumeration check).

// codeClockNoSync names a disabled client on a platform ze keeps the clock on.
const codeClockNoSync = "doctor-clock-no-sync"

// codeNTPServerUnreachable names an enabled client none of whose servers
// answers.
const codeNTPServerUnreachable = "doctor-ntp-server-unreachable"

// codeNTPClockPrivilege names a process without CAP_SYS_TIME.
const codeNTPClockPrivilege = "doctor-ntp-clock-privilege"

// doctorComponentNTP names this plugin as the owner of the checks it registers.
const doctorComponentNTP = "ntp"

// The server probe: one 48-byte SNTP client request (LI=0, VN=3, Mode=3) to
// the NTP port, bounded by ntpServerProbeTimeout, which DoctorProbeTimeout can
// only shorten.
const (
	ntpServerProbeTimeout      = 3 * time.Second
	ntpPort                    = "123"
	sntpClientRequest     byte = 0x1B
	sntpPacketLen              = 48
)

// ntpServerReachable is the probe checkNTPClient runs for each server. It is a
// variable so a test stands in an unreachable server; nothing else assigns it.
var ntpServerReachable = probeNTPServer

// ntpDoctorChecks are the checks this plugin owns, in the order the doctor
// runner ran them.
//
// Orders 2190 and 2195 reproduce the sequence the doctor runner printed before
// the checks moved onto the registry: both ran after the AS112 coordination
// checks (2150-2180, internal/component/doctor/doctor_checks.go) and before
// the RPKI cache check (2200, bgp/plugins/rpki/doctor.go), client first.
var ntpDoctorChecks = []diagnostic.DoctorCheck{{
	Name:         "ntp-client",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2190,
	Component:    doctorComponentNTP,
	Dependencies: []string{"ntp-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeClockNoSync, codeNTPServerUnreachable},
	Check:        checkNTPClient,
}, {
	Name:         "ntp-clock-privilege",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2195,
	Component:    doctorComponentNTP,
	Dependencies: []string{"capabilities"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeNTPClockPrivilege},
	Check:        checkNTPClockPrivilege,
}}

// registerNTPDoctorChecks installs every entry above. A refusal is a
// programmer error in the table beside it and stops the process rather than
// leaving `ze doctor` quietly short of a check.
func registerNTPDoctorChecks() {
	for i := range ntpDoctorChecks {
		if err := diagnostic.RegisterDoctorCheck(ntpDoctorChecks[i]); err != nil {
			panic("BUG: ntp doctor check registration refused: " + err.Error())
		}
	}
}

// ntpEnabled answers the environment/ntp container and whether the client is
// enabled. An absent block, or one without `enabled true`, is a disabled
// client.
func ntpEnabled(tree *config.Tree) (*config.Tree, bool) {
	ntp := tree.GetContainerPath(configRootEnvironment + "/ntp")
	if ntp == nil {
		return nil, false
	}
	enabled, _ := ntp.Get("enabled")
	return ntp, enabled == "true"
}

// checkNTPClient reports a disabled client on a platform that needs one, and
// an enabled client none of whose servers answers an SNTP probe. One answer is
// enough: the client takes the servers in order and needs only one.
//
// A nil tree is the missing-config phase, which this check does not run in,
// and a context carrying anything else is a runner defect the runner's own
// type assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkNTPClient(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ntp, enabled := ntpEnabled(tree)
	if !enabled {
		if severity, ok := clockNoSyncSeverity(ctx.Platform); ok {
			return []diagnostic.Diagnostic{{
				Code:     codeClockNoSync,
				Severity: severity,
				Message:  clockNoSyncMessage(ctx.Platform),
				Path:     "environment/ntp/enabled",
				Expected: "enabled Ze NTP or verified external clock synchronization",
				Actual:   "Ze NTP disabled",
			}}
		}
		return nil
	}

	reachable := false
	checked := false
	for _, s := range ntp.GetListOrdered("server") {
		addr, ok := s.Value.Get("address")
		if !ok || addr == "" {
			continue
		}
		checked = true
		if ntpServerReachable(net.JoinHostPort(addr, ntpPort), diagnostic.DoctorProbeTimeout(ntpServerProbeTimeout)) {
			reachable = true
			break
		}
	}
	if checked && !reachable {
		return []diagnostic.Diagnostic{{
			Code:     codeNTPServerUnreachable,
			Severity: diagnostic.SeverityWarning,
			Message:  "none of the configured NTP servers are reachable",
		}}
	}
	return nil
}

// clockNoSyncSeverity answers how a disabled client is reported on platform:
// an error on gokrazy, where nothing else keeps the clock, a warning on a
// Linux host, and nothing at all where the platform is unknown or not one ze
// manages the clock on.
func clockNoSyncSeverity(platform *host.PlatformInfo) (diagnostic.Severity, bool) {
	if platform == nil {
		return "", false
	}
	switch platform.Type {
	case host.PlatformGokrazy:
		return diagnostic.SeverityError, true
	case host.PlatformSystemd, host.PlatformContainer, host.PlatformPlainLinux:
		return diagnostic.SeverityWarning, true
	default:
		return "", false
	}
}

// clockNoSyncMessage names the platform in the disabled-client finding.
func clockNoSyncMessage(platform *host.PlatformInfo) string {
	if platform != nil && platform.Type == host.PlatformGokrazy {
		return "gokrazy platform has no configured clock synchronization; enable environment/ntp because Ze owns appliance services"
	}
	if platform != nil {
		var tb textbuf.Buffer
		return tb.Str("Ze NTP is disabled on ").Str(platform.Type.String()).Str("; verify external clock synchronization or enable environment/ntp").String()
	}
	return "Ze NTP is disabled; verify external clock synchronization or enable environment/ntp"
}

// probeNTPServer sends one SNTP client request to addr and reports whether
// anything came back. It reads no timestamp: the question is reachability,
// and the clock-skew check (internal/component/doctor) is the one that reads
// the answer.
func probeNTPServer(addr string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	if deadlineErr := conn.SetDeadline(time.Now().Add(timeout)); deadlineErr != nil {
		return false
	}
	req := make([]byte, sntpPacketLen)
	req[0] = sntpClientRequest
	if _, writeErr := conn.Write(req); writeErr != nil {
		return false
	}
	resp := make([]byte, sntpPacketLen)
	_, readErr := conn.Read(resp)
	return readErr == nil
}
