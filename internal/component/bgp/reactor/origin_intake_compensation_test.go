// Design: docs/architecture/api/process-protocol.md -- failed-retention compensation.
package reactor

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type originCompensationFixture struct {
	writer      *selectedWriterFixture
	host        *pluginserver.Server
	owner       plugin.SelectedOwner
	lifetime    context.Context
	cancel      context.CancelFunc
	original    *process.Process
	mu          sync.Mutex
	candidates  map[uint64]rpc.OriginIntakeInput
	afterRetain func(rpc.OriginIntakeInput) *rpc.UpdateRouteOutput
	withdrawal  chan rpc.OriginIntakeInput
	release     chan struct{}
	releaseOnce sync.Once
	refuse      bool
}

// This is a real SDK candidate owner over the existing native/framed mutation
// receiver. Its small current map tests retention cleanup, not RIB election.
func newOriginCompensationFixture(t *testing.T, typed bool) *originCompensationFixture {
	t.Helper()
	writer := newSelectedWriterFixture(t, false)
	f := &originCompensationFixture{writer: writer, candidates: make(map[uint64]rpc.OriginIntakeInput), withdrawal: make(chan rpc.OriginIntakeInput, 8), release: make(chan struct{})}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	f.lifetime = ctx
	f.cancel = cancel
	t.Cleanup(cancel)
	const name = "origin-compensation-provider-test"
	declarations := registry.Lookup(selectedWriterProcess).Dissemination
	require.NoError(t, registry.Register(registry.Registration{Name: name, CLIHandler: func([]string) int { return 0 },
		Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(), Dissemination: declarations,
		RunEngine: func(conn net.Conn) int {
			defer func() { f.mu.Lock(); clear(f.candidates); f.mu.Unlock() }() // Provider state dies only after its joined SDK lifetime.
			if !typed {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn(name, conn)
			p.OnOriginIntake(func(work context.Context, in rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
				if in.Operation == rpc.OriginIntakeWithdraw {
					if err := work.Err(); err != nil {
						return nil, err
					}
					f.withdrawal <- rpc.CloneOriginIntake(in)
					select {
					case <-f.release:
					case <-work.Done():
						return nil, work.Err()
					}
					if f.refuse {
						return &rpc.UpdateRouteOutput{Failure: "provider cleanup deliberately refused"}, nil
					}
					f.mu.Lock()
					previous, found := f.candidates[in.OriginReceipt]
					if found {
						expected := rpc.CloneOriginIntake(previous)
						expected.Operation = rpc.OriginIntakeWithdraw
						expected.DeadlineMS = in.DeadlineMS
						if !sameOriginCompensationInput(expected, in) {
							f.mu.Unlock()
							return nil, errors.New("cleanup changed original contribution identity")
						}
						delete(f.candidates, in.OriginReceipt)
					}
					f.mu.Unlock()
					return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
				}
				f.mu.Lock()
				for id := range f.candidates {
					previous := f.candidates[id]
					if previous.PeerOwner == in.PeerOwner && previous.Destination == in.Destination &&
						previous.Producer == in.Producer && previous.Source == in.Source &&
						previous.Route.Family == in.Route.Family && bytes.Equal(previous.Route.KeyNLRI, in.Route.KeyNLRI) &&
						previous.Route.SourcePathID == in.Route.SourcePathID && previous.Route.SourceAddPath == in.Route.SourceAddPath {
						delete(f.candidates, id)
					}
				}
				f.candidates[in.OriginReceipt] = rpc.CloneOriginIntake(in)
				f.mu.Unlock()
				if f.afterRetain != nil {
					if answer := f.afterRetain(in); answer != nil {
						return answer, nil
					}
				}
				return originCompensationReply(in), nil
			})
			if err := p.Run(ctx, &sdk.Registration{Sends: testAllBGPSendDecls(), Receives: testAllBGPReceiveDecls(), Dissemination: declarations}); err != nil {
				t.Errorf("compensation SDK: %v", err)
				return 1
			}
			return 0
		}}))
	host, err := pluginserver.NewServer(&pluginserver.ServerConfig{Plugins: []plugin.PluginConfig{{Name: name, Run: name, Internal: true}}}, nil)
	require.NoError(t, err)
	f.host = host
	destBus := testBusSelectedDest(name)
	host.UpdateDeliveryGraph("bgp", []pluginserver.DeliveryPeer{
		testDeliveryPeer("192.0.2.79", "", testBusSourceLease(name)),
		testDeliveryPeer(writer.peer.addrString, "", destBus),
	})
	startOriginIntakeTestHost(t, ctx, host)
	f.original = host.ProcessManager().GetProcess(name)
	require.NotNil(t, f.original)
	f.owner, err = f.original.SelectedOwner()
	require.NoError(t, err)
	t.Cleanup(func() {
		f.unblock()
		cancel()
		f.original.Stop()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, f.original.Wait(join))
	})
	writer.api.r.api = host
	writer.peer.settings.Bus = destBus
	configured, err := host.ResolveDisseminationOwnerForPeer(testDeliveryPeer(writer.peer.addrString, "", destBus), SelectedDisseminationRole)
	require.NoError(t, err)
	writer.peer.settings.DisseminationOwner = configured
	writer.peer.refreshForwardFacts()
	writer.api.r.originReceipts, err = newOriginReceiptStore(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.unblock()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(t, writer.api.r.originReceipts.close(join))
	})
	return f
}

