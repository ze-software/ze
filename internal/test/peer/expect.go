// Design: docs/architecture/testing/ci-format.md — .ci file loading and option parsing
// Overview: peer.go — Config struct populated by these parsers
// Related: checker.go — expect rules parsed here feed the Checker

package peer

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/test/ci"
)

// LoadExpectFile loads expected messages from a file.
// Uses format: action:type:key=value:key=value:...
func LoadExpectFile(path string) ([]string, *Config, error) {
	f, err := os.Open(path) //nolint:gosec // Path from user input (CLI arg)
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
			parseOptionConfig(config, lineType, kv)

		case "expect":
			if lineType == "bgp" {
				// Pass through new format: expect=bgp:conn=N:seq=N:hex=...
				expect = append(expect, line)
			}
			// Ignore json, stderr, syslog - handled by test runner

		case "action":
			if lineType == "notification" {
				// Pass through new format: action=notification:conn=N:seq=N:text=...
				expect = append(expect, line)
			}
			if lineType == "send" {
				// Pass through new format: action=send:conn=N:seq=N:hex=...
				expect = append(expect, line)
			}
			if lineType == "rewrite" {
				// Pass through: action=rewrite:conn=N:seq=N:source=FILE:dest=FILE
				expect = append(expect, line)
			}
			if lineType == actionClose {
				// Pass through: action=close:conn=N:seq=N
				expect = append(expect, line)
			}
			if lineType == actionSighup {
				// Pass through: action=sighup:conn=N:seq=N
				expect = append(expect, line)
			}
			if lineType == actionSigterm {
				// Pass through: action=sigterm:conn=N:seq=N
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
func parseOptionConfig(config *Config, optType string, kv map[string]string) {
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
				ASSet:    kv["as-set"] == "true",
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
			config.SendRoutes = append(config.SendRoutes, route)
		}

	case "timeout", "env":
		// Ignored - handled by test runner
	}
}
