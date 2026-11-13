// Design: docs/architecture/rib/unified-locrib.md -- acknowledged forwarding output.
// Related: register_route_view_output.go -- one owner operation for both transports.

package sysrib

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"reflect"
	"slices"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/process"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// routeViewPublication is a native boundary value. The receiver must own all
// mutable batch slices before publication or retention. Its zero is invalid.
type routeViewPublication struct {
	Session   string
	Cursor    uint64
	Operation uint64
	Batch     sysribevents.BestChangeBatch
}

// routeViewOutput owns every possibly delivered prefix. The session lease
// serializes all methods. Its zero value has no bus or initialized ledger.
type routeViewOutput struct {
	bus           ze.EventBus
	ledger        map[prefixKey]sysribevents.BestChangeEntry
	acknowledged  *routeViewOutputAck
	lastOperation uint64
}

type routeViewOutputAck struct {
	input  routeViewPublication
	output rpc.RouteViewPublishOutput
}

// routeViewPublicationReset retains one fully validated, owned pre-emission
// refusal. Only provePublicationReset constructs it. Its zero value is not proof.
// Each admitted publication MUST clear visibility before validating its input.
type routeViewPublicationReset struct {
	input   routeViewPublication
	visible bool
}

const routeViewOutputFix = "Resend ze-sysrib:route-view-publish with a complete valid system best-change batch"

// revokePublicationReset runs before adapter decoding, so a malformed attempt
// cannot borrow an earlier refusal's visible proof. It MUST use authenticated
// owner identity alone; an untrusted or missing payload token grants no access.
func (authority *routeViewAuthority) revokePublicationReset(ctx context.Context, owner *process.Process) error {
	authority.mu.Lock()
	active := authority.active
	if active == nil {
		authority.mu.Unlock()
		return nil
	}
	if active.owner != owner {
		authority.mu.Unlock()
		return nil
	}
	token := active.token
	authority.mu.Unlock()
	return authority.withSession(ctx, owner, token, func(session *routeViewSession) error {
		if session.publicationReset != nil {
			session.publicationReset.visible = false
		}
		return nil
	})
}

func (authority *routeViewAuthority) publish(ctx context.Context, owner *process.Process, input routeViewPublication, bus ze.EventBus) (rpc.RouteViewPublishOutput, error) {
	var result rpc.RouteViewPublishOutput
	err := authority.withSession(ctx, owner, input.Session, func(session *routeViewSession) error {
		if session.publicationReset != nil {
			session.publicationReset.visible = false
		}
		if input.Operation == 0 {
			return routeViewRefusal("Routing publication operation is zero", "An unset identity cannot establish output ordering", "Resend route-view-publish with a positive retained operation identity")
		}
		if err := routeViewValidateOutput(&input.Batch); err != nil {
			return err
		}
		if routeViewPublicationBytes(input) > rpc.RouteViewRequestBudget(rpc.MethodRouteViewPublish) {
			return routeViewRefusal("Routing publication exceeds the complete request frame", "The batch cannot cross both transports without losing metadata", "Split route-view-publish into smaller batches with separate ordered operation identities")
		}
		prepared := input
		prepared.Batch = routeViewCloneOutputBatch(input.Batch)
		state := session.output
		if state != nil {
			if previous := state.acknowledged; previous != nil {
				if input.Operation == previous.input.Operation {
					if !reflect.DeepEqual(prepared, previous.input) {
						return routeViewRefusal("Routing publication operation was reused with different values", "An acknowledged operation has one immutable batch and cursor", "Resend the original batch and cursor for this retained operation")
					}
					// An exact retained retry can predate the latest consumed
					// cursor, but still requires a healthy, ready generation.
					if err := session.validateConsumed(session.consumed); err != nil {
						return err
					}
					result = previous.output
					return nil
				}
				if previous.input.Operation == math.MaxUint64 {
					return routeViewRefusal("Routing publication operation sequence is exhausted", "The acknowledged maximum identity has no positive successor", "Close this view with ze-sysrib:route-view-close, then open a fresh view with ze-sysrib:route-view-open")
				}
				if input.Operation < previous.input.Operation {
					return routeViewRefusal("Routing publication operation precedes the acknowledged operation", "An old batch cannot replace current forwarding output", "Resend route-view-publish with an operation greater than the previous operation")
				}
			}
			if input.Operation <= state.lastOperation {
				return routeViewRefusal("Routing publication operation is not newer than completed output", "Fencing does not release previously completed identities for reuse", "Resend route-view-publish with a new operation greater than the last completed operation")
			}
		}
		if input.Cursor != session.consumed {
			return routeViewRefusal("Selected routing publication cursor was not consumed", "Publish only from the most recently consumed complete view", "Retry route-view-publish with the cursor from the last completed route-view-read")
		}
		if err := session.validateConsumed(input.Cursor); err != nil {
			session.provePublicationReset(prepared)
			return err
		}
		if err := session.checkRequest(ctx); err != nil {
			return routeViewStopped(err, owner)
		}
		if state == nil {
			if bus == nil {
				return routeViewRefusal("The daemon forwarding event bus is unavailable", "No output can be acknowledged or cleaned up", "ze request plugin restart "+owner.Name())
			}
			state = &routeViewOutput{bus: bus, ledger: make(map[prefixKey]sysribevents.BestChangeEntry)}
			if err := session.setFence(func() error { return state.withdraw(owner) }); err != nil {
				return err
			}
			session.output = state
		}
		emitted := prepared.Batch
		emitted.Changes = make([]sysribevents.BestChangeEntry, 0, len(prepared.Batch.Changes))
		for i := range prepared.Batch.Changes {
			entry := &prepared.Batch.Changes[i]
			key := prefixKey{family: prepared.Batch.Family, prefix: entry.Prefix}
			if entry.Action == routeaction.Withdraw {
				if _, owned := state.ledger[key]; !owned {
					continue
				}
				emitted.Changes = append(emitted.Changes, *entry)
				continue
			}
			// Record before the bus: it can deliver to some subscribers and fail.
			state.ledger[key] = *entry
			emitted.Changes = append(emitted.Changes, *entry)
		}
		shouldEmit := len(emitted.Changes) > 0
		if len(prepared.Batch.Changes) == 0 {
			shouldEmit = true
		}
		if shouldEmit {
			if _, err := sysribevents.BestChange.Emit(state.bus, &emitted); err != nil {
				reason := opererr.Wrap(err, "System routing output could not be delivered", "Every possibly delivered prefix must be withdrawn before forwarding resumes", "ze request plugin restart "+owner.Name())
				if fenceErr := session.invalidate(reason); fenceErr != nil {
					return fenceErr
				}
				return reason
			}
		}
		// Retire withdrawals only after complete acceptance. Later entries for
		// the same prefix still determine its final outstanding state.
		for i := range prepared.Batch.Changes {
			entry := &prepared.Batch.Changes[i]
			key := prefixKey{family: prepared.Batch.Family, prefix: entry.Prefix}
			if entry.Action == routeaction.Withdraw {
				delete(state.ledger, key)
				continue
			}
			state.ledger[key] = *entry
		}
		result = rpc.RouteViewPublishOutput{Session: session.token, Cursor: input.Cursor, Operation: input.Operation}
		state.acknowledged = &routeViewOutputAck{input: prepared, output: result}
		state.lastOperation = input.Operation
		return nil
	})
	return result, err
}

