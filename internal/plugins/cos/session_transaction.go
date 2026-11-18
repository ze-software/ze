// Design: docs/architecture/traffic/cos-dynamic.md -- acknowledged CoA map changes
// Related: shared_access.go -- shared membership and original native baseline

//go:build ze_l2tp

package cos

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/subscriberid"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const sessionCommand = "request class-of-service session"

// sessionTransactionRequest is the raw RPC value. Unknown fields, excessive
// payloads and fields belonging to another operation are refused at admission.
type sessionTransactionRequest struct {
	SubscriberID string `json:"subscriber-id,omitempty"`
	Operation    string `json:"operation"`
	Token        string `json:"token,omitempty"`
	TunnelID     uint16 `json:"tunnel-id,omitempty"`
	SessionID    uint16 `json:"session-id,omitempty"`
	Interface    string `json:"interface,omitempty"`
	Profile      string `json:"profile,omitempty"`
}

type sessionTransactionResult struct {
	Token string `json:"token"`
}

// cosSessionChange has prepared, attempted and applied variants. nil means no
// pending change.
// A prepared change MUST be applied then committed, or rolled back. An applied
// change MUST be committed or rolled back before another prepare is admitted.
type cosSessionChange interface {
	prepared() *preparedCoSChange
	restoreRequired() bool
}

// preparedCoSChange is published only after actual maps and a compatible user
// reservation have been captured. The nil/zero value is never a pending owner.
type preparedCoSChange struct {
	token           string
	peer            netip.Addr
	key             sessionKey
	labels          sessionDiagnostic
	accessInterface string
	profileName     string
	access          *sharedCoSAccess
	prior           *sessionCoSState
	priorProfile    sharedCoSProfile
	ingress         map[uint32]uint32
	egress          map[uint32]uint32
	before          sharedCoSProfile
	desired         sharedCoSProfile
	profile         coreCos.Profile
	expires         time.Time
}

func (p *preparedCoSChange) prepared() *preparedCoSChange { return p }
func (*preparedCoSChange) restoreRequired() bool          { return false }

// attemptedCoSChange records a write that may have partially changed the VLAN.
// It retains the original maps even if the first compensation also fails.
type attemptedCoSChange struct{ *preparedCoSChange }

func (*attemptedCoSChange) restoreRequired() bool { return true }

// appliedCoSChange can only be produced by a successful backend update.
type appliedCoSChange struct{ *preparedCoSChange }

func (*appliedCoSChange) restoreRequired() bool { return true }

type completionKind uint8

const (
	completionUnspecified completionKind = iota
	completionRolledBack
	completionCommitted
)

type completedCoSChange struct {
	token     string
	peer      netip.Addr
	operation completionKind
	change    *preparedCoSChange
}

func sessionCommandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{{Name: sessionCommand, Hidden: true, Args: []string{"transaction"},
		ShortHelp: "Apply or restore one subscriber's class-of-service maps.", Shape: "doc"}}
}

func registerSessionCommand(handlers map[string]func([]string, string) (string, any, error)) {
	handlers[sessionCommand] = executeSessionTransaction
}

func readSessionLink(name string) (*iface.InterfaceInfo, error) {
	device, err := iface.ResolveDevice(name)
	if err != nil {
		return nil, err
	}
	return iface.GetLink(device)
}

func executeSessionTransaction(args []string, peer string) (string, any, error) {
	if dynamicHandler == nil {
		return sessionCommandFailure(errors.New("the dynamic class-of-service owner is not running"))
	}
	return dynamicHandler.executeSessionTransaction(args, peer)
}

func sessionCommandFailure(cause error) (string, any, error) {
	refusal := opererr.Wrap(cause, "The subscriber class-of-service change did not finish",
		"The request cannot be acknowledged until its VLAN maps are installed or restored",
		"show interface")
	response := plugin.OperatorErrorResponse(refusal)
	return response.Status, response, refusal
}