func sameOriginCompensationInput(a, b rpc.OriginIntakeInput) bool { return reflect.DeepEqual(a, b) }

func originCompensationReply(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
	return &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{{Family: in.Route.Family, KeyNLRI: in.Route.KeyNLRI,
		Destination: in.Destination, PeerOwner: in.PeerOwner, Generation: in.OriginReceipt, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}}}
}
func (f *originCompensationFixture) unblock() { f.releaseOnce.Do(func() { close(f.release) }) }
func (f *originCompensationFixture) snapshot() map[uint64]rpc.OriginIntakeInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot := make(map[uint64]rpc.OriginIntakeInput, len(f.candidates))
	for id := range f.candidates {
		snapshot[id] = rpc.CloneOriginIntake(f.candidates[id])
	}
	return snapshot
}
func (f *originCompensationFixture) announce(ctx context.Context, batch bgptypes.NLRIBatch) (*rpc.UpdateRouteOutput, error) {
	return f.writer.api.AnnounceNLRIBatchResult(ctx, selector.ParseDefault(f.writer.peer.addrString), batch, plugin.OperatorSender())
}
func originCompensationSurvivorBatch() bgptypes.NLRIBatch {
	batch := originGapBatch()
	batch.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.1.0.0/24"), 0, false)}
	return batch
}

