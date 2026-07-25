// Design: docs/architecture/config/syntax.md -- conntrack config extraction

package system

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const boolTrue = "true"

// RegisterConntrackManagedKeys registers conntrack-managed sysctl keys
// with the sysctl registry for dual-setting prevention.
func RegisterConntrackManagedKeys() {
	sysctlreg.RegisterManagedKeys(ConntrackManagedKeys())
}

// ConntrackConfig holds connection tracking configuration.
type ConntrackConfig struct {
	Modules []string

	TableSize int
	HashSize  int
	ExpectMax int

	Accounting bool
	Timestamp  bool
	Checksum   bool
	LogInvalid string

	TimeoutGeneric int

	TimeoutTCP  TCPTimeouts
	TCPBehavior TCPBehavior

	TimeoutUDP    UDPTimeouts
	TimeoutICMP   ICMPTimeouts
	TimeoutICMPv6 ICMPTimeouts
	TimeoutGRE    GRETimeouts
	TimeoutSCTP   SCTPTimeouts
	TimeoutDCCP   DCCPTimeouts
}

// TCPTimeouts holds TCP connection tracking timeouts.
type TCPTimeouts struct {
	Established    int
	SynSent        int
	SynRecv        int
	FinWait        int
	CloseWait      int
	LastAck        int
	TimeWait       int
	Close          int
	Unacknowledged int
	MaxRetrans     int
}

// TCPBehavior holds TCP connection tracking behavior flags.
type TCPBehavior struct {
	BeLiberal        *bool
	Loose            *bool
	MaxRetrans       int
	IgnoreInvalidRST *bool
}

// UDPTimeouts holds UDP connection tracking timeouts.
type UDPTimeouts struct {
	Timeout int
	Stream  int
}

// ICMPTimeouts holds ICMP/ICMPv6 connection tracking timeouts.
type ICMPTimeouts struct {
	Timeout int
}

// GRETimeouts holds GRE connection tracking timeouts.
type GRETimeouts struct {
	Timeout int
	Stream  int
}

// SCTPTimeouts holds SCTP connection tracking timeouts.
type SCTPTimeouts struct {
	Closed          int
	CookieWait      int
	CookieEchoed    int
	Established     int
	ShutdownSent    int
	ShutdownRecd    int
	ShutdownAckSent int
	HeartbeatSent   int
}

// DCCPTimeouts holds DCCP connection tracking timeouts.
type DCCPTimeouts struct {
	Request  int
	Respond  int
	Partopen int
	Open     int
	Closereq int
	Closing  int
	Timewait int
}

var allowedModules = map[string]bool{
	"ftp":        true,
	"h323":       true,
	"sip":        true,
	"pptp":       true,
	"tftp":       true,
	"nfs":        true,
	"sane":       true,
	"irc":        true,
	"amanda":     true,
	"netbios-ns": true,
	"snmp":       true,
	"sqlnet":     true,
}

// ValidConntrackModule returns true if the module name is in the allowlist.
func ValidConntrackModule(name string) bool {
	return allowedModules[name]
}

// AllConntrackModules returns the list of valid module names.
func AllConntrackModules() []string {
	names := make([]string, 0, len(allowedModules))
	for name := range allowedModules {
		names = append(names, name)
	}
	return names
}

var logInvalidProtocols = map[string]int{
	"all":    255,
	"tcp":    6,
	"udp":    17,
	"icmp":   1,
	"icmpv6": 58,
}

