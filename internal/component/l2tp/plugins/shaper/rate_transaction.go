// Design: docs/research/l2tpv2-ze-integration.md -- confirmed CoA shaping changes
// Related: shaper.go -- serialized shaper state and backend application

package l2tpshaper

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/traffic"
	"github.com/ze-software/ze/internal/core/opererr"
)

const rateCommand = "request l2tp shaper rate"

const rateTransactionLifetime = time.Minute

// rateRequest is the command's raw boundary value. Only prepare consumes the
// requested rate; every later operation consumes the peer-bound owner token.
type rateRequest struct {
	Operation    string `json:"operation"`
	Token        string `json:"token,omitempty"`
	TunnelID     uint16 `json:"tunnel-id,omitempty"`
	SessionID    uint16 `json:"session-id,omitempty"`
	Interface    string `json:"interface,omitempty"`
	DownloadRate uint64 `json:"download-rate,omitempty"`
	UploadRate   uint64 `json:"upload-rate,omitempty"`
}

// rateChange holds a prepared change, an attempted or applied write, or its
// terminal outcome. nil is the absent owner transaction.
type rateChange interface {
	transaction() *rateTransaction
	restoreRequired() bool
}

// rateTransaction holds a validated prepared change. Its zero is uninitialized
// and is never admitted into the owner's transaction slots.
// A shaper retains at most one pending transaction and one last outcome. The
// snapshots are the complete inputs whose backend Apply last succeeded, not a
// claim that arbitrary changes made outside the owner were read from the kernel.
type rateTransaction struct {
	request rateRequest
	token   string
	peer    netip.Addr
	before  traffic.InterfaceQoS
	after   traffic.InterfaceQoS
	expires time.Time
}

func (tx *rateTransaction) transaction() *rateTransaction { return tx }
func (*rateTransaction) restoreRequired() bool            { return false }

type rateAttemptedChange struct{ *rateTransaction }

func (*rateAttemptedChange) restoreRequired() bool { return true }

type rateAppliedChange struct{ *rateTransaction }

func (*rateAppliedChange) restoreRequired() bool { return true }

type rateRolledBackChange struct{ *rateTransaction }

func (*rateRolledBackChange) restoreRequired() bool { return false }

type rateCommittedChange struct{ *rateTransaction }

func (*rateCommittedChange) restoreRequired() bool { return true }

func (s *shaperPlugin) executeRate(args []string, peer string) (any, error) {
	data, err := s.rateOperation(args, peer)
	if err != nil {
		return nil, opererr.Wrap(err, "Subscriber rate change failed", "The requested change was not confirmed by the traffic backend", "show l2tp shaper")
	}
	return data, nil
}

