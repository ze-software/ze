// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- TCP/UDP socket state from /proc/net (ss replacement)
// Related: fd_linux.go -- existing /proc reading pattern
//
//go:build linux

package show

import (
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/procfs"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-sockets", Handler: handleShowSystemSockets},
	)
}

func handleShowSystemSockets(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	const protoTCP = "tcp"
	proto := ""
	state := ""
	port := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case protoTCP, "udp":
			proto = args[i]
		case "state":
			if i+1 < len(args) {
				i++
				state = strings.ToUpper(args[i])
			}
		case "port":
			if i+1 < len(args) {
				i++
				p, err := strconv.Atoi(args[i])
				if err == nil && p >= 0 && p <= 65535 {
					port = p
				}
			}
		}
	}

	var sockets []map[string]any

	if proto == "" || proto == protoTCP {
		tcp4, err := parseProcNetSockets("/proc/net/tcp", protoTCP)
		if err == nil {
			sockets = append(sockets, tcp4...)
		}
		tcp6, err := parseProcNetSockets("/proc/net/tcp6", "tcp6")
		if err == nil {
			sockets = append(sockets, tcp6...)
		}
	}
	if proto == "" || proto == "udp" {
		udp4, err := parseProcNetSockets("/proc/net/udp", "udp")
		if err == nil {
			sockets = append(sockets, udp4...)
		}
		udp6, err := parseProcNetSockets("/proc/net/udp6", "udp6")
		if err == nil {
			sockets = append(sockets, udp6...)
		}
	}

	var filtered []map[string]any
	for _, s := range sockets {
		if state != "" {
			if st, ok := s["state"].(string); !ok || st != state {
				continue
			}
		}
		if port != 0 {
			lp, _ := s["local-port"].(int)
			rp, _ := s["remote-port"].(int)
			if lp != port && rp != port {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"sockets": filtered,
			keyCount:  len(filtered),
		},
	}, nil
}

func parseProcNetSockets(path, proto string) ([]map[string]any, error) {
	lines, err := procfs.ReadFileLines(path)
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for i, line := range lines {
		if i == 0 {
			continue
		}
		entry := parseProcNetLine(line, proto)
		if entry != nil {
			results = append(results, entry)
		}
	}
	return results, nil
}

func parseProcNetLine(line, proto string) map[string]any {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return nil
	}

	localParts := strings.SplitN(fields[1], ":", 2)
	remoteParts := strings.SplitN(fields[2], ":", 2)
	if len(localParts) < 2 || len(remoteParts) < 2 {
		return nil
	}

	stateHex := fields[3]
	stateInt, err := strconv.ParseInt(stateHex, 16, 32)
	if err != nil {
		return nil
	}

	entry := map[string]any{
		"protocol":    proto,
		"local-addr":  procfs.ParseHexAddr(localParts[0]),
		"local-port":  procfs.ParseHexPort(localParts[1]),
		"remote-addr": procfs.ParseHexAddr(remoteParts[0]),
		"remote-port": procfs.ParseHexPort(remoteParts[1]),
		"state":       procfs.TCPStateString(int(stateInt)),
	}

	if len(fields) >= 5 {
		queueParts := strings.SplitN(fields[4], ":", 2)
		if len(queueParts) == 2 {
			tx, _ := strconv.ParseInt(queueParts[0], 16, 64)
			rx, _ := strconv.ParseInt(queueParts[1], 16, 64)
			entry["tx-queue"] = int(tx)
			entry["rx-queue"] = int(rx)
		}
	}

	return entry
}
