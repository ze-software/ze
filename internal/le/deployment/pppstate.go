// Design: docs/architecture/testing/interop.md -- ze against a real PPP peer
// Overview: l2tpppp.go -- the proof that reads this state
// Related: netns.go -- the namespace each of these questions is put inside
// Related: gokrazyl2tp.go -- the appliance proof that reads this state
// Related: l2tpppp.go -- the on-host proof that reads this state
//
// pppstate.go asks the KERNEL what happened instead of asking ze. A daemon that
// logs "session up" states what it believes. A pppN interface that carries the
// two addresses negotiated by IPCP confirms that the kernel agrees. The proofs
// in this package assert both. A check of the log line alone would pass a
// control plane that never programmed anything.
//
// The proofs ask the same questions in reverse at the end. A torn-down L2TP
// session leaves no interface or tunnel. Without this check, a proof would pass
// a daemon that leaks a session for each tunnel.

package deployment

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// pppPrefix is what a kernel PPP interface is named with. The proofs match on
// it rather than on an exact name, because the number is the kernel's to pick.
const pppPrefix = "ppp"

// cleanupPoll is how often the teardown assertion re-reads the kernel's state.
// The assertion waits for a set of interfaces to disappear. The kernel removes
// them in one step after the last descriptor closes. A shorter poll only spins.
const cleanupPoll = 200 * time.Millisecond

// pppLinks answers the PPP interfaces that exist in ns.
//
// A failure returns an error instead of an empty set. An empty interface list
// differs from a failed query. The proof would then report a clean teardown.
// It would not examine the namespace.
func pppLinks(ns string) (map[string]bool, error) {
	out, ok := nsText(ns, "ip", "-o", "link", "show", "type", pppPrefix)
	if !ok {
		return nil, errors.New("ip link show type ppp failed")
	}

	found := map[string]bool{}
	for line := range strings.SplitSeq(out, "\n") {
		if name := linkName(line); name != "" {
			found[name] = true
		}
	}
	return found, nil
}

// linkName answers the interface name out of one `ip -o link show` line, or the
// empty string when the line carries none.
//
// A line reads `3: ppp0@if5: <POINTOPOINT,...`, so the name sits between the
// index and whichever of the colon or the at-sign comes first. The at-sign
// introduces the peer of a paired link and is not part of the name.
func linkName(line string) string {
	index, rest, ok := strings.Cut(line, ":")
	if !ok || index == "" || strings.TrimLeft(index, "0123456789") != "" {
		return ""
	}
	name := strings.TrimSpace(rest)
	if at := strings.IndexAny(name, ":@"); at >= 0 {
		name = name[:at]
	}
	return strings.TrimSpace(name)
}

// l2tpState answers what the kernel holds for ns: its tunnels, then its
// sessions.
func l2tpState(ns string) (string, string, error) {
	tunnel, tunnelOK := nsText(ns, "ip", "l2tp", "show", tunnelObjectName)
	session, sessionOK := nsText(ns, "ip", "l2tp", "show", "session")
	if !tunnelOK || !sessionOK {
		return "", "", errors.New("ip l2tp state inspection failed")
	}
	return tunnel, session, nil
}

// l2tpSnapshot is one namespace's kernel L2TP state in the form of its two
// listings. The code compares the listings instead of parsing them. The
// teardown assertion needs to know whether the state returned to its initial
// value. A difference in any field means that the states differ.
type l2tpSnapshot struct {
	tunnel  string
	session string
}

// readL2TPSnapshot answers the namespace's current L2TP state.
func readL2TPSnapshot(ns string) (l2tpSnapshot, error) {
	tunnel, session, err := l2tpState(ns)
	if err != nil {
		return l2tpSnapshot{}, err
	}
	return l2tpSnapshot{tunnel: tunnel, session: session}, nil
}

// interfaceField answers the value of an `interface=` field in one log line, or
// the empty string when the line carries none.
//
// The value is quoted when it carries a space, which a slog text handler does
// for any value it must escape, so the quotes are taken off whether or not they
// are there.
func interfaceField(line string) string {
	_, rest, ok := strings.Cut(line, "interface=")
	if !ok {
		return ""
	}
	value := rest
	if end := strings.IndexFunc(value, func(r rune) bool { return r == ' ' || r == '\t' }); end >= 0 {
		value = value[:end]
	}
	return strings.Trim(value, `"`)
}

// discoverPPPIface answers the PPP interface one side of the session came up
// on.
//
// It asks the daemon's own log first. The log names the interface that the
// daemon programmed and provides evidence for that answer. The set difference
// is the fallback for the peer, which logs nothing. It rejects an ambiguous
// answer. Two new interfaces show that something else on this machine made one.
// If the function picked either, the proof's later assertions would rest on a
// guess.
func discoverPPPIface(ns string, initial map[string]bool, lines []string, role string) (string, error) {
	current, err := pppLinks(ns)
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		candidate := interfaceField(line)
		if strings.HasPrefix(candidate, pppPrefix) && current[candidate] {
			return candidate, nil
		}
	}

	var appeared []string
	for name := range current {
		if !initial[name] {
			appeared = append(appeared, name)
		}
	}
	slices.Sort(appeared)

	var tb textbuf.Buffer
	switch len(appeared) {
	case 0:
		return "", errors.New(tb.Str("no new pppN interface appeared in ").Str(role).Str(" namespace").String())
	case 1:
		return appeared[0], nil
	default:
		return "", errors.New(tb.Str("more than one new PPP interface appeared in ").Str(role).
			Str(" namespace: ").Str(strings.Join(appeared, ", ")).String())
	}
}