// provePublicationReset MUST run under the authenticated lease, after complete
// payload, cursor and operation validation and before any output emission.
// Read MUST return the explicit same-lease Reset before a consumer can rebuild.
func (session *routeViewSession) provePublicationReset(input routeViewPublication) {
	if session.localReader {
		return
	}
	if session.terminalReason != nil {
		return
	}
	if session.fenceFailure != nil {
		return
	}
	if _, rebuild := errors.AsType[*routeViewRebuildReason](session.resetReason); !rebuild {
		return
	}
	if previous := session.publicationReset; previous != nil {
		if reflect.DeepEqual(previous.input, input) {
			previous.visible = true
		}
		return
	}
	session.publicationReset = &routeViewPublicationReset{input: input, visible: true}
}

// withdraw never scans a RIB or a data plane. Only retained owned prefixes are
// eligible, and a failed emission keeps its entire uncertain batch for retry.
func (output *routeViewOutput) withdraw(owner *process.Process) error {
	// Fencing ends publication, even when an uncertain withdrawal needs retry.
	output.acknowledged = nil
	groups := make(map[family.Family][]sysribevents.BestChangeEntry)
	var failures error
	emit := func(fam family.Family, entries []sysribevents.BestChangeEntry) {
		batch := sysribevents.BestChangeBatch{Family: fam, Changes: entries}
		if _, err := sysribevents.BestChange.Emit(output.bus, &batch); err != nil {
			failures = errors.Join(failures, err)
			return
		}
		for i := range entries {
			delete(output.ledger, prefixKey{family: fam, prefix: entries[i].Prefix})
		}
	}
	for key := range output.ledger {
		entries := groups[key.family]
		if entries == nil {
			entries = make([]sysribevents.BestChangeEntry, 0, rpc.RouteViewRecordsMax)
		}
		entries = append(entries, sysribevents.BestChangeEntry{Action: routeaction.Withdraw, Prefix: key.prefix, TableID: output.ledger[key].TableID})
		if len(entries) == rpc.RouteViewRecordsMax {
			emit(key.family, entries)
			// Event subscribers may retain this storage. The next chunk MUST
			// acquire fresh backing storage, including after a failed emit.
			delete(groups, key.family)
			continue
		}
		groups[key.family] = entries
	}
	for fam, entries := range groups {
		emit(fam, entries)
	}
	if failures != nil {
		return opererr.Wrap(failures, "System routing output cleanup failed", "A replacement cannot own forwarding until every previous output batch is withdrawn", "ze request plugin restart "+owner.Name())
	}
	output.ledger = nil
	return nil
}

