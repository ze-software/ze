// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract

// External-service reachability: the system clock skew probe.
// Owner-specific reachability checks register through the doctor check
// registry from their owning package: l2tp.auth.radius, the BGP RPKI caches
// (bgp/plugins/rpki/doctor.go), the BMP collectors (bgp/plugins/bmp/doctor.go),
// the NTP servers (plugins/ntp/doctor.go), the TACACS+ servers
// (tacacs/doctor.go), the DNS resolvers and the update-check URL
// (config/system/doctor.go) and the archive destinations
// (config/archive/doctor.go).

package doctor

import (
	"context"
	"net"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// reachProbeTimeout and the probes now live in internal/core/diagnostic, beside
// the doctor check registry, so a check an owner package registers runs the
// same probe this runner does (diagnostic.DoctorProbeTimeout,
// diagnostic.DoctorTCPReachable, diagnostic.DoctorHTTPReachable).

const clockSkewThreshold = 5 * time.Minute

// checkClockSkew queries a public NTP pool and warns if the system clock
// is off by more than 5 minutes. Uses a lightweight SNTP request (mode 3)
// rather than a full NTP client.
func checkClockSkew() []diagnostic.Diagnostic {
	skewTimeout := diagnostic.DoctorProbeTimeout(3 * time.Second)
	dialer := net.Dialer{Timeout: skewTimeout}
	conn, err := dialer.DialContext(context.Background(), "udp", "pool.ntp.org:123")
	if err != nil {
		return nil // network unavailable, skip silently
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(skewTimeout))

	// SNTP request: version 3, mode 3 (client), 48 bytes.
	req := make([]byte, 48)
	req[0] = 0x1B // LI=0, VN=3, Mode=3
	if _, err := conn.Write(req); err != nil {
		return nil
	}

	resp := make([]byte, 48)
	if _, err := conn.Read(resp); err != nil {
		return nil
	}

	// Transmit timestamp starts at byte 40 (seconds since 1900-01-01).
	const ntpEpochOffset = 2208988800 // seconds between 1900 and 1970
	secs := uint64(resp[40])<<24 | uint64(resp[41])<<16 | uint64(resp[42])<<8 | uint64(resp[43])
	if secs < ntpEpochOffset {
		return nil // invalid response
	}
	ntpTime := time.Unix(int64(secs-ntpEpochOffset), 0)
	skew := time.Since(ntpTime)
	if skew < 0 {
		skew = -skew
	}

	if skew > clockSkewThreshold {
		var b textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorClockSkew,
			Severity: diagnostic.SeverityWarning,
			Message:  b.Reset().Str("system clock skewed by ").Int(int64(skew / time.Second)).Str("s (threshold ").Int(int64(clockSkewThreshold / time.Second)).Str("s)").String(),
		}}
	}
	return nil
}
