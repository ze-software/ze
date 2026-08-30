// Design: docs/architecture/core-design.md — Linux-specific support collectors

//go:build linux

package support

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/nftables"

	"github.com/ze-software/ze/internal/core/procfs"
)

func collectDmesgInfo(since time.Time) (any, error) {
	f, err := os.OpenFile("/dev/kmsg", os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // fixed kernel path
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only fd

	var sinceUs uint64
	if !since.IsZero() {
		sinceUs = sinceToBootUs(since)
	}

	const maxEntries = 500
	entries := make([]map[string]any, 0, maxEntries)
	buf := make([]byte, 8192)

	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			line := string(buf[:n])
			if entry := parseDmesgLine(line); entry != nil {
				if sinceUs > 0 {
					if ts, ok := entry["timestamp-us"].(uint64); ok && ts < sinceUs {
						continue
					}
				}
				entries = append(entries, entry)
				if len(entries) >= maxEntries*2 {
					entries = entries[len(entries)-maxEntries:]
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, io.EOF) {
				break
			}
			return nil, readErr
		}
	}

	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}

	result := map[string]any{
		"entries": entries,
		keyCount:  len(entries),
	}
	if sinceUs > 0 {
		result["since"] = since.Format(time.RFC3339)
	}
	return result, nil
}

// sinceToBootUs converts an absolute wall-clock time to microseconds since
// boot, for comparison with /dev/kmsg timestamps.
func sinceToBootUs(since time.Time) uint64 {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0
	}
	bootTime := time.Now().Add(-time.Duration(info.Uptime) * time.Second)
	elapsed := since.Sub(bootTime)
	if elapsed < 0 {
		return 0
	}
	return uint64(elapsed.Microseconds())
}

// format: "level,sequence,timestamp,-;message\n"
func parseDmesgLine(line string) map[string]any {
	line = strings.TrimRight(line, "\n")
	header, message, found := strings.Cut(line, ";")
	if !found {
		return nil
	}

	parts := strings.SplitN(header, ",", 4)
	if len(parts) < 3 {
		return nil
	}

	level, _ := strconv.Atoi(parts[0])
	seq, _ := strconv.ParseUint(parts[1], 10, 64)
	ts, _ := strconv.ParseUint(parts[2], 10, 64)

	return map[string]any{
		"level":        level,
		"sequence":     seq,
		"timestamp-us": ts,
		"message":      message,
	}
}

// The transport protocol names the /proc/net socket tables are read under.
const (
	protoTCP = "tcp"
	protoUDP = "udp"
)

// fdCategoryUnknown is the file-descriptor category for a link this package
// cannot classify. It is not the nftables family of the same spelling.
const fdCategoryUnknown = "unknown"

func collectSocketsInfo() (any, error) {
	type socketFile struct {
		path  string
		proto string
		ipv   string
	}

	files := []socketFile{
		{"/proc/net/tcp", protoTCP, "4"},
		{"/proc/net/tcp6", protoTCP, "6"},
		{"/proc/net/udp", protoUDP, "4"},
		{"/proc/net/udp6", protoUDP, "6"},
	}

	var sockets []map[string]any
	for _, sf := range files {
		lines, err := procfs.ReadFileLines(sf.path)
		if err != nil {
			continue
		}
		if len(lines) < 2 {
			continue
		}
		// skip header line
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			localParts := strings.SplitN(fields[1], ":", 2)
			remoteParts := strings.SplitN(fields[2], ":", 2)
			if len(localParts) != 2 || len(remoteParts) != 2 {
				continue
			}

			stateHex, _ := strconv.ParseInt(fields[3], 16, 32)

			entry := map[string]any{
				"protocol":    sf.proto,
				"ip-version":  sf.ipv,
				"local-addr":  procfs.ParseHexAddr(localParts[0]),
				"local-port":  procfs.ParseHexPort(localParts[1]),
				"remote-addr": procfs.ParseHexAddr(remoteParts[0]),
				"remote-port": procfs.ParseHexPort(remoteParts[1]),
			}

			if sf.proto == protoTCP {
				entry["state"] = procfs.TCPStateString(int(stateHex))
			}

			sockets = append(sockets, entry)
		}
	}

	return map[string]any{
		"sockets": sockets,
		keyCount:  len(sockets),
	}, nil
}

