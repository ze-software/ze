// Design: docs/architecture/diagnostics/path-mtu.md -- the local state the payload reports
// Related: internal/core/probe/errqueue_linux.go -- KernelPathMTU, the kernel's cached path MTU this file classifies
// Related: internal/component/sysctl/backend.go -- Read, the exported sysctl read

package cmd

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/prometheus/procfs"

	"github.com/ze-software/ze/internal/component/sysctl"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// stateNote is one finding about the box itself, outside the tunnel table.
type stateNote struct {
	severity noteSeverity
	text     string
}

// procMountPoint is where the fragmentation counters are read from on the
// running box. A test roots the read elsewhere.
const procMountPoint = procfs.DefaultMountPoint

// fragmentationCounters are the kernel's fragmentation counters as ABSOLUTE
// values since boot (AC-20). The telemetry collector reads the same vendored
// parse as rates; a rate hides a blackhole that happened an hour ago, and
// this diagnostic wants the count.
type fragmentationCounters struct {
	// ipFragOKs and ip6FragOKs: datagrams this box split because they did
	// not fit the path it sent them on.
	ipFragOKs  uint64
	ip6FragOKs uint64
	// ipFragCreates: how many fragments those splits produced.
	ipFragCreates uint64
	// ipFragFails: datagrams dropped because they were too big to send and
	// carried Don't Fragment. A blackhole in progress.
	ipFragFails uint64
	// ipReasmReqds and ipReasmFails, and their IPv6 pair: inbound fragments
	// that had to be reassembled, and how many reassemblies failed.
	ipReasmReqds  uint64
	ipReasmFails  uint64
	ip6ReasmFails uint64
}

// errCounterAbsent is the answer when the kernel's SNMP table does not carry
// a counter this diagnostic reports. A zero would read as "never happened".
var errCounterAbsent = errors.New("mtu: the kernel reports no such counter")

// readFragmentationCounters reads the counters from the proc filesystem at
// mount. It answers an error when the table cannot be read or a counter is
// absent, never a zero in its place.
func readFragmentationCounters(mount string) (fragmentationCounters, error) {
	fs, err := procfs.NewFS(mount)
	if err != nil {
		return fragmentationCounters{}, fmt.Errorf("mtu: fragmentation counters: %w", err)
	}
	self, err := fs.Self()
	if err != nil {
		return fragmentationCounters{}, fmt.Errorf("mtu: fragmentation counters: %w", err)
	}
	snmp, err := self.Snmp()
	if err != nil {
		return fragmentationCounters{}, fmt.Errorf("mtu: fragmentation counters: %w", err)
	}
	snmp6, err := self.Snmp6()
	if err != nil {
		return fragmentationCounters{}, fmt.Errorf("mtu: fragmentation counters: %w", err)
	}
	var c fragmentationCounters
	for _, field := range []struct {
		name  string
		value *float64
		into  *uint64
	}{
		{"IpFragOKs", snmp.Ip.FragOKs, &c.ipFragOKs},
		{"IpFragCreates", snmp.Ip.FragCreates, &c.ipFragCreates},
		{"IpFragFails", snmp.Ip.FragFails, &c.ipFragFails},
		{"IpReasmReqds", snmp.Ip.ReasmReqds, &c.ipReasmReqds},
		{"IpReasmFails", snmp.Ip.ReasmFails, &c.ipReasmFails},
		{"Ip6FragOKs", snmp6.Ip6.FragOKs, &c.ip6FragOKs},
		{"Ip6ReasmFails", snmp6.Ip6.ReasmFails, &c.ip6ReasmFails},
	} {
		if field.value == nil {
			return fragmentationCounters{}, fmt.Errorf("%w: %s", errCounterAbsent, field.name)
		}
		if *field.value < 0 {
			return fragmentationCounters{}, fmt.Errorf("mtu: counter %s is negative", field.name)
		}
		*field.into = uint64(*field.value)
	}
	return c, nil
}