func (h *cosHandler) executeSessionTransaction(args []string, peer string) (string, any, error) {
	if len(args) != 2 {
		return sessionCommandFailure(errors.New("expected transaction followed by one JSON value"))
	}
	if args[0] != "transaction" {
		return sessionCommandFailure(errors.New("expected the transaction keyword"))
	}
	if len(args[1]) > 8192 {
		return sessionCommandFailure(errors.New("transaction exceeds 8192 bytes"))
	}
	var request sessionTransactionRequest
	decoder := json.NewDecoder(strings.NewReader(args[1]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return sessionCommandFailure(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return sessionCommandFailure(errors.New("transaction must contain exactly one JSON value"))
	}
	if request.Operation != "prepare" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(args[1]), &fields); err != nil {
			return sessionCommandFailure(err)
		}
		for field := range fields {
			if field == "operation" {
				continue
			}
			if field == "token" {
				continue
			}
			return sessionCommandFailure(fmt.Errorf("only prepare accepts %q", field))
		}
	}
	address, err := netip.ParseAddr(peer)
	if err != nil {
		return sessionCommandFailure(errors.New("transaction requires its RADIUS client address"))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return sessionCommandFailure(errors.New("the original class-of-service runner is stopped"))
	}
	if h.pending != nil {
		if time.Now().After(h.pending.prepared().expires) {
			if err := h.rollbackSessionCleanup(); err != nil {
				return sessionCommandFailure(err)
			}
		}
	}
	if request.Operation == "prepare" {
		result, err := h.prepareSessionChange(request, address)
		if err != nil {
			return sessionCommandFailure(err)
		}
		return rpc.StatusDone, result, nil
	}
	operations := map[string]func(string, netip.Addr) error{
		"apply": h.applySessionChange, "rollback": h.rollbackSessionToken, "commit": h.commitSessionChange,
	}
	operation, found := operations[request.Operation]
	if !found {
		return sessionCommandFailure(fmt.Errorf("unknown operation %q", request.Operation))
	}
	if err := operation(request.Token, address); err != nil {
		return sessionCommandFailure(err)
	}
	return rpc.StatusDone, sessionTransactionResult{Token: request.Token}, nil
}

func (h *cosHandler) prepareSessionChange(request sessionTransactionRequest, peer netip.Addr) (sessionTransactionResult, error) {
	if request.Token != "" {
		return sessionTransactionResult{}, errors.New("prepare does not accept a token")
	}
	birth, err := subscriberid.ParseL2TPBirth(request.SubscriberID)
	if err != nil {
		return sessionTransactionResult{}, fmt.Errorf("prepare requires canonical subscriber-id: %w", err)
	}
	if request.Interface == "" {
		return sessionTransactionResult{}, errors.New("prepare requires the access interface")
	}
	if request.SessionID == 0 {
		return sessionTransactionResult{}, errors.New("prepare requires a session-id")
	}
	if h.pending != nil {
		return sessionTransactionResult{}, errors.New("a class-of-service transaction is still pending")
	}
	profile, found := coreCos.Lookup(request.Profile)
	if !found {
		return sessionTransactionResult{}, fmt.Errorf("class-of-service profile %q does not exist", request.Profile)
	}
	desired, err := newSharedCoSProfile(profile)
	if err != nil {
		return sessionTransactionResult{}, err
	}
	access, err := h.captureSharedAccess(request.Interface)
	if err != nil {
		return sessionTransactionResult{}, err
	}
	accepted := false
	defer func() {
		if !accepted {
			h.discardUnusedSharedAccess(access)
		}
	}()
	key := sessionKey{birth: birth}
	prior, err := h.sharedSessionState(key, access)
	if err != nil {
		return sessionTransactionResult{}, err
	}
	if err := access.compatible(key, desired); err != nil {
		return sessionTransactionResult{}, err
	}
	actual := cloneSharedCoSMaps(access.original)
	if len(access.users) > 0 {
		actual, err = readSharedAccessMaps(access.readLink, request.Interface)
		if err != nil {
			return sessionTransactionResult{}, err
		}
	}
	before, err := newSharedCoSProfile(actual)
	if err != nil {
		return sessionTransactionResult{}, err
	}
	if err := access.compatible(key, before); err != nil {
		return sessionTransactionResult{}, fmt.Errorf("the actual compensation maps conflict with a surviving subscriber: %w", err)
	}
	change := &preparedCoSChange{
		token: rand.Text(), peer: peer, key: key, accessInterface: request.Interface,
		labels:      sessionDiagnostic{tunnelID: request.TunnelID, sessionID: request.SessionID},
		profileName: request.Profile, access: access, prior: prior,
		ingress: maps.Clone(actual.IngressMap), egress: maps.Clone(actual.EgressMap),
		before: before, desired: desired, profile: cloneSharedCoSMaps(profile),
		expires: time.Now().Add(time.Minute),
	}
	if prior != nil {
		previous, admitted := access.users[key]
		if !admitted {
			return sessionTransactionResult{}, errors.New("the prior class-of-service state has no admitted shared user")
		}
		change.priorProfile = previous
	}
	h.pending = change
	h.completed = completedCoSChange{}
	accepted = true
	return sessionTransactionResult{Token: change.token}, nil
}