// rateOperation serializes commands with all other shaper writes. The caller
// MUST retain a prepare token until commit or a successful rollback; those
// operations MUST name the same source peer that prepared the change.
// RFC 5176 Section 2.3: "State changes resulting from a CoA-Request MUST be
// atomic: if the CoA-Request is successful for all matching sessions, the NAS
// MUST send a CoA-ACK in reply, and all requested authorization changes MUST be
// made."
func (s *shaperPlugin) rateOperation(args []string, peer string) (any, error) {
	if len(args) != 2 {
		return nil, errors.New("rate transaction requires the transaction keyword and one JSON value")
	}
	if args[0] != "transaction" {
		return nil, errors.New("rate transaction requires the transaction keyword")
	}
	if len(args[1]) > 8192 {
		return nil, errors.New("rate transaction exceeds 8192 bytes")
	}
	if peer == "" {
		return nil, errors.New("rate transaction has no authenticated source peer")
	}
	source, err := netip.ParseAddr(peer)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization source address: %w", err)
	}
	var request rateRequest
	decoder := json.NewDecoder(strings.NewReader(args[1]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("rate transaction must contain exactly one JSON object")
	}
	if request.Operation == "prepare" {
		if request.Token != "" {
			return nil, errors.New("prepare must not carry an existing transaction token")
		}
	} else {
		followup := rateRequest{Operation: request.Operation, Token: request.Token}
		if request != followup {
			return nil, errors.New("only prepare may carry subscriber identity and rates")
		}
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	operations := map[string]func(rateRequest, netip.Addr) (any, error){
		"prepare":  s.prepareRate,
		"apply":    s.applyRate,
		"rollback": s.rollbackRate,
		"commit":   s.commitRate,
	}
	operation, found := operations[request.Operation]
	if !found {
		return nil, fmt.Errorf("unknown rate transaction operation %q", request.Operation)
	}
	if s.ratePending != nil {
		if !s.rateTime().Before(s.ratePending.transaction().expires) {
			if _, err := s.restoreRate(s.ratePending); err != nil {
				return nil, fmt.Errorf("recover expired rate transaction: %w", err)
			}
		}
	}
	return operation(request, source.Unmap())
}

// rateTime selects the owner's private clock. Its nil clock is the production
// system clock; a test may supply a deterministic clock without worker timers.
func (s *shaperPlugin) rateTime() time.Time {
	if s.rateNow != nil {
		return s.rateNow()
	}
	return time.Now()
}

func (s *shaperPlugin) prepareRate(request rateRequest, peer netip.Addr) (any, error) {
	if s.ratePending != nil {
		tx := s.ratePending.transaction()
		if tx.peer == peer && tx.request == request {
			return map[string]string{"token": tx.token}, nil
		}
		return nil, errors.New("another subscriber rate transaction is pending")
	}
	if traffic.GetBackend() == nil {
		return nil, errNoTrafficBackendLoadedConfigureTraffic
	}
	before, found := s.confirmed[request.Interface]
	if !found {
		return nil, fmt.Errorf("interface %q has no confirmed shaping state", request.Interface)
	}
	key := sessionKey{tunnelID: request.TunnelID, sessionID: request.SessionID}
	if value, found := s.sessions.Load(key); found {
		state, valid := value.(sessionState)
		if !valid {
			return nil, errors.New("subscriber shaping state is unavailable")
		}
		if state.iface != request.Interface {
			return nil, errors.New("subscriber identity does not own the requested shaping interface")
		}
	}
	if request.DownloadRate == 0 {
		return nil, errors.New("subscriber download rate must be positive")
	}
	if request.DownloadRate%8 != 0 {
		return nil, errors.New("subscriber download rate must be a whole number of bytes per second")
	}
	upload := request.UploadRate
	if upload == 0 {
		upload = before.Ingress.RateBps
	}
	if err := traffic.ValidateRate(upload); err != nil {
		return nil, err
	}
	if upload%8 != 0 {
		return nil, errors.New("subscriber upload rate must be a whole number of bytes per second")
	}
	after, err := subscriberQoS(request.Interface, before.Qdisc.Type, request.DownloadRate, upload)
	if err != nil {
		return nil, err
	}
	before.Qdisc.Classes = slices.Clone(before.Qdisc.Classes)
	tx := &rateTransaction{request: request, token: rand.Text(), peer: peer, before: before, after: after,
		expires: s.rateTime().Add(rateTransactionLifetime)}
	s.ratePending = tx
	return map[string]string{"token": tx.token}, nil
}

func (s *shaperPlugin) rateToken(request rateRequest, peer netip.Addr) (rateChange, error) {
	transactions := []rateChange{s.ratePending}
	if s.ratePending == nil {
		transactions = append(transactions, s.rateLast)
	}
	for _, change := range transactions {
		if change == nil {
			continue
		}
		tx := change.transaction()
		if tx.token == request.Token && tx.peer == peer {
			return change, nil
		}
	}
	return nil, errors.New("rate transaction token does not belong to this source peer")
}

func (s *shaperPlugin) applyRate(request rateRequest, peer netip.Addr) (any, error) {
	change, err := s.rateToken(request, peer)
	if err != nil {
		return nil, err
	}
	tx := change.transaction()
	if _, applied := change.(*rateAppliedChange); applied {
		return map[string]string{"token": tx.token}, nil
	}
	if _, prepared := change.(*rateTransaction); !prepared {
		return nil, errors.New("rate transaction is no longer prepared")
	}
	backend := traffic.GetBackend()
	if backend == nil {
		return nil, errNoTrafficBackendLoadedConfigureTraffic
	}
	// Apply may fail after changing one direction. Mark it before the call so
	// rollback also compensates that partial failure.
	attempted := &rateAttemptedChange{tx}
	s.ratePending = attempted
	if err := backend.Apply(context.Background(), map[string]traffic.InterfaceQoS{tx.after.Interface: tx.after}); err != nil {
		_, restoreErr := s.restoreRate(attempted)
		return nil, errors.Join(err, restoreErr)
	}
	s.rememberQoS(tx.after)
	s.publishRate(tx, tx.after)
	s.ratePending = &rateAppliedChange{tx}
	return map[string]string{"token": tx.token}, nil
}

func (s *shaperPlugin) rollbackRate(request rateRequest, peer netip.Addr) (any, error) {
	change, err := s.rateToken(request, peer)
	if err != nil {
		return nil, err
	}
	if _, restored := change.(*rateRolledBackChange); restored {
		return map[string]string{"token": change.transaction().token}, nil
	}
	return s.restoreRate(change)
}

func (s *shaperPlugin) restoreRate(change rateChange) (any, error) {
	tx := change.transaction()
	if change.restoreRequired() {
		backend := traffic.GetBackend()
		if backend == nil {
			return nil, errNoTrafficBackendLoadedConfigureTraffic
		}
		if err := backend.Apply(context.Background(), map[string]traffic.InterfaceQoS{tx.before.Interface: tx.before}); err != nil {
			return nil, fmt.Errorf("restore previous subscriber shaping: %w", err)
		}
		s.rememberQoS(tx.before)
		s.publishRate(tx, tx.before)
	}
	s.rateLast, s.ratePending = &rateRolledBackChange{tx}, nil
	return map[string]string{"token": tx.token}, nil
}

func (s *shaperPlugin) commitRate(request rateRequest, peer netip.Addr) (any, error) {
	change, err := s.rateToken(request, peer)
	if err != nil {
		return nil, err
	}
	tx := change.transaction()
	if _, committed := change.(*rateCommittedChange); committed {
		return map[string]string{"token": tx.token}, nil
	}
	if _, applied := change.(*rateAppliedChange); !applied {
		return nil, errors.New("rate transaction has not been applied")
	}
	s.rateLast, s.ratePending = &rateCommittedChange{tx}, nil
	return map[string]string{"token": tx.token}, nil
}

func (s *shaperPlugin) publishRate(tx *rateTransaction, qos traffic.InterfaceQoS) {
	key := sessionKey{tunnelID: tx.request.TunnelID, sessionID: tx.request.SessionID}
	value, found := s.sessions.Load(key)
	if !found {
		return
	}
	state, valid := value.(sessionState)
	if !valid {
		return
	}
	state.downloadRate = tx.request.DownloadRate
	if qos.Qdisc.Type == traffic.QdiscTBF {
		state.downloadRate = qos.Qdisc.RateBps
	}
	if qos.Qdisc.Type == traffic.QdiscHTB && len(qos.Qdisc.Classes) == 1 {
		state.downloadRate = qos.Qdisc.Classes[0].Rate
	}
	state.uploadRate = qos.Ingress.RateBps
	state.appliedAt = time.Now()
	s.sessions.Store(key, state)
}

// invalidateRateInterface prevents a late coordinator reply from restoring a
// departed subscriber's shaping on an interface another session has reused.
// Callers MUST hold mutationMu across the session lifetime transition.
func (s *shaperPlugin) invalidateRateInterface(iface string) {
	if s.ratePending != nil && s.ratePending.transaction().before.Interface == iface {
		s.ratePending = nil
	}
	if s.rateLast != nil && s.rateLast.transaction().before.Interface == iface {
		s.rateLast = nil
	}
}
