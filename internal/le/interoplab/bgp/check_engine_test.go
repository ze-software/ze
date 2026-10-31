package bgp

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestFailedCheckShowsThePeerOutputItSearched proves every one-shot assertion that
// read the peer's answer carries that answer when it fails, so a red names what the
// peer printed beside what the checker expected.
//
// VALIDATES: requireContains, requireAbsentWithProof and requireJSONFields wrap
// their verdict with the searched output; a pass stays nil.
// PREVENTS: bgp-addpath-frr's red, where FRR 10.3 printed "Paths: (2 available"
// and the checker said only that "2 paths" was missing, so reading the answer took
// a temporary code edit.
func TestFailedCheckShowsThePeerOutputItSearched(t *testing.T) {
	t.Parallel()
	frr := "BGP routing table entry for 10.0.0.0/24\nPaths: (2 available, best #1, table default)"

	err := requireContains(frr, "BGP routing table", "2 paths")
	if err == nil {
		t.Fatal("a missing needle must fail")
	}
	for _, want := range []string{`peer output is missing "2 paths"`, "searched output:\n", "Paths: (2 available"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("requireContains error lacks %q: %q", want, err.Error())
		}
	}
	if err := requireContains(frr, "Paths:"); err != nil {
		t.Errorf("a present needle must pass, got %v", err)
	}

	err = requireAbsentWithProof(frr, []string{"best #1"}, []string{"Paths:"})
	if err == nil || !strings.Contains(err.Error(), `unexpectedly contains "best #1"`) || !strings.Contains(err.Error(), "Paths: (2 available") {
		t.Errorf("an absence failure must name the needle and the searched output, got %v", err)
	}
	err = requireAbsentWithProof("session Idle", []string{"x"}, []string{"Established"})
	if err == nil || !strings.Contains(err.Error(), "searched output:\nsession Idle") {
		t.Errorf("an unexercised absence must carry the searched output, got %v", err)
	}

	err = requireJSONFields(`{"state":"Idle"}`, map[string]string{"state": "Established"}, nil)
	if err == nil || !strings.Contains(err.Error(), "searched output:\n"+`{"state":"Idle"}`) {
		t.Errorf("a JSON failure must carry the searched output, got %v", err)
	}

	err = requireContains("", "Established")
	if err == nil || !strings.Contains(err.Error(), "(the peer printed nothing)") {
		t.Errorf("an empty answer must be named as empty, got %v", err)
	}
}

// TestRewriteOperationJSONFieldsForNetwork checks that a rendered query and its
// JSON expectations name the same selected network without mutating the registry.
func TestRewriteOperationJSONFieldsForNetwork(t *testing.T) {
	t.Parallel()
	registered := operation{
		kind:    opWaitJSONFields,
		command: []string{"show", "172.30.0.9"},
		fields: map[string]string{
			"peer_ip":     "172.30.0.9",
			"bgp_nexthop": "172.30.0.5",
			"bgp_id":      "198.51.100.1",
			"ip_prefix":   "10.99.77.0/24",
		},
	}
	first := registered
	rewriteOperation(interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}, &first)
	if first.command[1] != "172.31.71.9" {
		t.Fatalf("query names %q, want selected-network peer", first.command[1])
	}
	if first.fields["peer_ip"] != "172.31.71.9" {
		t.Errorf("peer expectation=%q, want 172.31.71.9", first.fields["peer_ip"])
	}
	if first.fields["bgp_nexthop"] != "172.31.71.5" {
		t.Errorf("next-hop expectation=%q, want 172.31.71.5", first.fields["bgp_nexthop"])
	}
	if first.fields["bgp_id"] != "198.51.100.1" {
		t.Errorf("fixed Router ID was rewritten: %v", first.fields)
	}
	if first.fields["ip_prefix"] != "10.99.77.0/24" {
		t.Errorf("announced prefix was rewritten: %v", first.fields)
	}
	if registered.fields["peer_ip"] != "172.30.0.9" {
		t.Fatalf("rendering mutated registry expectations: %v", registered.fields)
	}
	second := registered
	rewriteOperation(interoplab.Network{IPv4: netip.MustParsePrefix("172.31.72.0/24")}, &second)
	if second.fields["peer_ip"] != "172.31.72.9" {
		t.Errorf("second run reused another network's expectation: %v", second.fields)
	}
	if first.fields["peer_ip"] != "172.31.71.9" {
		t.Errorf("second run mutated the first run: %v", first.fields)
	}
}

