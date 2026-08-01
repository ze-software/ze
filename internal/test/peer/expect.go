// Design: docs/architecture/testing/ci-format.md — .ci file loading and option parsing
// Overview: peer.go — Config struct populated by these parsers
// Related: checker.go — expect rules parsed here feed the Checker

package peer

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/test/ci"
)

// optTrue is the spelling a .ci boolean option must use to enable itself.
const optTrue = "true"

// Consumes reports whether ze-peer consumes the directive `action=lineType`,
// i.e. whether LoadExpectFile forwards such a line into Config.Expect.
//
// This is the single definition of the ze-peer-consumed directive set. It is
// deliberately exported through ConsumesLine so the test runner can reject, at
// parse time, a check-mode peer block that declares nothing ze-peer can act on.
// Such a block leaves Config.Expect empty, which makes ze-peer print
// "no test data available to test against" and exit 1 BEFORE it ever binds a
// listening socket (internal/test/cli/cmd_peer.go). Every ze dial then gets
// connection refused and the test can only ever pass vacuously.
//
// Keep this function and LoadExpectFile's switch in lockstep: they are the same
// decision, and a divergence reintroduces the silent-vacuous-test defect.
func consumes(action, lineType string) bool {
	switch action {
	case "expect":
		// json/stderr/syslog are handled by the test runner, not ze-peer.
		return lineType == "bgp"
	case "action":
		switch lineType {
		case "notification", "send", "rewrite", actionClose, actionSighup, actionSigterm:
			return true
		}
	}
	return false
}

// ConsumesLine reports whether ze-peer consumes the given .ci line. Blank lines,
// comments, and runner-only directives (expect=json, option=env, cmd=, reject=)
// return false. See consumes for why this is exported.
func ConsumesLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return false
	}
	head, _, _ := strings.Cut(line, ":")
	action, lineType, ok := strings.Cut(head, "=")
	if !ok {
		return false
	}
	return consumes(action, lineType)
}

// LoadExpectFile loads expected messages from a file.
// Uses format: action:type:key=value:key=value:...
func LoadExpectFile(path string) ([]string, *Config, error) {
	f, err := cliio.OpenReader(path) // "-" reads stdin
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	config := &Config{}
	var expect []string

	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: action=type:key=value:...
		// First segment is action=type, remaining segments are key=value pairs
		parts := strings.Split(line, ":")
		if len(parts) < 1 {
			return nil, nil, fmt.Errorf("line %d: invalid format %q", lineNum, line)
		}

		// First segment is action=type
		actionType := strings.SplitN(parts[0], "=", 2)
		if len(actionType) != 2 {
			return nil, nil, fmt.Errorf("line %d: invalid format %q, expected action=type:key=value", lineNum, line)
		}
		action := actionType[0]
		lineType := actionType[1]
		kv := ci.ParseKVPairs(parts[1:])

		switch action {
		case "option":
			if err := parseOptionConfig(config, lineType, kv); err != nil {
				return nil, nil, fmt.Errorf("line %d: %w", lineNum, err)
			}

		case "expect", "action":
			// Pass through the ze-peer-consumed directives only:
			//   expect=bgp:conn=N:seq=N:hex=...
			//   action=notification|send|rewrite|close|sighup|sigterm:conn=N:seq=N:...
			// expect=json/stderr/syslog are handled by the test runner.
			if consumes(action, lineType) {
				expect = append(expect, line)
			}

		case "cmd":
			// Ignore - documentation only

		case "reject":
			// Ignore - handled by test runner
		}
	}

	return expect, config, scanner.Err()
}