// The actual provider has retained before each invalidation. Old B returns the
// error with that candidate still present and never attempts the held removal.
func TestOriginIntakeFailureJoinsExactRetainedCandidateCleanup(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, failure := range []string{"revoked", "bad-reply", "canceled", "replaced"} {
			t.Run(map[bool]string{false: "framed", true: "typed"}[typed]+"/"+failure, func(t *testing.T) {
				f := newOriginCompensationFixture(t, typed)
				survivor, err := f.announce(t.Context(), originCompensationSurvivorBatch())
				require.NoError(t, err)
				require.True(t, survivor.Retained())
				survivorID := survivor.Admissions[0].Generation
				call, cancel := context.WithCancel(t.Context())
				defer cancel()
				retained := make(chan rpc.OriginIntakeInput, 1)
				f.afterRetain = func(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
					retained <- rpc.CloneOriginIntake(in)
					switch failure {
					case "revoked":
						if err := f.writer.api.r.originReceipts.retireOriginReceipt(t.Context(), in.OriginReceipt); err != nil {
							t.Errorf("invalidate actual issued receipt: %v", err)
						}
					case "bad-reply":
						answer := originCompensationReply(in)
						answer.Admissions[0].PeerOwner++
						return answer
					case "canceled":
						cancel()
					case "replaced":
						manager := f.host.ProcessManager()
						manager.RemoveProcess(f.original.Name())
						replacement := process.NewProcess(f.original.Config())
						if err := manager.AddProcess(f.original.Name(), replacement); err != nil {
							t.Errorf("publish actual replacement: %v", err)
						}
						t.Cleanup(func() { manager.RemoveProcess(f.original.Name()); replacement.Stop() })
					}
					return originCompensationReply(in)
				}
				type result struct {
					output *rpc.UpdateRouteOutput
					err    error
				}
				done := make(chan result, 1)
				go func() { out, err := f.announce(call, originGapBatch()); done <- result{out, err} }()
				var original rpc.OriginIntakeInput
				select {
				case original = <-retained:
				case <-f.lifetime.Done():
					t.Fatal(f.lifetime.Err())
				}
				var cleanup rpc.OriginIntakeInput
				select {
				case cleanup = <-f.withdrawal:
				case early := <-done:
					require.Error(t, early.err)
					require.Contains(t, f.snapshot(), original.OriginReceipt, "baseline must reach real provider retention")
					t.Fatalf("failed intake returned without removing its retained provider candidate: %v (output: %+v)", early.err, early.output)
				case <-f.lifetime.Done():
					t.Fatal(f.lifetime.Err())
				}
				require.Contains(t, f.snapshot(), original.OriginReceipt)
				expected := rpc.CloneOriginIntake(original)
				expected.Operation = rpc.OriginIntakeWithdraw
				expected.DeadlineMS = cleanup.DeadlineMS
				require.True(t, sameOriginCompensationInput(expected, cleanup), "cleanup must target only the original issued source/key/scope/generation")
				require.Greater(t, cleanup.DeadlineMS, time.Now().UnixMilli(), "canceled caller must not cancel cleanup")
				require.LessOrEqual(t, cleanup.DeadlineMS, time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli())
				select {
				case early := <-done:
					t.Fatalf("cleanup acknowledgment preceded held provider removal: %v", early.err)
				default:
				}
				f.unblock()
				settled := <-done
				require.Error(t, settled.err)
				require.NotNil(t, settled.output)
				require.NotEmpty(t, settled.output.Failure)
				require.Zero(t, settled.output.Announced)
				require.False(t, settled.output.Retained(), "failed/compensated scope is not accepted retention")
				require.NotContains(t, f.snapshot(), original.OriginReceipt)
				require.Contains(t, f.snapshot(), survivorID, "independent retained native contribution survives cleanup")
				require.Empty(t, f.writer.conn.written(), "retained losers never escape to direct output")
				healthy, wire := newGroupUpdatesPeer(t, "192.0.2.81", "absent")
				healthy.settings.ExportPolicy = f.writer.peer.settings.ExportPolicy
				healthy.reactor = f.writer.api.r
				f.writer.api.r.mu.Lock()
				f.writer.api.r.peers[healthy.Settings().PeerKey()] = healthy
				f.writer.api.r.mu.Unlock()
				healthy.refreshForwardFacts()
				direct, err := f.writer.api.AnnounceNLRIBatchResult(t.Context(), selector.ParseDefault(healthy.addrString), originGapBatch(), plugin.OperatorSender())
				require.NoError(t, err)
				require.Equal(t, uint32(1), direct.Announced)
				require.NotEmpty(t, wire.written(), "unrelated useful origination survives compensation")
				require.Empty(t, f.writer.conn.written())
				if failure != "replaced" {
					f.afterRetain = nil
					fresh, err := f.announce(t.Context(), originGapBatch())
					require.NoError(t, err)
					require.True(t, fresh.Retained())
					freshID := fresh.Admissions[0].Generation
					require.NotEqual(t, original.OriginReceipt, freshID)
					cleanup.DeadlineMS = 0
					late, err := f.owner.OriginIntake(t.Context(), cleanup)
					require.NoError(t, err)
					require.Equal(t, uint32(1), late.Withdrawn)
					require.Contains(t, f.snapshot(), freshID, "late old-receipt removal cannot prune its genuine successor")
					require.Contains(t, f.snapshot(), survivorID)
				}
			})
		}
	}
}

