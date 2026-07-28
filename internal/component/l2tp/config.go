// Design: docs/research/l2tpv2-ze-integration.md -- subsystem config extraction
// Related: subsystem.go -- consumes Parameters returned by ExtractParameters
// RFC: rfc/short/rfc2661.md -- RFC 2661 Section 6.1 (SCCRQ dial target), Section 4.2 (shared secret)

package l2tp

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/env"
)

var (
	errL2tpAuthMethodNoneRequiresAllow  = errors.New("l2tp auth-method none requires allow-no-auth true")
	errL2tpHelloIntervalMustBe0         = errors.New("l2tp hello-interval: must be > 0")
	errL2tpMaxLoginsMustBe1             = errors.New("l2tp max-logins: must be 1-1000000")
	errL2tpEventRingSizePerSession      = errors.New("l2tp event-ring-size-per-session: must be 16-4096")
	errL2tpSampleRetentionSecondsMustBe = errors.New("l2tp sample-retention-seconds: must be 100-86400")
	errL2tpRemoteMissingAddress         = errors.New("l2tp remote: address is required")
	errL2tpRemoteDuplicate              = errors.New("l2tp remote: duplicate name")
	errL2tpRelayUnknownRemote           = errors.New("l2tp relay: references unknown remote")
)

// Env var registrations. Each YANG leaf under `environment/l2tp/` that has
// a runtime-visible counterpart is listed here; the env value overrides the
// YANG default when both are present. See `rules/config-design.md`.
var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.log.l2tp",
		Type:        "string",
		Default:     "warn",
		Description: "Log level for the L2TP subsystem",
		Private:     true,
	})
	// Test-only: skip the modprobe l2tp_ppp / pppol2tp probe at Start.
	// The .ci test harness sets this so show-l2tp-empty.ci can verify
	// the CLI wiring without CAP_NET_ADMIN. Production paths leave it
	// unset, so the real probe runs and surfaces loader errors.
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.skip-kernel-probe",
		Type:        "bool",
		Default:     "false",
		Description: "Skip kernel module probe at Start (test-only; bypasses modprobe for L2TP CLI tests)",
		Private:     true,
	})
	// Test-only: build no kernel worker at all, so a session establishes on the
	// control plane and nothing is programmed into the kernel. This is what lets
	// the CLI-surface tests (show l2tp sessions/history/session-detail, teardown
	// session) run without CAP_NET_ADMIN on a host where l2tp_netlink IS loaded
	// -- there, skip-kernel-probe alone is not enough, because the probe is not
	// what needs the privilege; the genl tunnel create is.
	//
	// Deliberately SEPARATE from ze.l2tp.skip-kernel-probe rather than folded
	// into it: test/l2tp/session-stopccn-cascade.ci sets that knob and still
	// requires the data plane.
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.disable-kernel-dataplane",
		Type:        "bool",
		Default:     "false",
		Description: "Build no kernel worker, so sessions establish on the control plane only (test-only; L2TP CLI tests without CAP_NET_ADMIN)",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ze.l2tp.metrics.poll-interval",
		Type:        "string",
		Default:     "30s",
		Description: "Interval between pppN interface stats reads for Prometheus counters",
	})
)

// Default listener and protocol values.
const (
	DefaultListenIP   = "0.0.0.0"
	DefaultListenPort = 1701
	DefaultHelloSecs  = 60
	// DefaultHelloRetries is the number of consecutive unanswered HELLO
	// keepalive intervals tolerated before an Established tunnel's peer is
	// declared dead. Effective dead-peer detection time is
	// DefaultHelloRetries * hello-interval. Zero disables dead-peer
	// detection (retransmit exhaustion remains the only signal).
	DefaultHelloRetries uint8  = 2
	DefaultMaxTunnels   uint16 = 1024
	DefaultMaxSessions  uint16 = 1024
	DefaultAuthMethod          = ppp.AuthMethodCHAPMD5

	DefaultAuthTimeoutSecs    = 30
	DefaultReauthIntervalSecs = 0
	DefaultNCPTimeoutSecs     = 30

	configTrue = "true"
)

