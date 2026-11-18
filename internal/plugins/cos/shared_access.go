// Design: docs/architecture/traffic/cos-dynamic.md -- shared legacy map ownership
// Related: session_transaction.go -- one retained compensation reservation
//go:build ze_l2tp

package cos

import (
	"errors"
	"fmt"
	"maps"

	"github.com/ze-software/ze/internal/component/iface"
	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/report"
)

// sharedCoSProfile is the effective pair of eight supported PCP mappings.
// Its zero value is valid: every entry selects the kernel's default zero.
// Only newSharedCoSProfile admits raw maps; subsequent comparisons are exact.
type sharedCoSProfile struct {
	ingress [8]uint32
	egress  [8]uint32
}

func newSharedCoSProfile(profile coreCos.Profile) (sharedCoSProfile, error) {
	var result sharedCoSProfile
	for key, value := range profile.IngressMap {
		if key >= uint32(len(result.ingress)) {
			return sharedCoSProfile{}, errors.New("the ingress map cannot be restored by this interface backend")
		}
		if value >= uint32(len(result.ingress)) {
			return sharedCoSProfile{}, errors.New("the ingress priority cannot be restored by this interface backend")
		}
		result.ingress[key] = value
	}
	for key, value := range profile.EgressMap {
		if key >= uint32(len(result.egress)) {
			return sharedCoSProfile{}, errors.New("the egress priority cannot be restored by this interface backend")
		}
		if value >= uint32(len(result.egress)) {
			return sharedCoSProfile{}, errors.New("the egress map cannot be restored by this interface backend")
		}
		result.egress[key] = value
	}
	return result, nil
}

func cloneSharedCoSMaps(profile coreCos.Profile) coreCos.Profile {
	return coreCos.Profile{IngressMap: maps.Clone(profile.IngressMap), EgressMap: maps.Clone(profile.EgressMap)}
}

// sharedCoSAccess is one initialized legacy map lifetime, not a native grant or
// configured-access generation. Original maps are immutable. users and installed
// are changed only under the owning handler's mu. The nil/zero value is invalid;
// captureSharedAccess retains native and diagnostic custody before Snapshot;
// only its initialized state admits members after the successful baseline read.
type sharedCoSAccess struct {
	name        string
	reportKey   string
	warning     sharedCoSWarning
	reportScope *report.ResourceWarning
	episode     *report.ResourceWarningEpisode
	original    coreCos.Profile
	installed   coreCos.Profile
	users       map[sessionKey]sharedCoSProfile
	departures  map[sessionKey]sharedCoSDepartureFailure
	target      originalCoSTarget
	write       updateQoSFunc
	readLink    func(string) (*iface.InterfaceInfo, error)
}

// sharedCoSWarning belongs to one captured access and moves with that exact
// owner on runner transfer. Nil is invalid; capture initializes Unraised.
// Attempted includes unknown delivery: a Raise attempt already owes a CLEAR.
type sharedCoSWarning interface {
	sharedCoSWarningState()
}

type sharedCoSWarningUnraised struct{}

func (sharedCoSWarningUnraised) sharedCoSWarningState() {}

type sharedCoSWarningAttempted struct{}

func (sharedCoSWarningAttempted) sharedCoSWarningState() {}

// ClearPending is constructed only from an actual failed CLEAR. It retains
// that cause without implying that native maps remain unconfirmed.
type sharedCoSWarningClearPending struct{ cause error }

func (*sharedCoSWarningClearPending) sharedCoSWarningState() {}

// A retained failure cannot admit a user. The initialized owner stays reachable
// in the same table until its original compensation/restoration succeeds.
type sharedCoSAccessState interface {
	sharedOwner() *sharedCoSAccess
	sharedAccessState()
}

func (access *sharedCoSAccess) sharedOwner() *sharedCoSAccess { return access }
func (*sharedCoSAccess) sharedAccessState()                   {}

type sharedCoSRestore struct {
	access    *sharedCoSAccess
	departing sessionKey
}

