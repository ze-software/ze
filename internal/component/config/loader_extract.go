// Design: docs/architecture/config/syntax.md -- environment service config extraction
// Related: constants.go -- configTrue used for boolean checks

package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errEnvironmentMcpBindRemoteRequiresAuth   = errors.New("environment.mcp: bind-remote requires auth-mode != none")
	errEnvironmentMcpAuthModeBearerRequires   = errors.New("environment.mcp: auth-mode=bearer requires token")
	errEnvironmentMcpAuthModeBearerList       = errors.New("environment.mcp: auth-mode=bearer-list requires at least one identity")
	errEnvironmentMcpIdentityEntryMissingName = errors.New("environment.mcp: identity entry missing name")
	errEnvironmentMcpAuthModeOauthRequires    = errors.New("environment.mcp: auth-mode=oauth requires oauth.authorization-server")
	errEnvironmentMcpAuthModeOauthRequires2   = errors.New("environment.mcp: auth-mode=oauth requires oauth.audience")
	errEnvironmentMcpAuthModeOauthRequires3   = errors.New("environment.mcp: auth-mode=oauth requires tls.cert and tls.key on non-loopback listeners")
	errEnvironmentMcpTlsCertSetWithout        = errors.New("environment.mcp.tls: cert set without key")
	errEnvironmentMcpTlsKeySetWithout         = errors.New("environment.mcp.tls: key set without cert")

	// Names both sources of the token because this check reads a config FILE and
	// cannot see the daemon's environment: a deployment that supplies the token
	// through ze.gnmi.token boots fine and is still reported here.
	errEnvironmentGnmiNonLoopbackRequiresToken = errors.New("environment.gnmi: non-loopback listener requires token (set the token leaf, or ze.gnmi.token in the daemon environment)")
)

// loaderLogger is the config loader subsystem logger (lazy initialization).
var loaderLogger = slogutil.LazyLogger("config.loader")

const loopbackIP = "127.0.0.1"

// MCP auth-mode YANG enumeration values. These are the raw string forms
// parsed from YAML; typed enum lives in internal/component/mcp/auth.go.
const (
	mcpAuthNone       = "none"
	mcpAuthBearer     = "bearer"
	mcpAuthBearerList = "bearer-list"
	mcpAuthOAuth      = "oauth"
)

// ServerEndpoint is one parsed "ip:port" pair from a YANG `list server {}`
// under any environment.<service> block. Shared by web, mcp, and looking-glass
// extraction helpers. Transport-level flags (auth, tls, cors) live on the
// surrounding config struct, not on the endpoint, because they apply to every
// listener of the same service.
type ServerEndpoint struct {
	Host string // Listen host (e.g. 0.0.0.0)
	Port string // Listen port
}

// Listen returns host:port.
func (e ServerEndpoint) Listen() string {
	var tb textbuf.Buffer
	return tb.Str(e.Host).Byte(':').Str(e.Port).String()
}

// extractServerList reads the `server` list under a service container and
// returns every entry as a ServerEndpoint. When the list is empty, a single
// entry using the given YANG refine defaults is synthesized so callers always
// see at least one endpoint.
func extractServerList(svc *Tree, defaultHost, defaultPort string) []ServerEndpoint {
	entries := svc.GetListOrdered("server")
	if len(entries) == 0 {
		return []ServerEndpoint{{Host: defaultHost, Port: defaultPort}}
	}
	out := make([]ServerEndpoint, 0, len(entries))
	for _, entry := range entries {
		ep := ServerEndpoint{Host: defaultHost, Port: defaultPort}
		if v, ok := entry.Value.Get("ip"); ok && v != "" {
			ep.Host = v
		}
		if v, ok := entry.Value.Get("port"); ok && v != "" {
			ep.Port = v
		}
		out = append(out, ep)
	}
	return out
}

// WebListenConfig holds parsed environment.web settings.
// Servers is guaranteed non-empty when ExtractWebConfig returns ok=true.
type WebListenConfig struct {
	Servers  []ServerEndpoint // One entry per YANG `server <name> {}` block (defaults synthesized if empty)
	Insecure bool             // Disable authentication (forces every entry to 127.0.0.1)
	// Certificate names an entry in the PKI store to serve on the HTTPS
	// listener. Empty selects the self-signed certificate. A non-empty name
	// that does not resolve is a startup/reload error, never a fallback.
	Certificate string
}