func collectKernelModulesInfo() (any, error) {
	lines, err := procfs.ReadFileLines("/proc/modules")
	if err != nil {
		return nil, err
	}

	modules := make(map[string]int, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		size, _ := strconv.Atoi(fields[1])
		modules[fields[0]] = size
	}

	return map[string]any{
		"modules": modules,
		keyCount:  len(modules),
	}, nil
}

func collectConntrackInfo() (any, error) {
	result := map[string]any{
		keyCount:      readProcSysInt("net/netfilter/nf_conntrack_count"),
		"max":         readProcSysInt("net/netfilter/nf_conntrack_max"),
		"buckets":     readProcSysInt("net/netfilter/nf_conntrack_buckets"),
		"expect-max":  readProcSysInt("net/netfilter/nf_conntrack_expect_max"),
		"acct":        readProcSysBool("net/netfilter/nf_conntrack_acct"),
		"timestamp":   readProcSysBool("net/netfilter/nf_conntrack_timestamp"),
		"checksum":    readProcSysBool("net/netfilter/nf_conntrack_checksum"),
		"log-invalid": readProcSysInt("net/netfilter/nf_conntrack_log_invalid"),
		"timeouts": map[string]any{
			"generic":         readProcSysInt("net/netfilter/nf_conntrack_generic_timeout"),
			"tcp-syn-sent":    readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_syn_sent"),
			"tcp-syn-recv":    readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_syn_recv"),
			"tcp-established": readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_established"),
			"tcp-fin-wait":    readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_fin_wait"),
			"tcp-close-wait":  readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_close_wait"),
			"tcp-last-ack":    readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_last_ack"),
			"tcp-time-wait":   readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_time_wait"),
			"tcp-close":       readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_close"),
			"tcp-max-retrans": readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_max_retrans"),
			"tcp-unack":       readProcSysInt("net/netfilter/nf_conntrack_tcp_timeout_unacknowledged"),
			"udp-timeout":     readProcSysInt("net/netfilter/nf_conntrack_udp_timeout"),
			"udp-stream":      readProcSysInt("net/netfilter/nf_conntrack_udp_timeout_stream"),
			"icmp":            readProcSysInt("net/netfilter/nf_conntrack_icmp_timeout"),
			"icmpv6":          readProcSysInt("net/netfilter/nf_conntrack_icmpv6_timeout"),
			"gre-timeout":     readProcSysInt("net/netfilter/nf_conntrack_gre_timeout"),
			"gre-stream":      readProcSysInt("net/netfilter/nf_conntrack_gre_timeout_stream"),
		},
		"tcp-behavior": map[string]any{
			"be-liberal":         readProcSysBool("net/netfilter/nf_conntrack_tcp_be_liberal"),
			"loose":              readProcSysBool("net/netfilter/nf_conntrack_tcp_loose"),
			"max-retrans":        readProcSysInt("net/netfilter/nf_conntrack_tcp_max_retrans"),
			"ignore-invalid-rst": readProcSysBool("net/netfilter/nf_conntrack_tcp_ignore_invalid_rst"),
		},
	}

	return result, nil
}

func readProcSysInt(path string) int {
	data, err := os.ReadFile("/proc/sys/" + path) //nolint:gosec // paths are hardcoded conntrack sysctls
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return v
}

func readProcSysBool(path string) bool {
	return readProcSysInt(path) != 0
}