func (restore *sharedCoSRestore) sharedOwner() *sharedCoSAccess { return restore.access }
func (*sharedCoSRestore) sharedAccessState()                    {}

type sharedCoSUncertain struct {
	access   *sharedCoSAccess
	key      sessionKey
	previous coreCos.Profile
	cause    error
}

func (uncertain *sharedCoSUncertain) sharedOwner() *sharedCoSAccess { return uncertain.access }
func (*sharedCoSUncertain) sharedAccessState()                      {}

// sharedCoSReportPending retains only the original warning lifetime after
// native cleanup succeeded. No path may use this variant for a native write;
// the cleanup command can only retry that same report-clear publication.
type sharedCoSReportPending struct{ access *sharedCoSAccess }

func (pending *sharedCoSReportPending) sharedOwner() *sharedCoSAccess { return pending.access }
func (*sharedCoSReportPending) sharedAccessState()                    {}

// readSharedMaps is an observation, never a config-derived default. GetLink's
// VLAN type is the backend's declaration that its map fields were reported.
func (h *cosHandler) readSharedMaps(name string) (coreCos.Profile, error) {
	return readSharedAccessMaps(h.readLink, name)
}

func readSharedAccessMaps(read func(string) (*iface.InterfaceInfo, error), name string) (coreCos.Profile, error) {
	link, err := read(name)
	if err != nil {
		return coreCos.Profile{}, fmt.Errorf("read access interface %q: %w", name, err)
	}
	if link == nil {
		return coreCos.Profile{}, errors.New("the access interface backend returned no link")
	}
	if link.Type != "vlan" {
		return coreCos.Profile{}, fmt.Errorf("the interface backend does not report VLAN QoS maps for access interface %q", name)
	}
	profile := cloneSharedCoSMaps(coreCos.Profile{IngressMap: link.IngressQoSMap, EgressMap: link.EgressQoSMap})
	if _, err := newSharedCoSProfile(profile); err != nil {
		return coreCos.Profile{}, err
	}
	return profile, nil
}

func (h *cosHandler) captureSharedAccess(name string) (*sharedCoSAccess, error) {
	if state := h.accesses[name]; state != nil {
		if _, pending := state.(*sharedCoSReportPending); pending {
			return nil, errors.New("the original access cleanup succeeded but its warning could not be cleared; run ze request class-of-service cleanup")
		}
		access, active := state.(*sharedCoSAccess)
		if !active {
			return nil, errors.New("the original access retains unfinished cleanup or native release; run ze request class-of-service cleanup")
		}
		return access, nil
	}
	reportKey := newSharedCoSReportKey()
	reservation, err := report.NewResourceWarning(Name, reportKey)
	if err != nil {
		return nil, err
	}
	target, err := h.captureOriginalCoSTarget(name)
	if err != nil {
		return nil, errors.Join(err, reservation.Close())
	}
	access := &sharedCoSAccess{name: name, reportKey: reportKey, reportScope: reservation,
		warning: sharedCoSWarningUnraised{}, users: make(map[sessionKey]sharedCoSProfile),
		departures: make(map[sessionKey]sharedCoSDepartureFailure), target: target,
		write: target.write, readLink: target.readLink}
	if h.accesses == nil {
		h.accesses = make(map[string]sharedCoSAccessState)
	}
	// Both native and diagnostic custody precede the first Snapshot. Unknown
	// return keeps the exact target and reservation without inventing maps.
	h.accesses[name] = &sharedCoSCapturePending{access: access}
	original, err := readSharedAccessMaps(target.readLink, name)
	if err != nil {
		h.accesses[name] = &sharedCoSCapturePending{access: access, cause: err}
		return nil, h.cleanupFailure(access, sessionDiagnostic{}, err)
	}
	access.original = original
	access.installed = cloneSharedCoSMaps(original)
	h.accesses[name] = access
	return access, nil
}

// Compatible compares every other admitted user of this one access. The set is
// bounded by the existing admitted session coordinates; no wire name is interned.
func (access *sharedCoSAccess) compatible(key sessionKey, desired sharedCoSProfile) error {
	for user, profile := range access.users {
		if user == key {
			continue
		}
		if profile != desired {
			return errors.New("another subscriber on this access requires different class-of-service maps")
		}
	}
	return nil
}