// ExtractWebSettings returns the environment.web settings whenever the block
// exists, whether or not it asks for a listener.
//
// ok=true means "the operator wrote a web block".
func ExtractWebSettings(tree *Tree) (WebListenConfig, bool) {
	cfg, _, present := extractWebBlock(tree)
	if !present {
		return WebListenConfig{}, false
	}
	return cfg, true
}

// ExtractWebConfig returns the environment.web config if enabled.
// Every YANG list entry is returned in insertion order; callers that only
// want the first endpoint take `cfg.Servers[0]`.
//
// ok=true means "config asks for a web listener".
func ExtractWebConfig(tree *Tree) (WebListenConfig, bool) {
	cfg, enabled, present := extractWebBlock(tree)
	if !present || !enabled {
		return WebListenConfig{}, false
	}
	return cfg, true
}

// extractWebBlock parses environment.web in full and reports both whether the
// block exists (present) and whether it asks for a listener (enabled). It
// applies no gate of its own; each caller above decides what its own ok value
// means, and neither inherits the other's meaning.
//
// The split exists because cmd/ze/hub/main.go starts the web server from the
// --web flag, ze.web.listen, or ze.web.enabled as well as from this block. A
// single `enabled` gate therefore discarded `certificate` for every listener
// the config file did not start, silently serving a self-signed certificate to
// an operator who had named their own
// (plan/learned/1327-enabled-gate-discards-service-settings.md).
func extractWebBlock(tree *Tree) (WebListenConfig, bool, bool) {
	if tree == nil {
		return WebListenConfig{}, false, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return WebListenConfig{}, false, false
	}
	web := envBlock.GetContainer("web")
	if web == nil {
		return WebListenConfig{}, false, false
	}

	// Service must be explicitly enabled (default false) for config to START a
	// web listener. Reported, not enforced here: the settings below are parsed
	// either way, so an address supplied by a flag or env var still gets the
	// operator's certificate.
	enabledLeaf, _ := web.Get("enabled")
	enabled := enabledLeaf == configTrue

	cfg := WebListenConfig{Servers: extractServerList(web, "0.0.0.0", "3443")}

	if v, ok := web.Get("insecure"); ok && v == configTrue {
		cfg.Insecure = true
	}

	if v, ok := web.Get("certificate"); ok {
		cfg.Certificate = v
	}

	if v, ok := web.Get("ui-mode"); ok && v != "" && env.Get("ze.web.ui-mode") == "" {
		_ = env.Set("ze.web.ui-mode", v)
	}

	// Insecure forces every entry to 127.0.0.1 binding.
	if cfg.Insecure {
		for i := range cfg.Servers {
			if cfg.Servers[i].Host != loopbackIP {
				loaderLogger().Error("environment.web: insecure forces host to 127.0.0.1", "host", cfg.Servers[i].Host)
				cfg.Servers[i].Host = loopbackIP
			}
		}
	}

	return cfg, enabled, true
}

// HasWebConfig returns true if the parsed config tree has an enabled environment.web block.
func HasWebConfig(tree *Tree) bool {
	_, ok := ExtractWebConfig(tree)
	return ok
}

// MCPIdentity is one entry in the bearer-list identity table.
// Token is sensitive; callers MUST NOT log it.
type MCPIdentity struct {
	Name   string
	Token  string
	Scopes []string
}

// MCPOAuthConfig holds OAuth 2.1 resource-server settings.
type MCPOAuthConfig struct {
	AuthorizationServer string
	Audience            string
	RequiredScopes      []string
}

// MCPTLSConfig holds TLS cert/key paths. Empty cert means plaintext HTTP.
type MCPTLSConfig struct {
	Cert string // PEM certificate file path
	Key  string // PEM private-key file path
}

// MCPListenConfig holds parsed environment.mcp settings.
// Servers is guaranteed non-empty when ExtractMCPConfig returns ok=true.
//
// With BindRemote=false (default) the loopback clamp applies and every
// Server entry is forced to 127.0.0.1. With BindRemote=true the
// operator-configured ip is preserved.
type MCPListenConfig struct {
	Servers    []ServerEndpoint
	BindRemote bool
	AuthMode   string // Raw YANG value ("", "none", "bearer", "bearer-list", "oauth"); typed in mcp package
	Token      string // Single-token path; used when AuthMode=="bearer"
	Identities []MCPIdentity
	OAuth      MCPOAuthConfig
	TLS        MCPTLSConfig
}