func (h *cosHandler) pendingSessionChange(token string, peer netip.Addr) (*preparedCoSChange, error) {
	if token == "" {
		return nil, errors.New("transaction token is required")
	}
	if h.pending == nil {
		return nil, errors.New("transaction token is no longer pending")
	}
	change := h.pending.prepared()
	if change.token != token {
		return nil, errors.New("transaction token does not match the pending change")
	}
	if change.peer != peer {
		return nil, errors.New("transaction belongs to another RADIUS client")
	}
	return change, nil
}

func (h *cosHandler) applySessionChange(token string, peer netip.Addr) error {
	if h.completed.token == token {
		if h.completed.peer == peer {
			if h.completed.operation == completionCommitted {
				return nil
			}
		}
	}
	change, err := h.pendingSessionChange(token, peer)
	if err != nil {
		return err
	}
	if _, applied := h.pending.(*appliedCoSChange); applied {
		return nil
	}
	if _, failed := h.pending.(*failedCoSChange); failed {
		return errors.New("the original transaction needs confirmed compensation; run ze request class-of-service cleanup before applying another policy")
	}
	if h.accesses[change.accessInterface] != change.access {
		return errors.New("the prepared access no longer has its original map owner")
	}
	if err := change.access.compatible(change.key, change.desired); err != nil {
		return err
	}
	h.pending = &attemptedCoSChange{change}
	if err := change.access.write(change.accessInterface, change.profile.IngressMap, change.profile.EgressMap); err != nil {
		rollbackErr := change.access.write(change.accessInterface, change.ingress, change.egress)
		if rollbackErr == nil {
			change.access.installed = cloneSharedCoSMaps(coreCos.Profile{IngressMap: change.ingress, EgressMap: change.egress})
			return fmt.Errorf("install class-of-service profile %q: %w", change.profileName, err)
		}
		failure := errors.Join(fmt.Errorf("install class-of-service profile %q: %w", change.profileName, err),
			fmt.Errorf("restore previous access maps after failed profile: %w", rollbackErr))
		return h.retainSessionCleanupFailure(change, failure)
	}
	change.access.installed = cloneSharedCoSMaps(change.profile)
	h.keepSharedSession(change.key, change.access, change.profileName, token, change.desired)
	h.pending = &appliedCoSChange{change}
	return nil
}

// Departure invalidates only this original user's command tokens. Its map
// owner/cleanup remains separately retained until native restoration succeeds.
func (h *cosHandler) forgetSessionChange(key sessionKey) {
	if h.pending != nil {
		if h.pending.prepared().key == key {
			h.pending = nil
		}
	}
	if h.completed.change != nil {
		if h.completed.change.key == key {
			h.completed = completedCoSChange{}
		}
	}
}

func (h *cosHandler) rollbackSessionToken(token string, peer netip.Addr) error {
	if h.completed.token == token {
		if h.completed.peer == peer {
			if h.completed.operation == completionRolledBack {
				return h.acknowledgeCompletedRollback()
			}
			if h.completed.operation == completionCommitted {
				if h.pending != nil {
					return errors.New("another class-of-service transaction is still pending")
				}
				if err := h.repairSharedUncertainty(h.completed.change.access); err != nil {
					return err
				}
				h.pending = &appliedCoSChange{h.completed.change}
				return h.rollbackSessionCleanup()
			}
		}
	}
	if _, err := h.pendingSessionChange(token, peer); err != nil {
		return err
	}
	return h.rollbackSessionCleanup()
}