// fragmentedOutbound reports whether this box split a datagram of either
// family since boot.
func (c *fragmentationCounters) fragmentedOutbound() bool {
	if c.ipFragOKs > 0 {
		return true
	}
	return c.ip6FragOKs > 0
}

// reassemblyFailed reports whether an inbound reassembly of either family
// failed since boot.
func (c *fragmentationCounters) reassemblyFailed() bool {
	if c.ipReasmFails > 0 {
		return true
	}
	return c.ip6ReasmFails > 0
}

// findings turns the counters into notes with the ported tool's severities:
// outbound fragmentation is a caution, a Don't Fragment drop is a fault
// because it is a PMTU blackhole in progress, a reassembly failure is a
// caution, and all-zero counters are one information note so the reader can
// see the counters were read.
func (c *fragmentationCounters) findings() []stateNote {
	var notes []stateNote
	var b textbuf.Buffer
	if c.fragmentedOutbound() {
		b.Reset()
		b.Str("this box has fragmented outbound packets: IpFragOKs ").Uint(c.ipFragOKs)
		b.Str(", Ip6FragOKs ").Uint(c.ip6FragOKs)
		b.Str(", split into IpFragCreates ").Uint(c.ipFragCreates)
		b.Str(" fragments; something here is emitting packets too large for the path it sends them on")
		notes = append(notes, stateNote{noteSeverityCaution, b.String()})
	}
	if c.ipFragFails > 0 {
		b.Reset()
		b.Str("IpFragFails ").Uint(c.ipFragFails)
		b.Str(": packets were dropped because they were too big to send and had Don't Fragment set; that is a PMTU blackhole, not a warning")
		notes = append(notes, stateNote{noteSeverityFault, b.String()})
	}
	if c.reassemblyFailed() {
		b.Reset()
		b.Str("IpReasmFails ").Uint(c.ipReasmFails)
		b.Str(" of IpReasmReqds ").Uint(c.ipReasmReqds)
		b.Str(" inbound fragments, and Ip6ReasmFails ").Uint(c.ip6ReasmFails)
		b.Str(", could not be reassembled: the far end is fragmenting and some of it is being lost")
		notes = append(notes, stateNote{noteSeverityCaution, b.String()})
	}
	if len(notes) == 0 {
		notes = append(notes, stateNote{noteSeverityInfo, "no fragmentation counted since boot: IpFragOKs, Ip6FragOKs, IpFragFails, IpReasmFails and Ip6ReasmFails are all 0"})
	}
	return notes
}

// tcpMTUProbing is the value of net.ipv4.tcp_mtu_probing as Linux defines it.
// The zero value is Unspecified so an unread setting never passes for the
// kernel default.
type tcpMTUProbing uint8

const (
	tcpMTUProbingUnspecified tcpMTUProbing = iota
	// tcpMTUProbingDisabled is 0, the Linux default: TCP stalls rather than
	// backs off when a path blackholes.
	tcpMTUProbingDisabled
	// tcpMTUProbingOnBlackhole is 1: probing starts once a blackhole is
	// detected.
	tcpMTUProbingOnBlackhole
	// tcpMTUProbingAlways is 2: every connection probes.
	tcpMTUProbingAlways
)

// String answers the wire spelling of the setting. It is written into the
// payload and never compared.
func (p tcpMTUProbing) String() string {
	switch p {
	case tcpMTUProbingDisabled:
		return "disabled"
	case tcpMTUProbingOnBlackhole:
		return "on-blackhole"
	case tcpMTUProbingAlways:
		return "always"
	default:
		panic("BUG: tcpMTUProbing written to the payload before it was set")
	}
}

// sysctlTCPMTUProbing is the key the sysctl component maps to
// /proc/sys/net/ipv4/tcp_mtu_probing.
const sysctlTCPMTUProbing = "net.ipv4.tcp_mtu_probing"