// anyEndpointNonLoopback reports whether at least one endpoint binds to a
// non-loopback address. It is THE non-loopback rule for this package: every
// service's AnyListenerNonLoopback delegates here so exactly one definition of
// "reachable from off-box" exists, and the hub's boot guard
// (cmd/ze/hub/mgmt_guard.go listenAddrIsNonLoopback) mirrors it exactly.
//
// Fail-closed: a host that does not parse as an IP address (`localhost`, any
// unresolvable name, an empty host) counts as non-loopback, so an operator
// cannot smuggle remote reachability through a DNS name.
// netip.Addr.IsLoopback covers the full 127.0.0.0/8 range and ::1.
func anyEndpointNonLoopback(servers []ServerEndpoint) bool {
	for _, s := range servers {
		addr, err := netip.ParseAddr(s.Host)
		if err != nil || !addr.IsLoopback() {
			return true
		}
	}
	return false
}

// AnyListenerNonLoopback reports whether at least one Server entry binds to
// a non-loopback address. Used by Validate to decide whether TLS is required.
func (c MCPListenConfig) AnyListenerNonLoopback() bool {
	return anyEndpointNonLoopback(c.Servers)
}

// Validate returns a non-nil error when the config combination is internally
// inconsistent. Intended to be called by `ze config verify` so the operator
// sees precise messages BEFORE the daemon tries to start.
//
// Enforces the exact-or-reject rule (`.claude/rules/exact-or-reject.md`):
// silent fallback to a less-secure mode is never acceptable.
func (c MCPListenConfig) Validate() error {
	switch c.AuthMode {
	case "", mcpAuthNone, mcpAuthBearer, mcpAuthBearerList, mcpAuthOAuth:
	default:
		return fmt.Errorf("environment.mcp: auth-mode: unknown value %q", c.AuthMode)
	}

	if c.BindRemote && (c.AuthMode == "" || c.AuthMode == mcpAuthNone) {
		return errEnvironmentMcpBindRemoteRequiresAuth
	}

	switch c.AuthMode {
	case mcpAuthBearer:
		if c.Token == "" {
			return errEnvironmentMcpAuthModeBearerRequires
		}
	case mcpAuthBearerList:
		if len(c.Identities) == 0 {
			return errEnvironmentMcpAuthModeBearerList
		}
		seenNames := make(map[string]bool, len(c.Identities))
		seenTokens := make(map[string]bool, len(c.Identities))
		for _, id := range c.Identities {
			if id.Name == "" {
				return errEnvironmentMcpIdentityEntryMissingName
			}
			if id.Token == "" {
				return fmt.Errorf("environment.mcp.identity %q: token is required", id.Name)
			}
			if seenNames[id.Name] {
				return fmt.Errorf("environment.mcp.identity %q: duplicate name", id.Name)
			}
			// Duplicate tokens across identities silently collapse at match
			// time (first-match wins) -- the operator's intent for the
			// second identity's scopes becomes unreachable. Reject early so
			// the misconfiguration is visible.
			if seenTokens[id.Token] {
				return fmt.Errorf("environment.mcp.identity %q: token is shared with another identity", id.Name)
			}
			seenNames[id.Name] = true
			seenTokens[id.Token] = true
		}
	case mcpAuthOAuth:
		if c.OAuth.AuthorizationServer == "" {
			return errEnvironmentMcpAuthModeOauthRequires
		}
		if c.OAuth.Audience == "" {
			return errEnvironmentMcpAuthModeOauthRequires2
		}
		if c.AnyListenerNonLoopback() && c.TLS.Cert == "" {
			return errEnvironmentMcpAuthModeOauthRequires3
		}
		if c.TLS.Cert != "" && c.TLS.Key == "" {
			return errEnvironmentMcpTlsCertSetWithout
		}
		if c.TLS.Key != "" && c.TLS.Cert == "" {
			return errEnvironmentMcpTlsKeySetWithout
		}
	}

	return nil
}

