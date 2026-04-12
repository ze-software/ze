// Design: rfc/short/rfc5880.md -- BFD configuration model
// Related: bfd.go -- plugin lifecycle that consumes the parsed config
//
// BFD plugin config parser. Consumes the JSON-encoded `bfd` section
// delivered to the SDK Configure callback and returns a validated
// in-memory representation that the lifecycle code drives the engine
// from.
//
// The plugin config tree stringifies every leaf value before sending
// it across the SDK boundary, so this parser walks a
// `map[string]any` tree (mirroring `internal/component/iface/config.go`)
// rather than json-tagging into typed structs. Structural errors
// (missing peer, malformed address, unknown profile reference) abort
// verify with a descriptive message so the operator sees the failure
// at config-validate time.
package bfd

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/session"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// pluginConfig is the validated, in-memory shape of a `bfd { ... }` block.
type pluginConfig struct {
	enabled    bool
	persistDir string
	bindV6     bool
	profiles   map[string]profileConfig
	sessions   []sessionConfig
}

// profileConfig holds the timer parameters reusable across sessions.
type profileConfig struct {
	name            string
	detectMult      uint8
	desiredMinTxUs  uint32
	requiredMinRxUs uint32
	passive         bool
	auth            *authConfig
	echo            *echoConfig
}

// echoConfig captures the resolved RFC 5880 §6.4 Echo mode settings.
// An empty pointer means echo is disabled for sessions using this
// profile; a non-nil pointer means the engine schedules echo TX on
// UDP port 3785 at max(DesiredMinEchoTxUs, peer.RequiredMinEchoRx).
type echoConfig struct {
	desiredMinEchoTxUs uint32
}

// authConfig captures the resolved authentication parameters from a
// profile's `auth { ... }` block. The Secret is held as raw bytes so
// logs and the config show RPC can redact it without parsing the
// profile again.
type authConfig struct {
	authType   uint8
	keyID      uint8
	secret     []byte //nolint:gosec // BFD auth key; pluginConfig is never serialized
	meticulous bool
}

// sessionConfig is one pinned session entry from `single-hop-session` or
// `multi-hop-session`. The mode disambiguates the two list types so the
// lifecycle code can route them to the correct engine.Loop.
type sessionConfig struct {
	mode     api.HopMode
	peer     netip.Addr
	local    netip.Addr // unspecified == not configured
	iface    string     // single-hop only
	vrf      string
	profile  string
	shutdown bool
	minTTL   uint8 // multi-hop only; zero means default 254
}

// parseSections walks the SDK ConfigSection slice and parses the first
// section whose Root matches "bfd". Returns an empty disabled config if
// the section list does not contain a bfd block, so callers never have
// to nil-check the result.
func parseSections(sections []sdk.ConfigSection) (*pluginConfig, error) {
	for _, section := range sections {
		if section.Root != "bfd" {
			continue
		}
		return parseBFDSection(section.Data)
	}
	return &pluginConfig{profiles: map[string]profileConfig{}}, nil
}

