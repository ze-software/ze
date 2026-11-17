// Design: docs/architecture/l2tp/subscriber-session-model.md -- provider-wait retirement

package ppp

import (
	"errors"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// Both capabilities come from the actual address-request emitter on the same
// protocol worker. The old wait completes before the new one starts. Its late
// answer must be expired; a wrong-family answer must not consume the current
// slot, and the fresh current decision must still complete with its exact data.
func TestIPProviderAnswerCannotEnterLaterWait(t *testing.T) {
	ipEvents := make(chan IPEvent, 1)
	stop := make(chan struct{})
	session := &pppSession{
		subscriberID: "opaque/same-protocol-worker",
		ipEventsOut:  ipEvents, stopCh: stop,
		sessStop: make(chan struct{}), ipTimeout: time.Hour, logger: discardLogger(),
	}
	type outcome struct {
		decision ipResponseMsg
		accepted bool
	}
	var workers sync.WaitGroup
	t.Cleanup(func() { close(stop); workers.Wait() })
	oldResult := make(chan outcome, 1)
	workers.Go(func() {
		decision, accepted := session.awaitIPDecision(EventIPRequest{}, AddressFamilyIPv4)
		oldResult <- outcome{decision, accepted}
	})
	oldRequest := readProviderLifetimeIPRequest(t, ipEvents)
	first := ipResponseMsg{accept: true, family: AddressFamilyIPv4,
		local: netip.MustParseAddr("192.0.2.1"), peer: netip.MustParseAddr("192.0.2.2")}
	if err := oldRequest.answer(first); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-oldResult:
		if !result.accepted {
			t.Fatal("the first current response was not consumed")
		}
		if result.decision.peer != first.peer {
			t.Fatal("the first wait changed its response")
		}
	case <-time.After(time.Second):
		t.Fatal("the first address wait did not complete")
	}
	currentResult := make(chan outcome, 1)
	workers.Go(func() {
		decision, accepted := session.awaitIPDecision(EventIPRequest{}, AddressFamilyIPv4)
		currentResult <- outcome{decision, accepted}
	})
	currentRequest := readProviderLifetimeIPRequest(t, ipEvents)
	if err := oldRequest.answer(first); !errors.Is(err, ErrRequestExpired) {
		t.Fatalf("old address provider result = %v, want expired", err)
	}
	if err := currentRequest.answer(ipResponseMsg{family: AddressFamilyIPv6}); !errors.Is(err, ErrIPResponseFamilyMismatch) {
		t.Fatalf("wrong-family result = %v, want family-mismatch", err)
	}
	select {
	case <-currentResult:
		t.Fatal("old or wrong-family answer completed the current wait")
	default:
	}
	fresh := first
	fresh.peer = netip.MustParseAddr("192.0.2.3")
	if err := currentRequest.answer(fresh); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-currentResult:
		if !result.accepted {
			t.Fatal("the fresh current response was not consumed")
		}
		if result.decision.peer != fresh.peer {
			t.Fatal("an old response entered the current wait")
		}
	case <-time.After(time.Second):
		t.Fatal("the current address wait did not complete")
	}
}

// A timeout must expire its actual request BEFORE final-down publication can
// block. Otherwise an allocator finishing during that publication could receive
// a false success, omitting the pool's undelivered-answer release.
func TestIPProviderAnswerExpiresBeforeDownPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		events := make(chan Event)
		ipEvents := make(chan IPEvent, 1)
		stop := make(chan struct{})
		session := &pppSession{
			subscriberID: "opaque/timeout-publication", eventsOut: events,
			ipEventsOut: ipEvents, stopCh: stop,
			sessStop: make(chan struct{}), ipTimeout: time.Second, logger: discardLogger(),
		}
		exited := make(chan struct{})
		accepted := true
		go func() {
			defer close(exited)
			_, accepted = session.awaitIPDecision(EventIPRequest{}, AddressFamilyIPv4)
		}()
		t.Cleanup(func() { close(stop); <-exited })
		request := readProviderLifetimeIPRequest(t, ipEvents)
		time.Sleep(2 * time.Second)
		if err := request.answer(ipResponseMsg{family: AddressFamilyIPv4}); !errors.Is(err, ErrRequestExpired) {
			t.Fatalf("provider result during blocked timeout publication = %v, want expired", err)
		}
		select {
		case <-exited:
			t.Fatal("timeout returned before its final down could be published")
		default:
		}
		select {
		case event := <-events:
			if _, ok := event.(EventSessionDown); !ok {
				t.Fatalf("timeout event = %T, want final down", event)
			}
		default:
			t.Fatal("timed-out worker was not waiting to publish its final down")
		}
		<-exited
		if accepted {
			t.Fatal("timed-out address wait accepted a provider answer")
		}
	})
}

func readProviderLifetimeIPRequest(t *testing.T, events <-chan IPEvent) EventIPRequest {
	t.Helper()
	select {
	case event := <-events:
		request, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("provider event = %T, want address request", event)
		}
		return request
	case <-time.After(time.Second):
		t.Fatal("PPP did not emit the address-provider request")
		return EventIPRequest{}
	}
}