// ExtractMCPSettings returns the environment.mcp settings whenever the block
// exists, WITHOUT regard to `enabled` or to whether a listen port was given.
//
// This exists because `enabled` answers one question: "does config ask for an
// MCP listener?". Readers took `enabled` to answer a second, unrelated
// question: "do the MCP auth settings apply?". The two questions differ,
// because `ze --mcp <port>` and `ze.mcp.listen` also start the listener. One
// answer for both questions meant that an operator who wrote
//
//	environment { mcp { auth-mode bearer; token secret; } }
//
// and started `ze --mcp 9718` got an UNAUTHENTICATED listener. ExtractMCPConfig
// returned ok=false, so every caller skipped the config. AuthMode then stayed
// zero, and the mode inference in NewStreamable selected AuthNone. The daemon
// discarded the operator's explicit instruction, which
// ai/rules/protocol.md forbids.
//
// Callers that need "did config ask for a listener" must use ExtractMCPConfig.
// Callers that need "how does this listener authenticate" must use
// ExtractMCPSettings, so the answer cannot depend on which mechanism supplied
// the address.
func ExtractMCPSettings(tree *Tree) (MCPListenConfig, bool) {
	cfg, _, present := extractMCPBlock(tree)
	if !present {
		return MCPListenConfig{}, false
	}
	return cfg, true
}

// ExtractMCPConfig returns the environment.mcp config if enabled.
//
// With BindRemote=false the loopback clamp forces every server entry to
// 127.0.0.1. With BindRemote=true the operator-configured ip is preserved.
// Runtime-fatal inconsistencies (auth-mode oauth without authorization-server,
// bind-remote without auth, oauth without TLS) are reported by Validate, which
// the verifier calls. Extraction itself never rewrites to "best guess".
//
// ok=true means "config asks for an MCP listener and named a port for it". It
// deliberately does NOT mean "these are the auth settings" -- see
// ExtractMCPSettings for that half.
func ExtractMCPConfig(tree *Tree) (MCPListenConfig, bool) {
	cfg, enabled, present := extractMCPBlock(tree)
	if !present || !enabled {
		return MCPListenConfig{}, false
	}
	// A port leaf is mandatory on EVERY server. extractServerList seeds each
	// entry with an empty default port (mcp has no fallback), so an entry that
	// names none leaves Port "" and ServerEndpoint.Listen joins it to "<ip>:",
	// which the kernel binds on a port it chooses. Testing only Servers[0] let a
	// list whose FIRST entry named a port carry a second that did not, and the
	// daemon bound that one where no operator asked and no doctor check probes.
	// MCPServersMissingPort reports the same shape, because an unusable block
	// that says nothing is indistinguishable from one that works.
	if len(cfg.Servers) == 0 {
		return MCPListenConfig{}, false
	}
	for _, s := range cfg.Servers {
		if s.Port == "" {
			return MCPListenConfig{}, false
		}
	}
	return cfg, true
}

// MCPMissingPortAdvice is the tail of the missing-port message, shared by the two
// operator surfaces that report it (ValidateSemantics for `ze doctor`, and
// cmd_validate.go for `ze config validate`) so they cannot drift.
//
// It names the schema divergence rather than denying it. ze-mcp-conf.yang really
// does declare `refine port { default 8080; }`, so "mcp has no default port" is
// false about the document the operator reads, and the operator who FOLLOWED that
// refine is precisely the one being rejected. The refine is inert twice over: the
// Ze YANG compiler drops refine defaults, and extractMCPBlock passes an empty
// default port regardless. Closing that divergence would start an MCP listener
// where none starts today, which is an owner decision, so it is recorded in
// plan/deferrals/mcp-port-default-divergence.md and the message tells the truth
// in the meantime.
const MCPMissingPortAdvice = " names no port, so MCP starts no listener at all. " +
	"The YANG default of 8080 is NOT applied to environment.mcp, unlike environment.web and environment.gnmi, " +
	"so every mcp server entry must name its port explicitly"