// verifyPPPAddress asserts that iface carries the negotiated address pair.
//
// The check names both ends because IPCP negotiates both. An interface with the
// local address but no peer address is a half-finished negotiation. A control
// plane that programs an address without a route leaves exactly this state
// behind.
func verifyPPPAddress(ns, iface, local, peer string) error {
	out, ok := nsText(ns, "ip", "-o", "addr", "show", "dev", iface)
	var tb textbuf.Buffer
	if !ok {
		return errors.New(tb.Str("ip addr show dev ").Str(iface).Str(" failed").String())
	}
	if addressPresent(out, local) && addressPresent(out, peer) {
		return nil
	}
	return errors.New(tb.Str(iface).Str(" lacks expected ").Str(local).Str(" peer ").Str(peer).
		Str(" address state:\n").Str(out).String())
}

// addressPresent reports whether the listing carries want as a WHOLE address.
//
// A substring test cannot do this job. It gets the exact addresses this proof
// uses wrong. The pool runs from 10.100.0.2 to 10.100.0.10, and the gateway is
// 10.100.0.1. The gateway is a substring of the last address in the range. An
// interface on the wrong end of the pool satisfies a substring test for the
// gateway. That assertion can never come out red on its own.
//
// A whole address is a field of the listing. It appears alone or with a prefix
// length. This is how `ip -o addr show` writes the two ends of a peer pair.
func addressPresent(listing, want string) bool {
	for field := range strings.FieldsSeq(listing) {
		if field == want {
			return true
		}
		if rest, ok := strings.CutPrefix(field, want); ok && strings.HasPrefix(rest, "/") {
			return true
		}
	}
	return false
}

// pppBaseline is what one namespace held before the session, and what the
// teardown assertion requires it to hold again.
type pppBaseline struct {
	ns    string
	links map[string]bool
	l2tp  l2tpSnapshot
	// iface is the interface the session came up on, which MUST be gone.
	iface string
}

// readPPPBaselines answers what each namespace holds now.
//
// The function reads every namespace's interfaces before it reads any
// namespace's L2TP state. teardownSettled uses the same order. This order makes
// the two halves of the proof comparable. Each question is one `ip` process. A
// comparison of the argv from two implementations is a comparison of two
// SEQUENCES.
func readPPPBaselines(namespaces []string) ([]pppBaseline, error) {
	baselines := make([]pppBaseline, len(namespaces))
	for i, ns := range namespaces {
		links, err := pppLinks(ns)
		if err != nil {
			return nil, err
		}
		baselines[i] = pppBaseline{ns: ns, links: links}
	}
	for i := range baselines {
		state, err := readL2TPSnapshot(baselines[i].ns)
		if err != nil {
			return nil, err
		}
		baselines[i].l2tp = state
	}
	return baselines, nil
}

// awaitTeardown waits until every namespace holds what it held before the
// session. If the wait expires, it answers an error that names the remaining
// state.
//
// It compares the whole state instead of only the session's interface. This
// assertion catches a kernel object that the daemon forgot. A check of only the
// interface does not detect a tunnel left after its session went away.
func awaitTeardown(baselines []pppBaseline, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		settled, why := teardownSettled(baselines)
		if settled {
			return nil
		}
		if !time.Now().Before(deadline) {
			var tb textbuf.Buffer
			return errors.New(tb.Str("kernel L2TP/PPP cleanup did not return to initial state: ").
				Str(why).String())
		}
		time.Sleep(cleanupPoll)
	}
}

// teardownSettled reports whether every baseline holds again. If a baseline
// does not hold, it describes the difference.
//
// It asks every namespace about its interfaces before it asks any namespace
// about its L2TP state. readPPPBaselines states the reason. The sequence of
// questions is itself part of how this proof is compared.
func teardownSettled(baselines []pppBaseline) (bool, string) {
	links := make([]map[string]bool, len(baselines))
	for i, base := range baselines {
		current, err := pppLinks(base.ns)
		if err != nil {
			return false, err.Error()
		}
		links[i] = current
	}
	states := make([]l2tpSnapshot, len(baselines))
	for i, base := range baselines {
		state, err := readL2TPSnapshot(base.ns)
		if err != nil {
			return false, err.Error()
		}
		states[i] = state
	}

	var tb textbuf.Buffer
	for i, base := range baselines {
		if !links[i][base.iface] && maps.Equal(links[i], base.links) && states[i] == base.l2tp {
			continue
		}
		tb.Str(base.ns).Str(" ppp=").Str(strings.Join(slices.Sorted(maps.Keys(links[i])), ",")).
			Str(" l2tp-changed=").Str(boolWord(states[i] != base.l2tp))
		return false, tb.String()
	}
	return true, ""
}

// boolWord answers a bool as the word a reader of a diagnosis reads.
func boolWord(yes bool) string {
	if yes {
		return "true"
	}
	return "false"
}
