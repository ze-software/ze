// Design: docs/features/ai-first.md -- the readiness checks this component owns
// Overview: register.go -- the init() that installs every entry below
// Related: system.go -- ExtractSystemConfig, the parse of the leaves these checks read
// Related: resolv_linux.go -- WriteResolvConf, the writer of the path the resolv.conf check judges
// Related: update.go, backend_gokrazy.go -- the update check and the platform that ignores its config
//
// These four checks lived in internal/component/doctor and the runner reached
// each one by writing its name out. Each reads a leaf ze-system-conf.yang
// declares and this package consumes: the name servers it writes to
// resolv.conf, the resolv.conf path itself, and the update-check block
// (ai/patterns/registration.md, "Doctor Check Registry"). So they are owned
// here now, and dropping this component drops them with it.
//
// The Order of each entry reproduces the position it held in the runner's
// sequence. Every one ran after the runner's own phase dispatch, so each sorts
// in the 2000 band that internal/component/doctor/doctor_checks.go reserves
// for those; what the phase guarantees is that the config is loaded, and
// order inside it decides only the order the diagnostics print in.

package system

import (
	"context"
	"errors"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeDNSResolver names a configuration none of whose name servers answers.
// internal/core/diagnostic/codes.go declares it, as it does the two codes
// below, so `ze explain <code>` answers for each.
const codeDNSResolver = "doctor-dns-resolver"

// codeUpdateCheckUnreachable names an update-check URL that answers no HEAD.
const codeUpdateCheckUnreachable = "doctor-update-check-unreachable"

// codeConfigPlatformMismatch names a leaf whose value fits another platform
// than the one the daemon runs on.
const codeConfigPlatformMismatch = "doctor-config-platform-mismatch"

// doctorComponentSystem names this package as the owner of every check it
// registers.
const doctorComponentSystem = "system"

// The resolv.conf path each platform expects. The gokrazy path is the YANG
// default (ze-system-conf.yang, dns/resolv-conf-path), because that rootfs is
// read-only under /etc.
const (
	gokrazyResolvConfPath = "/tmp/resolv.conf"
	linuxResolvConfPath   = "/etc/resolv.conf"
)

// Probe bounds. DoctorProbeTimeout can only shorten them.
const (
	dnsProbeTimeout         = 3 * time.Second
	updateCheckProbeTimeout = 5 * time.Second
)

// The two probes the checks below run. Each is a variable so a test can stand
// in an unreachable server; nothing else assigns them.
var (
	dnsResolverResponds = dnsServerResponds
	updateCheckHTTPHead = diagnostic.DoctorHTTPReachable
)

// systemDoctorChecks are the readiness checks this package owns, in the order
// the doctor runner ran them.
var systemDoctorChecks = []diagnostic.DoctorCheck{{
	// Ran right after the SSH host key check (2010,
	// internal/component/ssh/doctor.go) and before the TACACS+ server check
	// (2032, internal/component/tacacs/doctor.go).
	Name:         "dns-resolvers",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2030,
	Component:    doctorComponentSystem,
	Dependencies: []string{"dns-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeDNSResolver},
	Check:        checkDNSResolvers,
}, {
	// Ran after the BMP collector check (2210, bgp/plugins/bmp/doctor.go) and
	// before the update backend check right below.
	Name:         "update-check-url",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2230,
	Component:    doctorComponentSystem,
	Dependencies: []string{"update-server"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeUpdateCheckUnreachable},
	Check:        checkUpdateCheckURL,
}, {
	Name:         "update-backend-config",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2240,
	Component:    doctorComponentSystem,
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeConfigPlatformMismatch},
	Check:        checkUpdateBackendConfig,
}, {
	// Ran after the BGP capture directory check (2280,
	// bgp/config/doctor_checks.go).
	Name:         "resolv-conf-path",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2290,
	Component:    doctorComponentSystem,
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeConfigPlatformMismatch},
	Check:        checkResolvConfPath,
}}

// registerSystemDoctorChecks installs every entry above. A refusal is a
// programmer error in the table beside it -- a duplicate name, a phase that
// does not exist, a code without the doctor prefix -- and none of them can be
// reached from a config or a peer, so it stops the process rather than leaving
// `ze doctor` quietly short of a check.
func registerSystemDoctorChecks() {
	for i := range systemDoctorChecks {
		if err := diagnostic.RegisterDoctorCheck(systemDoctorChecks[i]); err != nil {
			panic("BUG: system doctor check registration refused: " + err.Error())
		}
	}
}

