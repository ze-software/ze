// Design: docs/features/ai-first.md — system readiness checks for agent tooling
// Related: doctor.go — readiness check runner and output contract
// Related: checks_helpers.go — shared config-tree navigation helpers

// External-service reachability checks: probe every remote dependency named
// in config (TACACS+, DNS resolvers, NTP servers, RPKI caches, BMP
// collectors, update-check and archive URLs) plus system clock skew.
// Owner-specific reachability checks (e.g. l2tp.auth.radius) register
// through the doctor check registry from their owning package.

package doctor

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorProbeTimeoutEnv caps every external-service reachability probe timeout.
// Production leaves it unset, so each check uses its own multi-second default
// (appropriate for a real operator). Functional tests set it to a small value
// so probes to deliberately unreachable fixtures fail fast instead of waiting
// out the full default; those waits (5s per HTTP HEAD, 3s per TCP/UDP dial, run
// sequentially) otherwise dominate doctor test wall-clock and tip the tests over
// their timeout budget under parallel load. See reachProbeTimeout.
const doctorProbeTimeoutEnv = "ze.test.doctor.probe-timeout"

var _ = env.MustRegister(env.EnvEntry{
	Key:         doctorProbeTimeoutEnv,
	Type:        "duration",
	Description: "Cap external-service reachability probe timeouts (doctor functional tests)",
	Private:     true,
})

// reachProbeTimeout returns the effective timeout for an external-service
// reachability probe: the per-check default, capped by doctorProbeTimeoutEnv
// when that override is set and smaller. The override can only shorten a probe,
// never lengthen it, so production behavior is unchanged when the var is unset.
func reachProbeTimeout(def time.Duration) time.Duration {
	if override := env.GetDuration(doctorProbeTimeoutEnv, 0); override > 0 && override < def {
		return override
	}
	return def
}

var tcpReachable = tcpServerReachable

func checkTACACSServers(tree *config.Tree) []diagnostic.Diagnostic {
	tacacs := getContainerPath(tree, "system", "authentication", "tacacs")
	if tacacs == nil {
		return nil
	}

	timeout := reachProbeTimeout(configTimeout(tacacs, "timeout", 5))
	checked := false
	for _, s := range tacacs.GetListOrdered("server") {
		address := valueOrDefault(s.Value, "address", s.Key)
		if address == "" {
			continue
		}
		checked = true
		if tcpReachable(net.JoinHostPort(address, valueOrDefault(s.Value, "port", "49")), timeout) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-tacacs-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured TACACS+ servers are reachable",
	}}
}

func tcpServerReachable(addr string, timeout time.Duration) bool {
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
func checkDNSResolvers(tree *config.Tree) []diagnostic.Diagnostic {
	sysBlock := tree.GetContainer("system")
	if sysBlock == nil {
		return nil
	}
	servers := sysBlock.GetSlice("name-server")
	if len(servers) == 0 {
		return nil
	}

	if slices.ContainsFunc(servers, dnsServerResponds) {
		return nil
	}

	return []diagnostic.Diagnostic{{
		Code:     "doctor-dns-resolver",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured name servers responded",
	}}
}

// dnsServerResponds probes a DNS server with a query. Returns true if the
// server responds at all (including NXDOMAIN or SERVFAIL), false only if
// the server is unreachable or times out.
func dnsServerResponds(addr string) bool {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(addr, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), reachProbeTimeout(3*time.Second))
	defer cancel()
	_, err := resolver.LookupHost(ctx, "_dns-probe.invalid.")
	if err == nil {
		return true
	}
	// A DNS error (NXDOMAIN, SERVFAIL) means the server responded.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && !dnsErr.IsTimeout && !dnsErr.IsTemporary {
		return true
	}
	return false
}

const clockSkewThreshold = 5 * time.Minute

