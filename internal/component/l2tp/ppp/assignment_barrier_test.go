// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting baseline before forwarding

package ppp

import (
	"net"
	"testing"
	"testing/synctest"
	"time"
)

// Exercise the real session loop and NCP exchange through the kernel connection
// boundary. Restoring unacknowledged assignment sends connects before the
// consumer completes publication.
func TestIPAssignmentAcknowledgmentBeforeConnect(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, _, events := newRFC1661Session(LCPStateInitial)
				connects := 0
				s.ops.connect = func(_, _ int) error {
					connects++
					return nil
				}
				finish, _, _ := startAssignmentNCP(t, s, events, family)
				finish()
				synctest.Wait()
				ev := <-events
				assigned, ok := ev.(EventSessionIPAssigned)
				if !ok || assigned.Family != family {
					t.Fatalf("NCP completion event = %+v, want assignment for %s", ev, family)
				}
				if connects != 0 {
					t.Fatalf("connected %d times before assignment publication completed", connects)
				}
				select {
				case ev := <-events:
					t.Fatalf("event before assignment acknowledgment: %+v", ev)
				default:
				}

				assigned.Acknowledge()
				awaitLifecycleUp(t, events)
				synctest.Wait()
				if connects != 1 {
					t.Fatalf("acknowledged assignment enabled %d connections, want one", connects)
				}
			})
		})
	}
}

func TestIPAssignmentCancellation(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		for _, owner := range []string{"driver", "session"} {
			for _, phase := range []string{"enqueue", "acknowledgment", "stop-before-acknowledgment"} {
				t.Run(family.String()+"/"+owner+"/"+phase, func(t *testing.T) {
					synctest.Test(t, func(t *testing.T) {
						s, _, events := newRFC1661Session(LCPStateInitial)
						if phase == "enqueue" {
							// No receiver is available for the assignment. Cancellation
							// must release the send itself, not just the subsequent wait.
							events = make(chan Event)
						}
						connects := 0
						s.ops.connect = func(_, _ int) error {
							connects++
							return nil
						}
						finish, exited, stop := startAssignmentNCP(t, s, events, family)
						finish()
						synctest.Wait()
						var assigned EventSessionIPAssigned
						if phase != "enqueue" {
							ev := <-events
							var ok bool
							assigned, ok = ev.(EventSessionIPAssigned)
							if !ok || assigned.Family != family {
								t.Fatalf("NCP completion event = %+v, want assignment for %s", ev, family)
							}
						}
						if connects != 0 {
							t.Fatalf("connected %d times while assignment was blocked", connects)
						}
						if owner == "driver" {
							close(stop)
						} else {
							close(s.sessStop)
						}
						if phase == "stop-before-acknowledgment" {
							// The acknowledgment must not turn an already-ready stop
							// into success when both receive cases can proceed.
							assigned.Acknowledge()
						}
						synctest.Wait()
						if owner == "session" {
							select {
							case ev := <-events:
								if _, ok := ev.(EventSessionDown); !ok {
									t.Fatalf("session cancellation event = %+v, want final teardown", ev)
								}
							default:
								t.Fatal("session cancellation omitted final teardown")
							}
							synctest.Wait()
						}
						select {
						case <-exited:
						default:
							t.Fatal("cancellation did not release the assignment barrier")
						}
						if connects != 0 {
							t.Fatalf("canceled assignment enabled %d connections", connects)
						}
						select {
						case ev := <-events:
							if _, ok := ev.(EventSessionUp); ok {
								t.Fatal("canceled assignment published SessionUp")
							}
						default:
						}
					})
				})
			}
		}
	}
}

func TestSessionStopPublishesDownOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, _, events := newRFC1661Session(LCPStateInitial)
		_, exited, _ := startAssignmentNCP(t, s, events, AddressFamilyIPv4)
		close(s.sessStop)
		synctest.Wait()
		select {
		case ev := <-events:
			if _, ok := ev.(EventSessionDown); !ok {
				t.Fatalf("session cancellation event = %+v, want final teardown", ev)
			}
		default:
			t.Fatal("session cancellation omitted final teardown")
		}
		synctest.Wait()
		select {
		case <-exited:
		default:
			t.Fatal("session cancellation did not finish")
		}
		select {
		case ev := <-events:
			t.Fatalf("event after final teardown: %+v", ev)
		default:
		}
	})
}

// startAssignmentNCP leaves the actual NCP exchange one peer Ack short of
// completion. Unlike runLifecycleSession, it does not acknowledge events for
// the consumer: each test controls the assignment barrier itself.
func startAssignmentNCP(t *testing.T, s *pppSession, events chan Event, family AddressFamily) (finish func(), exited <-chan struct{}, stop chan struct{}) {
	t.Helper()
	server, peer := net.Pipe()
	s.chanFile = server
	s.eventsOut = events
	s.disableIPCP = family != AddressFamilyIPv4
	s.disableIPv6CP = family != AddressFamilyIPv6
	s.authTimeout = 20 * time.Second
	s.ipTimeout = 20 * time.Second
	s.echoInterval = time.Hour
	s.authRespCh <- authResponseMsg{accept: true}
	ips := make(chan IPEvent, 1)
	s.ipEventsOut = ips
	stop = make(chan struct{})
	s.stopCh = stop
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.run(&StartSession{})
	}()
	t.Cleanup(func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
		_ = peer.Close()
		<-done
	})
	openLifecycleLCP(t, peer)
	if ev := <-events; ev == nil {
		t.Fatal("missing LCP-up event")
	} else if _, ok := ev.(EventLCPUp); !ok {
		t.Fatalf("first session event = %T, want LCP-up", ev)
	}
	request, _ := pendingLifecycleNCP(t, s, peer, ips, family)
	return func() {
		writeLifecyclePacket(t, peer, ncpProto(family), LCPConfigureAck, request.Identifier, request.Data)
	}, done, stop
}