// Parameters is the parsed L2TP subsystem configuration.
//
// The zero value is a disabled subsystem. Start is safe to call on the zero
// value and is a no-op (returns nil) until Enabled is true.
type Parameters struct {
	Enabled       bool
	ListenAddrs   []netip.AddrPort
	MaxTunnels    uint16
	MaxSessions   uint16
	AuthMethod    ppp.AuthMethod
	AllowNoAuth   bool
	HelloInterval time.Duration
	// HelloRetries is the number of consecutive unanswered HELLO keepalive
	// intervals tolerated before an Established tunnel's peer is declared
	// dead and the tunnel is torn down. Effective dead-peer detection time
	// is HelloRetries * HelloInterval, measured from the last proof of peer
	// liveness (a delivered control message OR an acknowledgement of one of
	// our outstanding messages, including a ZLB ACK of a HELLO). Zero
	// disables dead-peer detection. See spec-l2tp-dead-peer-detection.
	HelloRetries uint8
	// SharedSecret is the CHAP-MD5 tunnel authentication secret (RFC 2661
	// S4.2). Empty means peers that include a Challenge AVP in SCCRQ will
	// be rejected with StopCCN Result Code 4 (Not Authorized).
	SharedSecret string

	// PPP authentication phase settings.
	AuthTimeout    time.Duration
	ReauthInterval time.Duration

	// NCP settings.
	EnableIPCP   bool
	EnableIPv6CP bool
	NCPTimeout   time.Duration

	// CQM observer parameters (spec-l2tp-9-observer).
	CQMEnabled              bool
	MaxLogins               int
	EventRingSizePerSession int
	SampleRetentionSeconds  int

	// Remotes are configured L2TP dial targets (spec-followup-l2tp-call
	// AC-6). Each names a remote LNS/LAC endpoint ze can initiate a tunnel
	// toward. Referenced by name from the outgoing-call RPC and PPPoE relay
	// bindings. Empty when no `remote` blocks are configured.
	Remotes []Remote
	// Relays bind a PPPoE Service-Name to the remote its subscribers are
	// relayed to (LAC incoming call). Empty when no `relay` blocks exist.
	Relays []RelayBinding
}

// Remote is a configured L2TP dial target: a remote LNS/LAC endpoint ze
// initiates tunnels toward (sends SCCRQ to). Address carries the resolved
// control-plane endpoint (IP + port, RFC 2661 default 1701).
type Remote struct {
	Name          string
	Address       netip.AddrPort
	SharedSecret  string
	OutgoingCalls bool
}

// RelayBinding maps a PPPoE Service-Name to the L2TP remote its subscribers
// are relayed to. Service is the PPPoE Service-Name to match (empty string
// matches a request with no service-name); Remote references a Remote.Name.
type RelayBinding struct {
	Service string
	Remote  string
}