// ConntrackSysctlKeys returns the sysctl key/value pairs for this config.
// Only non-zero values are included.
func (c *ConntrackConfig) ConntrackSysctlKeys() map[string]string {
	m := make(map[string]string)

	if c.TableSize > 0 {
		m["net.netfilter.nf_conntrack_max"] = strconv.Itoa(c.TableSize)
	}
	if c.HashSize > 0 {
		m["net.netfilter.nf_conntrack_buckets"] = strconv.Itoa(c.HashSize)
	}
	if c.ExpectMax > 0 {
		m["net.netfilter.nf_conntrack_expect_max"] = strconv.Itoa(c.ExpectMax)
	}
	if c.Accounting {
		m["net.netfilter.nf_conntrack_acct"] = "1"
	}
	if c.Timestamp {
		m["net.netfilter.nf_conntrack_timestamp"] = "1"
	}
	if c.Checksum {
		m["net.netfilter.nf_conntrack_checksum"] = "1"
	}
	if c.LogInvalid != "" {
		if proto, ok := logInvalidProtocols[c.LogInvalid]; ok {
			m["net.netfilter.nf_conntrack_log_invalid"] = strconv.Itoa(proto)
		}
	}
	if c.TimeoutGeneric > 0 {
		m["net.netfilter.nf_conntrack_generic_timeout"] = strconv.Itoa(c.TimeoutGeneric)
	}

	addTCPTimeouts(m, c.TimeoutTCP)
	addTCPBehavior(m, c.TCPBehavior)
	addUDPTimeouts(m, c.TimeoutUDP)
	addICMPTimeouts(m, "net.netfilter.nf_conntrack_icmp_timeout", c.TimeoutICMP)
	addICMPTimeouts(m, "net.netfilter.nf_conntrack_icmpv6_timeout", c.TimeoutICMPv6)
	addGRETimeouts(m, c.TimeoutGRE)
	addSCTPTimeouts(m, c.TimeoutSCTP)
	addDCCPTimeouts(m, c.TimeoutDCCP)

	return m
}

func addTCPTimeouts(m map[string]string, t TCPTimeouts) {
	prefix := "net.netfilter.nf_conntrack_tcp_timeout_"
	addIfPositive(m, prefix+"established", t.Established)
	addIfPositive(m, prefix+"syn_sent", t.SynSent)
	addIfPositive(m, prefix+"syn_recv", t.SynRecv)
	addIfPositive(m, prefix+"fin_wait", t.FinWait)
	addIfPositive(m, prefix+"close_wait", t.CloseWait)
	addIfPositive(m, prefix+"last_ack", t.LastAck)
	addIfPositive(m, prefix+"time_wait", t.TimeWait)
	addIfPositive(m, prefix+"close", t.Close)
	addIfPositive(m, prefix+"unacknowledged", t.Unacknowledged)
	addIfPositive(m, prefix+"max_retrans", t.MaxRetrans)
}

func addTCPBehavior(m map[string]string, b TCPBehavior) {
	prefix := "net.netfilter.nf_conntrack_tcp_"
	if b.BeLiberal != nil {
		m[prefix+"be_liberal"] = boolToSysctl(*b.BeLiberal)
	}
	if b.Loose != nil {
		m[prefix+"loose"] = boolToSysctl(*b.Loose)
	}
	if b.MaxRetrans > 0 {
		m[prefix+"max_retrans"] = strconv.Itoa(b.MaxRetrans)
	}
	if b.IgnoreInvalidRST != nil {
		m[prefix+"ignore_invalid_rst"] = boolToSysctl(*b.IgnoreInvalidRST)
	}
}

func addUDPTimeouts(m map[string]string, t UDPTimeouts) {
	addIfPositive(m, "net.netfilter.nf_conntrack_udp_timeout", t.Timeout)
	addIfPositive(m, "net.netfilter.nf_conntrack_udp_timeout_stream", t.Stream)
}

func addICMPTimeouts(m map[string]string, key string, t ICMPTimeouts) {
	addIfPositive(m, key, t.Timeout)
}

func addGRETimeouts(m map[string]string, t GRETimeouts) {
	addIfPositive(m, "net.netfilter.nf_conntrack_gre_timeout", t.Timeout)
	addIfPositive(m, "net.netfilter.nf_conntrack_gre_timeout_stream", t.Stream)
}

func addSCTPTimeouts(m map[string]string, t SCTPTimeouts) {
	prefix := "net.netfilter.nf_conntrack_sctp_timeout_"
	addIfPositive(m, prefix+"closed", t.Closed)
	addIfPositive(m, prefix+"cookie_wait", t.CookieWait)
	addIfPositive(m, prefix+"cookie_echoed", t.CookieEchoed)
	addIfPositive(m, prefix+"established", t.Established)
	addIfPositive(m, prefix+"shutdown_sent", t.ShutdownSent)
	addIfPositive(m, prefix+"shutdown_recd", t.ShutdownRecd)
	addIfPositive(m, prefix+"shutdown_ack_sent", t.ShutdownAckSent)
	addIfPositive(m, prefix+"heartbeat_sent", t.HeartbeatSent)
}

