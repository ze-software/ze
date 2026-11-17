// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact PPP admission capabilities

package ppp

import (
	"errors"
	"net/netip"
	"testing"
	"time"
)

// Construction preserves an opaque ID and creates one distinct binding per
// transport admission, even when the same ID and comparable lease are reused.
func TestSessionAttemptConstructionPreservesExactHandoff(t *testing.T) {
	_, peer, owner := frameLeaseStart(t, 1)
	defer closeConn(peer)
	const id = "opaque/access-4294967294/session-9"
	first, err := NewSessionAttempt(id, owner)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSessionAttempt(id, owner)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("a reused ID and lease reused the old transport binding")
	}
	if first.SubscriberID() != id {
		t.Fatalf("opaque ID = %q, want %q", first.SubscriberID(), id)
	}
	if firstCopy := first; firstCopy != first {
		t.Fatal("copying an attempt changed its transport binding")
	}
	if owner.closes.Load() != 0 {
		t.Fatal("constructing an attempt transferred or closed its borrowed lease")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Invalid identity and missing, typed-nil or incomparable ownership fail before
// minting a binding. Construction never closes a caller's valid borrowed owner.
func TestSessionAttemptConstructionRefusesInvalidInputs(t *testing.T) {
	_, peer, owner := frameLeaseStart(t, 1)
	defer closeConn(peer)
	defer func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}()
	var typedNil *sessionLeaseCounter
	type incomparableLease struct {
		SessionHandle
		_ []byte
	}
	for _, input := range []struct {
		name  string
		id    string
		lease SessionHandle
	}{
		{name: "missing ID", lease: owner},
		{name: "missing lease", id: "opaque/admission"},
		{name: "typed nil lease", id: "opaque/admission", lease: typedNil},
		{name: "incomparable lease", id: "opaque/admission", lease: incomparableLease{SessionHandle: owner}},
	} {
		t.Run(input.name, func(t *testing.T) {
			attempt, err := NewSessionAttempt(input.id, input.lease)
			if err == nil {
				t.Fatal("invalid attempt was accepted")
			}
			if attempt != (SessionAttempt{}) {
				t.Fatal("invalid construction returned an admission binding")
			}
		})
	}
	if owner.closes.Load() != 0 {
		t.Fatal("refusing an attempt closed its caller's borrowed lease")
	}
}

// A real Driver admission mints one control that follows the actual LCP,
// authentication, up and final down events. The transport binds it by its exact
// retained attempt, rather than by a wire label or by parsing the opaque ID.
func TestDriverAdmittedEventsCarryExactSessionControl(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, owner := frameLeaseStart(t, 9)
	start.SubscriberID = "opaque/access-4294967294/session-9"
	attempt, err := NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.Attempt = attempt
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	first := waitForEvent(t, driver.EventsOut(), time.Second)
	lcp, ok := first.(EventLCPUp)
	if !ok {
		t.Fatalf("first event = %T, want LCP-up", first)
	}
	control := lcp.Control
	if control == (SessionControl{}) {
		t.Fatal("admitted LCP event has no control")
	}
	if control.Attempt() != attempt {
		t.Fatal("admission substituted a different transport attempt")
	}
	if control.SubscriberID() != start.SubscriberID {
		t.Fatal("admission changed the opaque subscriber identity")
	}
	if !control.Active() {
		t.Fatal("newly admitted worker is not active")
	}
	var observedAuth EventAuthRequest
	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		observedAuth = request
		if !ok {
			t.Fatalf("authentication event = %T, want request", event)
		}
		if request.Control != control {
			t.Fatal("authentication request lost the admitted control")
		}
		if request.SubscriberID != start.SubscriberID {
			t.Fatal("authentication request changed the opaque identity")
		}
	case <-time.After(time.Second):
		t.Fatal("admitted worker did not request authentication")
	}
	if err := driver.AuthResponse(observedAuth, true, "", nil); err != nil {
		t.Fatal(err)
	}
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	up, ok := event.(EventSessionUp)
	if !ok {
		t.Fatalf("authenticated event = %T, want session-up", event)
	}
	if up.Control != control {
		t.Fatal("session-up lost the admitted control")
	}
	frameLeaseEcho(t, peer)
	if err := driver.StopSession(control); err != nil {
		t.Fatal(err)
	}
	event = waitForEvent(t, driver.EventsOut(), time.Second)
	down, ok := event.(EventSessionDown)
	if !ok {
		t.Fatalf("stopped event = %T, want session-down", event)
	}
	if down.Control != control {
		t.Fatal("session-down changed the retired worker's identity")
	}
	if control.Active() {
		t.Fatal("retired worker still has active mutation authority")
	}
	if owner.closes.Load() != 1 {
		t.Fatalf("retired lease closes = %d, want one", owner.closes.Load())
	}
}