// ExtractParameters pulls L2TP configuration out of the parsed config tree.
//
// Protocol settings (enabled, shared-secret, auth-method, allow-no-auth,
// hello-interval, max-tunnels, max-sessions) live under the root-level
// `l2tp {}` block. Listener endpoints live under `environment { l2tp {
// server <name> { ip ...; port ...; } } }`.
//
// Returns a zero-value Parameters (Enabled=false) if no `l2tp {}` block is
// present.
func ExtractParameters(tree *config.Tree) (Parameters, error) {
	if tree == nil {
		return Parameters{}, nil
	}

	// Protocol settings from root-level l2tp{}.
	l2tpRoot := tree.GetContainer("l2tp")
	if l2tpRoot == nil {
		return Parameters{}, nil
	}

	p := Parameters{
		Enabled:        true, // presence of l2tp{} implies enabled
		HelloInterval:  time.Duration(DefaultHelloSecs) * time.Second,
		HelloRetries:   DefaultHelloRetries,
		MaxTunnels:     DefaultMaxTunnels,
		MaxSessions:    DefaultMaxSessions,
		AuthMethod:     DefaultAuthMethod,
		AuthTimeout:    DefaultAuthTimeoutSecs * time.Second,
		ReauthInterval: DefaultReauthIntervalSecs * time.Second,
		EnableIPCP:     true,
		EnableIPv6CP:   true,
		NCPTimeout:     DefaultNCPTimeoutSecs * time.Second,
	}

	if v, ok := l2tpRoot.Get("enabled"); ok {
		p.Enabled = v == configTrue
	}

	if v, ok := l2tpRoot.Get("max-tunnels"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-tunnels: %w", err)
		}
		p.MaxTunnels = uint16(n)
	}

	if v, ok := l2tpRoot.Get("max-sessions"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-sessions: %w", err)
		}
		p.MaxSessions = uint16(n)
	}

	if v, ok := l2tpRoot.Get("auth-method"); ok {
		m, err := parsePPPAuthMethod(v)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp auth-method: %w", err)
		}
		p.AuthMethod = m
	}

	if v, ok := l2tpRoot.Get("allow-no-auth"); ok {
		p.AllowNoAuth = v == configTrue
	}
	if p.AuthMethod == ppp.AuthMethodNone && !p.AllowNoAuth {
		return Parameters{}, errL2tpAuthMethodNoneRequiresAllow
	}

	if v, ok := l2tpRoot.Get("hello-interval"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp hello-interval: %w", err)
		}
		if n == 0 {
			return Parameters{}, errL2tpHelloIntervalMustBe0
		}
		p.HelloInterval = time.Duration(n) * time.Second
	}

	if v, ok := l2tpRoot.Get("hello-retries"); ok {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp hello-retries: %w", err)
		}
		p.HelloRetries = uint8(n)
	}

	if v, ok := l2tpRoot.Get("shared-secret"); ok {
		p.SharedSecret = v
	}

	// Authentication container.
	if authC := l2tpRoot.GetContainer("authentication"); authC != nil {
		if v, ok := authC.Get("timeout"); ok {
			n, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return Parameters{}, fmt.Errorf("l2tp authentication timeout: %w", err)
			}
			p.AuthTimeout = time.Duration(n) * time.Second
		}
		if v, ok := authC.Get("reauth-interval"); ok {
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return Parameters{}, fmt.Errorf("l2tp authentication reauth-interval: %w", err)
			}
			p.ReauthInterval = time.Duration(n) * time.Second
		}
	}

	// NCP container.
	if ncpC := l2tpRoot.GetContainer("ncp"); ncpC != nil {
		if v, ok := ncpC.Get("enable-ipcp"); ok {
			p.EnableIPCP = v == configTrue
		}
		if v, ok := ncpC.Get("enable-ipv6cp"); ok {
			p.EnableIPv6CP = v == configTrue
		}
		if v, ok := ncpC.Get("timeout"); ok {
			n, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return Parameters{}, fmt.Errorf("l2tp ncp timeout: %w", err)
			}
			p.NCPTimeout = time.Duration(n) * time.Second
		}
	}

	// CQM observer (spec-l2tp-9-observer).
	if v, ok := l2tpRoot.Get("cqm-enabled"); ok {
		p.CQMEnabled = v == configTrue
	}
	p.MaxLogins = 1000
	if v, ok := l2tpRoot.Get("max-logins"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp max-logins: %w", err)
		}
		if n == 0 || n > 1000000 {
			return Parameters{}, errL2tpMaxLoginsMustBe1
		}
		p.MaxLogins = int(n)
	}
	p.EventRingSizePerSession = 256
	if v, ok := l2tpRoot.Get("event-ring-size-per-session"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp event-ring-size-per-session: %w", err)
		}
		if n < 16 || n > 4096 {
			return Parameters{}, errL2tpEventRingSizePerSession
		}
		p.EventRingSizePerSession = int(n)
	}
	p.SampleRetentionSeconds = 86400
	if v, ok := l2tpRoot.Get("sample-retention-seconds"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return Parameters{}, fmt.Errorf("l2tp sample-retention-seconds: %w", err)
		}
		if n < 100 || n > 86400 {
			return Parameters{}, errL2tpSampleRetentionSecondsMustBe
		}
		p.SampleRetentionSeconds = int(n)
	}

	// Dial targets (spec-followup-l2tp-call AC-6): remote list under root
	// l2tp{}. Each `remote <name>` names an endpoint ze can dial.
	remotes := l2tpRoot.GetListOrdered("remote")
	for _, r := range remotes {
		addr, _ := r.Value.Get("address")
		port := strconv.Itoa(DefaultListenPort)
		if v, ok := r.Value.Get("port"); ok && v != "" {
			port = v
		}
		secret, _ := r.Value.Get("shared-secret")
		outgoing := false
		if v, ok := r.Value.Get("outgoing-calls"); ok {
			outgoing = v == configTrue
		}
		rem, err := buildRemote(r.Key, addr, port, secret, outgoing)
		if err != nil {
			return Parameters{}, err
		}
		if err := appendRemote(&p, rem); err != nil {
			return Parameters{}, err
		}
	}

	// Relay bindings: relay list under root l2tp{}. Each binds a PPPoE
	// Service-Name to a remote declared above.
	for _, rl := range l2tpRoot.GetListOrdered("relay") {
		remoteName, _ := rl.Value.Get("remote")
		p.Relays = append(p.Relays, RelayBinding{Service: rl.Key, Remote: remoteName})
	}
	if err := validateRelays(&p); err != nil {
		return Parameters{}, err
	}

	// Listener endpoints from environment { l2tp { server ... } }.
	if envC := tree.GetContainer("environment"); envC != nil {
		if l2tpEnv := envC.GetContainer("l2tp"); l2tpEnv != nil {
			servers := l2tpEnv.GetListOrdered("server")
			for _, s := range servers {
				ip := DefaultListenIP
				port := strconv.Itoa(DefaultListenPort)
				if v, ok := s.Value.Get("ip"); ok && v != "" {
					ip = v
				}
				if v, ok := s.Value.Get("port"); ok && v != "" {
					port = v
				}
				addr, err := parseListen(ip, port)
				if err != nil {
					return Parameters{}, fmt.Errorf("l2tp server %q: %w", s.Key, err)
				}
				p.ListenAddrs = append(p.ListenAddrs, addr)
			}
		}
	}

	return p, nil
}