// rollbackSessionChange retains native causes without publishing them. Its
// caller MUST publish a native failure once at its command or event boundary;
// cleanupClear separately owns a failed publication after native confirmation.
func (h *cosHandler) rollbackSessionChange() error {
	change := h.pending.prepared()
	if err := h.repairSharedUncertainty(change.access); err != nil {
		return err
	}
	if h.accesses[change.accessInterface] != change.access {
		return errors.New("the rollback no longer has its original access map owner")
	}
	if h.pending.restoreRequired() {
		if err := change.access.compatible(change.key, change.before); err != nil {
			return err
		}
		if err := change.access.write(change.accessInterface, change.ingress, change.egress); err != nil {
			return h.retainSessionCleanupCause(change, fmt.Errorf("restore access interface %q: %w", change.accessInterface, err))
		}
		change.access.installed = cloneSharedCoSMaps(coreCos.Profile{IngressMap: change.ingress, EgressMap: change.egress})
	}
	h.restoreSharedMembership(change)
	h.completed = completedCoSChange{token: change.token, peer: change.peer, operation: completionRolledBack, change: change}
	h.pending = nil
	h.discardUnusedSharedAccess(change.access)
	return h.clearSharedCleanupIfConfirmed(change.access)
}

func (h *cosHandler) commitSessionChange(token string, peer netip.Addr) error {
	if h.completed.token == token {
		if h.completed.peer == peer {
			if h.completed.operation == completionCommitted {
				return nil
			}
		}
	}
	change, err := h.pendingSessionChange(token, peer)
	if err != nil {
		return err
	}
	if _, applied := h.pending.(*appliedCoSChange); !applied {
		return errors.New("the class-of-service change has not been applied")
	}
	h.completed = completedCoSChange{token: token, peer: peer, operation: completionCommitted, change: change}
	h.pending = nil
	recordCoAChanged()
	return nil
}

// failedCoSChange retains only a failed native compensation. A healthy pending
// change never has this variant, so the operator cleanup cannot roll it back.
// The first and latest causes bound repeated failures without losing custody.
type failedCoSChange struct {
	*preparedCoSChange
	first  error
	latest error
}

func (*failedCoSChange) restoreRequired() bool { return true }

// rollbackSessionCleanup owns command-context publication. Departure callers
// MUST use rollbackSessionChange and publish its raw failure at their event or
// cleanup boundary. A failed CLEAR already logs and retains confirmed maps.
func (h *cosHandler) rollbackSessionCleanup() error {
	err := h.rollbackSessionChange()
	if err == nil {
		return nil
	}
	failure, failed := h.pending.(*failedCoSChange)
	if !failed {
		return err
	}
	return h.cleanupFailure(failure.access, failure.labels, err)
}

// Caller MUST hold h.mu and pass the actual native failure, never a previously
// published operator-error wrapper. One warning remains on the original owner.
func (h *cosHandler) retainSessionCleanupFailure(change *preparedCoSChange, cause error) error {
	return h.cleanupFailure(change.access, change.labels, h.retainSessionCleanupCause(change, cause))
}

// retainSessionCleanupCause keeps bounded raw causes. Callers MUST pass the
// result to their own single publication boundary, never back into this keeper.
func (h *cosHandler) retainSessionCleanupCause(change *preparedCoSChange, cause error) error {
	failure, retained := h.pending.(*failedCoSChange)
	if !retained {
		failure = &failedCoSChange{preparedCoSChange: change, first: cause}
		h.pending = failure
	} else {
		failure.latest = cause
	}
	return errors.Join(failure.first, failure.latest)
}

// acknowledgeCompletedRollback is reached only after the exact completed
// rollback token and original authenticated peer matched. An already released,
// cleared old access may acknowledge that historical result after a new owner
// uses its name; it must not write or clear anything on that replacement.
// Caller MUST hold h.mu and have checked completionRolledBack/token/peer.
func (h *cosHandler) acknowledgeCompletedRollback() error {
	access := h.completed.change.access
	if current := h.accesses[access.name]; current != nil {
		if current.sharedOwner() != access {
			if len(access.users) != 0 {
				return errors.New("the old completed rollback still retains native members")
			}
			if access.episode != nil {
				return errors.New("the old completed rollback still retains its original warning episode")
			}
			if _, unraised := access.warning.(sharedCoSWarningUnraised); !unraised {
				return errors.New("the old completed rollback still retains warning clearance")
			}
			// A different current owner could be admitted only after this old
			// target's final release removed its original access record. Close
			// is idempotent on that original cleared reservation alone.
			return access.reportScope.Close()
		}
	}
	return h.clearSharedCleanupIfConfirmed(access)
}
