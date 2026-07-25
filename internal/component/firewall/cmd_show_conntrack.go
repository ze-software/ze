// Design: docs/guide/command-catalogue.md -- show system conntrack

package firewall

import (
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:system-conntrack",
			Handler:    handleShowSystemConntrack,
		},
	)
}

func handleShowSystemConntrack(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	data := map[string]any{}

	data["count"] = readProcSysctl("net/netfilter/nf_conntrack_count")
	data["max"] = readProcSysctl("net/netfilter/nf_conntrack_max")
	data["buckets"] = readProcSysctl("net/netfilter/nf_conntrack_buckets")
	data["expect-max"] = readProcSysctl("net/netfilter/nf_conntrack_expect_max")

	data["accounting"] = readProcSysctlBool("net/netfilter/nf_conntrack_acct")
	data["timestamp"] = readProcSysctlBool("net/netfilter/nf_conntrack_timestamp")
	data["checksum"] = readProcSysctlBool("net/netfilter/nf_conntrack_checksum")
	data["log-invalid"] = readProcSysctl("net/netfilter/nf_conntrack_log_invalid")

	data["modules"] = system.LoadedConntrackModules()

	timeouts := map[string]any{
		"generic": readProcSysctl("net/netfilter/nf_conntrack_generic_timeout"),
	}
	timeouts["tcp"] = map[string]any{
		"established":    readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_established"),
		"syn-sent":       readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_syn_sent"),
		"syn-recv":       readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_syn_recv"),
		"fin-wait":       readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_fin_wait"),
		"close-wait":     readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_close_wait"),
		"last-ack":       readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_last_ack"),
		"time-wait":      readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_time_wait"),
		"close":          readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_close"),
		"unacknowledged": readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_unacknowledged"),
		"max-retrans":    readProcSysctl("net/netfilter/nf_conntrack_tcp_timeout_max_retrans"),
	}
	timeouts["udp"] = map[string]any{
		"timeout": readProcSysctl("net/netfilter/nf_conntrack_udp_timeout"),
		"stream":  readProcSysctl("net/netfilter/nf_conntrack_udp_timeout_stream"),
	}
	timeouts["icmp"] = readProcSysctl("net/netfilter/nf_conntrack_icmp_timeout")
	timeouts["icmpv6"] = readProcSysctl("net/netfilter/nf_conntrack_icmpv6_timeout")
	timeouts["gre"] = map[string]any{
		"timeout": readProcSysctl("net/netfilter/nf_conntrack_gre_timeout"),
		"stream":  readProcSysctl("net/netfilter/nf_conntrack_gre_timeout_stream"),
	}
	data["timeouts"] = timeouts

	tcpBehavior := map[string]any{
		"be-liberal":         readProcSysctlBool("net/netfilter/nf_conntrack_tcp_be_liberal"),
		"loose":              readProcSysctlBool("net/netfilter/nf_conntrack_tcp_loose"),
		"max-retrans":        readProcSysctl("net/netfilter/nf_conntrack_tcp_max_retrans"),
		"ignore-invalid-rst": readProcSysctlBool("net/netfilter/nf_conntrack_tcp_ignore_invalid_rst"),
	}
	data["tcp-behavior"] = tcpBehavior

	return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
}

func readProcSysctl(path string) int {
	b, err := os.ReadFile("/proc/sys/" + path) //nolint:gosec // path is always a hardcoded sysctl key, never user input
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return v
}

func readProcSysctlBool(path string) bool {
	return readProcSysctl(path) != 0
}