// Both actual Driver NCP request producers must retain the same admitted
// control and opaque ID. A current address response must still be deliverable.
func TestDriverAddressRequestsCarryExactSessionControl(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			driver := NewProductionDriver(discardLogger())
			if err := driver.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Stop)
			start, _, owner := frameLeaseStart(t, 9)
			start.SubscriberID = "opaque/access-4294967294/session-9"
			attempt, err := NewSessionAttempt(start.SubscriberID, owner)
			if err != nil {
				t.Fatal(err)
			}
			start.Attempt = attempt
			start.DisableIPCP = family != AddressFamilyIPv4
			start.DisableIPv6CP = family != AddressFamilyIPv6
			frameLeaseProxy(&start)
			control := authenticateControlTestSession(t, driver, &start)
			var observedAddress EventIPRequest
			select {
			case event := <-driver.IPEventsOut():
				request, ok := event.(EventIPRequest)
				observedAddress = request
				if !ok {
					t.Fatalf("address event = %T, want request", event)
				}
				if request.Control != control {
					t.Fatal("address request lost exact admitted control")
				}
				if request.SubscriberID != start.SubscriberID {
					t.Fatal("address request changed opaque identity")
				}
				if request.Family != family {
					t.Fatal("address request changed family")
				}
			case <-time.After(time.Second):
				t.Fatal("admitted worker did not request an address")
			}
			if err := driver.IPResponse(observedAddress, IPResponseArgs{Family: family, Reason: "fixture refusal"}); err != nil {
				t.Fatalf("current address response was refused: %v", err)
			}
			event := waitForEvent(t, driver.EventsOut(), time.Second)
			if family == AddressFamilyIPv4 {
				down, ok := event.(EventSessionDown)
				if !ok {
					t.Fatalf("IPv4 refusal event = %T, want teardown", event)
				}
				if down.Control != control {
					t.Fatal("IPv4 refusal changed exact control")
				}
				if down.Reason != "ipcp: handler rejected: fixture refusal" {
					t.Fatal("current IPv4 response was not consumed")
				}
			} else {
				up, ok := event.(EventSessionUp)
				if !ok {
					t.Fatalf("IPv6 decline event = %T, want completion", event)
				}
				if up.Control != control {
					t.Fatal("IPv6 decline changed exact control")
				}
			}
		})
	}
}

