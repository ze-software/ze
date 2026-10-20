// Design: docs/architecture/testing/interop.md -- DHCPv6 Confirm evidence read from Kea's log and ze's eth0.
// Related: check_dhcpv6_confirm.go -- the lab I/O that feeds these readings.
// Related: check_dhcpv6_predicate.go -- the Kea log fields and the eth0 lease reading shared with the renewal scenario.
package bgp

import (
	"errors"
	"strings"
)

// Beyond the INFO lines check_dhcpv6_predicate.go reads, the Confirm scenario
// reads two more. The kea-dhcp6.packets logger at debug level 55 logs every
// packet Kea sends, its options on the lines after it, and the status code
// option among them (measured against Kea 2.6.2 on 2026-10-20):
//
//	... DHCP6_RESPONSE_DATA duid=[00:03:..], [no hwaddr info], tid=0xa8ebac: responding with packet REPLY (type 7), packet details: localAddr=[..]:547 ...
//	  type=00013, len=00039: NotOnLink(4) "Address fd00:1e:1::50 is not on link."
//
// and to a Confirm whose addresses fit the link
//
//	type=00013, len=00027: Success(0) "All addresses are on-link"
//
// A SIGHUP that reloads the configuration logs
//
//	... DHCP6_DYNAMIC_RECONFIGURATION_SUCCESS dynamic server reconfiguration succeeded with file: /run/kea/kea-dhcp6.conf
const (
	keaResponse     = "DHCP6_RESPONSE_DATA "
	keaRespondingTo = "responding with packet "
	keaStatusOption = "type=00013"
	keaReloaded     = "DHCP6_DYNAMIC_RECONFIGURATION_SUCCESS "
	keaReloadFailed = "DHCP6_DYNAMIC_RECONFIGURATION_FAIL "

	keaConfirm = "CONFIRM"
	keaReply   = "REPLY"

	keaStatusSuccess   = "Success"
	keaStatusNotOnLink = "NotOnLink"
)

type keaEventKind uint8

const (
	keaEventUnspecified keaEventKind = iota
	// keaEventReceived is a packet Kea received: client, tid, message.
	keaEventReceived
	// keaEventGranted is a lease Kea granted or extended: tid, address.
	keaEventGranted
	// keaEventAnswered is a packet Kea sent: tid, message, and status when
	// the packet carried a status code option.
	keaEventAnswered
	// keaEventReloaded is a configuration reload that succeeded.
	keaEventReloaded
)

type keaEvent struct {
	kind    keaEventKind
	client  string
	tid     string
	message string
	address string
	status  string
}

// keaTimeline reads a Kea log into its events, in the order Kea logged them.
// A line that does not open with a digit continues the entry before it, the
// way Kea writes a packet's options under its DHCP6_RESPONSE_DATA line.
func keaTimeline(log string) []keaEvent {
	var events []keaEvent
	answer := -1 // the index of the response whose options are being read
	for line := range strings.Lines(log) {
		if !keaEntryStart(line) {
			if answer >= 0 && strings.Contains(line, keaStatusOption) {
				events[answer].status = keaStatusName(line)
			}
			continue
		}
		answer = -1
		if strings.Contains(line, keaReloaded) {
			events = append(events, keaEvent{kind: keaEventReloaded})
			continue
		}
		if rest, found := cutAfter(line, keaReceived); found {
			client, tid, message, ok := keaClientFields(rest)
			if ok {
				kind, _, _ := strings.Cut(message, " ")
				events = append(events, keaEvent{kind: keaEventReceived, client: client, tid: tid, message: kind})
			}
			continue
		}
		if rest, found := cutAfter(line, keaResponse); found {
			client, tid, message, ok := keaClientFields(rest)
			if !ok {
				continue
			}
			sent, isResponse := strings.CutPrefix(message, keaRespondingTo)
			if !isResponse {
				continue
			}
			kind, _, _ := strings.Cut(sent, " ")
			events = append(events, keaEvent{kind: keaEventAnswered, client: client, tid: tid, message: kind})
			answer = len(events) - 1
			if strings.Contains(sent, keaStatusOption) {
				events[answer].status = keaStatusName(sent)
			}
			continue
		}
		rest, found := cutAfter(line, keaAlloc)
		if !found {
			rest, found = cutAfter(line, keaRenew)
		}
		if !found {
			continue
		}
		client, tid, message, ok := keaClientFields(rest)
		if !ok {
			continue
		}
		leased, isLease := strings.CutPrefix(message, keaLeaseFor)
		if !isLease {
			continue
		}
		address, _, _ := strings.Cut(leased, " ")
		events = append(events, keaEvent{kind: keaEventGranted, client: client, tid: tid, address: address})
	}
	return events
}