// MCPServersMissingPort returns the names of the environment.mcp server entries
// the operator WROTE that name no port, in config order.
//
// ExtractMCPConfig returns ok=false for THREE different configs -- no mcp block,
// a block that is not enabled, and a block whose servers do not all name a port
// -- and only the third is a mistake. This answers the third alone, so a caller
// on the else branch of that ok cannot attribute a switched-off service to a
// missing port. A disabled block is a deliberate operator choice and must
// validate exactly as it did before this check existed.
//
// It reads the tree rather than MCPListenConfig.Servers so a synthesized entry
// cannot be mistaken for an authored one: an empty server list is the "mcp is on
// but unconfigured" shape, which is also not this mistake.
//
// It exists so ValidateSemantics and `ze config validate` can speak about the
// config ExtractMCPConfig silently refuses. Nothing else can: every
// MCPListenConfig.Validate call sits behind that same ok gate.
func MCPServersMissingPort(tree *Tree) []string {
	_, enabled, present := extractMCPBlock(tree)
	if !present || !enabled {
		return nil
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	mcp := envBlock.GetContainer("mcp")
	if mcp == nil {
		return nil
	}
	var missing []string
	for _, entry := range mcp.GetListOrdered("server") {
		if port, _ := entry.Value.Get("port"); port == "" {
			missing = append(missing, entry.Key)
		}
	}
	return missing
}

// extractMCPBlock parses environment.mcp in full and reports both whether the
// block exists at all (present) and whether it asks for a listener (enabled).
// It applies no gate of its own. Each of the two callers above decides what
// its own ok value means. And neither caller can inherit the other's meaning.
func extractMCPBlock(tree *Tree) (MCPListenConfig, bool, bool) {
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return MCPListenConfig{}, false, false
	}
	mcp := envBlock.GetContainer("mcp")
	if mcp == nil {
		return MCPListenConfig{}, false, false
	}

	// Service must be explicitly enabled (default false) for config to START a
	// listener. Reported, not enforced here: the settings below are parsed
	// either way so an address supplied by CLI or env still authenticates.
	enabledLeaf, _ := mcp.Get("enabled")
	enabled := enabledLeaf == configTrue

	cfg := MCPListenConfig{Servers: extractServerList(mcp, loopbackIP, "")}

	if bindRemote, ok := mcp.Get("bind-remote"); ok && bindRemote == configTrue {
		cfg.BindRemote = true
	}

	if mode, ok := mcp.Get("auth-mode"); ok {
		cfg.AuthMode = mode
	}

	if token, ok := mcp.Get("token"); ok {
		cfg.Token = token
	}

	// Token set without explicit auth-mode infers bearer.
	if cfg.AuthMode == "" && cfg.Token != "" {
		cfg.AuthMode = mcpAuthBearer
	}

	cfg.Identities = extractMCPIdentities(mcp)

	if oauth := mcp.GetContainer("oauth"); oauth != nil {
		if v, ok := oauth.Get("authorization-server"); ok {
			cfg.OAuth.AuthorizationServer = v
		}
		if v, ok := oauth.Get("audience"); ok {
			cfg.OAuth.Audience = v
		}
		cfg.OAuth.RequiredScopes = extractLeafList(oauth, "required-scopes")
	}

	if tls := mcp.GetContainer("tls"); tls != nil {
		if v, ok := tls.Get("cert"); ok {
			cfg.TLS.Cert = v
		}
		if v, ok := tls.Get("key"); ok {
			cfg.TLS.Key = v
		}
	}

	// Loopback clamp applies unless bind-remote is true.
	if !cfg.BindRemote {
		for i := range cfg.Servers {
			if cfg.Servers[i].Host != loopbackIP {
				loaderLogger().Error("environment.mcp: host forced to 127.0.0.1 (bind-remote false)", "host", cfg.Servers[i].Host)
				cfg.Servers[i].Host = loopbackIP
			}
		}
	}

	// The port gate lives in ExtractMCPConfig, not here. A block with auth
	// settings and no port is unusable as a LISTENER source. But that block is
	// still the operator's authentication instruction for a listener that
	// another mechanism starts.
	return cfg, enabled, true
}

// extractMCPIdentities reads the environment.mcp.identity list.
// The list key is the identity name, so entry.Key is authoritative for Name
// (the `name` leaf inside the value is redundant and may not be populated
// by the parser on all code paths).
func extractMCPIdentities(mcp *Tree) []MCPIdentity {
	entries := mcp.GetListOrdered("identity")
	if len(entries) == 0 {
		return nil
	}
	out := make([]MCPIdentity, 0, len(entries))
	for _, entry := range entries {
		id := MCPIdentity{Name: entry.Key}
		if v, ok := entry.Value.Get("token"); ok {
			id.Token = v
		}
		id.Scopes = extractLeafList(entry.Value, "scope")
		out = append(out, id)
	}
	return out
}