// Actual Driver admission must reject malformed raw transfers without
// substituting another canonical key's ExistingSession classification. Every
// refusal retains the incoming attempt and preserves the existing live owner.
func TestDriverRejectsMalformedAttemptWithoutChangingActiveAdmission(t *testing.T) {
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	first, firstPeer, firstOwner := frameLeaseStart(t, 9)
	first.SubscriberID = "opaque/active-7/session-9"
	firstAttempt, err := NewSessionAttempt(first.SubscriberID, firstOwner)
	if err != nil {
		t.Fatal(err)
	}
	first.Attempt = firstAttempt
	frameLeaseProxy(&first)
	control := authenticateControlTestSession(t, driver, &first)
	if event := waitForEvent(t, driver.EventsOut(), time.Second); event == nil {
		t.Fatal("active predecessor did not complete admission")
	} else if _, ok := event.(EventSessionUp); !ok {
		t.Fatalf("predecessor event = %T, want session-up", event)
	}
	for _, name := range []string{"missing attempt", "missing ID", "ID mismatch", "lease mismatch", "raw ID collides with active ID"} {
		t.Run(name, func(t *testing.T) {
			incoming, _, incomingOwner := frameLeaseStart(t, 10)
			incoming.SubscriberID = "opaque/incoming/" + name
			boundID := incoming.SubscriberID
			var boundLease SessionHandle = incomingOwner
			switch name {
			case "missing attempt":
				incoming.Attempt = SessionAttempt{}
			case "missing ID":
				incoming.SubscriberID = ""
			case "ID mismatch":
				boundID = "opaque/different-bound-ID"
			case "lease mismatch":
				boundLease = firstOwner
			case "raw ID collides with active ID":
				incoming.SubscriberID = first.SubscriberID
			}
			if name != "missing attempt" {
				attempt, err := NewSessionAttempt(boundID, boundLease)
				if err != nil {
					t.Fatal(err)
				}
				incoming.Attempt = attempt
			}
			driver.SessionsIn() <- incoming
			event := waitForEvent(t, driver.EventsOut(), time.Second)
			refused, ok := event.(EventSessionRejected)
			if !ok {
				t.Fatalf("malformed transfer event = %T, want refusal", event)
			}
			if refused.Attempt != incoming.Attempt {
				t.Fatal("refusal lost its exact incoming attempt")
			}
			if refused.ExistingSession {
				t.Fatal("unbound raw ID protected an unrelated pending admission")
			}
			if incomingOwner.closes.Load() != 1 {
				t.Fatalf("incoming lease closes = %d, want one", incomingOwner.closes.Load())
			}
			if firstOwner.closes.Load() != 0 {
				t.Fatal("refusing malformed transfer closed the active lease")
			}
			if !control.Active() {
				t.Fatal("refusing malformed transfer removed the active admission")
			}
			frameLeaseEcho(t, firstPeer)
		})
	}
}

// This real admission precondition stops after authentication, allowing its
// caller to observe either the first NCP request or a no-NCP session-up event.
func authenticateControlTestSession(t *testing.T, driver *Driver, input *StartSession) SessionControl {
	t.Helper()
	start := *input
	driver.SessionsIn() <- start
	event := waitForEvent(t, driver.EventsOut(), time.Second)
	opened, ok := event.(EventLCPUp)
	if !ok {
		t.Fatalf("first event = %T, want LCP-up", event)
	}
	if opened.Control.Attempt() != start.Attempt {
		t.Fatal("LCP admission changed exact attempt")
	}
	var observedAuth EventAuthRequest
	select {
	case event := <-driver.AuthEventsOut():
		request, ok := event.(EventAuthRequest)
		observedAuth = request
		if !ok {
			t.Fatalf("auth event = %T, want request", event)
		}
		if request.Control != opened.Control {
			t.Fatal("auth request changed admitted control")
		}
	case <-time.After(time.Second):
		t.Fatal("admitted worker did not request authentication")
	}
	if err := driver.AuthResponse(observedAuth, true, "", nil); err != nil {
		t.Fatal(err)
	}
	return opened.Control
}