// keaEntryStart reports whether line opens a log entry: Kea starts each one
// with its date.
func keaEntryStart(line string) bool {
	return line != "" && line[0] >= '0' && line[0] <= '9'
}

// keaStatusName reads the status name from Kea's rendering of a status code
// option, "type=00013, len=00025: NotOnLink(4) ...": the word before the
// numeric code. An option it cannot read yields an empty name, which no
// verdict accepts.
func keaStatusName(text string) string {
	_, option, _ := strings.Cut(text, keaStatusOption)
	_, value, found := strings.Cut(option, ": ")
	if !found {
		return ""
	}
	name, _, found := strings.Cut(value, "(")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}

// keaPhase is what Kea logged between two reloads: the exchanges by message
// type, the order in which each transaction first arrived, the leases granted
// and the status of each Reply.
type keaPhase struct {
	exchanges map[string]map[string]bool
	// first holds the position of each tid's first packet in the phase.
	first   map[string]int
	granted map[string]map[string]bool // address -> tids
	replies map[string]string          // tid -> status of Kea's Reply
	clients map[string]bool
}

// keaPhases splits a Kea timeline at each successful reload.
func keaPhases(events []keaEvent) []*keaPhase {
	current := newKeaPhase()
	phases := []*keaPhase{current}
	for position, event := range events {
		switch event.kind {
		case keaEventReloaded:
			current = newKeaPhase()
			phases = append(phases, current)
		case keaEventReceived:
			current.clients[event.client] = true
			addTid(current.exchanges, event.message, event.tid)
			if _, seen := current.first[event.tid]; !seen {
				current.first[event.tid] = position
			}
		case keaEventGranted:
			addTid(current.granted, event.address, event.tid)
		case keaEventAnswered:
			if event.message == keaReply {
				current.replies[event.tid] = event.status
			}
		case keaEventUnspecified:
			panic("BUG: keaTimeline produced an event of no kind")
		}
	}
	return phases
}

func newKeaPhase() *keaPhase {
	return &keaPhase{
		exchanges: map[string]map[string]bool{}, first: map[string]int{},
		granted: map[string]map[string]bool{}, replies: map[string]string{}, clients: map[string]bool{},
	}
}

// only returns the one transaction of kind, or an error naming how many ran.
func (p *keaPhase) only(kind, when string) (string, error) {
	tids := p.exchanges[kind]
	if len(tids) != 1 {
		return "", kea6CountError("ze ran", len(tids), kind+" exchanges "+when+", want 1")
	}
	for tid := range tids {
		return tid, nil
	}
	return "", errors.New("unreachable: one tid counted and none read")
}

// none fails when any transaction of kind ran.
func (p *keaPhase) none(kind, when string) error {
	if n := len(p.exchanges[kind]); n != 0 {
		return kea6CountError("ze ran", n, kind+" exchanges "+when+", want 0")
	}
	return nil
}

// confirmAnswered returns the one CONFIRM exchange of the phase after
// checking that Kea answered it with status.
func (p *keaPhase) confirmAnswered(status, when string) (string, error) {
	tid, err := p.only(keaConfirm, when)
	if err != nil {
		return "", err
	}
	answer, replied := p.replies[tid]
	if !replied {
		return "", errors.New("Kea sent no REPLY to ze's CONFIRM " + tid + " " + when)
	}
	if answer != status {
		return "", errors.New("Kea answered ze's CONFIRM " + tid + " " + when + " with status \"" + answer + "\", want " + status)
	}
	return tid, nil
}

