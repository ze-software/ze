// Design: docs/features/ai-first.md -- the probes a registered doctor check runs
// Related: doctor_registry.go -- the registry the owner packages below register with
//
// A doctor check probes a runtime dependency before the daemon starts. Three
// of those probes are shared by checks that different packages own: can this
// TCP endpoint be reached, does this URL answer an HTTP HEAD, and can this
// directory be written. They live beside the registry because that is the one
// package every owner of a check imports, so a check moved out of
// internal/component/doctor keeps the behavior the runner had rather than
// growing a second copy of it.

package diagnostic

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// DoctorProbeTimeoutEnv caps every external-service reachability probe timeout.
// Production leaves it unset, so each check uses its own multi-second default
// (appropriate for a real operator). Functional tests set it to a small value
// so probes to deliberately unreachable fixtures fail fast instead of waiting
// out the full default; those waits (5s per HTTP HEAD, 3s per TCP/UDP dial, run
// sequentially) otherwise dominate doctor test wall-clock and tip the tests over
// their timeout budget under parallel load. See DoctorProbeTimeout.
const DoctorProbeTimeoutEnv = "ze.test.doctor.probe-timeout"

var _ = env.MustRegister(env.EnvEntry{
	Key:         DoctorProbeTimeoutEnv,
	Type:        "duration",
	Description: "Cap external-service reachability probe timeouts (doctor functional tests)",
	Private:     true,
})

// DoctorProbeTimeout returns the effective timeout for an external-service
// reachability probe: the per-check default, capped by DoctorProbeTimeoutEnv
// when that override is set and smaller. The override can only shorten a probe,
// never lengthen it, so production behavior is unchanged when the var is unset.
func DoctorProbeTimeout(def time.Duration) time.Duration {
	if override := env.GetDuration(DoctorProbeTimeoutEnv, 0); override > 0 && override < def {
		return override
	}
	return def
}

// DoctorTCPReachable reports whether a TCP connection to addr completes inside
// timeout. It opens and closes one connection and sends nothing, so a server
// that answers the handshake counts as reachable whatever it speaks.
func DoctorTCPReachable(addr string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// DoctorHTTPReachable reports whether url answers an HTTP HEAD inside timeout.
// Any answer counts, whatever its status: the probe asks whether the endpoint
// is there, and a 404 from a live server is a fact about the path rather than
// about reachability.
func DoctorHTTPReachable(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// DoctorProbeWritableDir reports whether the daemon will be able to write into
// dir, by creating and removing one file in it. It does not create dir: a
// caller that wants the daemon's own create-then-write behavior creates the
// directory first.
func DoctorProbeWritableDir(dir string) error {
	if dir == "" {
		return errors.New("empty path")
	}
	f, err := os.CreateTemp(dir, ".ze-doctor-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}
