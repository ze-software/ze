// Design: docs/architecture/testing/interop.md -- peer-local form changes and wire evidence.
// Related: action.go -- the standalone helper's portable action boundary.
package main

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

// TestParseCommandAcceptsActions pins the three complete argv forms without a socket.
func TestParseCommandAcceptsActions(t *testing.T) {
	for _, test := range []struct {
		name string
		argv []string
		want action
	}{
		{"udp", []string{"udp", "172.28.0.3", "172.28.0.2", "0x1234", "4500", "4500"}, actionUDP},
		{"bare", []string{"bare", "172.28.0.3", "172.28.0.2", "4660"}, actionBare},
		{"probe", []string{"probe", "eth0", "172.28.0.3", "172.28.0.2", "0x1234", "71", "1"}, actionProbe},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCommand(test.argv)
			if err != nil {
				t.Fatalf("valid %s command: %v", test.name, err)
			}
			if got.action != test.want {
				t.Fatalf("action = %d, want %d", got.action, test.want)
			}
			if got.key != fixtureKey() {
				t.Fatalf("state key = %+v, want %+v", got.key, fixtureKey())
			}
		})
	}
}

// TestParseCommandRefusesInvalidInputs keeps invalid identities outside the adapter.
func TestParseCommandRefusesInvalidInputs(t *testing.T) {
	for _, argv := range [][]string{
		nil, {"unknown"}, {"bare", "172.28.0.3", "172.28.0.2", "0"},
		{"bare", "bad", "172.28.0.2", "1"},
		{"bare", "::1", "172.28.0.2", "1"},
		{"bare", "172.28.0.3", "172.28.0.2", "4294967296"},
		{"bare", "172.28.0.3", "172.28.0.2", "1", "extra"},
		{"udp", "172.28.0.3", "172.28.0.2", "1", "0", "4500"},
		{"udp", "172.28.0.3", "172.28.0.2", "1", "4500", "65536"},
		{"probe", "", "172.28.0.3", "172.28.0.2", "1", "71", "1"},
		{"probe", "eth0", "172.28.0.3", "172.28.0.2", "1", "65536", "1"},
	} {
		if _, err := parseCommand(argv); err == nil {
			t.Fatalf("invalid argv %q was accepted", argv)
		}
	}
}

// TestRunActionMigratesOnlyTheExactOutboundState checks selection and retained identity.
func TestRunActionMigratesOnlyTheExactOutboundState(t *testing.T) {
	want := fixtureState()
	sibling := want
	sibling.key.spi++
	reversed := want
	reversed.key.source, reversed.key.target = reversed.key.target, reversed.key.source
	peer := &fakePeer{snapshots: [][]state{{sibling, reversed, want}, {want}}}
	got, err := runAction(context.Background(), command{
		action: actionUDP, key: want.key, sourcePort: 4500, targetPort: 4500,
	}, peer)
	if err != nil {
		t.Fatal(err)
	}
	if peer.migrations != 1 {
		t.Fatalf("migrations = %d, want one atomic request", peer.migrations)
	}
	if peer.selected != want {
		t.Fatalf("selected state = %+v, want %+v", peer.selected, want)
	}
	if peer.encapsulation.form != formUDP {
		t.Fatalf("requested form = %d, want UDP", peer.encapsulation.form)
	}
	if peer.encapsulation.sourcePort != 4500 {
		t.Fatal("source UDP port changed")
	}
	if peer.encapsulation.targetPort != 4500 {
		t.Fatal("target UDP port changed")
	}
	if got.SPI != want.key.spi {
		t.Fatal("result changed the SPI")
	}
}

// TestRunActionRefusesAmbiguousState refuses zero or several matching SAs before migration.
func TestRunActionRefusesAmbiguousState(t *testing.T) {
	for _, states := range [][]state{nil, {fixtureState(), fixtureState()}} {
		peer := &fakePeer{snapshots: [][]state{states}}
		if _, err := runAction(context.Background(), command{action: actionBare, key: fixtureKey()}, peer); err == nil {
			t.Fatal("missing or duplicate exact state was accepted")
		}
		if peer.migrations != 0 {
			t.Fatal("ambiguous state reached the migration socket")
		}
	}
}

// TestRunActionRefusesIdentityOrReplayLoss judges the fresh state after the acknowledged change.
func TestRunActionRefusesIdentityOrReplayLoss(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*state)
	}{
		{"reqid", func(s *state) { s.reqid++ }},
		{"sequence", func(s *state) { s.sequence-- }},
		{"spi", func(s *state) { s.key.spi++ }},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, after := fixtureState(), fixtureState()
			test.edit(&after)
			peer := &fakePeer{snapshots: [][]state{{before}, {after}}}
			if _, err := runAction(context.Background(), command{action: actionBare, key: before.key}, peer); err == nil {
				t.Fatal("the migration lost identity or replay state but reported success")
			}
		})
	}
}

