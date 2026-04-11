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
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/session"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// pluginConfig is the validated, in-memory shape of a `bfd { ... }` block.
type pluginConfig struct {
	enabled  bool
	profiles map[string]profileConfig
	sessions []sessionConfig
}

// profileConfig holds the timer parameters reusable across sessions.
type profileConfig struct {
	name            string
	detectMult      uint8
	desiredMinTxUs  uint32
	requiredMinRxUs uint32
	passive         bool
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
	return p, nil
}

// parseSingleHopSession decodes one entry under `single-hop-session`. The
// YANG list key is "peer vrf interface", but the engine treats the parsed
// fields independently so the YANG key tuple is not stored as a string.
func parseSingleHopSession(_ string, fields map[string]any, profiles map[string]profileConfig) (sessionConfig, error) {
	peerStr, _ := fields["peer"].(string)
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

// parseMultiHopSession decodes one entry under `multi-hop-session`. RFC
// 5883 §5 requires a local source address; the parser enforces it so the
// engine can rely on netip.Addr.IsValid() at session creation time.
func parseMultiHopSession(_ string, fields map[string]any, profiles map[string]profileConfig) (sessionConfig, error) {
	peerStr, _ := fields["peer"].(string)
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

// defaultVRF normalises an unset VRF to "default" so the rest of the
// plugin compares VRF names without special-casing the empty string.
func defaultVRF(v string) string {
	if v == "" {
		return "default"
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
	}
	if s.profile != "" {
		if p, ok := profiles[s.profile]; ok {
			req.DesiredMinTxInterval = p.desiredMinTxUs
			req.RequiredMinRxInterval = p.requiredMinRxUs
			req.DetectMult = p.detectMult
			req.Passive = p.passive
		}
	}
	return req
}
