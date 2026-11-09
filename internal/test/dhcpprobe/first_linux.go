// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server the probe exchanges with
// Related: probe_linux.go -- the sockets, write and read this sends and reads with
// Related: step.go -- Exchange, whose Check judges the reply

//go:build linux

package dhcpprobe

import (
	"errors"
	"strconv"
	"time"
)

// First judges the first reply the server gives ex's request, and no later
// one. It sends the request every interval until a reply carrying its XID
// arrives or window passes, so a request reaches a listener within interval
// of the moment it binds. Where exchange resends replyWait apart and waits
// for a reply of the wanted type, First takes whatever the server answers
// first: it is the probe for a server that is starting, whose first answers
// are the ones a startup hold exists to correct. A late reply to an earlier
// transmission answers too, because every transmission is the same request.
func (p *Probe) First(ex *Exchange, interval, window time.Duration, emit func(string)) error {
	if interval <= 0 {
		return errors.New("BUG: dhcpprobe: First needs a positive interval")
	}
	if err := p.drain(); err != nil {
		return err
	}
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if err := p.write(&ex.Request); err != nil {
			return err
		}
		reply, ok, err := p.read(ex.Request.XID, interval)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		emit(ex.Line(reply))
		return ex.Check(reply)
	}
	return errors.New("dhcpprobe: exchange " + ex.Label + ": no reply to a " + ex.Request.Type.String() +
		" sent every " + interval.String() + " for " + window.String() +
		" (" + strconv.Itoa(int(window/interval)) + " requests at most)")
}