// extractLeafList reads a YANG leaf-list into a []string preserving insertion
// order. Returns nil when the leaf-list is absent or empty so callers can
// compare against nil for the default case.
func extractLeafList(t *Tree, name string) []string {
	values := t.GetSlice(name)
	if len(values) == 0 {
		return nil
	}
	return values
}

// LGListenConfig holds parsed environment.looking-glass settings.
// Servers is guaranteed non-empty when ExtractLGConfig returns ok=true.
type LGListenConfig struct {
	Servers []ServerEndpoint
	TLS     bool // Enable TLS on every listener (default ON)
	// TLSExplicit records that the operator wrote the tls leaf. It separates
	// "the operator demands TLS" from "TLS is the default nobody overrode",
	// which decides whether a missing certificate store is a hard failure or a
	// warned fallback to plaintext.
	TLSExplicit bool
	Token       string // Optional bearer token gating every route; empty leaves the LG open
}

// ExtractLGSettings returns the transport and authentication settings of an
// environment.looking-glass block, whatever supplied the listen address.
//
// ok=true means "the block exists". It deliberately does NOT mean "config
// starts a looking glass" -- see ExtractLGConfig for that half. The split
// matters because ze.looking-glass.enabled and ze.looking-glass.listen start
// the server without any `enabled true` leaf. Gating the settings on that leaf
// discarded the operator's `tls true` and `token`, so a block that asked for
// TLS and a bearer token produced a plaintext, open looking glass
// (ai/rules/protocol.md; same defect as environment.mcp).
func ExtractLGSettings(tree *Tree) (LGListenConfig, bool) {
	cfg, _, present := extractLGBlock(tree)
	if !present {
		return LGListenConfig{}, false
	}
	return cfg, true
}

// ExtractLGConfig returns the environment.looking-glass config if enabled.
//
// ok=true means "config asks for a looking-glass listener".
func ExtractLGConfig(tree *Tree) (LGListenConfig, bool) {
	cfg, enabled, present := extractLGBlock(tree)
	if !present || !enabled {
		return LGListenConfig{}, false
	}
	return cfg, true
}

// extractLGBlock parses environment.looking-glass in full and reports both
// whether the block exists (present) and whether it asks for a listener
// (enabled). It applies no gate of its own; each caller above decides what its
// own ok value means, and neither inherits the other's meaning.
func extractLGBlock(tree *Tree) (LGListenConfig, bool, bool) {
	if tree == nil {
		return LGListenConfig{}, false, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return LGListenConfig{}, false, false
	}
	lg := envBlock.GetContainer("looking-glass")
	if lg == nil {
		return LGListenConfig{}, false, false
	}

	// Service must be explicitly enabled (default false) for config to START a
	// looking glass. Reported, not enforced here: the settings below are parsed
	// either way, so an address supplied by an env var still gets the operator's
	// TLS and token.
	enabledLeaf, _ := lg.Get("enabled")
	enabled := enabledLeaf == configTrue

	cfg := LGListenConfig{Servers: extractServerList(lg, "0.0.0.0", "8443")}

	// TLS defaults ON. The raw tree carries no YANG defaults, so an absent leaf
	// must be read as true here; only an explicit `tls false` opts out. The
	// looking glass binds 0.0.0.0 by default, so a plaintext default published
	// route data and session state in the clear.
	cfg.TLS = true
	if v, ok := lg.Get("tls"); ok {
		cfg.TLS = v == configTrue
		cfg.TLSExplicit = true
	}

	if v, ok := lg.Get("token"); ok {
		cfg.Token = v
	}

	return cfg, enabled, true
}

// GNMIListenConfig holds parsed environment.gnmi settings.
type GNMIListenConfig struct {
	Servers []ServerEndpoint
	Token   string
	TLS     struct {
		Cert string
		Key  string
	}
}

// AnyListenerNonLoopback reports whether at least one gNMI Server entry binds
// to a non-loopback address, including the 0.0.0.0:9339 default that
// ExtractGNMIConfig synthesizes when the block names no server.
func (c GNMIListenConfig) AnyListenerNonLoopback() bool {
	return anyEndpointNonLoopback(c.Servers)
}