func routeViewCloneOutputBatch(batch sysribevents.BestChangeBatch) sysribevents.BestChangeBatch {
	batch.Changes = slices.Clone(batch.Changes)
	for i := range batch.Changes {
		batch.Changes[i] = routeViewCloneOutputEntry(batch.Changes[i])
	}
	return batch
}

func routeViewCloneOutputEntry(entry sysribevents.BestChangeEntry) sysribevents.BestChangeEntry {
	entry.Labels = slices.Clone(entry.Labels)
	entry.ECMPPaths = routeViewCloneOutputHops(entry.ECMPPaths)
	entry.Backup = routeViewCloneOutputHops(entry.Backup)
	return entry
}

func routeViewCloneOutputHops(paths []sysribevents.ECMPPath) []sysribevents.ECMPPath {
	paths = slices.Clone(paths)
	for i := range paths {
		paths[i].Labels = slices.Clone(paths[i].Labels)
	}
	return paths
}

func routeViewValidateOutput(batch *sysribevents.BestChangeBatch) error {
	registered, known := family.LookupFamily(batch.Family.String())
	if !known {
		return routeViewRefusal("Routing publication family is not registered", "Unknown families cannot name forwarding state", routeViewOutputFix)
	}
	if registered != batch.Family {
		return routeViewRefusal("Routing publication family does not match its registered identity", "Forwarding requires the exact registered family", routeViewOutputFix)
	}
	if batch.Changes == nil {
		return routeViewRefusal("Routing publication batch has no changes array", "The batch must carry an explicit complete change list", routeViewOutputFix)
	}
	if len(batch.Changes) > rpc.RouteViewRecordsMax {
		return routeViewRefusal("Routing publication exceeds 4096 changes", "The entire batch must fit the bounded publication contract", "Split route-view-publish into batches of at most 4096 changes")
	}
	if batch.ReplayID != 0 {
		if batch.ReplayID != replay.Broadcast {
			return routeViewRefusal("Routing publication replay identity is not a broadcast replay", "The system forwarding hop preserves the broadcast replay marker", routeViewOutputFix)
		}
	}
	for i := range batch.Changes {
		if err := routeViewValidateOutputEntry(batch.Family, &batch.Changes[i]); err != nil {
			return err
		}
	}
	return nil
}