// Each admitted lifecycle variant receives the emitter's exact control at the
// central producer. A refusal instead preserves its incoming attempt and never
// acquires a fabricated admitted control.
func TestLifecycleProducerStampsEveryAdmittedControl(t *testing.T) {
	session := &pppSession{subscriberID: "opaque/emitter", sessStop: make(chan struct{})}
	session.control = SessionControl{worker: session}
	for _, event := range []Event{
		EventLCPUp{}, EventLCPDown{}, EventSessionUp{},
		EventSessionIPAssigned{}, EventSessionDown{}, EventEchoRTT{},
	} {
		bound := session.bindEventControl(event)
		var control SessionControl
		switch value := bound.(type) {
		case EventLCPUp:
			control = value.Control
		case EventLCPDown:
			control = value.Control
		case EventSessionUp:
			control = value.Control
		case EventSessionIPAssigned:
			control = value.Control
		case EventSessionDown:
			control = value.Control
		case EventEchoRTT:
			control = value.Control
		case EventSessionRejected:
			t.Fatal("admitted event became a rejection")
		}
		if control != session.control {
			t.Fatalf("%T lost the exact emitter control", event)
		}
	}
	_, peer, owner := frameLeaseStart(t, 1)
	defer closeConn(peer)
	attempt, err := NewSessionAttempt("opaque/refused", owner)
	if err != nil {
		t.Fatal(err)
	}
	event := session.bindEventControl(EventSessionRejected{Attempt: attempt})
	refused, ok := event.(EventSessionRejected)
	if !ok {
		t.Fatalf("refusal event = %T, want EventSessionRejected", event)
	}
	if refused.Attempt != attempt {
		t.Fatal("rejection changed its incoming binding")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

// Each operation retains an actual request emitted for the old indexed worker.
// A distinct replacement then owns the SAME canonical ID and SAME lease. No
// current-ID or recycled-lease fallback may authorize that retired control.
func TestRetiredControlCannotMutateSameIDAndLeaseReplacement(t *testing.T) {
	for _, operation := range []struct {
		name    string
		address bool
		run     func(*Driver, EventAuthRequest, EventIPRequest, SessionControl) error
	}{
		{"auth", false, func(d *Driver, a EventAuthRequest, _ EventIPRequest, _ SessionControl) error {
			return d.AuthResponse(a, true, "", nil)
		}},
		{"IP", true, func(d *Driver, _ EventAuthRequest, i EventIPRequest, _ SessionControl) error {
			return d.IPResponse(i, IPResponseArgs{Family: i.Family})
		}},
		{"stop", false, func(d *Driver, _ EventAuthRequest, _ EventIPRequest, c SessionControl) error { return d.StopSession(c) }},
		{"quiesce", false, func(d *Driver, a EventAuthRequest, _ EventIPRequest, _ SessionControl) error {
			return d.QuiesceSession(a)
		}},
		{"release", false, func(d *Driver, a EventAuthRequest, _ EventIPRequest, _ SessionControl) error {
			return d.ReleaseSession(a)
		}},
		{"refuse", false, func(d *Driver, a EventAuthRequest, _ EventIPRequest, _ SessionControl) error {
			return d.RefuseAuth(a, "old failure")
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			driver := NewProductionDriver(discardLogger())
			_, _, lease := frameLeaseStart(t, 1)
			const id = "opaque/reused-id-and-lease"
			authEvents := make(chan AuthEvent, 1)
			ipEvents := make(chan IPEvent, 1)
			stop := make(chan struct{})
			original := &pppSession{
				subscriberID: id, dataplane: lease, sessStop: make(chan struct{}),
				stopCh: stop, authEventsOut: authEvents,
				ipEventsOut: ipEvents, authTimeout: time.Hour,
				ipTimeout: time.Hour, logger: discardLogger(), state: LCPStateOpened,
			}
			control := SessionControl{driver: driver, worker: original}
			original.control = control
			driver.sessions[id] = original
			driver.owners[lease] = original
			exited := make(chan struct{})
			t.Cleanup(func() {
				close(stop)
				<-exited
				if err := lease.Close(); err != nil {
					t.Error(err)
				}
			})
			var auth EventAuthRequest
			var address EventIPRequest
			if operation.address {
				go func() { defer close(exited); original.awaitIPDecision(EventIPRequest{}, AddressFamilyIPv4) }()
				address = readProviderLifetimeIPRequest(t, ipEvents)
			} else {
				go func() { defer close(exited); original.awaitAuthDecision(EventAuthRequest{}, "") }()
				auth = readProviderLifetimeAuthRequest(t, authEvents)
			}
			replacementAuthEvents := make(chan AuthEvent, 1)
			replacementIPEvents := make(chan IPEvent, 1)
			replacement := &pppSession{
				subscriberID: id, dataplane: lease,
				sessStop: make(chan struct{}), done: make(chan struct{}),
				stopCh: stop, authEventsOut: replacementAuthEvents,
				ipEventsOut: replacementIPEvents, authTimeout: time.Hour,
				ipTimeout: time.Hour, logger: discardLogger(), state: LCPStateOpened,
			}
			replacement.control = SessionControl{driver: driver, worker: replacement}
			driver.mu.Lock()
			driver.sessions[id] = replacement
			driver.owners[lease] = replacement
			driver.mu.Unlock()
			type replacementOutcome struct {
				auth     authResponseMsg
				address  ipResponseMsg
				accepted bool
			}
			replacementResult := make(chan replacementOutcome, 1)
			var currentAuth EventAuthRequest
			var currentAddress EventIPRequest
			if operation.address {
				go func() {
					defer close(replacement.done)
					decision, accepted := replacement.awaitIPDecision(EventIPRequest{}, AddressFamilyIPv4)
					replacementResult <- replacementOutcome{address: decision, accepted: accepted}
				}()
				currentAddress = readProviderLifetimeIPRequest(t, replacementIPEvents)
			} else {
				go func() {
					defer close(replacement.done)
					decision, accepted := replacement.awaitAuthDecision(EventAuthRequest{}, "")
					replacementResult <- replacementOutcome{auth: decision, accepted: accepted}
				}()
				currentAuth = readProviderLifetimeAuthRequest(t, replacementAuthEvents)
			}
			t.Cleanup(func() { replacement.control.Cancel(); <-replacement.done })
			if operation.address {
				if currentAddress.Control != replacement.control {
					t.Fatal("actual replacement IP request lost its control")
				}
			} else {
				if currentAuth.Control != replacement.control {
					t.Fatal("actual replacement auth request lost its control")
				}
			}
			result := make(chan error, 1)
			operationDone := make(chan struct{})
			go func() { defer close(operationDone); result <- operation.run(driver, auth, address, control) }()
			t.Cleanup(func() { replacement.control.Cancel(); <-operationDone })
			select {
			case err := <-result:
				if !errors.Is(err, ErrSessionNotFound) {
					t.Fatalf("retired control result = %v, want session-not-found", err)
				}
			case <-time.After(time.Second):
				t.Fatal("retired control reached a replacement worker's blocking operation")
			}
			if control.Active() {
				t.Fatal("retired control became active for its successor")
			}
			if !control.Cancel() {
				t.Fatal("retired control lost exact-worker cancellation")
			}
			if !control.Cancel() {
				t.Fatal("exact-worker cancellation is not idempotent")
			}
			select {
			case <-original.sessStop:
			default:
				t.Fatal("original worker was not canceled")
			}
			select {
			case <-replacement.sessStop:
				t.Fatal("stale control canceled replacement")
			default:
			}
			if driver.sessions[id] != replacement {
				t.Fatal("stale control removed replacement ID")
			}
			if driver.owners[lease] != replacement {
				t.Fatal("stale control removed replacement lease")
			}
			if lease.closes.Load() != 0 {
				t.Fatal("stale control closed replacement lease")
			}
			select {
			case <-replacementResult:
				t.Fatal("stale control delivered a decision to the replacement wait")
			default:
			}
			if operation.address {
				if !currentAddress.Active() {
					t.Fatal("stale operation consumed the replacement address slot")
				}
				fresh := IPResponseArgs{Family: AddressFamilyIPv4, Accept: true,
					Peer: netip.MustParseAddr("192.0.2.99")}
				if err := driver.IPResponse(currentAddress, fresh); err != nil {
					t.Fatalf("fresh replacement address refused: %v", err)
				}
				select {
				case actual := <-replacementResult:
					if !actual.accepted || !actual.address.accept || actual.address.peer != fresh.Peer {
						t.Fatal("replacement did not consume its own fresh address decision")
					}
				case <-time.After(time.Second):
					t.Fatal("replacement address wait did not consume its fresh decision")
				}
			} else {
				if !currentAuth.Active() {
					t.Fatal("stale operation consumed the replacement auth slot")
				}
				if err := driver.AuthResponse(currentAuth, true, "fresh replacement decision", nil); err != nil {
					t.Fatalf("fresh replacement auth refused: %v", err)
				}
				select {
				case actual := <-replacementResult:
					if !actual.accepted || !actual.auth.accept || actual.auth.message != "fresh replacement decision" {
						t.Fatal("replacement did not consume its own fresh authentication decision")
					}
				case <-time.After(time.Second):
					t.Fatal("replacement auth wait did not consume its fresh decision")
				}
			}
		})
	}
	if (SessionControl{}).Cancel() {
		t.Fatal("zero control claimed a worker")
	}
}
