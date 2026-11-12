// Design: docs/architecture/traffic/cos-dynamic.md -- acknowledged CoA map changes
// Related: handler.go -- the owner serializes events and transactions with mu

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
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const sessionCommand = "request class-of-service session"

// sessionTransactionRequest is the raw RPC value. Unknown fields, excessive
// payloads and fields belonging to another operation are refused at admission.
type sessionTransactionRequest struct {
	Operation string `json:"operation"`
	Token     string `json:"token,omitempty"`
	TunnelID  uint16 `json:"tunnel-id,omitempty"`
	SessionID uint16 `json:"session-id,omitempty"`
	Interface string `json:"interface,omitempty"`
	Profile   string `json:"profile,omitempty"`
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

// preparedCoSChange is built only after prepareSessionChange reads the actual
// maps. Its zero value is uninitialized and never installed in pending.
type preparedCoSChange struct {
	token           string
	peer            netip.Addr
	key             sessionKey
	accessInterface string
	profileName     string
	prior           *sessionCoSState
	ingress         map[uint32]uint32
	egress          map[uint32]uint32
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
	if h.pending != nil {
		if time.Now().After(h.pending.prepared().expires) {
			if err := h.rollbackSessionChange(); err != nil {
				return sessionCommandFailure(err)
			}
		}
	}
	if request.Operation == "prepare" {
		result, err := h.prepareSessionChange(request, address)
		if err != nil {
			return sessionCommandFailure(err)
		}
		return "done", result, nil
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
	return "done", sessionTransactionResult{Token: request.Token}, nil
}

func (h *cosHandler) prepareSessionChange(request sessionTransactionRequest, peer netip.Addr) (sessionTransactionResult, error) {
	if request.Token != "" {
		return sessionTransactionResult{}, errors.New("prepare does not accept a token")
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
	link, err := h.readLink(request.Interface)
	if err != nil {
		return sessionTransactionResult{}, fmt.Errorf("read access interface %q: %w", request.Interface, err)
	}
	if link == nil {
		return sessionTransactionResult{}, errors.New("the access interface backend returned no link")
	}
	if link.Type != "vlan" {
		return sessionTransactionResult{}, fmt.Errorf("the interface backend does not report VLAN QoS maps for access interface %q", request.Interface)
	}
	change := &preparedCoSChange{
		token: rand.Text(), peer: peer, key: sessionKey{tunnelID: request.TunnelID, sessionID: request.SessionID},
		accessInterface: request.Interface, profileName: request.Profile,
		ingress: maps.Clone(link.IngressQoSMap), egress: maps.Clone(link.EgressQoSMap),
		profile: coreCos.Profile{IngressMap: maps.Clone(profile.IngressMap), EgressMap: maps.Clone(profile.EgressMap)},
		expires: time.Now().Add(time.Minute),
	}
	if value, loaded := sessionStore.Load(change.key); loaded {
		state, valid := value.(sessionCoSState)
		if !valid {
			return sessionTransactionResult{}, errors.New("the session class-of-service state is unreadable")
		}
		if state.accessInterface != request.Interface {
			return sessionTransactionResult{}, errors.New("the session uses a different access interface")
		}
		state.staticIngress = maps.Clone(state.staticIngress)
		state.staticEgress = maps.Clone(state.staticEgress)
		change.prior = &state
	}
	h.pending = change
	h.completed = completedCoSChange{}
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
	h.pending = &attemptedCoSChange{change}
	if err := h.updateQoS(change.accessInterface, change.profile.IngressMap, change.profile.EgressMap); err != nil {
		rollbackErr := h.updateQoS(change.accessInterface, change.ingress, change.egress)
		return errors.Join(fmt.Errorf("install class-of-service profile %q: %w", change.profileName, err), rollbackErr)
	}
	state := sessionCoSState{accessInterface: change.accessInterface, profileName: change.profileName, commandToken: token,
		staticIngress: maps.Clone(change.ingress), staticEgress: maps.Clone(change.egress)}
	if change.prior != nil {
		state.staticIngress = maps.Clone(change.prior.staticIngress)
		state.staticEgress = maps.Clone(change.prior.staticEgress)
	}
	sessionStore.Store(change.key, state)
	h.pending = &appliedCoSChange{change}
	return nil
}

// forgetSessionChange invalidates tokens when a session leaves. A delayed apply
// or compensation must never recreate the departed session's dynamic policy.
func (h *cosHandler) forgetSessionChange(key sessionKey) *preparedCoSChange {
	var change *preparedCoSChange
	if h.pending != nil {
		if h.pending.prepared().key == key {
			change = h.pending.prepared()
			h.pending = nil
		}
	}
	if h.completed.change != nil {
		if h.completed.change.key == key {
			if change == nil {
				change = h.completed.change
			}
			h.completed = completedCoSChange{}
		}
	}
	return change
}

func (h *cosHandler) rollbackSessionToken(token string, peer netip.Addr) error {
	if h.completed.token == token {
		if h.completed.peer == peer {
			if h.completed.operation == completionRolledBack {
				return nil
			}
			if h.completed.operation == completionCommitted {
				h.pending = &appliedCoSChange{h.completed.change}
				return h.rollbackSessionChange()
			}
		}
	}
	if _, err := h.pendingSessionChange(token, peer); err != nil {
		return err
	}
	return h.rollbackSessionChange()
}

func (h *cosHandler) rollbackSessionChange() error {
	change := h.pending.prepared()
	if h.pending.restoreRequired() {
		if err := h.updateQoS(change.accessInterface, change.ingress, change.egress); err != nil {
			return fmt.Errorf("restore access interface %q: %w", change.accessInterface, err)
		}
	}
	if change.prior == nil {
		sessionStore.Delete(change.key)
	} else {
		sessionStore.Store(change.key, *change.prior)
	}
	h.completed = completedCoSChange{token: change.token, peer: change.peer, operation: completionRolledBack}
	h.pending = nil
	return nil
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