func routeViewValidateOutputEntry(fam family.Family, entry *sysribevents.BestChangeEntry) error {
	if !entry.Prefix.IsValid() {
		return routeViewRefusal("Routing publication prefix is invalid", "Every forwarding change needs a prefix", routeViewOutputFix)
	}
	if entry.Prefix != entry.Prefix.Masked() {
		return routeViewRefusal("Routing publication prefix contains host bits", "A forwarding key must be a masked prefix", routeViewOutputFix)
	}
	if entry.Prefix.Addr().Is4In6() {
		return routeViewRefusal("Routing publication prefix is IPv4-mapped", "Mapped addresses cannot identify an IPv6 forwarding prefix", routeViewOutputFix)
	}
	if entry.Prefix.Addr().Zone() != "" {
		return routeViewRefusal("Routing publication prefix has an interface zone", "Zones belong to gateways rather than destination prefixes", routeViewOutputFix)
	}
	switch fam.AFI {
	case family.AFIIPv4:
		if !entry.Prefix.Addr().Is4() {
			return routeViewRefusal("Routing publication IPv4 prefix has another family", "A family cannot own another family's destination", routeViewOutputFix)
		}
	case family.AFIIPv6:
		if !entry.Prefix.Addr().Is6() {
			return routeViewRefusal("Routing publication IPv6 prefix has another family", "A family cannot own another family's destination", routeViewOutputFix)
		}
	case family.AFIL2VPN, family.AFIBGPLS:
		return routeViewRefusal("Routing publication family has no IP forwarding prefix", "The system forwarding contract carries IP prefixes", routeViewOutputFix)
	default: // AFI is open at the plugin boundary.
		return routeViewRefusal("Routing publication address family is unknown", "An unknown family cannot identify a forwarding prefix", routeViewOutputFix)
	}
	if entry.TableID != 0 {
		if entry.TableID != 254 {
			return routeViewRefusal("Routing publication names a table outside the system RIB", "This session owns only main-table forwarding", routeViewOutputFix)
		}
	}
	if entry.Protocol != "" {
		if _, known := redistevents.ProtocolIDOf(entry.Protocol); !known {
			return routeViewRefusal("Routing publication protocol is not registered", "Unknown protocol names cannot describe forwarding output", routeViewOutputFix)
		}
	}
	switch entry.Action {
	case routeaction.Withdraw:
	case routeaction.Add, routeaction.Update:
		if _, known := redistevents.ProtocolIDOf(entry.Protocol); !known {
			return routeViewRefusal("Routing publication protocol is not registered", "Unknown protocol names cannot acquire forwarding ownership", routeViewOutputFix)
		}
		if protocolInstalledByOS(entry.Protocol) {
			return routeViewRefusal("Routing publication protocol is installed by the operating system", "This session cannot acquire output ownership of operating-system routes", routeViewOutputFix)
		}
	case routeaction.Unspecified, routeaction.Del:
		return routeViewRefusal("Routing publication action is not add, update or withdraw", "Only system best-change actions can change forwarding", routeViewOutputFix)
	default: // Action is open at the plugin boundary.
		return routeViewRefusal("Routing publication action is unknown", "An unknown action cannot change forwarding", routeViewOutputFix)
	}
	if utf8.RuneCountInString(entry.Protocol) > 64 {
		return routeViewRefusal("Routing publication protocol exceeds its name limit", "Protocol names contain at most 64 characters", routeViewOutputFix)
	}
	if err := routeViewValidateOutputTarget(entry.NextHop, entry.Interface, entry.Labels); err != nil {
		return err
	}
	if err := projectionSID(entry.SRv6SID); err != nil {
		return opererr.Wrap(err, "Routing publication Service SID is invalid", "The entire batch was refused", routeViewOutputFix)
	}
	switch entry.RouteType {
	case 0, routetype.Unicast:
		if entry.Action != routeaction.Withdraw {
			if !entry.NextHop.IsValid() {
				if entry.Interface == "" {
					if !entry.SRv6SID.IsValid() {
						if len(entry.ECMPPaths) == 0 {
							return routeViewRefusal("Routing publication has no forwarding target", "Unicast output needs a gateway, device, Service SID or complete ECMP group", routeViewOutputFix)
						}
					}
				}
			}
		}
	case routetype.Blackhole, routetype.Unreachable, routetype.Prohibit:
	default: // Route type is open at the plugin boundary.
		return routeViewRefusal("Routing publication route type is unknown", "An unknown forwarding action cannot be installed", routeViewOutputFix)
	}
	for _, hops := range [][]sysribevents.ECMPPath{entry.ECMPPaths, entry.Backup} {
		if len(hops) > sysribevents.MaxECMPPaths {
			return routeViewRefusal("Routing publication forwarding group exceeds 128 paths", "The existing FIB group limit applies to primary and backup groups", routeViewOutputFix)
		}
		for _, hop := range hops {
			if !hop.NextHop.IsValid() {
				if hop.Interface == "" {
					return routeViewRefusal("Routing publication forwarding member has no target", "Each member needs a gateway or device", routeViewOutputFix)
				}
			}
			if err := routeViewValidateOutputTarget(hop.NextHop, hop.Interface, hop.Labels); err != nil {
				return err
			}
		}
	}
	return nil
}

func routeViewValidateOutputTarget(addr netip.Addr, device string, labels []uint32) error {
	if !utf8.ValidString(device) {
		return routeViewRefusal("Routing publication target contains invalid UTF-8", "Interface and gateway zone text must retain the same identity through both transports", routeViewOutputFix)
	}
	if !utf8.ValidString(addr.Zone()) {
		return routeViewRefusal("Routing publication target contains invalid UTF-8", "Interface and gateway zone text must retain the same identity through both transports", routeViewOutputFix)
	}
	if utf8.RuneCountInString(device) > 255 {
		return routeViewRefusal("Routing publication interface name exceeds 255 characters", "The interface must fit the declared forwarding contract", routeViewOutputFix)
	}
	if addr.Is4In6() {
		return routeViewRefusal("Routing publication gateway is IPv4-mapped", "The forwarding target must retain an unambiguous IP family", routeViewOutputFix)
	}
	if utf8.RuneCountInString(addr.Zone()) > 255 {
		return routeViewRefusal("Routing publication gateway zone is too long", "A scoped gateway must fit its 255-character address field", routeViewOutputFix)
	}
	if addr.IsValid() {
		var text [2048]byte
		if utf8.RuneCount(addr.AppendTo(text[:0])) > 255 {
			return routeViewRefusal("Routing publication gateway exceeds 255 characters", "The complete scoped address must fit its declared field", routeViewOutputFix)
		}
	}
	if len(labels) > 0 {
		if err := mplsfib.ValidateOutLabels(labels); err != nil {
			return opererr.Wrap(err, "Routing publication label stack is invalid", "The entire batch was refused before forwarding", routeViewOutputFix)
		}
	}
	return nil
}