func (h *cosHandler) sharedSessionState(key sessionKey, access *sharedCoSAccess) (*sessionCoSState, error) {
	value, present := sessionStore.Load(key)
	if !present {
		return nil, nil //nolint:nilnil // No prior dynamic policy is a valid first CoA/admission input.
	}
	state, valid := value.(sessionCoSState)
	if !valid {
		return nil, errors.New("the session class-of-service state is unreadable")
	}
	if state.accessInterface != access.name {
		return nil, errors.New("the session uses a different access interface")
	}
	if state.owner != access {
		return nil, errors.New("the session class-of-service state belongs to another retained owner")
	}
	return &state, nil
}

func (h *cosHandler) keepSharedSession(key sessionKey, access *sharedCoSAccess, name, token string, profile sharedCoSProfile) {
	access.users[key] = profile
	sessionStore.Store(key, sessionCoSState{accessInterface: access.name,
		profileName: name, commandToken: token, owner: access})
}

func (h *cosHandler) discardUnusedSharedAccess(access *sharedCoSAccess) {
	if err := h.releaseUnusedSharedAccess(access); err != nil {
		h.cleanupFailure(access, sessionDiagnostic{}, err) //nolint:errcheck // No response at this discard boundary; original warning and custody remain.
	}
}

// A different legacy user must be compatible with the last committed change's
// immediate beforeimage too. The old token remains compensatable until the next
// prepare, even when the post-commit notification has already been consumed.
func (h *cosHandler) compatibleCompensation(key sessionKey, access *sharedCoSAccess, desired sharedCoSProfile) error {
	if h.completed.operation != completionCommitted {
		return nil
	}
	change := h.completed.change
	if change == nil {
		return nil
	}
	if change.access != access {
		return nil
	}
	if change.key == key {
		return nil
	}
	if desired != change.before {
		return errors.New("the previous class-of-service transaction still requires incompatible compensation maps")
	}
	return nil
}

func (h *cosHandler) applyLegacySharedProfile(key sessionKey, name, profileName string, profile coreCos.Profile) error {
	desired, err := newSharedCoSProfile(profile)
	if err != nil {
		return err
	}
	if h.pending != nil {
		if h.pending.prepared().accessInterface == name {
			return errors.New("the access has a pending class-of-service transaction")
		}
	}
	access, err := h.captureSharedAccess(name)
	if err != nil {
		return err
	}
	if _, err := h.sharedSessionState(key, access); err != nil {
		h.discardUnusedSharedAccess(access)
		return err
	}
	if err := access.compatible(key, desired); err != nil {
		h.discardUnusedSharedAccess(access)
		return err
	}
	if err := h.compatibleCompensation(key, access, desired); err != nil {
		h.discardUnusedSharedAccess(access)
		return err
	}
	previous := cloneSharedCoSMaps(access.installed)
	if err := access.write(name, profile.IngressMap, profile.EgressMap); err != nil {
		compensation := access.write(name, previous.IngressMap, previous.EgressMap)
		if compensation != nil {
			h.accesses[name] = &sharedCoSUncertain{access: access, key: key, previous: previous, cause: errors.Join(err, compensation)}
			return errors.Join(err, compensation)
		}
		h.discardUnusedSharedAccess(access)
		return err
	}
	access.installed = cloneSharedCoSMaps(profile)
	h.keepSharedSession(key, access, profileName, "", desired)
	return nil
}

func (h *cosHandler) sharedDepartureOwner(key sessionKey) *sharedCoSAccess {
	if value, present := sessionStore.Load(key); present {
		if state, valid := value.(sessionCoSState); valid {
			return state.owner
		}
	}
	if h.pending != nil {
		if h.pending.prepared().key == key {
			return h.pending.prepared().access
		}
	}
	// An initial write can fail before membership is published. Its retained
	// compensation still belongs to the original session's eventual Down.
	for _, state := range h.accesses {
		if uncertain, retained := state.(*sharedCoSUncertain); retained {
			if uncertain.key == key {
				return uncertain.access
			}
		}
	}
	return nil
}

