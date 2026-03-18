// Design: docs/architecture/testing/ci-format.md — .ci file loading and option parsing
// Overview: peer.go — Config struct populated by these parsers
// Related: checker.go — expect rules parsed here feed the Checker

package peer

import (
	"bufio"
	"encoding/hex"
	"fmt"
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
		if kv["value"] == "ipv6" {
			config.IPv6 = true
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
			config.SendRoutes = append(config.SendRoutes, route)
		}

	case "timeout", "env":
		// Ignored - handled by test runner
	}
}