func parsePPPAuthMethod(v string) (ppp.AuthMethod, error) {
	switch v {
	case "none":
		return ppp.AuthMethodNone, nil
	case "pap":
		return ppp.AuthMethodPAP, nil
	case "chap-md5":
		return ppp.AuthMethodCHAPMD5, nil
	case "ms-chap-v2":
		return ppp.AuthMethodMSCHAPv2, nil
	}
	return ppp.AuthMethodNone, fmt.Errorf("unsupported method %q", v)
}

func parseListen(ip, port string) (netip.AddrPort, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("ip %q: %w", ip, err)
	}
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("port %q: %w", port, err)
	}
	if p == 0 {
		return netip.AddrPort{}, fmt.Errorf("port %q: must be 1-65535", port)
	}
	return netip.AddrPortFrom(addr, uint16(p)), nil
}

// buildRemote validates and assembles a Remote from its raw config fields.
// address is mandatory (a dial target with no address is meaningless);
// port defaults to 1701 upstream when absent. The parsed endpoint reuses
// parseListen's IP+port validation.
func buildRemote(name, address, port, secret string, outgoing bool) (Remote, error) {
	if address == "" {
		return Remote{}, fmt.Errorf("%w (remote %q)", errL2tpRemoteMissingAddress, name)
	}
	addr, err := parseListen(address, port)
	if err != nil {
		return Remote{}, fmt.Errorf("l2tp remote %q: %w", name, err)
	}
	return Remote{
		Name:          name,
		Address:       addr,
		SharedSecret:  secret,
		OutgoingCalls: outgoing,
	}, nil
}

// appendRemote adds rem to p.Remotes, rejecting a duplicate name. The YANG
// list key already enforces uniqueness in the config tree; this guards the
// provider-map path (which is not key-deduplicated) and documents intent.
func appendRemote(p *Parameters, rem Remote) error {
	for i := range p.Remotes {
		if p.Remotes[i].Name == rem.Name {
			return fmt.Errorf("%w %q", errL2tpRemoteDuplicate, rem.Name)
		}
	}
	p.Remotes = append(p.Remotes, rem)
	return nil
}

// validateRelays confirms every relay binding references a declared remote.
// No leafref exists in ze's YANG engine, so referential integrity is
// enforced here (config-load time) rather than by the schema.
func validateRelays(p *Parameters) error {
	for _, rl := range p.Relays {
		found := false
		for i := range p.Remotes {
			if p.Remotes[i].Name == rl.Remote {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: relay service %q -> remote %q", errL2tpRelayUnknownRemote, rl.Service, rl.Remote)
		}
	}
	return nil
}

// LookupRemote returns the configured remote with the given name and true,
// or a zero Remote and false when no such remote is configured.
func (p *Parameters) LookupRemote(name string) (Remote, bool) {
	for i := range p.Remotes {
		if p.Remotes[i].Name == name {
			return p.Remotes[i], true
		}
	}
	return Remote{}, false
}
