// Design: docs/architecture/edge-cases/extended-message.md -- directional message limits.
// RFC: rfc/short/rfc8654.md -- Sections 4, 5 and 6.
package reactor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8654ReceiveLimitFollowsLocalOPEN exchanges real OPENs over a pipe,
// then drives both readers with standard, first-extended and maximum UPDATEs.
// Withdrawal-only UPDATEs need no path attributes; each zero NLRI octet names /0.
// RFC requirement: RFC8654-6-1 positive -- after local-only or bilateral capability 6 advertisement, both session readers accept UPDATE Length 4097 and 65535 and dispatch the exact complete payload.
// RFC requirement: RFC8654-6-1 negative -- with no local capability 6, peer-only advertisement cannot admit Length 4097 or 65535: both readers send NOTIFICATION 1/2 with the erroneous Length and dispatch nothing; Length 4096 remains accepted.
// RFC requirement: RFC8654-4-1 positive -- a local-only capability 6 advertisement admits an UPDATE of exactly 65535 octets without truncating its dispatched payload.
// RFC requirement: RFC8654-5-1 positive -- peer-only capability 6 does not license extended receives: NOTIFICATION 1/2 is returned with nothing dispatched.
// RFC requirement: RFC8654-5-1 negative -- a locally advertised capability 6 permits extended receives even when the peer omits capability 6.
func TestRFC8654ReceiveLimitFollowsLocalOPEN(t *testing.T) {
	for _, local := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			for _, coalesced := range []bool{false, true} {
				for _, size := range []uint16{message.MaxMsgLen, message.MaxMsgLen + 1, message.ExtMsgLen} {
					name := "local=" + strconv.FormatBool(local) + "/peer=" + strconv.FormatBool(remote) +
						"/coalesced=" + strconv.FormatBool(coalesced) + "/length=" + strconv.Itoa(int(size))
					t.Run(name, func(t *testing.T) {
						// RFC 8654 Sections 4 and 6: exercise the actual OPEN exchange.
						session, client, capture, cleanup := setupCapturingSession(t, 65002, local, remote)
						defer cleanup()
						frame := make([]byte, size)
						copy(frame, message.Marker[:])
						binary.BigEndian.PutUint16(frame[16:18], size)
						frame[18] = byte(msgtype.TypeUPDATE)
						binary.BigEndian.PutUint16(frame[19:21], size-message.HeaderLen-4)

						answer := make(chan []byte, 1)
						go func() {
							data, _ := io.ReadAll(client)
							answer <- data
						}()
						written := make(chan error, 1)
						go func() {
							_, err := client.Write(frame)
							written <- err
						}()

						var err error
						if coalesced {
							// RFC 8654 Section 6: the batched reader owes the same limit.
							err = session.readAndProcessCoalesced(session.Conn(), session.bufReader)
						} else {
							// RFC 8654 Section 6: public single-message receive entry point.
							err = session.ReadAndProcess()
						}
						accepted := local || size == message.MaxMsgLen
						if accepted {
							if err != nil {
								t.Fatalf("advertised receive limit rejected length %d: %v", size, err)
							}
							if err := <-written; err != nil {
								t.Fatal(err)
							}
							if got := capture.all(); len(got) != 1 || !bytes.Equal(got[0], frame[message.HeaderLen:]) {
								t.Fatal("accepted UPDATE was not dispatched intact exactly once")
							}
							if session.State() != fsm.StateEstablished {
								t.Fatal("accepted UPDATE ended the session")
							}
							cleanup()
							if got := <-answer; len(got) != 0 {
								t.Fatalf("accepted UPDATE elicited bytes %x", got)
							}
							return
						}
						if err == nil {
							t.Fatal("unadvertised extended receive was accepted")
						}
						if len(capture.all()) != 0 {
							t.Fatal("rejected UPDATE reached consumers")
						}
						// RFC 4271 Section 6.1: the Data field is the erroneous Length.
						assertNotification(t, <-answer, message.NotifyMessageHeader, message.NotifyHeaderBadLength, frame[16:18])
						<-written
					})
				}
			}
		}
	}
}