// parseBFDSection unmarshals the JSON-encoded bfd container and validates
// it. The JSON shape is `{"bfd": { "enabled": "true", "profile": {
// "<name>": {...} }, "single-hop-session": { "<key>": {...} } }}` --
// every leaf value is a string regardless of the YANG type because the
// config tree serializer is type-erased over the SDK boundary.
func parseBFDSection(data string) (*pluginConfig, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("bfd: parse config json: %w", err)
	}

	cfg := &pluginConfig{
		enabled:  true,
		profiles: make(map[string]profileConfig),
	}

	bfdMap, ok := root["bfd"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := bfdMap["enabled"].(string); ok {
		cfg.enabled = parseBool(v, true)
	}
	if v, ok := bfdMap["persist-dir"].(string); ok {
		cfg.persistDir = v
	}
	if v, ok := bfdMap["bind-v6"].(string); ok {
		cfg.bindV6 = parseBool(v, false)
	}

	if profMap, ok := bfdMap["profile"].(map[string]any); ok {
		for name, raw := range profMap {
			fields, _ := raw.(map[string]any)
			p, err := parseProfile(name, fields)
			if err != nil {
				return nil, err
			}
			cfg.profiles[name] = p
		}
	}

	if singleMap, ok := bfdMap["single-hop-session"].(map[string]any); ok {
		for key, raw := range singleMap {
			fields, _ := raw.(map[string]any)
			s, err := parseSingleHopSession(key, fields, cfg.profiles)
			if err != nil {
				return nil, err
			}
			cfg.sessions = append(cfg.sessions, s)
		}
	}

	if multiMap, ok := bfdMap["multi-hop-session"].(map[string]any); ok {
		for key, raw := range multiMap {
			fields, _ := raw.(map[string]any)
			s, err := parseMultiHopSession(key, fields, cfg.profiles)
			if err != nil {
				return nil, err
			}
			cfg.sessions = append(cfg.sessions, s)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseProfile decodes a single profile entry. Defaults mirror the YANG
// `default` clauses so callers can rely on every field being populated
// even when the operator omitted optional leaves.
func parseProfile(name string, fields map[string]any) (profileConfig, error) {
	p := profileConfig{
		name:            name,
		detectMult:      session.DefaultDetectMult,
		desiredMinTxUs:  300_000,
		requiredMinRxUs: 300_000,
	}
	if v, ok := fields["detect-multiplier"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil || n == 0 {
			return profileConfig{}, fmt.Errorf("bfd: profile %q: detect-multiplier must be 1..255 (got %q)", name, v)
		}
		p.detectMult = uint8(n)
	}
	if v, ok := fields["desired-min-tx-us"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return profileConfig{}, fmt.Errorf("bfd: profile %q: desired-min-tx-us %q: %w", name, v, err)
		}
		p.desiredMinTxUs = uint32(n)
	}
	if v, ok := fields["required-min-rx-us"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return profileConfig{}, fmt.Errorf("bfd: profile %q: required-min-rx-us %q: %w", name, v, err)
		}
		p.requiredMinRxUs = uint32(n)
	}
	if v, ok := fields["passive"].(string); ok {
		p.passive = parseBool(v, false)
	}
	if authRaw, ok := fields["auth"].(map[string]any); ok {
		ac, err := parseAuthConfig(name, authRaw)
		if err != nil {
			return profileConfig{}, err
		}
		p.auth = ac
	}
	if echoRaw, ok := fields["echo"].(map[string]any); ok {
		ec, err := parseEchoConfig(name, echoRaw)
		if err != nil {
			return profileConfig{}, err
		}
		p.echo = ec
	}
	return p, nil
}

// parseEchoConfig decodes the `echo { ... }` block inside a profile.
// The block's presence alone enables echo for sessions inheriting
// this profile; the single leaf is the target TX rate. The parser
// defends against zero by rejecting it because `max(0, peer.min-rx)`
// degenerates to "send as fast as possible" if the peer leaves its
// echo-rx at zero.
func parseEchoConfig(profileName string, fields map[string]any) (*echoConfig, error) {
	ec := &echoConfig{desiredMinEchoTxUs: 50_000}
	if v, ok := fields["desired-min-echo-tx-us"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("bfd: profile %q: echo desired-min-echo-tx-us %q: %w", profileName, v, err)
		}
		if n == 0 {
			return nil, fmt.Errorf("bfd: profile %q: echo desired-min-echo-tx-us must be > 0", profileName)
		}
		ec.desiredMinEchoTxUs = uint32(n)
	}
	return ec, nil
}

// authTypeFromEnum resolves the YANG `auth type` enum string to the
// RFC 5880 wire type and the meticulous flag. Simple Password is
// explicitly rejected because RFC 5880 §6.7.2 warns it provides no
// cryptographic protection.
func authTypeFromEnum(s string) (wire uint8, meticulous, ok bool) {
	if s == "keyed-md5" {
		return packet.AuthTypeKeyedMD5, false, true
	}
	if s == "meticulous-keyed-md5" {
		return packet.AuthTypeMeticulousKeyedMD5, true, true
	}
	if s == "keyed-sha1" {
		return packet.AuthTypeKeyedSHA1, false, true
	}
	if s == "meticulous-keyed-sha1" {
		return packet.AuthTypeMeticulousKeyedSHA1, true, true
	}
	return 0, false, false
}