func TestOriginIntakeCleanupRefusalRemainsExplicit(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginCompensationFixture(t, typed)
			f.refuse = true
			f.unblock()
			f.afterRetain = func(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
				answer := originCompensationReply(in)
				answer.Admissions[0].PeerOwner++
				return answer
			}
			output, err := f.announce(t.Context(), originGapBatch())
			require.ErrorContains(t, err, "scope differs", "original reply failure remains visible")
			require.ErrorContains(t, err, "provider cleanup deliberately refused", "unsettled provider retention cannot be acknowledged")
			require.NotNil(t, output)
			require.NotEmpty(t, output.Failure)
			require.Zero(t, output.Announced)
			require.Len(t, f.snapshot(), 1, "the explicit failure reports actual unsettled membership")
		})
	}
}

// Stop is not a receipt for provider destruction. Refuse cleanup on a canceled
// captured token, preserve failure, and observe actual SDK/process join separately.
func TestOriginIntakeStoppedProviderDoesNotClaimCleanupAcknowledgment(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginCompensationFixture(t, typed)
			f.afterRetain = func(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
				manager := f.host.ProcessManager()
				manager.RemoveProcess(f.original.Name())
				replacement := process.NewProcess(f.original.Config())
				if err := manager.AddProcess(f.original.Name(), replacement); err != nil {
					t.Errorf("publish replacement: %v", err)
				}
				t.Cleanup(func() { manager.RemoveProcess(f.original.Name()); replacement.Stop() })
				f.cancel()
				f.original.Stop()
				return originCompensationReply(in)
			}
			output, err := f.announce(t.Context(), originGapBatch())
			require.ErrorContains(t, err, "candidate cleanup did not settle")
			require.NotNil(t, output)
			require.NotEmpty(t, output.Failure)
			require.False(t, output.Retained())
			select {
			case in := <-f.withdrawal:
				t.Fatalf("stopped original or replacement received cleanup: %+v", in)
			default:
			}
			joined, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			require.NoError(t, f.original.Wait(joined), "observe real original SDK/process join independently of Stop")
			require.Empty(t, f.snapshot(), "provider teardown actually cleared its owned current candidates")
		})
	}
}

func TestOriginIntakeCompensationJoinsOriginalWriterBeforeProviderRemoval(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			f := newOriginCompensationFixture(t, typed)
			admitted := make(chan originWrite, 1)
			f.afterRetain = func(in rpc.OriginIntakeInput) *rpc.UpdateRouteOutput {
				store := f.writer.api.r.originReceipts
				store.mu.Lock()
				record := store.receipts[in.OriginReceipt]
				store.mu.Unlock()
				guard, err := store.admit(in.OriginReceipt, f.owner, f.writer.peer, f.writer.peer.selectedOwnerSnapshot(), record.build.key)
				if err != nil {
					t.Errorf("admit actual original issuer write: %v", err)
					answer := originCompensationReply(in)
					answer.Admissions[0].PeerOwner++
					return answer
				}
				admitted <- guard
				answer := originCompensationReply(in)
				answer.Admissions[0].PeerOwner++
				return answer
			}
			done := make(chan error, 1)
			go func() { _, err := f.announce(t.Context(), originGapBatch()); done <- err }()
			var guard originWrite
			select {
			case guard = <-admitted:
			case <-f.lifetime.Done():
				t.Fatal(f.lifetime.Err())
			}
			t.Cleanup(guard.complete)
			select {
			case <-guard.done():
			case <-f.lifetime.Done():
				t.Fatal("compensation did not revoke original write authority")
			}
			select {
			case in := <-f.withdrawal:
				t.Fatalf("provider removal preceded actual original-write join: %+v", in)
			default:
			}
			select {
			case err := <-done:
				t.Fatalf("failed offer returned before actual original-write join: %v", err)
			default:
			}
			guard.complete()
			select {
			case <-f.withdrawal:
			case err := <-done:
				t.Fatalf("provider membership was never removed after joining issuer writes: %v", err)
			case <-f.lifetime.Done():
				t.Fatal(f.lifetime.Err())
			}
			f.unblock()
			require.Error(t, <-done)
			require.Empty(t, f.snapshot())
		})
	}
}