// scriptedQueryLab answers every Query with one scripted reply and counts the
// probes. Only Query is implemented: the positive waits call nothing else.
type scriptedQueryLab struct {
	interoplab.CheckerLab
	output string
	err    error
	calls  int
}

func (l *scriptedQueryLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	l.calls++
	return l.output, l.err
}

// TestPositiveWaitReadsAFinishedEmptyAnswerAsMeasured proves a positive wait
// tells "the peer answered with nothing" from "the peer never answered".
//
// VALIDATES: waitContains, waitContainsFold and waitContainsAny time out with
// "timed out before the peer became ready" and "(the peer printed nothing)" when
// every probe finished empty, never pass on an empty answer (even with no
// needle), still report "never measured peer state" for a failed exec, and still
// pass when the needle arrives.
// PREVENTS: the ospfv3-redist-frr red, where FRR's vtysh printed nothing for an
// empty "show ipv6 route ospf6" on every probe and the red read "never measured
// peer state: peer frr query returned no output; the final probe ... did not
// finish within 30s", which named a hung vtysh instead of an absent prefix.
func TestPositiveWaitReadsAFinishedEmptyAnswerAsMeasured(t *testing.T) {
	t.Parallel()
	command := []string{cmdVtysh, "-c", frrShowOSPF6Route}
	const timeout = 50 * time.Millisecond
	waits := map[string]func(interoplab.CheckerLab, ...string) error{
		"contains": func(lab interoplab.CheckerLab, needles ...string) error {
			return waitContains(t.Context(), lab, peerFRR, command, timeout, needles...)
		},
		"fold": func(lab interoplab.CheckerLab, needles ...string) error {
			return waitContainsFold(t.Context(), lab, peerFRR, command, timeout, needles...)
		},
		"any": func(lab interoplab.CheckerLab, needles ...string) error {
			return waitContainsAny(t.Context(), lab, peerFRR, command, timeout, needles...)
		},
	}
	for name, wait := range waits {
		empty := &scriptedQueryLab{err: &interoplab.EmptyAnswerError{Peer: peerFRR}}
		err := wait(empty, "2001:db8:5e5::/48")
		if err == nil {
			t.Fatalf("%s: an empty answer must not satisfy a wait for a prefix", name)
		}
		for _, want := range []string{"timed out before the peer became ready", "(the peer printed nothing)"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: empty-answer error lacks %q: %q", name, want, err.Error())
			}
		}
		if strings.Contains(err.Error(), "never measured") {
			t.Errorf("%s: a finished empty answer was reported as unmeasured: %q", name, err.Error())
		}
		if empty.calls == 0 {
			t.Errorf("%s: the wait never probed", name)
		}
		if name != "any" {
			if err := wait(&scriptedQueryLab{err: &interoplab.EmptyAnswerError{Peer: peerFRR}}); err == nil {
				t.Errorf("%s: an empty answer must not satisfy a wait with no needle", name)
			}
		}

		failed := &scriptedQueryLab{err: errors.New("docker exec ze-iop-frr vtysh did not finish within 30s")}
		err = wait(failed, "2001:db8:5e5::/48")
		if err == nil || !strings.Contains(err.Error(), "never measured peer state") || !strings.Contains(err.Error(), "did not finish") {
			t.Errorf("%s: a failed exec must stay unmeasured and name its failure, got %v", name, err)
		}

		present := &scriptedQueryLab{output: "O>* 2001:db8:5e5::/48 [110/20] via fe80::1, eth0\n"}
		if err := wait(present, "2001:db8:5e5::/48"); err != nil {
			t.Errorf("%s: a present prefix must pass, got %v", name, err)
		}
	}
}