// parseAuthConfig decodes the `auth { ... }` block inside a profile.
// Simple Password is rejected here with a descriptive error.
func parseAuthConfig(profileName string, fields map[string]any) (*authConfig, error) {
	typeStr := stringField(fields, "type")
	if typeStr == "" {
		return nil, fmt.Errorf("bfd: profile %q: auth block missing type", profileName)
	}
	if typeStr == "simple-password" {
		return nil, fmt.Errorf("bfd: profile %q: auth type simple-password rejected (RFC 5880 Section 6.7.2 warns against use)", profileName)
	}
	wire, meticulous, ok := authTypeFromEnum(typeStr)
	if !ok {
		return nil, fmt.Errorf("bfd: profile %q: unknown auth type %q", profileName, typeStr)
	}
	keyIDStr := stringField(fields, "key-id")
	if keyIDStr == "" {
		return nil, fmt.Errorf("bfd: profile %q: auth block missing key-id", profileName)
	}
	keyID, err := strconv.ParseUint(keyIDStr, 10, 8)
	if err != nil {
		return nil, fmt.Errorf("bfd: profile %q: auth key-id %q: %w", profileName, keyIDStr, err)
	}
	secret := stringField(fields, "secret")
	if secret == "" {
		return nil, fmt.Errorf("bfd: profile %q: auth block missing secret", profileName)
	}
	return &authConfig{
		authType:   wire,
		keyID:      uint8(keyID),
		secret:     []byte(secret),
		meticulous: meticulous,
	}, nil
}

// parseSingleHopSession decodes one entry under `single-hop-session`. The
// YANG list key is "peer vrf interface", but ze's config file parser only
// carries the first positional value as the list key, so operators write
// `single-hop-session 203.0.113.9 { interface eth0; vrf default }` and
// the peer address comes from listKey. Fall back to fields["peer"] when
// the config is delivered via an API writer that populated the leaves
// directly.
func parseSingleHopSession(listKey string, fields map[string]any, profiles map[string]profileConfig) (sessionConfig, error) {
	peerStr, _ := fields["peer"].(string)
	if peerStr == "" {
		peerStr = listKey
	}
	if peerStr == "" {
		return sessionConfig{}, fmt.Errorf("bfd: single-hop-session: missing peer")
	}
	peer, err := netip.ParseAddr(peerStr)
	if err != nil {
		return sessionConfig{}, fmt.Errorf("bfd: single-hop-session: invalid peer %q: %w", peerStr, err)
	}
	s := sessionConfig{
		mode:    api.SingleHop,
		peer:    peer,
		vrf:     defaultVRF(stringField(fields, "vrf")),
		profile: stringField(fields, "profile"),
	}
	if v := stringField(fields, "interface"); v != "" {
		s.iface = v
	}
	if v := stringField(fields, "local"); v != "" {
		local, lerr := netip.ParseAddr(v)
		if lerr != nil {
			return sessionConfig{}, fmt.Errorf("bfd: single-hop-session %s: invalid local %q: %w", peer, v, lerr)
		}
		s.local = local
	}
	if v, ok := fields["shutdown"].(string); ok {
		s.shutdown = parseBool(v, false)
	}
	if s.profile != "" {
		if _, ok := profiles[s.profile]; !ok {
			return sessionConfig{}, fmt.Errorf("bfd: single-hop-session %s: unknown profile %q", peer, s.profile)
		}
	}
	return s, nil
}