func collectFDsInfo() (any, error) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, err
	}

	counts := map[string]int{
		"socket":          0,
		"pipe":            0,
		"anon-inode":      0,
		"device":          0,
		"file":            0,
		fdCategoryUnknown: 0,
	}

	var fds []map[string]string
	for _, entry := range entries {
		target, linkErr := os.Readlink("/proc/self/fd/" + entry.Name())
		if linkErr != nil {
			continue
		}

		category := categorizeFD(target)
		counts[category]++
		fds = append(fds, map[string]string{
			"fd":       entry.Name(),
			"target":   target,
			"category": category,
		})
	}

	result := map[string]any{
		"fds":    fds,
		keyCount: len(fds),
		"counts": counts,
	}

	if soft, hard, ok := parseFDLimits(); ok {
		result["soft-limit"] = soft
		result["hard-limit"] = hard
	}

	return result, nil
}

func categorizeFD(target string) string {
	switch {
	case strings.HasPrefix(target, "socket:"):
		return "socket"
	case strings.HasPrefix(target, "pipe:"):
		return "pipe"
	case strings.HasPrefix(target, "anon_inode:"):
		return "anon-inode"
	case strings.HasPrefix(target, "/dev/"):
		return "device"
	case target == "(unknown)":
		return fdCategoryUnknown
	default:
		return "file"
	}
}

func parseFDLimits() (soft, hard int, ok bool) {
	lines, err := procfs.ReadFileLines("/proc/self/limits")
	if err != nil {
		return 0, 0, false
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "Max open files") {
			continue
		}
		// "Max open files            1048576              1048576              files"
		rest := line[len("Max open files"):]
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			return 0, 0, false
		}
		s, err1 := strconv.Atoi(fields[0])
		h, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return s, h, true
	}

	return 0, 0, false
}

func collectDNSInfo() (any, error) {
	data, err := os.ReadFile("/etc/resolv.conf") //nolint:gosec // fixed system path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{keyAvailable: false}, nil
		}
		return nil, err
	}
	return map[string]any{"content": string(data)}, nil
}

func collectFirewallInfo() (any, error) {
	conn := &nftables.Conn{}

	tables, err := conn.ListTables()
	if err != nil {
		return nil, err
	}

	chains, err := conn.ListChains()
	if err != nil {
		return nil, err
	}

	chainsByTable := make(map[string][]map[string]any)
	for _, c := range chains {
		tableName := ""
		if c.Table != nil {
			tableName = c.Table.Name
		}
		entry := map[string]any{
			keyName: c.Name,
			"type":  string(c.Type),
		}
		if c.Policy != nil {
			switch *c.Policy {
			case nftables.ChainPolicyAccept:
				entry["policy"] = "accept"
			case nftables.ChainPolicyDrop:
				entry["policy"] = "drop"
			}
		}
		if c.Table != nil {
			rules, rulesErr := conn.GetRules(c.Table, c)
			if rulesErr == nil {
				entry["rule-count"] = len(rules)
			}
		}
		chainsByTable[tableName] = append(chainsByTable[tableName], entry)
	}

	tableList := make([]map[string]any, 0, len(tables))
	for _, t := range tables {
		entry := map[string]any{
			keyName:  t.Name,
			"family": tableFamilyName(t.Family),
		}
		if cs, ok := chainsByTable[t.Name]; ok {
			entry["chains"] = cs
		}
		tableList = append(tableList, entry)
	}

	return map[string]any{
		"tables": tableList,
		keyCount: len(tableList),
	}, nil
}

func tableFamilyName(f nftables.TableFamily) string {
	switch f {
	case nftables.TableFamilyIPv4:
		return "ip"
	case nftables.TableFamilyIPv6:
		return "ip6"
	case nftables.TableFamilyINet:
		return "inet"
	case nftables.TableFamilyARP:
		return "arp"
	case nftables.TableFamilyNetdev:
		return "netdev"
	case nftables.TableFamilyBridge:
		return "bridge"
	default:
		return "unknown"
	}
}
