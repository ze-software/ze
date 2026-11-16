// Design: docs/research/l2tpv2-ze-integration.md -- failed activation prohibits SessionUp

package ppp

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Each activation operation must hand its exact error to the lifecycle
// consumer and stop before any later operation or SessionUp publication.
func TestSessionActivationFailureProhibitsSessionUp(t *testing.T) {
	stages := []string{"SetMTU", "Connect", "SetReceiveMRU", "SetAdminUp"}
	for i, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			session, _, events := newRFC1661Session(LCPStateOpened)
			session.authRespCh <- authResponseMsg{accept: true}
			owner := &activationGuardOwner{
				SessionHandle: session.dataplane,
				failureStage:  stage,
				failure:       errors.New("activation resource refused"),
			}
			session.dataplane = owner
			if session.afterLCPOpen() {
				t.Fatal("failed activation permitted network use")
			}
			if !reflect.DeepEqual(owner.calls, stages[:i+1]) {
				t.Fatalf("activation operations %v, want %v", owner.calls, stages[:i+1])
			}
			var down *EventSessionDown
			for len(events) > 0 {
				switch event := (<-events).(type) {
				case EventSessionUp:
					t.Fatal("failed activation published SessionUp")
				case EventSessionDown:
					down = &event
				}
			}
			if down == nil {
				t.Fatal("failed activation did not publish SessionDown")
			}
			if !strings.Contains(down.Reason, owner.failure.Error()) {
				t.Fatalf("session-down reason %q lost activation error", down.Reason)
			}
		})
	}
}

type activationGuardOwner struct {
	SessionHandle
	failureStage string
	failure      error
	calls        []string
}

func (o *activationGuardOwner) operation(stage string) error {
	o.calls = append(o.calls, stage)
	if stage == o.failureStage {
		return o.failure
	}
	return nil
}

func (o *activationGuardOwner) SetMTU(int) error           { return o.operation("SetMTU") }
func (o *activationGuardOwner) Connect() error             { return o.operation("Connect") }
func (o *activationGuardOwner) SetReceiveMRU(uint16) error { return o.operation("SetReceiveMRU") }
func (o *activationGuardOwner) SetAdminUp() error          { return o.operation("SetAdminUp") }

// Failed disconnect preserves the old network lifetime and prevents the
// transition caller from treating that still-connected owner as reusable.
func TestNCPResetDisconnectFailurePreservesLifetime(t *testing.T) {
	session, _, events := newRFC1661Session(LCPStateOpened)
	want := errors.New("disconnect resource refused")
	testHandleFor(session).ops.disconnect = func(int) error { return want }
	if err := testHandleFor(session).Connect(); err != nil {
		t.Fatal(err)
	}
	session.channelConnected = true
	session.ipcpState = LCPStateOpened
	if session.resetNCP() {
		t.Fatal("failed disconnect permitted lifetime reuse")
	}
	if !session.channelConnected {
		t.Fatal("failed disconnect forgot the connected owner")
	}
	if session.ipcpState != LCPStateOpened {
		t.Fatal("failed disconnect discarded the old network lifetime")
	}
	select {
	case raw := <-events:
		event, ok := raw.(EventSessionDown)
		if !ok {
			t.Fatalf("disconnect failure published %T, want SessionDown", raw)
		}
		if !strings.Contains(event.Reason, want.Error()) {
			t.Fatalf("disconnect failure did not reach lifecycle consumer: %+v", event)
		}
	default:
		t.Fatal("failed disconnect did not publish SessionDown")
	}
}