// TestRFC8654AnnouncementAccumulation crosses the old and extended limits with
// actual OPENs and buffered wire announcements, not the withdrawal bypass.
// A final KEEPALIVE flushes the last batch; overflow flushes the preceding one.
// MUTATION: Hardcode maxBody to 4096-19 in readAndProcessCoalesced: the exact
// consumer batch count and combined NLRI assertions fail, without needing a panic.
// RFC requirement: RFC8654-6-1 positive -- locally advertised capability 6 permits announcement accumulation beyond 4096 through exactly 65535 octets, preserving all attributes and NLRI at the consumer and flushing before overflow.
// RFC requirement: RFC8654-6-1 negative -- without local advertisement the same valid standard-sized announcements are flushed into standard-sized batches, even when the peer advertises capability 6.
func TestRFC8654AnnouncementAccumulation(t *testing.T) {
	for _, local := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			for _, count := range []int{1014, 16373} {
				name := "local=" + strconv.FormatBool(local) + "/peer=" + strconv.FormatBool(remote) +
					"/prefixes=" + strconv.Itoa(count)
				t.Run(name, func(t *testing.T) {
					session, client, capture, cleanup := setupCapturingSession(t, 65002, local, remote)
					defer cleanup()
					attrs := fatalLengthAnnouncement().PathAttributes
					nlri := ipv4Slash24s(count + 1)
					var stream []byte
					var expected [][]byte
					var pending []byte
					limit := message.MaxMsgLen
					if local {
						limit = message.ExtMsgLen
					}
					messages := 0
					// Each input remains standard-sized, so a wrong accumulation
					// limit fails payload assertions rather than header admission.
					for start := 0; start < count; {
						end := min(start+1000, count)
						chunk := nlri[start*4 : end*4]
						stream = append(stream, buildUpdateMsg(receivedUpdateBody(attrs, chunk))...)
						messages++
						if message.HeaderLen+4+len(attrs)+len(pending)+len(chunk) > limit {
							expected = append(expected, receivedUpdateBody(attrs, pending))
							pending = nil
						}
						pending = append(pending, chunk...)
						start = end
					}
					if local {
						require.Equal(t, count*4, len(pending))
						require.Greater(t, message.HeaderLen+4+len(attrs)+len(pending), message.MaxMsgLen)
						if count == 16373 {
							require.Equal(t, message.ExtMsgLen, message.HeaderLen+4+len(attrs)+len(pending))
						}
					}
					last := nlri[count*4:]
					stream = append(stream, buildUpdateMsg(receivedUpdateBody(attrs, last))...)
					if message.HeaderLen+4+len(attrs)+len(pending)+len(last) > limit {
						expected = append(expected, receivedUpdateBody(attrs, pending))
						pending = nil
					}
					pending = append(pending, last...)
					expected = append(expected, receivedUpdateBody(attrs, pending))
					stream = append(stream, message.PackTo(message.NewKeepalive(), nil)...)
					answer := make(chan []byte, 1)
					go func() {
						data, _ := io.ReadAll(client)
						answer <- data
					}()
					written := make(chan error, 1)
					go func() {
						_, err := client.Write(stream)
						written <- err
					}()
					// Fill from the actual connection before reading. Buffered()
					// must describe this known batch, not OS scheduling.
					reader := bufio.NewReaderSize(session.Conn(), len(stream))
					_, err := reader.Peek(len(stream))
					require.NoError(t, err)
					require.NoError(t, <-written)
					for range messages + 1 {
						require.NoError(t, session.readAndProcessCoalesced(session.Conn(), reader))
					}
					require.Len(t, capture.all(), len(expected)-1, "last batch remains pending before KEEPALIVE")
					require.NoError(t, session.readAndProcessCoalesced(session.Conn(), reader))
					require.Equal(t, expected, capture.all(), "exact consumer batches, including every attribute and NLRI")
					require.Nil(t, session.coalesce.body, "KEEPALIVE releases the pending batch")
					require.NoError(t, session.flushCoalesce())
					require.Equal(t, expected, capture.all(), "empty flush must not duplicate routes")
					require.Equal(t, fsm.StateEstablished, session.State())
					cleanup()
					require.Empty(t, <-answer, "valid accumulation emits no NOTIFICATION")
				})
			}
		}
	}
}

