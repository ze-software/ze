// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server the probe exchanges with
// Related: frame.go -- the request an exchange step becomes and the reply it reads
// Related: probe_linux.go -- Run, which performs the steps against the daemon

package dhcpprobe

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// StepKind is what one probe argument asks for.
type StepKind uint8

// StepKind values. Zero is not a step, so an unparsed step is refused.
const (
	StepUnspecified StepKind = iota
	// StepAddrAdd and StepAddrDel add or remove an IPv4 address on the
	// server's end of the link. The server reads its addresses for each
	// request, so the next exchange sees the change with no reload.
	StepAddrAdd
	StepAddrDel
	// StepRoute routes a prefix through the server's end, so a reply to a
	// relay address (giaddr) leaves on the link.
	StepRoute
	// StepNeigh writes a permanent neighbour entry for an address on the
	// server's end, naming the probe's end, so a reply unicast to that
	// address (giaddr, ciaddr) reaches the probe without ARP.
	StepNeigh
	// StepReload replaces ze-bgp.conf with config2.conf and sends SIGHUP.
	StepReload
	// StepSentinel names the MAC of the client whose DHCPDISCOVER closes the
	// window in which an expected silence is observed.
	StepSentinel
	// StepExchange sends one request and checks the reply, or its absence.
	StepExchange
)

// Step is one parsed probe argument. Only the fields of its Kind are set.
type Step struct {
	Kind     StepKind
	Prefix   netip.Prefix
	Addr     netip.Addr
	MAC      net.HardwareAddr
	Exchange Exchange
}

// Exchange is one request and what its reply must carry. A Want field that is
// not valid is not checked.
type Exchange struct {
	Label   string
	Request Request
	// Silent is true when the server must not answer (expect=none).
	Silent bool
	Want   Reply
	// Attempts is how many times the request is sent until the reply has the
	// expected type: a client retransmits, and after a reload the first
	// answers can still come from the replaced config.
	Attempts int
}

// exchangeAttempts is the default retransmission count of an exchange. The
// first exchange of a test also covers the time the daemon takes to bind its
// listener after it reports ready.
const exchangeAttempts = 10

// ParseStep reads one probe argument: a verb, then comma-separated values.
// MAC addresses are written with hyphens (02-00-00-00-00-01), because a .ci
// refuses a cmd= line carrying a `:<word>=` span.
func ParseStep(arg string) (Step, error) {
	fields := strings.Split(arg, ",")
	switch fields[0] {
	case "addr-add", "addr-del", "route":
		return parsePrefixStep(fields)
	case "neigh":
		if len(fields) != 2 {
			return Step{}, errors.New("dhcpprobe: neigh takes one address: neigh,<ip>")
		}
		addr, err := netip.ParseAddr(fields[1])
		if err != nil {
			return Step{}, errors.New("dhcpprobe: neigh: " + err.Error())
		}
		return Step{Kind: StepNeigh, Addr: addr}, nil
	case "reload":
		if len(fields) != 1 {
			return Step{}, errors.New("dhcpprobe: reload takes no value")
		}
		return Step{Kind: StepReload}, nil
	case "sentinel":
		if len(fields) != 2 {
			return Step{}, errors.New("dhcpprobe: sentinel takes one MAC: sentinel,<mac>")
		}
		mac, err := net.ParseMAC(fields[1])
		if err != nil {
			return Step{}, errors.New("dhcpprobe: sentinel: " + err.Error())
		}
		return Step{Kind: StepSentinel, MAC: mac}, nil
	case "x":
		return parseExchange(fields)
	}
	return Step{}, errors.New("dhcpprobe: unknown step " + strconv.Quote(arg) +
		" (want addr-add, addr-del, route, neigh, reload, sentinel or x)")
}

func parsePrefixStep(fields []string) (Step, error) {
	if len(fields) != 2 {
		return Step{}, errors.New("dhcpprobe: " + fields[0] + " takes one prefix: " + fields[0] + ",<cidr>")
	}
	prefix, err := netip.ParsePrefix(fields[1])
	if err != nil {
		return Step{}, errors.New("dhcpprobe: " + fields[0] + ": " + err.Error())
	}
	kind := StepRoute
	switch fields[0] {
	case "addr-add":
		kind = StepAddrAdd
	case "addr-del":
		kind = StepAddrDel
	}
	if kind == StepRoute {
		prefix = prefix.Masked()
	}
	return Step{Kind: kind, Prefix: prefix}, nil
}