// Validate returns a non-nil error when the gNMI config would expose an
// unauthenticated management surface: a non-loopback listener (including the
// 0.0.0.0:9339 default synthesized by ExtractGNMIConfig) with no token. gNMI's
// interceptors are installed only when a token is set and checkAuth allows on
// an empty token, so a tokenless non-loopback bind is an unauthenticated
// read+write config surface. Called by ValidateSemantics (which `ze doctor`
// reaches through checkSemanticValidation) and by `ze config validate`, so the
// exposure is reported offline as well as refused at boot; the hub
// management-listener guard refuses the resolved (env-or-YANG) gNMI bind.
// Fail-closed per .claude/rules/exact-or-reject.md.
func (c GNMIListenConfig) Validate() error {
	if c.AnyListenerNonLoopback() && c.Token == "" {
		return errEnvironmentGnmiNonLoopbackRequiresToken
	}
	return nil
}

// ExtractGNMIConfig returns the environment.gnmi config if enabled.
func ExtractGNMIConfig(tree *Tree) (GNMIListenConfig, bool) {
	if tree == nil {
		return GNMIListenConfig{}, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return GNMIListenConfig{}, false
	}
	gnmi := envBlock.GetContainer("gnmi")
	if gnmi == nil {
		return GNMIListenConfig{}, false
	}

	enabled, _ := gnmi.Get("enabled")
	if enabled != configTrue {
		return GNMIListenConfig{}, false
	}

	cfg := GNMIListenConfig{Servers: extractServerList(gnmi, "0.0.0.0", "9339")}

	if v, ok := gnmi.Get("token"); ok {
		cfg.Token = v
	}

	if tls := gnmi.GetContainer("tls"); tls != nil {
		if v, ok := tls.Get("cert"); ok {
			cfg.TLS.Cert = v
		}
		if v, ok := tls.Get("key"); ok {
			cfg.TLS.Key = v
		}
	}

	return cfg, true
}

// minTokenLength is the minimum length for hub auth tokens.
const minTokenLength = 32

// ExtractHubConfig extracts plugin hub transport config from a parsed config tree.
// Returns zero-value HubConfig with no servers/clients if no hub block is present.
func ExtractHubConfig(tree *Tree) (plugin.HubConfig, error) {
	pluginContainer := tree.GetContainer("plugin")
	if pluginContainer == nil {
		return plugin.HubConfig{}, nil
	}
	hubContainer := pluginContainer.GetContainer("hub")
	if hubContainer == nil {
		return plugin.HubConfig{}, nil
	}

	var hub plugin.HubConfig

	for _, entry := range hubContainer.GetListOrdered("server") {
		srv, err := extractHubServerConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub server %q: %w", entry.Key, err)
		}
		hub.Servers = append(hub.Servers, srv)
	}

	for _, entry := range hubContainer.GetListOrdered("client") {
		cli, err := extractHubClientConfig(entry.Key, entry.Value)
		if err != nil {
			return plugin.HubConfig{}, fmt.Errorf("hub client %q: %w", entry.Key, err)
		}
		hub.Clients = append(hub.Clients, cli)
	}

	return hub, nil
}

func extractHubServerConfig(name string, tree *Tree) (plugin.HubServerConfig, error) {
	srv := plugin.HubServerConfig{Name: name}

	if ip, ok := tree.Get("ip"); ok {
		srv.Host = ip
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return srv, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		srv.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return srv, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		srv.Secret = secret
	}

	clients := tree.GetList("client")
	if len(clients) > 0 {
		srv.Clients = make(map[string]string, len(clients))
		for clientName, clientTree := range clients {
			clientSecret, ok := clientTree.Get("secret")
			if !ok || clientSecret == "" {
				return srv, fmt.Errorf("client %q: secret required", clientName)
			}
			if len(clientSecret) < minTokenLength {
				return srv, fmt.Errorf("client %q: secret too short: minimum %d characters, got %d", clientName, minTokenLength, len(clientSecret))
			}
			srv.Clients[clientName] = clientSecret
		}
	}

	return srv, nil
}

// APIListenConfig holds one parsed api-server listen endpoint.
// Transport-level fields (cors-origin, tls-cert, tls-key) live on APIConfig
// because they apply to every listener of the same transport.
type APIListenConfig struct {
	Host string // Listen host (e.g. 0.0.0.0)
	Port string // Listen port
}