// kea6ReplugVerdict reads the phase before the reload: ze bound address with
// one SOLICIT and one REQUEST, then its replug on the same link drew one
// CONFIRM after the bind, which Kea answered Success, and nothing else: no
// second SOLICIT, no RENEW, no REBIND.
func kea6ReplugVerdict(log, address string) error {
	phase := keaPhases(keaTimeline(log))[0]
	if len(phase.clients) != 1 {
		return kea6CountError("Kea heard", len(phase.clients), "client DUIDs, want ze's one")
	}
	when := "before the subnet moved"
	if _, err := phase.only(keaSolicit, when); err != nil {
		return err
	}
	request, err := phase.only(keaRequest, when)
	if err != nil {
		return err
	}
	if !phase.granted[address][request] {
		return errors.New("ze's REQUEST " + request + " was not granted " + address)
	}
	confirm, err := phase.confirmAnswered(keaStatusSuccess, when)
	if err != nil {
		return err
	}
	if phase.first[confirm] < phase.first[request] {
		return errors.New("ze's CONFIRM " + confirm + " reached Kea before the REQUEST that bound " + address)
	}
	if err := phase.none(keaRenewal, when); err != nil {
		return err
	}
	return phase.none(keaRebind, when)
}

// kea6MovedVerdict reads the phase after the one reload: ze's CONFIRM, which
// Kea answered NotOnLink, then one SOLICIT and one REQUEST that leased moved,
// with no RENEW or REBIND and no lease of old.
func kea6MovedVerdict(log, old, moved string) error {
	phases := keaPhases(keaTimeline(log))
	if len(phases) != 2 {
		return kea6CountError("Kea logged", len(phases)-1, "successful reloads, want 1")
	}
	phase := phases[1]
	when := "after the subnet moved"
	confirm, err := phase.confirmAnswered(keaStatusNotOnLink, when)
	if err != nil {
		return err
	}
	solicit, err := phase.only(keaSolicit, when)
	if err != nil {
		return err
	}
	if phase.first[solicit] < phase.first[confirm] {
		return errors.New("ze's SOLICIT " + solicit + " reached Kea before its CONFIRM " + confirm + ": the NotOnLink did not start it")
	}
	request, err := phase.only(keaRequest, when)
	if err != nil {
		return err
	}
	if !phase.granted[moved][request] {
		return errors.New("ze's REQUEST " + request + " was not granted " + moved)
	}
	if len(phase.granted[old]) != 0 {
		return errors.New("Kea granted " + old + " again " + when)
	}
	if err := phase.none(keaRenewal, when); err != nil {
		return err
	}
	return phase.none(keaRebind, when)
}

// keaReloadVerdict reads whether Kea has reloaded its configuration. The
// scenario sends one SIGHUP, so one success line is the answer. A failed
// reload is an error at once.
func keaReloadVerdict(log string) (bool, error) {
	if strings.Contains(log, keaReloadFailed) {
		return false, errors.New("Kea refused the moved configuration: " + strings.TrimSpace(lineWith(log, keaReloadFailed)))
	}
	return strings.Contains(log, keaReloaded), nil
}

// zeV6AbsentVerdict reads `ip -6 -o addr show dev eth0`: address is on no
// line, under any prefix length.
func zeV6AbsentVerdict(output, address string) error {
	for line := range strings.Lines(output) {
		for field := range strings.FieldsSeq(line) {
			held, _, _ := strings.Cut(field, "/")
			if held == address {
				return errors.New("ze's eth0 still holds " + field + " after the move")
			}
		}
	}
	return nil
}

func lineWith(text, marker string) string {
	for line := range strings.Lines(text) {
		if strings.Contains(line, marker) {
			return line
		}
	}
	return ""
}