// parseExchange reads x,<label>,key=value,... . type, mac and expect are
// required; every other key is optional.
func parseExchange(fields []string) (Step, error) {
	if len(fields) < 2 {
		return Step{}, errors.New("dhcpprobe: an exchange needs a label: x,<label>,type=...,mac=...,expect=...")
	}
	ex := Exchange{Label: fields[1], Attempts: exchangeAttempts}
	broadcastSet := false
	expectSet := false
	for _, field := range fields[2:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return Step{}, errors.New("dhcpprobe: exchange " + ex.Label + ": " + strconv.Quote(field) + " is not key=value")
		}
		var err error
		switch key {
		case "type":
			ex.Request.Type, err = parseMsgType(value)
		case "mac":
			ex.Request.MAC, err = net.ParseMAC(value)
		case "ciaddr":
			ex.Request.CIAddr, err = netip.ParseAddr(value)
		case "giaddr":
			ex.Request.GIAddr, err = netip.ParseAddr(value)
		case "requested":
			ex.Request.Requested, err = netip.ParseAddr(value)
		case "server-id":
			ex.Request.ServerID, err = netip.ParseAddr(value)
		case "broadcast":
			ex.Request.Broadcast, err = strconv.ParseBool(value)
			broadcastSet = true
		case "attempts":
			ex.Attempts, err = strconv.Atoi(value)
			if err == nil && ex.Attempts < 1 {
				err = errors.New("must be at least 1")
			}
		case "expect":
			expectSet = true
			if value == "none" {
				ex.Silent = true
				break
			}
			ex.Want.Type, err = parseMsgType(value)
		default:
			err = parseWant(&ex.Want, key, value)
		}
		if err != nil {
			return Step{}, errors.New("dhcpprobe: exchange " + ex.Label + ", " + key + ": " + err.Error())
		}
	}
	if ex.Request.Type == MsgUnspecified {
		return Step{}, errors.New("dhcpprobe: exchange " + ex.Label + " names no type=")
	}
	if ex.Request.MAC == nil {
		return Step{}, errors.New("dhcpprobe: exchange " + ex.Label + " names no mac=")
	}
	if !expectSet {
		return Step{}, errors.New("dhcpprobe: exchange " + ex.Label + " names no expect= (a reply type, or none)")
	}
	// A client that holds no address cannot receive a unicast to an address
	// it does not have yet, so it asks for a broadcast; one with ciaddr set
	// receives at ciaddr (RFC 2131 Section 4.1).
	if !broadcastSet {
		ex.Request.Broadcast = !ex.Request.CIAddr.IsValid()
	}
	ex.Request.XID = xidFor(ex.Label)
	return Step{Kind: StepExchange, Exchange: ex}, nil
}

// parseWant reads one expected reply field.
func parseWant(want *Reply, key, value string) error {
	if key == "to" {
		dest, err := netip.ParseAddrPort(value)
		want.Dest = dest
		return err
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return err
	}
	switch key {
	case "yiaddr":
		want.YIAddr = addr
	case "sid":
		want.ServerID = addr
	case "router":
		want.Router = addr
	case "mask":
		want.Mask = addr
	case "dns":
		want.DNS = addr
	default:
		return errors.New("unknown key (want type, mac, ciaddr, giaddr, requested, server-id, broadcast, " +
			"attempts, expect, yiaddr, sid, router, mask, dns or to)")
	}
	return nil
}

// xidFor answers a transaction ID derived from the label, so each exchange of
// a test has its own and a rerun sends the same one.
func xidFor(label string) uint32 {
	var h uint32 = 2166136261
	for i := range len(label) {
		h ^= uint32(label[i])
		h *= 16777619
	}
	return h
}

// Check answers an error naming every field of got that differs from what
// the exchange wants.
func (e *Exchange) Check(got Reply) error {
	var errs []error
	if got.Type != e.Want.Type {
		errs = append(errs, errors.New("reply "+got.Type.String()+", want "+e.Want.Type.String()))
	}
	checks := []struct {
		name      string
		got, want netip.Addr
	}{
		{"yiaddr", got.YIAddr, e.Want.YIAddr},
		{"server identifier (option 54)", got.ServerID, e.Want.ServerID},
		{"router (option 3)", got.Router, e.Want.Router},
		{"mask (option 1)", got.Mask, e.Want.Mask},
		{"dns (option 6)", got.DNS, e.Want.DNS},
	}
	for _, c := range checks {
		if !c.want.IsValid() {
			continue
		}
		if c.got != c.want {
			errs = append(errs, errors.New(c.name+" "+addrText(c.got)+", want "+c.want.String()))
		}
	}
	if e.Want.Dest.IsValid() && got.Dest != e.Want.Dest {
		errs = append(errs, errors.New("sent to "+got.Dest.String()+", want "+e.Want.Dest.String()))
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New("dhcpprobe: exchange " + e.Label + ": " + errors.Join(errs...).Error())
}

// Line answers the line the probe prints for an exchange's reply, which a
// .ci matches with expect=stdout:contains=.
func (e *Exchange) Line(got Reply) string {
	var b strings.Builder
	b.WriteString("dhcp ")
	b.WriteString(e.Label)
	b.WriteString(" sent=")
	b.WriteString(e.Request.Type.String())
	b.WriteString(" reply=")
	b.WriteString(got.Type.String())
	b.WriteString(" yiaddr=")
	b.WriteString(addrText(got.YIAddr))
	b.WriteString(" sid=")
	b.WriteString(addrText(got.ServerID))
	b.WriteString(" router=")
	b.WriteString(addrText(got.Router))
	b.WriteString(" mask=")
	b.WriteString(addrText(got.Mask))
	b.WriteString(" dns=")
	b.WriteString(addrText(got.DNS))
	b.WriteString(" to=")
	b.WriteString(got.Dest.String())
	return b.String()
}

// SilentLine answers the line the probe prints when an expected silence held.
func (e *Exchange) SilentLine() string {
	return "dhcp " + e.Label + " sent=" + e.Request.Type.String() + " reply=none"
}

func addrText(addr netip.Addr) string {
	if !addr.IsValid() {
		return "absent"
	}
	return addr.String()
}