// parseMultiHopSession decodes one entry under `multi-hop-session`. Same
// listKey/peer fallback as parseSingleHopSession; the YANG key is
// "peer local vrf" but the config file writer carries only the first
// positional value.
func parseMultiHopSession(listKey string, fields map[string]any, profiles map[string]profileConfig) (sessionConfig, error) {
	peerStr, _ := fields["peer"].(string)
	if peerStr == "" {
		peerStr = listKey
	}
	if peerStr == "" {
		return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session: missing peer")
	}
	peer, err := netip.ParseAddr(peerStr)
	if err != nil {
		return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session: invalid peer %q: %w", peerStr, err)
	}
	localStr := stringField(fields, "local")
	if localStr == "" {
		return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session %s: local source address is required (RFC 5883 §5)", peer)
	}
	local, err := netip.ParseAddr(localStr)
	if err != nil {
		return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session %s: invalid local %q: %w", peer, localStr, err)
	}
	s := sessionConfig{
		mode:    api.MultiHop,
		peer:    peer,
		local:   local,
		vrf:     defaultVRF(stringField(fields, "vrf")),
		profile: stringField(fields, "profile"),
	}
	if v, ok := fields["min-ttl"].(string); ok && v != "" {
		n, perr := strconv.ParseUint(v, 10, 8)
		if perr != nil {
			return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session %s: invalid min-ttl %q: %w", peer, v, perr)
		}
		s.minTTL = uint8(n)
	}
	if v, ok := fields["shutdown"].(string); ok {
		s.shutdown = parseBool(v, false)
	}
	if s.profile != "" {
		if _, ok := profiles[s.profile]; !ok {
			return sessionConfig{}, fmt.Errorf("bfd: multi-hop-session %s: unknown profile %q", peer, s.profile)
		}
	}
	return s, nil
}

// stringField returns a leaf value as a string, or "" if the key is
// missing or not a string. Used by the parsers to read optional fields
// without per-call type assertions.
func stringField(fields map[string]any, key string) string {
	if v, ok := fields[key].(string); ok {
		return v
	}
	return ""
}

// parseBool returns true for "true"/"1"/"yes"/"on" (case-sensitive,
// matching the YANG canonical encoding), false for "false"/"0"/"no"/"off",
// and the supplied default for any other input.
func parseBool(v string, def bool) bool {
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}

// defaultVRFName is the canonical string used throughout the plugin for
// "no VRF configured / global routing table". Kept as a constant so the
// loop dispatcher and the device-resolution helper share one source of
// truth instead of scattering the literal across comparisons.
const defaultVRFName = "default"

// defaultVRF normalises an unset VRF to defaultVRFName so the rest of
// the plugin compares VRF names without special-casing the empty
// string.
func defaultVRF(v string) string {
	if v == "" {
		return defaultVRFName
	}
	return v
}

// toSessionRequest builds an api.SessionRequest from a parsed entry,
// inheriting timer parameters from the named profile when one is set.
// Profiles are looked up by name; an unknown name was already rejected
// in parse* but the lookup defends against caller mistakes.
func (s sessionConfig) toSessionRequest(profiles map[string]profileConfig) api.SessionRequest {
	req := api.SessionRequest{
		Peer:      s.peer,
		Local:     s.local,
		Interface: s.iface,
		VRF:       s.vrf,
		Mode:      s.mode,
		MinTTL:    s.minTTL,
		Profile:   s.profile,
	}
	if s.profile != "" {
		if p, ok := profiles[s.profile]; ok {
			req.DesiredMinTxInterval = p.desiredMinTxUs
			req.RequiredMinRxInterval = p.requiredMinRxUs
			req.DetectMult = p.detectMult
			req.Passive = p.passive
			if p.auth != nil {
				req.Auth = &api.AuthSettings{
					Type:       p.auth.authType,
					KeyID:      p.auth.keyID,
					Secret:     p.auth.secret,
					Meticulous: p.auth.meticulous,
				}
			}
			if p.echo != nil {
				req.DesiredMinEchoTxInterval = p.echo.desiredMinEchoTxUs
			}
		}
	}
	return req
}

// validate checks a parsed pluginConfig for post-parse constraints
// that cross profile/session boundaries. RFC 5883 Section 4 forbids
// multi-hop echo, so a multi-hop session referencing an
// echo-enabled profile is rejected here rather than silently.
func (cfg *pluginConfig) validate() error {
	for _, s := range cfg.sessions {
		if s.mode != api.MultiHop || s.profile == "" {
			continue
		}
		p, ok := cfg.profiles[s.profile]
		if !ok {
			continue
		}
		if p.echo != nil {
			return fmt.Errorf("bfd: multi-hop-session %s uses profile %q with echo enabled (RFC 5883 Section 4 prohibits multi-hop echo)", s.peer, s.profile)
		}
	}
	return nil
}