// readTCPMTUProbing reads the setting through the sysctl component's one
// exported read and parses it into the enum. A value outside 0..2, or a
// platform with no sysctl backend, answers an error rather than a default.
func readTCPMTUProbing() (tcpMTUProbing, error) {
	raw, err := sysctl.Read(sysctlTCPMTUProbing)
	if err != nil {
		return tcpMTUProbingUnspecified, err
	}
	return parseTCPMTUProbing(raw)
}

func parseTCPMTUProbing(raw string) (tcpMTUProbing, error) {
	value, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return tcpMTUProbingUnspecified, fmt.Errorf("mtu: %s holds %q, not a number: %w", sysctlTCPMTUProbing, raw, err)
	}
	switch value {
	case 0:
		return tcpMTUProbingDisabled, nil
	case 1:
		return tcpMTUProbingOnBlackhole, nil
	case 2:
		return tcpMTUProbingAlways, nil
	default:
		return tcpMTUProbingUnspecified, fmt.Errorf("mtu: %s holds %d, outside the 0..2 the kernel defines", sysctlTCPMTUProbing, value)
	}
}

// finding is the note the setting earns. Only the disabled value earns one:
// 0 is the Linux default, a standing property of the platform rather than a
// fault, and it is information because a blackhole is not something this run
// measures. The other two values earn nothing, and the bool says so.
func (p tcpMTUProbing) finding() (stateNote, bool) {
	switch p {
	case tcpMTUProbingDisabled:
		return stateNote{noteSeverityInfo, "tcp_mtu_probing is 0, the Linux default: TCP stalls rather than backs off if a path blackholes"}, true
	case tcpMTUProbingOnBlackhole, tcpMTUProbingAlways:
		return stateNote{}, false
	default:
		panic("BUG: finding of an unread tcpMTUProbing")
	}
}

// sysctlRouteMTUExpires is the key holding how long a cached path MTU lives,
// in seconds. The cache note names it when it could be read.
const sysctlRouteMTUExpires = "net.ipv4.route.mtu_expires"

// readRouteMTUExpires reads how many seconds the kernel keeps a cached path
// MTU. A platform with no sysctl backend, or a value that is not a number,
// answers an error; the cache note then names no duration.
func readRouteMTUExpires() (uint32, error) {
	raw, err := sysctl.Read(sysctlRouteMTUExpires)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("mtu: %s holds %q, not a number: %w", sysctlRouteMTUExpires, raw, err)
	}
	return uint32(value), nil
}

// pathMTUCache is what the kernel believed about one destination before the
// run probed it: the estimate probe.KernelPathMTU answered, or none when it
// answered ErrPathMTUUnknown.
type pathMTUCache struct {
	cached    uint32
	hasCached bool
}

// pathMTUCacheFinding is the cache note of AC-11. A cached value that
// differs from the measurement is a caution naming how long it caps traffic;
// one that agrees is information, because some probes may have been answered
// from it, unless the run was exhaustive, when the wire was measured regardless
// and nothing is owed. No cached value earns no note. expires is the value of
// sysctlRouteMTUExpires in seconds when it could be read, and 0 when it could
// not, in which case the note names no duration rather than a default.
func pathMTUCacheFinding(target netip.Addr, cache *pathMTUCache, measured uint16, exhaustive bool, expires uint32) (stateNote, bool) {
	if !cache.hasCached {
		return stateNote{}, false
	}
	var b textbuf.Buffer
	if uint32(measured) != cache.cached {
		b.Addr(target).Str(" had a stale PMTU of ").Uint32(cache.cached)
		b.Str(" cached before this run; the wire measures ").Uint16(measured)
		b.Str(". The cached value caps traffic to that destination until it expires")
		if expires > 0 {
			b.Str(" (").Uint32(expires).Str("s)")
		}
		b.Str(" or is flushed")
		return stateNote{noteSeverityCaution, b.String()}, true
	}
	if exhaustive {
		return stateNote{}, false
	}
	b.Addr(target).Str(" had ").Uint32(cache.cached)
	b.Str(" cached before this run, agreeing with the measurement; some probes may have been answered from it. Re-run with exhaustive to measure the wire instead")
	return stateNote{noteSeverityInfo, b.String()}, true
}