// TestRunActionPropagatesMigrationFailure cannot claim a change the kernel refused.
func TestRunActionPropagatesMigrationFailure(t *testing.T) {
	want := errors.New("atomic migration refused")
	peer := &fakePeer{snapshots: [][]state{{fixtureState()}}, migrationErr: want}
	_, err := runAction(context.Background(), command{action: actionBare, key: fixtureKey()}, peer)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

// TestRunActionProbeRequiresWireAndReply couples independent outbound evidence with useful delivery.
func TestRunActionProbeRequiresWireAndReply(t *testing.T) {
	for _, replied := range []bool{true, false} {
		peer := &fakePeer{snapshots: [][]state{{fixtureState()}}, observation: wireObservation{
			key: fixtureKey(), form: formBare, sequence: 42, replied: replied,
		}}
		got, err := runAction(context.Background(), command{
			action: actionProbe, key: fixtureKey(), interfaceName: "eth0", echoID: 71, echoSequence: 1,
		}, peer)
		if replied {
			if err != nil {
				t.Fatal(err)
			}
			if got.Sequence != 42 {
				t.Fatalf("observed sequence = %d, want actual wire sequence 42", got.Sequence)
			}
			if got.Form != "bare" {
				t.Fatalf("form = %q, want bare", got.Form)
			}
			continue
		}
		if err == nil {
			t.Fatal("wire ESP with no matching echo reply was accepted")
		}
	}
}

// TestRunActionProbePropagatesCaptureFailure keeps a failed observation distinct from no traffic.
func TestRunActionProbePropagatesCaptureFailure(t *testing.T) {
	want := errors.New("packet capture refused")
	peer := &fakePeer{snapshots: [][]state{{fixtureState()}}, probeErr: want}
	_, err := runAction(context.Background(), command{action: actionProbe, key: fixtureKey()}, peer)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want capture refusal", err)
	}
}

// TestRunActionAcceptsBackgroundSequenceAdvance does not require an exact quiet-SA delta.
func TestRunActionAcceptsBackgroundSequenceAdvance(t *testing.T) {
	before, after := fixtureState(), fixtureState()
	after.sequence += 100
	peer := &fakePeer{snapshots: [][]state{{before}, {after}}}
	got, err := runAction(context.Background(), command{action: actionBare, key: before.key}, peer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sequence != after.sequence {
		t.Fatal("fresh advancing replay metadata was not retained")
	}
}

// TestRunActionProbeRefusesWrongIdentityAndStaleSequence cannot substitute configured state.
func TestRunActionProbeRefusesWrongIdentityAndStaleSequence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*wireObservation)
	}{
		{"other-spi", func(value *wireObservation) { value.key.spi++ }},
		{"stale-sequence", func(value *wireObservation) { value.sequence = fixtureState().sequence }},
		{"unknown-form", func(value *wireObservation) { value.form = formUnspecified }},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := wireObservation{key: fixtureKey(), form: formUDP, sequence: 42, replied: true}
			test.edit(&observation)
			peer := &fakePeer{snapshots: [][]state{{fixtureState()}}, observation: observation}
			if _, err := runAction(context.Background(), command{action: actionProbe, key: fixtureKey()}, peer); err == nil {
				t.Fatal("unusable wire observation was accepted")
			}
		})
	}
}

func fixtureKey() stateKey {
	return stateKey{source: netip.MustParseAddr("172.28.0.3"), target: netip.MustParseAddr("172.28.0.2"), spi: 0x1234}
}

func fixtureState() state {
	return state{key: fixtureKey(), reqid: 7, sequence: 41}
}

type fakePeer struct {
	snapshots     [][]state
	reads         int
	migrations    int
	selected      state
	encapsulation encapsulation
	migrationErr  error
	observation   wireObservation
	probeErr      error
}

func (*fakePeer) close() error { return nil }

func (f *fakePeer) states(context.Context) ([]state, error) {
	if f.reads >= len(f.snapshots) {
		return nil, errors.New("unexpected state read")
	}
	value := f.snapshots[f.reads]
	f.reads++
	return value, nil
}

func (f *fakePeer) migrate(_ context.Context, selected state, encap encapsulation) error {
	f.migrations++
	f.selected, f.encapsulation = selected, encap
	return f.migrationErr
}

func (f *fakePeer) probe(context.Context, command) (wireObservation, error) {
	return f.observation, f.probeErr
}

// TestResultDoesNotExposeKeys pins the helper's small output schema before the Linux adapter exists.
func TestResultDoesNotExposeKeys(t *testing.T) {
	var output strings.Builder
	if err := writeResult(&output, result{Action: "bare", Source: "172.28.0.3", Target: "172.28.0.2", SPI: 0x1234, ReqID: 7, Form: "bare", Sequence: 41}); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"auth", "crypt", "aead", "key", "alg"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("output exposes %q: %s", forbidden, output.String())
		}
	}
	if !strings.Contains(output.String(), `"sequence":41`) {
		t.Fatalf("output lacks the sanitized sequence evidence: %s", output.String())
	}
}