// checkClockSkew queries a public NTP pool and warns if the system clock
// is off by more than 5 minutes. Uses a lightweight SNTP request (mode 3)
// rather than a full NTP client.
func checkClockSkew() []diagnostic.Diagnostic {
	skewTimeout := reachProbeTimeout(3 * time.Second)
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
			Code:     "doctor-clock-skew",
			Severity: diagnostic.SeverityWarning,
			Message:  b.Reset().Str("system clock skewed by ").Int(int64(skew / time.Second)).Str("s (threshold ").Int(int64(clockSkewThreshold / time.Second)).Str("s)").String(),
		}}
	}
	return nil
}
func checkNTPClient(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	ntp := getContainerPath(tree, "environment", "ntp")
	if !configEnabled(ntp, false) {
		if severity, ok := clockNoSyncSeverity(platform); ok {
			return []diagnostic.Diagnostic{{
				Code:     "doctor-clock-no-sync",
				Severity: severity,
				Message:  clockNoSyncMessage(platform),
				Path:     "environment/ntp/enabled",
				Expected: "enabled Ze NTP or verified external clock synchronization",
				Actual:   "Ze NTP disabled",
			}}
		}
		return nil
	}

	var diags []diagnostic.Diagnostic

	servers := ntp.GetListOrdered("server")
	reachable := false
	checked := false
	for _, s := range servers {
		addr, ok := s.Value.Get("address")
		if !ok || addr == "" {
			continue
		}
		checked = true
		if ntpServerReachable(net.JoinHostPort(addr, "123"), reachProbeTimeout(3*time.Second)) {
			reachable = true
			break
		}
	}
	if checked && !reachable {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "doctor-ntp-server-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  "none of the configured NTP servers are reachable",
		})
	}

	return diags
}

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

func checkRPKIServers(tree *config.Tree) []diagnostic.Diagnostic {
	rpki := getContainerPath(tree, "bgp", "rpki")
	if rpki == nil {
		return nil
	}
	cacheServers := rpki.GetListOrdered("cache-server")
	if len(cacheServers) == 0 {
		return nil
	}

	checked := false
	for _, s := range cacheServers {
		port := valueOrDefault(s.Value, "port", "323")
		addr := s.Key
		if addr == "" {
			continue
		}
		checked = true
		if tcpReachable(net.JoinHostPort(addr, port), reachProbeTimeout(3*time.Second)) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-rpki-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured RPKI cache servers are reachable",
	}}
}

func checkBMPCollectors(tree *config.Tree) []diagnostic.Diagnostic {
	bmp := getContainerPath(tree, "bgp", "bmp", "sender")
	if bmp == nil {
		return nil
	}
	collectors := bmp.GetListOrdered("collector")
	if len(collectors) == 0 {
		return nil
	}

	checked := false
	for _, c := range collectors {
		addr, ok := c.Value.Get("address")
		if !ok || addr == "" {
			continue
		}
		checked = true
		port := valueOrDefault(c.Value, "port", "11019")
		if tcpReachable(net.JoinHostPort(addr, port), reachProbeTimeout(3*time.Second)) {
			return nil
		}
	}
	if !checked {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-bmp-unreachable",
		Severity: diagnostic.SeverityWarning,
		Message:  "none of the configured BMP collectors are reachable",
	}}
}

var ntpServerReachable = probeNTPServer

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
	req := make([]byte, 48)
	req[0] = 0x1B // SNTP: LI=0, VN=3, Mode=3 (client)
	if _, writeErr := conn.Write(req); writeErr != nil {
		return false
	}
	resp := make([]byte, 48)
	_, readErr := conn.Read(resp)
	return readErr == nil
}

var httpHead = defaultHTTPHead

func defaultHTTPHead(url string, timeout time.Duration) error {
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

func checkUpdateCheckURL(tree *config.Tree, platform *host.PlatformInfo) []diagnostic.Diagnostic {
	uc := getContainerPath(tree, "system", "update-check")
	if uc == nil {
		return nil
	}
	if platform != nil && platform.Type == host.PlatformGokrazy {
		return nil
	}
	url, ok := uc.Get("url")
	if !ok || url == "" {
		return nil
	}

	if err := httpHead(url, reachProbeTimeout(5*time.Second)); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     "doctor-update-check-unreachable",
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("update-check URL unreachable: ").Err(err).String(),
			Path:     url,
		}}
	}
	return nil
}

func checkArchiveDestinations(tree *config.Tree) []diagnostic.Diagnostic {
	system := tree.GetContainer("system")
	if system == nil {
		return nil
	}
	archives := system.GetListOrdered("archive")
	if len(archives) == 0 {
		return nil
	}

	var diags []diagnostic.Diagnostic
	for _, a := range archives {
		loc, ok := a.Value.Get("location")
		if !ok || loc == "" {
			continue
		}
		if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") {
			continue
		}
		if err := httpHead(loc, reachProbeTimeout(5*time.Second)); err != nil {
			var tb textbuf.Buffer
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "doctor-archive-unreachable",
				Severity: diagnostic.SeverityWarning,
				Message:  tb.Str("archive ").Str(a.Key).Str(": location unreachable: ").Err(err).String(),
				Path:     loc,
			})
		}
	}
	return diags
}