// Listen returns host:port.
func (c APIListenConfig) Listen() string {
	var tb textbuf.Buffer
	return tb.Str(c.Host).Byte(':').Str(c.Port).String()
}

// APIConfig holds parsed environment.api settings.
// REST and GRPC each carry a slice of listen endpoints (one entry per
// YANG `list server {}` block). When the transport is enabled but no
// server entries are present, extraction synthesizes a single default
// entry from the YANG refine defaults so downstream binders always see
// at least one endpoint.
type APIConfig struct {
	Token string // Shared bearer token for both transports

	RESTOn         bool
	REST           []APIListenConfig
	RESTCORSOrigin string // REST-only: allowed CORS origin

	GRPCOn      bool
	GRPC        []APIListenConfig
	GRPCTLSCert string // gRPC-only: TLS certificate path
	GRPCTLSKey  string // gRPC-only: TLS key path
}

// ExtractAPIConfig returns the environment.api config if either REST or gRPC is enabled.
// Each transport returns every YANG list entry; if the list is empty the
// YANG refine defaults are used to synthesize one entry so the transport
// always has at least one listener to bind.
func ExtractAPIConfig(tree *Tree) (APIConfig, bool) {
	if tree == nil {
		return APIConfig{}, false
	}
	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return APIConfig{}, false
	}
	apiBlock := envBlock.GetContainer("api-server")
	if apiBlock == nil {
		return APIConfig{}, false
	}

	var cfg APIConfig

	if token, ok := apiBlock.Get("token"); ok {
		cfg.Token = token
	}

	// REST transport.
	if rest := apiBlock.GetContainer("rest"); rest != nil {
		enabled, _ := rest.Get("enabled")
		if enabled == configTrue {
			cfg.RESTOn = true
			cfg.REST = extractAPIServerList(rest, "0.0.0.0", "8081")
			if v, ok := rest.Get("cors-origin"); ok {
				cfg.RESTCORSOrigin = v
			}
		}
	}

	// gRPC transport.
	if grpcBlock := apiBlock.GetContainer("grpc"); grpcBlock != nil {
		enabled, _ := grpcBlock.Get("enabled")
		if enabled == configTrue {
			cfg.GRPCOn = true
			cfg.GRPC = extractAPIServerList(grpcBlock, "0.0.0.0", "50051")
			if v, ok := grpcBlock.Get("tls-cert"); ok {
				cfg.GRPCTLSCert = v
			}
			if v, ok := grpcBlock.Get("tls-key"); ok {
				cfg.GRPCTLSKey = v
			}
		}
	}

	if !cfg.RESTOn && !cfg.GRPCOn {
		return APIConfig{}, false
	}

	return cfg, true
}

// extractAPIServerList reads the `server` list under a transport container
// (rest or grpc) and returns every entry as an APIListenConfig. When the
// list is empty, a single entry using the given YANG refine defaults is
// synthesized so callers always see at least one endpoint.
func extractAPIServerList(transport *Tree, defaultHost, defaultPort string) []APIListenConfig {
	entries := transport.GetListOrdered("server")
	if len(entries) == 0 {
		return []APIListenConfig{{Host: defaultHost, Port: defaultPort}}
	}
	out := make([]APIListenConfig, 0, len(entries))
	for _, entry := range entries {
		ep := APIListenConfig{Host: defaultHost, Port: defaultPort}
		if v, ok := entry.Value.Get("ip"); ok && v != "" {
			ep.Host = v
		}
		if v, ok := entry.Value.Get("port"); ok && v != "" {
			ep.Port = v
		}
		out = append(out, ep)
	}
	return out
}

func extractHubClientConfig(name string, tree *Tree) (plugin.HubClientConfig, error) {
	cli := plugin.HubClientConfig{Name: name}

	if host, ok := tree.Get("host"); ok {
		cli.Host = host
	}

	if portStr, ok := tree.Get("port"); ok {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return cli, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		cli.Port = uint16(port)
	}

	if secret, ok := tree.Get("secret"); ok && secret != "" {
		if len(secret) < minTokenLength {
			return cli, fmt.Errorf("secret too short: minimum %d characters, got %d", minTokenLength, len(secret))
		}
		cli.Secret = secret
	}

	if sa, ok := tree.Get("source-address"); ok {
		cli.SourceAddress = sa
	}

	return cli, nil
}