func (h *cosHandler) departSharedSession(key sessionKey) error {
	access := h.sharedDepartureOwner(key)
	if access == nil {
		h.forgetSessionChange(key)
		return nil
	}
	// The real Down occurs once. Retain its original key before attempting
	// compensation so an operator retry does not need a second event.
	if _, retained := access.departures[key]; !retained {
		access.departures[key] = sharedCoSDepartureFailure{}
	}
	if err := h.repairSharedUncertainty(access); err != nil {
		return access.retainDepartureFailure(key, err)
	}
	if h.pending != nil {
		if h.pending.prepared().access == access {
			if err := h.rollbackSessionChange(); err != nil {
				return access.retainDepartureFailure(key, err)
			}
		}
	}
	state := h.accesses[access.name]
	if state == nil {
		if len(access.users) == 0 {
			delete(access.departures, key)
			h.forgetSessionChange(key)
			return nil
		}
		return errors.New("the departing session's original map owner is not retained by this handler")
	}
	if state.sharedOwner() != access {
		return errors.New("the departing session belongs to another retained access map owner")
	}
	if restore, retained := state.(*sharedCoSRestore); retained {
		if restore.departing != key {
			return errors.New("another original session still owns the failed final map restore")
		}
	}
	h.forgetSessionChange(key)
	if _, admitted := access.users[key]; !admitted {
		delete(access.departures, key)
		h.discardUnusedSharedAccess(access)
		return nil
	}
	if len(access.users) > 1 {
		delete(access.departures, key)
		delete(access.users, key)
		sessionStore.Delete(key)
		return nil
	}
	if err := access.write(access.name, access.original.IngressMap, access.original.EgressMap); err != nil {
		h.accesses[access.name] = &sharedCoSRestore{access: access, departing: key}
		return access.retainDepartureFailure(key, err)
	}
	// Confirmed restoration retires the failed-restore phase before original
	// release. Otherwise that variant would keep the same owner from finishing.
	h.accesses[access.name] = access
	delete(access.departures, key)
	delete(access.users, key)
	sessionStore.Delete(key)
	return h.releaseUnusedSharedAccess(access)
}

// repairSharedUncertainty first rejoins the failed legacy write's immediate
// beforeimage. A committed token keeps its own, older beforeimage and can then
// compensate normally. Caller MUST hold h.mu and retain this exact owner.
func (h *cosHandler) repairSharedUncertainty(access *sharedCoSAccess) error {
	state := h.accesses[access.name]
	if state == nil {
		return errors.New("the original access map owner is no longer retained")
	}
	if state.sharedOwner() != access {
		return errors.New("the access maps belong to another original owner")
	}
	uncertain, retained := state.(*sharedCoSUncertain)
	if !retained {
		return nil
	}
	if err := access.write(access.name, uncertain.previous.IngressMap, uncertain.previous.EgressMap); err != nil {
		return errors.Join(uncertain.cause, err)
	}
	access.installed = cloneSharedCoSMaps(uncertain.previous)
	h.accesses[access.name] = access
	return nil
}

// A zero value marks a requested departure that has not failed yet. Each
// retained failure keeps its first cause and latest retry cause, never an
// unbounded error chain from repeated operator attempts.
type sharedCoSDepartureFailure struct {
	first  error
	latest error
}

func (access *sharedCoSAccess) retainDepartureFailure(key sessionKey, cause error) error {
	failure := access.departures[key]
	if failure.first == nil {
		failure.first = cause
	} else {
		failure.latest = cause
	}
	access.departures[key] = failure
	return errors.Join(failure.first, failure.latest)
}

func (h *cosHandler) restoreSharedMembership(change *preparedCoSChange) {
	if change.prior == nil {
		delete(change.access.users, change.key)
		sessionStore.Delete(change.key)
		return
	}
	change.access.users[change.key] = change.priorProfile
	sessionStore.Store(change.key, *change.prior)
}
