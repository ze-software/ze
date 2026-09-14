// Design: ai/rules/repo-maintenance.md -- self-contained doctor checks owned by
// the plugin that owns the runtime dependency.
// Overview: doctor.go -- registerDoctorChecks, which register.go's init() runs
// Related: health.go -- the health check that dials the same API socket
//
// The two checks here lived in internal/component/doctor and the runner
// reached them by writing their names out. Both fire only under `interface {
// backend vpp }`, which is this backend's trigger, so they are owned here now
// (ai/patterns/registration.md, "Doctor Check Registry") and dropping the
// backend drops them with it:
//
//   - api socket: the backend programs every interface through the VPP API
//     socket, so an unreachable socket is an apply that cannot start. The
//     probe dials the path `vpp { api-socket }` names, or the default the vpp
//     component applies when the leaf is absent.
//   - version: `vppctl show version` proves a VPP is running and answers on
//     its CLI socket. The check warns rather than errors, because a version
//     it cannot read is not evidence that the API socket is dead.

package ifacevpp

import (
	"context"
	"io"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeVPPUnreachable names an API socket the backend cannot dial.
// internal/core/diagnostic/codes.go declares it, so `ze explain` answers.
const codeVPPUnreachable = "doctor-vpp-unreachable"

// codeVPPVersion names a VPP whose version cannot be read, declared beside
// codeVPPUnreachable in the same registry.
const codeVPPVersion = "doctor-vpp-version"

// vppSocketProbeTimeout bounds the API socket dial. It is the timeout the
// doctor runner used before the check moved, and it is not shortened by the
// functional-test probe override because a unix socket answers or refuses at
// once.
const vppSocketProbeTimeout = 5 * time.Second

// vppVersionMarker is the text `vppctl show version` prints when a VPP
// answers; output without it is something the check does not understand.
const vppVersionMarker = "vpp v"

// vppAPISocketDial is the probe checkVPPAPISocket runs: it opens the API socket
// and hands the connection back for the check to close, so a socket that
// accepts and then fails to close is told apart from one that refuses. It is
// NIL on every platform VPP does not run on, for the reason lcpPluginProbe is:
// the skip is then a property of the value, not of a branch no test can reach.
// A var so the unit tests drive every answer on any platform; every test that
// calls checkVPPAPISocket MUST set it and MUST restore it.
var vppAPISocketDial = defaultVPPAPISocketDial()

// vppVersionProbe reports the text of `vppctl show version`, nil off Linux,
// under the same rule as vppAPISocketDial.
var vppVersionProbe = defaultVPPVersionProbe()

// defaultVPPAPISocketDial returns the socket dial for this platform, and nil
// where VPP does not run.
func defaultVPPAPISocketDial() func(context.Context, string) (io.Closer, error) {
	if runtime.GOOS != goosLinux {
		return nil
	}
	return dialVPPAPISocket
}

// defaultVPPVersionProbe returns the version probe for this platform, and nil
// where VPP does not run.
func defaultVPPVersionProbe() func(context.Context) (string, error) {
	if runtime.GOOS != goosLinux {
		return nil
	}
	return vppctlShowVersion
}

// dialVPPAPISocket connects to the VPP API socket.
func dialVPPAPISocket(ctx context.Context, path string) (io.Closer, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}

// vppctlShowVersion asks the running VPP for its version, through the CLI
// socket vppctl dials.
func vppctlShowVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "vppctl", "show", "version").Output() //nolint:gosec // fixed command, no user input
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// vppBackendConfigured reports whether the config selects this backend. A nil
// tree is the missing-config phase, which neither check runs in, and a context
// carrying anything else is a runner defect the runner's own type assertion
// reports (doctorTree, internal/component/doctor/registry.go).
func vppBackendConfigured(ctx diagnostic.DoctorCheckContext) (*config.Tree, bool) {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil, false
	}
	ifaceTree := tree.GetContainer("interface")
	if ifaceTree == nil {
		return nil, false
	}
	backend, _ := ifaceTree.Get("backend")
	if backend != backendVPP {
		return nil, false
	}
	return tree, true
}

// checkVPPAPISocket reports an API socket the backend cannot dial. A refused
// dial is an error, because no interface can be programmed; a connection that
// opened and failed to close is a warning.
func checkVPPAPISocket(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := vppBackendConfigured(ctx)
	if !ok {
		return nil
	}
	if vppAPISocketDial == nil {
		return nil
	}
	sockPath := ""
	if vpp := tree.GetContainer("vpp"); vpp != nil {
		sockPath, _ = vpp.Get("api-socket")
	}
	if sockPath == "" {
		sockPath = vppSocketPath
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), vppSocketProbeTimeout)
	defer cancel()
	var tb textbuf.Buffer
	conn, err := vppAPISocketDial(probeCtx, sockPath)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeVPPUnreachable,
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("VPP API socket unreachable: ").Str(sockPath).Str(": ").Err(err).String(),
			Path:     sockPath,
		}}
	}
	if closeErr := conn.Close(); closeErr != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeVPPUnreachable,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("VPP API socket close: ").Str(sockPath).Str(": ").Err(closeErr).String(),
			Path:     sockPath,
		}}
	}
	return nil
}

// checkVPPVersion runs `vppctl show version` and warns when the output does
// not carry a version, or when the probe fails.
func checkVPPVersion(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if _, ok := vppBackendConfigured(ctx); !ok {
		return nil
	}
	if vppVersionProbe == nil {
		return nil
	}

	probeCtx, cancel := context.WithTimeout(context.Background(), vppProbeTimeout)
	defer cancel()
	out, err := vppVersionProbe(probeCtx)
	var tb textbuf.Buffer
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeVPPVersion,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("cannot determine VPP version: ").Err(err).String(),
		}}
	}
	version := strings.TrimSpace(out)
	if !strings.Contains(version, vppVersionMarker) {
		return []diagnostic.Diagnostic{{
			Code:     codeVPPVersion,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("unexpected VPP version output: ").Str(version).String(),
		}}
	}
	return nil
}
