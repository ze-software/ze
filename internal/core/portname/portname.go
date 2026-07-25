// Design: plan/learned/1019-traffic-usage-monitor.md -- port-to-service-name lookup

package portname

import "github.com/ze-software/ze/internal/core/textbuf"

// Info holds the resolved service name for a port and, when applicable,
// an amplification-vector label for known reflection ports.
type Info struct {
	Name          string
	Amplification string
}

var amplificationPorts = map[uint16]bool{
	19:    true, // chargen
	53:    true, // dns
	123:   true, // ntp
	161:   true, // snmp
	389:   true, // ldap
	1900:  true, // ssdp
	11211: true, // memcached
}

// Lookup returns the service name for a (port, protocol) pair.
// Proto uses IANA IP protocol numbers (6=TCP, 17=UDP). When proto is 0
// or the exact (port, proto) pair is not in the table, a port-only
// fallback is tried. Unknown ports return the numeric string.
func Lookup(port uint16, proto uint8) Info {
	name := resolve(port, proto)
	var amp string
	if amplificationPorts[port] {
		var tb textbuf.Buffer
		amp = tb.Str(name).Str("-amplification").String()
	}
	return Info{
		Name:          name,
		Amplification: amp,
	}
}

func resolve(port uint16, proto uint8) string {
	if proto != 0 {
		if name, ok := services[serviceKey{port, proto}]; ok {
			return name
		}
	}
	if name, ok := serviceFallback[port]; ok {
		return name
	}
	return textbuf.StringUint16(port)
}