func addDCCPTimeouts(m map[string]string, t DCCPTimeouts) {
	prefix := "net.netfilter.nf_conntrack_dccp_timeout_"
	addIfPositive(m, prefix+"request", t.Request)
	addIfPositive(m, prefix+"respond", t.Respond)
	addIfPositive(m, prefix+"partopen", t.Partopen)
	addIfPositive(m, prefix+"open", t.Open)
	addIfPositive(m, prefix+"closereq", t.Closereq)
	addIfPositive(m, prefix+"closing", t.Closing)
	addIfPositive(m, prefix+"timewait", t.Timewait)
}

func addIfPositive(m map[string]string, key string, val int) {
	if val > 0 {
		m[key] = strconv.Itoa(val)
	}
}

func boolToSysctl(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// ConntrackManagedKeys returns all sysctl keys that conntrack can manage.
// Used for dual-setting prevention: if any of these keys appear in the
// sysctl {} config block, config validation rejects them.
func ConntrackManagedKeys() map[string]string {
	return map[string]string{
		"net.netfilter.nf_conntrack_max":                            "table-size",
		"net.netfilter.nf_conntrack_buckets":                        "hash-size",
		"net.netfilter.nf_conntrack_expect_max":                     "expect-max",
		"net.netfilter.nf_conntrack_acct":                           "accounting",
		"net.netfilter.nf_conntrack_timestamp":                      "timestamp",
		"net.netfilter.nf_conntrack_checksum":                       "checksum",
		"net.netfilter.nf_conntrack_log_invalid":                    "log-invalid",
		"net.netfilter.nf_conntrack_generic_timeout":                "timeout generic",
		"net.netfilter.nf_conntrack_tcp_timeout_established":        "timeout tcp established",
		"net.netfilter.nf_conntrack_tcp_timeout_syn_sent":           "timeout tcp syn-sent",
		"net.netfilter.nf_conntrack_tcp_timeout_syn_recv":           "timeout tcp syn-recv",
		"net.netfilter.nf_conntrack_tcp_timeout_fin_wait":           "timeout tcp fin-wait",
		"net.netfilter.nf_conntrack_tcp_timeout_close_wait":         "timeout tcp close-wait",
		"net.netfilter.nf_conntrack_tcp_timeout_last_ack":           "timeout tcp last-ack",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait":          "timeout tcp time-wait",
		"net.netfilter.nf_conntrack_tcp_timeout_close":              "timeout tcp close",
		"net.netfilter.nf_conntrack_tcp_timeout_unacknowledged":     "timeout tcp unacknowledged",
		"net.netfilter.nf_conntrack_tcp_timeout_max_retrans":        "timeout tcp max-retrans",
		"net.netfilter.nf_conntrack_tcp_be_liberal":                 "tcp be-liberal",
		"net.netfilter.nf_conntrack_tcp_loose":                      "tcp loose",
		"net.netfilter.nf_conntrack_tcp_max_retrans":                "tcp max-retrans",
		"net.netfilter.nf_conntrack_tcp_ignore_invalid_rst":         "tcp ignore-invalid-rst",
		"net.netfilter.nf_conntrack_udp_timeout":                    "timeout udp timeout",
		"net.netfilter.nf_conntrack_udp_timeout_stream":             "timeout udp stream",
		"net.netfilter.nf_conntrack_icmp_timeout":                   "timeout icmp timeout",
		"net.netfilter.nf_conntrack_icmpv6_timeout":                 "timeout icmpv6 timeout",
		"net.netfilter.nf_conntrack_gre_timeout":                    "timeout gre timeout",
		"net.netfilter.nf_conntrack_gre_timeout_stream":             "timeout gre stream",
		"net.netfilter.nf_conntrack_sctp_timeout_closed":            "timeout sctp closed",
		"net.netfilter.nf_conntrack_sctp_timeout_cookie_wait":       "timeout sctp cookie-wait",
		"net.netfilter.nf_conntrack_sctp_timeout_cookie_echoed":     "timeout sctp cookie-echoed",
		"net.netfilter.nf_conntrack_sctp_timeout_established":       "timeout sctp established",
		"net.netfilter.nf_conntrack_sctp_timeout_shutdown_sent":     "timeout sctp shutdown-sent",
		"net.netfilter.nf_conntrack_sctp_timeout_shutdown_recd":     "timeout sctp shutdown-recd",
		"net.netfilter.nf_conntrack_sctp_timeout_shutdown_ack_sent": "timeout sctp shutdown-ack-sent",
		"net.netfilter.nf_conntrack_sctp_timeout_heartbeat_sent":    "timeout sctp heartbeat-sent",
		"net.netfilter.nf_conntrack_dccp_timeout_request":           "timeout dccp request",
		"net.netfilter.nf_conntrack_dccp_timeout_respond":           "timeout dccp respond",
		"net.netfilter.nf_conntrack_dccp_timeout_partopen":          "timeout dccp partopen",
		"net.netfilter.nf_conntrack_dccp_timeout_open":              "timeout dccp open",
		"net.netfilter.nf_conntrack_dccp_timeout_closereq":          "timeout dccp closereq",
		"net.netfilter.nf_conntrack_dccp_timeout_closing":           "timeout dccp closing",
		"net.netfilter.nf_conntrack_dccp_timeout_timewait":          "timeout dccp timewait",
	}
}

// HasConfig returns true if any conntrack configuration is set.
func (c *ConntrackConfig) HasConfig() bool {
	return len(c.Modules) > 0 || c.TableSize > 0 || c.HashSize > 0 ||
		c.ExpectMax > 0 || c.Accounting || c.Timestamp || c.Checksum ||
		c.LogInvalid != "" || c.TimeoutGeneric > 0 ||
		c.hasTCPTimeouts() || c.hasUDPTimeouts() ||
		c.TimeoutICMP.Timeout > 0 || c.TimeoutICMPv6.Timeout > 0 ||
		c.hasGRETimeouts() || c.hasSCTPTimeouts() || c.hasDCCPTimeouts() ||
		c.hasTCPBehavior()
}

func (c *ConntrackConfig) hasTCPTimeouts() bool {
	t := c.TimeoutTCP
	return t.Established > 0 || t.SynSent > 0 || t.SynRecv > 0 ||
		t.FinWait > 0 || t.CloseWait > 0 || t.LastAck > 0 ||
		t.TimeWait > 0 || t.Close > 0 || t.Unacknowledged > 0 || t.MaxRetrans > 0
}

func (c *ConntrackConfig) hasUDPTimeouts() bool {
	return c.TimeoutUDP.Timeout > 0 || c.TimeoutUDP.Stream > 0
}

func (c *ConntrackConfig) hasGRETimeouts() bool {
	return c.TimeoutGRE.Timeout > 0 || c.TimeoutGRE.Stream > 0
}

func (c *ConntrackConfig) hasSCTPTimeouts() bool {
	t := c.TimeoutSCTP
	return t.Closed > 0 || t.CookieWait > 0 || t.CookieEchoed > 0 ||
		t.Established > 0 || t.ShutdownSent > 0 || t.ShutdownRecd > 0 ||
		t.ShutdownAckSent > 0 || t.HeartbeatSent > 0
}

func (c *ConntrackConfig) hasDCCPTimeouts() bool {
	t := c.TimeoutDCCP
	return t.Request > 0 || t.Respond > 0 || t.Partopen > 0 ||
		t.Open > 0 || t.Closereq > 0 || t.Closing > 0 || t.Timewait > 0
}

func (c *ConntrackConfig) hasTCPBehavior() bool {
	b := c.TCPBehavior
	return b.BeLiberal != nil || b.Loose != nil || b.MaxRetrans > 0 || b.IgnoreInvalidRST != nil
}

// ValidateModules checks that all module names are in the allowlist.
// Returns an error for the first invalid module name.
func (c *ConntrackConfig) ValidateModules() error {
	for _, m := range c.Modules {
		if !ValidConntrackModule(m) {
			names := AllConntrackModules()
			sort.Strings(names)
			return fmt.Errorf("conntrack: unknown module %q (valid: %s)", m, textbuf.Join(names, ", "))
		}
	}
	return nil
}

func extractConntrack(sys *config.Tree) ConntrackConfig {
	var cc ConntrackConfig

	ct := sys.GetContainer("conntrack")
	if ct == nil {
		return cc
	}

	cc.Modules = ct.GetSlice("module")
	cc.TableSize = extractPositiveInt(ct, "table-size")
	cc.HashSize = extractPositiveInt(ct, "hash-size")
	cc.ExpectMax = extractPositiveInt(ct, "expect-max")

	if _, ok := ct.Get("accounting"); ok {
		cc.Accounting = true
	}
	if _, ok := ct.Get("timestamp"); ok {
		cc.Timestamp = true
	}
	if _, ok := ct.Get("checksum"); ok {
		cc.Checksum = true
	}
	if v, ok := ct.Get("log-invalid"); ok {
		cc.LogInvalid = v
	}

	if timeout := ct.GetContainer("timeout"); timeout != nil {
		cc.TimeoutGeneric = extractPositiveInt(timeout, "generic")
		cc.TimeoutTCP = extractTCPTimeouts(timeout)
		cc.TimeoutUDP = extractUDPTimeouts(timeout)
		cc.TimeoutICMP = extractICMPTimeouts(timeout, "icmp")
		cc.TimeoutICMPv6 = extractICMPTimeouts(timeout, "icmpv6")
		cc.TimeoutGRE = extractGRETimeouts(timeout)
		cc.TimeoutSCTP = extractSCTPTimeouts(timeout)
		cc.TimeoutDCCP = extractDCCPTimeouts(timeout)
	}

	if tcp := ct.GetContainer("tcp"); tcp != nil {
		cc.TCPBehavior = extractTCPBehaviorConfig(tcp)
	}

	return cc
}

// ExtractConntrackFromMap extracts conntrack config from a map[string]any
// tree (used by the reload path which has no *config.Tree).
func ExtractConntrackFromMap(tree map[string]any) ConntrackConfig {
	var cc ConntrackConfig

	sys, _ := tree["system"].(map[string]any)
	if sys == nil {
		return cc
	}
	ct, _ := sys["conntrack"].(map[string]any)
	if ct == nil {
		return cc
	}

	cc.Modules = extractStringSliceFromMap(ct, "module")
	cc.TableSize = extractPositiveIntFromMap(ct, "table-size")
	cc.HashSize = extractPositiveIntFromMap(ct, "hash-size")
	cc.ExpectMax = extractPositiveIntFromMap(ct, "expect-max")

	if _, ok := ct["accounting"]; ok {
		cc.Accounting = true
	}
	if _, ok := ct["timestamp"]; ok {
		cc.Timestamp = true
	}
	if _, ok := ct["checksum"]; ok {
		cc.Checksum = true
	}
	if v, ok := ct["log-invalid"].(string); ok {
		cc.LogInvalid = v
	}

	if timeout, _ := ct["timeout"].(map[string]any); timeout != nil {
		cc.TimeoutGeneric = extractPositiveIntFromMap(timeout, "generic")
		cc.TimeoutTCP = extractTCPTimeoutsFromMap(timeout)
		cc.TimeoutUDP = extractUDPTimeoutsFromMap(timeout)
		cc.TimeoutICMP = extractICMPTimeoutsFromMap(timeout, "icmp")
		cc.TimeoutICMPv6 = extractICMPTimeoutsFromMap(timeout, "icmpv6")
		cc.TimeoutGRE = extractGRETimeoutsFromMap(timeout)
		cc.TimeoutSCTP = extractSCTPTimeoutsFromMap(timeout)
		cc.TimeoutDCCP = extractDCCPTimeoutsFromMap(timeout)
	}

	if tcp, _ := ct["tcp"].(map[string]any); tcp != nil {
		cc.TCPBehavior = extractTCPBehaviorFromMap(tcp)
	}

	return cc
}

func extractTCPTimeouts(timeout *config.Tree) TCPTimeouts {
	tcp := timeout.GetContainer("tcp")
	if tcp == nil {
		return TCPTimeouts{}
	}
	return TCPTimeouts{
		Established:    extractPositiveInt(tcp, "established"),
		SynSent:        extractPositiveInt(tcp, "syn-sent"),
		SynRecv:        extractPositiveInt(tcp, "syn-recv"),
		FinWait:        extractPositiveInt(tcp, "fin-wait"),
		CloseWait:      extractPositiveInt(tcp, "close-wait"),
		LastAck:        extractPositiveInt(tcp, "last-ack"),
		TimeWait:       extractPositiveInt(tcp, "time-wait"),
		Close:          extractPositiveInt(tcp, "close"),
		Unacknowledged: extractPositiveInt(tcp, "unacknowledged"),
		MaxRetrans:     extractPositiveInt(tcp, "max-retrans"),
	}
}

func extractTCPBehaviorConfig(tcp *config.Tree) TCPBehavior {
	var b TCPBehavior
	if v, ok := tcp.Get("be-liberal"); ok {
		val := v == boolTrue || v == "1"
		b.BeLiberal = &val
	}
	if v, ok := tcp.Get("loose"); ok {
		val := v == boolTrue || v == "1"
		b.Loose = &val
	}
	b.MaxRetrans = extractPositiveInt(tcp, "max-retrans")
	if v, ok := tcp.Get("ignore-invalid-rst"); ok {
		val := v == boolTrue || v == "1"
		b.IgnoreInvalidRST = &val
	}
	return b
}

func extractUDPTimeouts(timeout *config.Tree) UDPTimeouts {
	udp := timeout.GetContainer("udp")
	if udp == nil {
		return UDPTimeouts{}
	}
	return UDPTimeouts{
		Timeout: extractPositiveInt(udp, "timeout"),
		Stream:  extractPositiveInt(udp, "stream"),
	}
}

func extractICMPTimeouts(timeout *config.Tree, name string) ICMPTimeouts {
	icmp := timeout.GetContainer(name)
	if icmp == nil {
		return ICMPTimeouts{}
	}
	return ICMPTimeouts{
		Timeout: extractPositiveInt(icmp, "timeout"),
	}
}

func extractGRETimeouts(timeout *config.Tree) GRETimeouts {
	gre := timeout.GetContainer("gre")
	if gre == nil {
		return GRETimeouts{}
	}
	return GRETimeouts{
		Timeout: extractPositiveInt(gre, "timeout"),
		Stream:  extractPositiveInt(gre, "stream"),
	}
}

func extractSCTPTimeouts(timeout *config.Tree) SCTPTimeouts {
	sctp := timeout.GetContainer("sctp")
	if sctp == nil {
		return SCTPTimeouts{}
	}
	return SCTPTimeouts{
		Closed:          extractPositiveInt(sctp, "closed"),
		CookieWait:      extractPositiveInt(sctp, "cookie-wait"),
		CookieEchoed:    extractPositiveInt(sctp, "cookie-echoed"),
		Established:     extractPositiveInt(sctp, "established"),
		ShutdownSent:    extractPositiveInt(sctp, "shutdown-sent"),
		ShutdownRecd:    extractPositiveInt(sctp, "shutdown-recd"),
		ShutdownAckSent: extractPositiveInt(sctp, "shutdown-ack-sent"),
		HeartbeatSent:   extractPositiveInt(sctp, "heartbeat-sent"),
	}
}

func extractDCCPTimeouts(timeout *config.Tree) DCCPTimeouts {
	dccp := timeout.GetContainer("dccp")
	if dccp == nil {
		return DCCPTimeouts{}
	}
	return DCCPTimeouts{
		Request:  extractPositiveInt(dccp, "request"),
		Respond:  extractPositiveInt(dccp, "respond"),
		Partopen: extractPositiveInt(dccp, "partopen"),
		Open:     extractPositiveInt(dccp, "open"),
		Closereq: extractPositiveInt(dccp, "closereq"),
		Closing:  extractPositiveInt(dccp, "closing"),
		Timewait: extractPositiveInt(dccp, "timewait"),
	}
}

func extractPositiveInt(t *config.Tree, key string) int {
	v, ok := t.Get(key)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Map-based extractors for reload path.

func extractTCPTimeoutsFromMap(timeout map[string]any) TCPTimeouts {
	tcp, _ := timeout["tcp"].(map[string]any)
	if tcp == nil {
		return TCPTimeouts{}
	}
	return TCPTimeouts{
		Established:    extractPositiveIntFromMap(tcp, "established"),
		SynSent:        extractPositiveIntFromMap(tcp, "syn-sent"),
		SynRecv:        extractPositiveIntFromMap(tcp, "syn-recv"),
		FinWait:        extractPositiveIntFromMap(tcp, "fin-wait"),
		CloseWait:      extractPositiveIntFromMap(tcp, "close-wait"),
		LastAck:        extractPositiveIntFromMap(tcp, "last-ack"),
		TimeWait:       extractPositiveIntFromMap(tcp, "time-wait"),
		Close:          extractPositiveIntFromMap(tcp, "close"),
		Unacknowledged: extractPositiveIntFromMap(tcp, "unacknowledged"),
		MaxRetrans:     extractPositiveIntFromMap(tcp, "max-retrans"),
	}
}

func extractTCPBehaviorFromMap(tcp map[string]any) TCPBehavior {
	var b TCPBehavior
	if v, ok := tcp["be-liberal"].(string); ok {
		val := v == boolTrue || v == "1"
		b.BeLiberal = &val
	}
	if v, ok := tcp["loose"].(string); ok {
		val := v == boolTrue || v == "1"
		b.Loose = &val
	}
	b.MaxRetrans = extractPositiveIntFromMap(tcp, "max-retrans")
	if v, ok := tcp["ignore-invalid-rst"].(string); ok {
		val := v == boolTrue || v == "1"
		b.IgnoreInvalidRST = &val
	}
	return b
}

func extractUDPTimeoutsFromMap(timeout map[string]any) UDPTimeouts {
	udp, _ := timeout["udp"].(map[string]any)
	if udp == nil {
		return UDPTimeouts{}
	}
	return UDPTimeouts{
		Timeout: extractPositiveIntFromMap(udp, "timeout"),
		Stream:  extractPositiveIntFromMap(udp, "stream"),
	}
}

func extractICMPTimeoutsFromMap(timeout map[string]any, name string) ICMPTimeouts {
	icmp, _ := timeout[name].(map[string]any)
	if icmp == nil {
		return ICMPTimeouts{}
	}
	return ICMPTimeouts{
		Timeout: extractPositiveIntFromMap(icmp, "timeout"),
	}
}

func extractGRETimeoutsFromMap(timeout map[string]any) GRETimeouts {
	gre, _ := timeout["gre"].(map[string]any)
	if gre == nil {
		return GRETimeouts{}
	}
	return GRETimeouts{
		Timeout: extractPositiveIntFromMap(gre, "timeout"),
		Stream:  extractPositiveIntFromMap(gre, "stream"),
	}
}

func extractSCTPTimeoutsFromMap(timeout map[string]any) SCTPTimeouts {
	sctp, _ := timeout["sctp"].(map[string]any)
	if sctp == nil {
		return SCTPTimeouts{}
	}
	return SCTPTimeouts{
		Closed:          extractPositiveIntFromMap(sctp, "closed"),
		CookieWait:      extractPositiveIntFromMap(sctp, "cookie-wait"),
		CookieEchoed:    extractPositiveIntFromMap(sctp, "cookie-echoed"),
		Established:     extractPositiveIntFromMap(sctp, "established"),
		ShutdownSent:    extractPositiveIntFromMap(sctp, "shutdown-sent"),
		ShutdownRecd:    extractPositiveIntFromMap(sctp, "shutdown-recd"),
		ShutdownAckSent: extractPositiveIntFromMap(sctp, "shutdown-ack-sent"),
		HeartbeatSent:   extractPositiveIntFromMap(sctp, "heartbeat-sent"),
	}
}

func extractDCCPTimeoutsFromMap(timeout map[string]any) DCCPTimeouts {
	dccp, _ := timeout["dccp"].(map[string]any)
	if dccp == nil {
		return DCCPTimeouts{}
	}
	return DCCPTimeouts{
		Request:  extractPositiveIntFromMap(dccp, "request"),
		Respond:  extractPositiveIntFromMap(dccp, "respond"),
		Partopen: extractPositiveIntFromMap(dccp, "partopen"),
		Open:     extractPositiveIntFromMap(dccp, "open"),
		Closereq: extractPositiveIntFromMap(dccp, "closereq"),
		Closing:  extractPositiveIntFromMap(dccp, "closing"),
		Timewait: extractPositiveIntFromMap(dccp, "timewait"),
	}
}

func extractPositiveIntFromMap(m map[string]any, key string) int {
	v, ok := m[key].(string)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func extractStringSliceFromMap(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	case string:
		return []string{v}
	}
	return nil
}