// doctorTree answers the config tree a registered check reads.
//
// A nil tree is the missing-config phase, which these checks do not run in,
// and a context carrying anything else is a runner defect the runner's own
// type assertion reports (doctorTree, internal/component/doctor/registry.go).
func doctorTree(ctx diagnostic.DoctorCheckContext) *config.Tree {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return nil
	}
	return tree
}

// checkDNSResolvers warns when the config names name servers and none of them
// answers a query. One answer is enough, whatever it says: the resolver takes
// the servers in order and needs only one to be there.
func checkDNSResolvers(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	sysBlock := tree.GetContainer("system")
	if sysBlock == nil {
		return nil
	}
	servers := sysBlock.GetSlice("name-server")
	if len(servers) == 0 {
		return nil
	}

	if slices.ContainsFunc(servers, dnsResolverResponds) {
		return nil
	}

	return []diagnostic.Diagnostic{{
		Code:     codeDNSResolver,
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
	ctx, cancel := context.WithTimeout(context.Background(), diagnostic.DoctorProbeTimeout(dnsProbeTimeout))
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

// checkUpdateCheckURL warns when the update-check URL answers no HTTP HEAD.
// gokrazy is skipped: that platform ignores the block, which
// checkUpdateBackendConfig reports instead.
func checkUpdateCheckURL(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	uc := updateCheckBlock(tree)
	if uc == nil {
		return nil
	}
	if ctx.Platform != nil && ctx.Platform.Type == host.PlatformGokrazy {
		return nil
	}
	url, ok := uc.Get("url")
	if !ok || url == "" {
		return nil
	}

	if err := updateCheckHTTPHead(url, diagnostic.DoctorProbeTimeout(updateCheckProbeTimeout)); err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     codeUpdateCheckUnreachable,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("update-check URL unreachable: ").Err(err).String(),
			Path:     url,
		}}
	}
	return nil
}

// checkUpdateBackendConfig warns when a gokrazy appliance carries an
// update-check block: gokrazy manages the image, so the block is ignored
// (backend_gokrazy.go).
func checkUpdateBackendConfig(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Platform == nil || ctx.Platform.Type != host.PlatformGokrazy {
		return nil
	}
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	if updateCheckBlock(tree) == nil {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeConfigPlatformMismatch,
		Severity: diagnostic.SeverityWarning,
		Message:  "system update-check config is ignored on gokrazy; image updates are managed by gokrazy",
		Path:     "system/update-check",
	}}
}

// updateCheckBlock answers the system/update-check container, or nil when the
// config carries none.
func updateCheckBlock(tree *config.Tree) *config.Tree {
	sysBlock := tree.GetContainer("system")
	if sysBlock == nil {
		return nil
	}
	return sysBlock.GetContainer("update-check")
}

// checkResolvConfPath warns when the resolv.conf path the config names does
// not fit the platform: a path under /etc on gokrazy, whose rootfs is
// read-only there, or the gokrazy default on a standard Linux, where the
// resolver reads /etc/resolv.conf.
func checkResolvConfPath(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	if ctx.Platform == nil {
		return nil
	}
	tree := doctorTree(ctx)
	if tree == nil {
		return nil
	}
	path := effectiveResolvConfPath(tree)
	if path == "" {
		return nil
	}
	switch ctx.Platform.Type {
	case host.PlatformGokrazy:
		if strings.HasPrefix(path, "/etc/") {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path points at read-only gokrazy root filesystem",
				gokrazyResolvConfPath,
				path,
			)}
		}
	case host.PlatformSystemd, host.PlatformPlainLinux:
		if path == gokrazyResolvConfPath {
			return []diagnostic.Diagnostic{platformMismatch(
				"DNS resolv-conf-path uses gokrazy default on "+ctx.Platform.Type.String(),
				linuxResolvConfPath,
				path,
			)}
		}
	default:
		return nil
	}
	return nil
}

// effectiveResolvConfPath answers the resolv-conf-path leaf as written, or the
// YANG default when the config carries none.
func effectiveResolvConfPath(tree *config.Tree) string {
	path := gokrazyResolvConfPath
	if sysBlock := tree.GetContainer("system"); sysBlock != nil {
		if dns := sysBlock.GetContainer("dns"); dns != nil {
			if value, ok := dns.Get("resolv-conf-path"); ok {
				path = value
			}
		}
	}
	return path
}

// platformMismatch builds the resolv.conf finding: the path the config names
// against the one the platform expects.
func platformMismatch(message, expected, actual string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     codeConfigPlatformMismatch,
		Severity: diagnostic.SeverityWarning,
		Message:  message,
		Path:     "system/dns/resolv-conf-path",
		Expected: expected,
		Actual:   actual,
	}
}