// Release must win against a timeout or already-buffered LCP restart. This uses
// bounded real-time state observations: mutex waits do not durably block a
// synctest bubble. The runtime frame profile confirms that the actual worker
// reached retirement before Release proceeds, rather than assuming a sleep
// scheduled it. Assertions observe public events, completion and owner closure.
func TestRelayReleaseWinsPendingRetirementWithoutTeardownPublication(t *testing.T) {
	for _, cause := range []string{"timeout", "queued LCP restart"} {
		t.Run(cause, func(t *testing.T) {
			driver := NewProductionDriver(discardLogger())
			if err := driver.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(driver.Stop)
			start, peer, owner := frameLeaseStart(t, 12)
			start.SubscriberID = "opaque/release-retirement"
			start.CaptureProxyLCP = true
			start.AuthTimeout = time.Hour
			if cause == "timeout" {
				start.AuthTimeout = 2 * time.Second
			}
			attempt, err := NewSessionAttempt(start.SubscriberID, owner)
			if err != nil {
				t.Fatal(err)
			}
			start.Attempt = attempt
			frameLeaseProxy(&start)
			driver.SessionsIn() <- start
			request := readProviderLifetimeAuthRequest(t, driver.AuthEventsOut())
			event := waitForEvent(t, driver.EventsOut(), time.Second)
			if _, ok := event.(EventLCPUp); !ok {
				t.Fatalf("admission event = %T, want LCP-up", event)
			}
			worker := request.Control.worker
			driverLocked, workerLocked, releaseStarted := false, false, false
			releaseDone := make(chan struct{})
			t.Cleanup(func() {
				if driverLocked {
					driver.mu.Unlock()
				}
				if workerLocked {
					worker.mu.Unlock()
				}
				driver.Stop()
				if releaseStarted {
					<-releaseDone
				}
			})
			// One bounded buffer is reused for every reached-state observation.
			stacks := make([]runtime.StackRecord, 2048)
			if cause == "queued LCP restart" {
				worker.mu.Lock()
				workerLocked = true
				writeLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest, 0x42, nil)
				waitProviderBarrier(t, "the queued LCP frame to be retained", func() bool {
					if len(worker.framesIn) != 0 {
						return true
					}
					return providerStackHasFrames(t, stacks, ".(*pppSession).serveFrameAwaitingDecision", ".(*pppSession).currentState")
				})
			}
			if err := driver.QuiesceSession(request); err != nil {
				t.Fatal(err)
			}
			driver.mu.Lock()
			driverLocked = true
			releaseResult := make(chan error, 1)
			releaseStarted = true
			go func() { defer close(releaseDone); releaseResult <- driver.ReleaseSession(request) }()
			waitProviderBarrier(t, "Release to retain the actual pending request", func() bool {
				if request.pending.mu.TryLock() {
					request.pending.mu.Unlock()
					return false
				}
				return true
			})
			if workerLocked {
				worker.mu.Unlock()
				workerLocked = false
			}
			causeFrame := ".(*pppSession).authDecisionTimedOut"
			if cause == "queued LCP restart" {
				causeFrame = ".(*pppSession).performAction"
			}
			waitProviderBarrier(t, "the actual old worker to reach retirement", func() bool {
				return providerStackHasFrames(t, stacks, ".(*authRequest).retire", causeFrame)
			})
			driver.mu.Unlock()
			driverLocked = false
			select {
			case err := <-releaseResult:
				if err != nil {
					t.Fatalf("winning release was refused: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("winning release did not join its exact worker")
			}
			for len(driver.eventsOut) != 0 {
				event := <-driver.eventsOut
				switch event.(type) {
				case EventSessionDown:
					t.Fatal("released worker published final transport teardown")
				case EventLCPDown:
					t.Fatal("released worker published network withdrawal")
				case EventLCPUp, EventSessionUp, EventSessionIPAssigned, EventEchoRTT, EventSessionRejected:
					t.Fatalf("released worker published unexpected %T", event)
				}
			}
			select {
			case event := <-driver.AuthEventsOut():
				t.Fatalf("released worker published auth outcome %T", event)
			default:
			}
			if owner.closes.Load() != 1 {
				t.Fatalf("released lease closes = %d, want one", owner.closes.Load())
			}
		})
	}
}

func waitProviderBarrier(t *testing.T, description string, ready func() bool) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("did not observe %s", description)
		case <-ticker.C:
		}
	}
}

func providerStackHasFrames(t *testing.T, records []runtime.StackRecord, first, second string) bool {
	t.Helper()
	n, complete := runtime.GoroutineProfile(records)
	if !complete {
		t.Fatalf("runtime needs %d goroutine records; fixture observation is incomplete", n)
	}
	for i := range n {
		frames := runtime.CallersFrames(records[i].Stack())
		hasFirst, hasSecond := false, false
		for {
			frame, more := frames.Next()
			if strings.HasSuffix(frame.Function, first) {
				hasFirst = true
			}
			if strings.HasSuffix(frame.Function, second) {
				hasSecond = true
			}
			if hasFirst {
				if hasSecond {
					return true
				}
			}
			if !more {
				break
			}
		}
	}
	return false
}