// parseOptionConfig parses option lines into Config.
//
// It returns an error only for a directive whose misreading would make a test
// assert against an input it never sent. Most options still fail soft, which is
// the long-standing behavior of this parser and is not changed here.
func parseOptionConfig(config *Config, optType string, kv map[string]string) error {
	switch optType {
	case "file":
		// Ignored - handled by test runner

	case "asn":
		if v, err := strconv.Atoi(kv["value"]); err == nil {
			config.ASN = v
		}

	case "bind":
		v := kv["value"]
		if v == "ipv6" {
			config.IPv6 = true
		} else if v != "" {
			config.BindAddr = v
		}

	case "conn_map":
		config.ConnMap = kv["value"]

	case "await_eor":
		config.AwaitEOR = kv["value"] == optTrue

	case "linger":
		config.Linger = kv["value"] == optTrue

	case "tcp_connections":
		if v, err := strconv.Atoi(kv["value"]); err == nil {
			config.TCPConnections = v
		}

	case "open":
		switch kv["value"] {
		case "send-unknown-capability":
			config.SendUnknownCapability = true
		case "inspect-open-message":
			config.InspectOpenMessage = true
		case "send-unknown-message":
			config.SendUnknownMessage = true
		case "drop-capability":
			if codeStr := kv["code"]; codeStr != "" {
				code, err := strconv.Atoi(codeStr)
				if err == nil && code > 0 && code <= 255 {
					config.CapabilityOverrides = append(config.CapabilityOverrides, CapabilityOverride{
						Code: uint8(code), Add: false, //nolint:gosec // range checked
					})
				}
			}
		case "router-id":
			// option=open:value=router-id:id=<a.b.c.d> -- send this BGP Identifier instead of
			// the mirrored ze identifier + 1. Drives RFC 6286 Section 2.2 rejection tests.
			if addr, err := netip.ParseAddr(kv["id"]); err == nil && addr.Is4() {
				octets := addr.As4()
				id := binary.BigEndian.Uint32(octets[:])
				config.RouterID = &id
			}

		case "add-capability":
			if codeStr := kv["code"]; codeStr != "" {
				code, err := strconv.Atoi(codeStr)
				if err == nil && code > 0 && code <= 255 {
					val, _ := hex.DecodeString(kv["hex"])
					config.CapabilityOverrides = append(config.CapabilityOverrides, CapabilityOverride{
						Code: uint8(code), Value: val, Add: true, //nolint:gosec // range checked
					})
				}
			}
		}

	case "update":
		switch kv["value"] {
		case "send-default-route":
			config.SendDefaultRoute = true
		case "send-route":
			asn, _ := strconv.ParseUint(kv["origin-as"], 10, 32)
			route := RouteToSend{
				Prefix:   kv["prefix"],
				OriginAS: uint32(asn), //nolint:gosec // range checked by ParseUint
				NextHop:  kv["next-hop"],
				ASSet:    kv["as-set"] == optTrue,
			}
			// Extended fields for loop detection tests.
			if v := kv["as-path"]; v != "" {
				for s := range strings.SplitSeq(v, ",") {
					a, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
					if err == nil {
						route.ASPath = append(route.ASPath, uint32(a)) //nolint:gosec // range checked
					}
				}
			}
			if v := kv["originator-id"]; v != "" {
				ip := net.ParseIP(v)
				if ip != nil {
					ip4 := ip.To4()
					if ip4 != nil {
						route.OriginatorID = uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
					}
				}
			}
			if v := kv["cluster-list"]; v != "" {
				for s := range strings.SplitSeq(v, ",") {
					ip := net.ParseIP(strings.TrimSpace(s))
					if ip != nil {
						ip4 := ip.To4()
						if ip4 != nil {
							route.ClusterList = append(route.ClusterList, uint32(ip4[0])<<24|uint32(ip4[1])<<16|uint32(ip4[2])<<8|uint32(ip4[3]))
						}
					}
				}
			}
			if v := kv["label"]; v != "" {
				l, err := strconv.ParseUint(v, 10, 32)
				if err == nil {
					route.Labels = []uint32{uint32(l)} //nolint:gosec // range checked
				}
			}
			config.SendRoutes = append(config.SendRoutes, route)
		case "send-bulk":
			spec, err := parseBulkSpec(kv)
			if err != nil {
				return err
			}
			config.SendBulk = append(config.SendBulk, spec)
		}

	case "timeout", "env":
		// Ignored - handled by test runner
	}
	return nil
}

// parseBulkSpec reads option=update:value=send-bulk into an InjectSpec.
//
// Keys: prefix, count, next-hop, origin-as, max-msg (optional, default 4096),
// eor (optional, "true" appends an End-of-RIB marker).
//
// Every key is validated and a bad one is an error rather than a zero value: a
// spec that silently degrades to count=0 sends nothing, and a test asserting
// that a route was NOT forwarded would then pass for the wrong reason
// (ai/rules/fail-closed-guards.md).
func parseBulkSpec(kv map[string]string) (InjectSpec, error) {
	spec := InjectSpec{}

	prefix, err := netip.ParsePrefix(kv["prefix"])
	if err != nil {
		return spec, fmt.Errorf("send-bulk prefix %q: %w", kv["prefix"], err)
	}
	spec.Prefix = prefix

	count, err := strconv.Atoi(kv["count"])
	if err != nil || count < 1 {
		return spec, fmt.Errorf("send-bulk count %q: want a positive integer", kv["count"])
	}
	spec.Count = count

	nextHop, err := netip.ParseAddr(kv["next-hop"])
	if err != nil {
		return spec, fmt.Errorf("send-bulk next-hop %q: %w", kv["next-hop"], err)
	}
	spec.NextHop = nextHop

	asn, err := strconv.ParseUint(kv["origin-as"], 10, 32)
	if err != nil {
		return spec, fmt.Errorf("send-bulk origin-as %q: want a 32-bit ASN", kv["origin-as"])
	}
	spec.ASN = uint32(asn) //nolint:gosec // G115: bounded by ParseUint's 32-bit width

	if v := kv["max-msg"]; v != "" {
		maxMsg, err := strconv.Atoi(v)
		if err != nil || maxMsg < bgpEORLen || maxMsg > bgpExtMsgLen {
			return spec, fmt.Errorf("send-bulk max-msg %q: want %d..%d", v, bgpEORLen, bgpExtMsgLen)
		}
		spec.MaxMsgLen = maxMsg
	}

	// Default false: the EOR marker is a separate message, and a test that
	// asserts on frame sequence must be able to choose whether one arrives.
	spec.EndOfRIB = kv["eor"] == optTrue

	return spec, nil
}