// TestRFC8654SendLimitFollowsPeerOPEN sends the same large withdrawal through
// the Peer splitter after each of the four actual OPEN exchanges, then reads
// every emitted UPDATE and compares its complete withdrawn NLRI sequence.
// RFC requirement: RFC8654-4-6 positive -- a peer-only capability 6 advertisement allows the sender to emit one UPDATE larger than 4096 octets.
// RFC requirement: RFC8654-4-6 negative -- a local-only capability 6 advertisement does not permit extended sends: the same payload is split into UPDATEs no larger than 4096 octets with every withdrawal retained.
func TestRFC8654SendLimitFollowsPeerOPEN(t *testing.T) {
	for _, local := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			t.Run("local="+strconv.FormatBool(local)+"/peer="+strconv.FormatBool(remote), func(t *testing.T) {
				// RFC 8654 Sections 4 and 6: negotiate from actual OPEN advertisements.
				session, client, _, cleanup := setupCapturingSession(t, 65002, local, remote)
				defer cleanup()
				peer := NewPeer(session.settings)
				peer.session = session
				peer.negotiated.Store(NewNegotiatedCapabilities(session.Negotiated()))
				peer.setEncodingContexts(session.Negotiated())
				maximum := message.MaxMsgLen
				if remote {
					maximum = message.ExtMsgLen
				}
				if session.writeBuf.Cap() != maximum {
					t.Fatalf("write capacity = %d, want %d", session.writeBuf.Cap(), maximum)
				}
				stream := make(chan []byte, 1)
				go func() {
					data, _ := io.ReadAll(client)
					stream <- data
				}()
				nlri := ipv4Slash24s(1100)
				update := &message.Update{WithdrawnRoutes: nlri}
				// RFC 8654 Section 4: use the production outbound capability projection.
				maxSize := int(message.MaxMessageLength(msgtype.TypeUPDATE, peer.negotiated.Load().ExtendedMessage))
				if err := peer.sendUpdateWithSplit(t.Context(), update, maxSize, false); err != nil {
					t.Fatal(err)
				}
				cleanup()
				frames := framesOnTheWire(t, <-stream)
				wantFrames := 2
				if remote {
					wantFrames = 1
				}
				if len(frames) != wantFrames {
					t.Fatalf("UPDATE count = %d, want %d", len(frames), wantFrames)
				}
				var got []byte
				for _, frame := range frames {
					if frame.msgType != byte(msgtype.TypeUPDATE) || frame.length > maximum {
						t.Fatalf("unexpected outbound type %d / length %d", frame.msgType, frame.length)
					}
					sections, err := wire.ParseUpdateSections(frame.body)
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, sections.Withdrawn(frame.body)...)
				}
				if !bytes.Equal(got, nlri) {
					t.Fatal("send split changed the withdrawn NLRI sequence")
				}
			})
		}
	}
}

// TestRFC8654AdvertisementDoesNotExtendControlBounds supplies invalid control
// headers after unilateral and bilateral advertisement; both readers must reject
// OPEN Length 4097 and KEEPALIVE Length 20 before reading a body.
// RFC requirement: RFC8654-4-3 negative -- extended receive permission does not admit OPEN Length 4097 or KEEPALIVE Length 20; each yields NOTIFICATION 1/2 with the exact Length bytes.
func TestRFC8654AdvertisementDoesNotExtendControlBounds(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, coalesced := range []bool{false, true} {
			for _, tc := range []struct {
				kind   msgtype.MessageType
				length uint16
			}{
				{kind: msgtype.TypeOPEN, length: message.MaxMsgLen + 1},
				{kind: msgtype.TypeKEEPALIVE, length: message.HeaderLen + 1},
			} {
				name := tc.kind.String() + "/peer=" + strconv.FormatBool(remote) + "/coalesced=" + strconv.FormatBool(coalesced)
				t.Run(name, func(t *testing.T) {
					// RFC 8654 Section 6: local capability 6, independently selected peer capability.
					session, client, capture, cleanup := setupCapturingSession(t, 65002, true, remote)
					defer cleanup()
					header := make([]byte, message.HeaderLen)
					copy(header, message.Marker[:])
					binary.BigEndian.PutUint16(header[16:18], tc.length)
					header[18] = byte(tc.kind)
					answer := make(chan []byte, 1)
					go func() {
						if _, err := client.Write(header); err != nil {
							answer <- nil
							return
						}
						data, _ := io.ReadAll(client)
						answer <- data
					}()
					var err error
					if coalesced {
						// RFC 8654 Section 4: OPEN and KEEPALIVE remain exceptions.
						err = session.readAndProcessCoalesced(session.Conn(), session.bufReader)
					} else {
						// RFC 8654 Section 4: the unbatched reader enforces the same exceptions.
						err = session.ReadAndProcess()
					}
					if err == nil {
						t.Fatal("extended capability relaxed a control-message bound")
					}
					if len(capture.all()) != 0 {
						t.Fatal("invalid control message dispatched an UPDATE")
					}
					// RFC 4271 Section 6.1: exact Bad Message Length NOTIFICATION.
					assertNotification(t, <-answer, message.NotifyMessageHeader, message.NotifyHeaderBadLength, header[16:18])
				})
			}
		}
	}
}

// TestExtendedSendRewriteScratchUsesOutboundSize covers the two send rewrites
// that borrow receive-pool storage. A peer-only advertisement permits a large
// send even though this session selects the small pool for incoming messages.
func TestExtendedSendRewriteScratchUsesOutboundSize(t *testing.T) {
	for _, rewrite := range []string{"aigp", "paths-limit"} {
		t.Run(rewrite, func(t *testing.T) {
			// RFC 8654 Section 4: only the receiving peer advertises capability 6.
			session, client, _, cleanup := setupCapturingSession(t, 65002, false, true)
			defer cleanup()
			nlri := ipv4Slash24s(1100)
			update := &message.Update{WithdrawnRoutes: nlri}
			if rewrite == "aigp" {
				// RFC 7311 Section 3.3: eBGP has AIGP disabled in this fixture,
				// so the final send boundary must remove this attribute.
				update.PathAttributes = []byte{0x80, 26, 11, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 5}
			} else {
				// Exercise the shared PATHS-LIMIT rewrite scratch with a limit
				// on another family: these ordinary IPv4 withdrawals stay exact.
				session.writeMu.Lock()
				session.initPathsLimit(&capability.EncodingCaps{
					AddPathMode:    map[family.Family]capability.AddPathMode{family.IPv6Unicast: capability.AddPathSend},
					PathsLimitSend: map[family.Family]uint16{family.IPv6Unicast: 1},
				})
				session.writeMu.Unlock()
			}
			stream := make(chan []byte, 1)
			go func() {
				data, _ := io.ReadAll(client)
				stream <- data
			}()
			// RFC 4271 Section 4.3: the raw UPDATE body retains both lengths.
			packet := message.PackTo(update, nil)
			if err := session.sendRawUpdateBody(packet[message.HeaderLen:]); err != nil {
				t.Fatal(err)
			}
			cleanup()
			frames := framesOnTheWire(t, <-stream)
			if len(frames) != 1 || frames[0].msgType != byte(msgtype.TypeUPDATE) {
				t.Fatal("rewrite did not emit exactly one UPDATE")
			}
			frame := frames[0]
			if frame.length <= message.MaxMsgLen {
				t.Fatal("rewritten UPDATE no longer exercises extended scratch")
			}
			sections, err := wire.ParseUpdateSections(frame.body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(sections.Withdrawn(frame.body), nlri) || sections.AttrsLen() != 0 || sections.NLRILen(frame.body) != 0 {
				t.Fatal("outbound rewrite truncated withdrawals or retained the AIGP attribute")
			}
		})
	}
}
