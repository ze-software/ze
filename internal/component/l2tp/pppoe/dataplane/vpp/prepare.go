// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP prepared access lifetime
// Related: access_map.go -- reciprocal map lease; prepare_linux.go -- CP readback
package vpp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"unicode"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
)

var preparedVPPAccesses = accessPreparations{records: make(map[string]*accessPreparation)}
var preparedVPPMaps = newControlMapOwners()

type controlLink struct {
	name  string
	index int
	mac   [6]byte
	mtu   int
}
type accessPreparationOps struct {
	resolve func(context.Context, string) (iface.Binding, iface.LCPPair, error)
	control func(context.Context, iface.LCPPair) (controlLink, error)
	api     func(context.Context) (*binaryAPI, error)
}

// Prepare resolves both actual devices and acquires the PPPoE control map
// before discovery starts. The caller MUST close the returned lease only after
// discovery detaches and its sessions stop. Safe for concurrent use; a logical
// access cannot silently change identity while another lease is active.
func Prepare(ctx context.Context, name string) (dataplane.Access, error) {
	access, err := prepareAccess(ctx, name, accessPreparationOps{
		resolve: resolveAccess, control: readControlLink, api: captureAPI,
	}, &preparedVPPAccesses, preparedVPPMaps)
	if err != nil {
		return nil, opererr.Wrap(err, fmt.Sprintf("PPPoE access %q could not be prepared in VPP", name),
			"Discovery remains stopped for this access because its actual LCP devices and control mapping must be available.",
			"ze doctor")
	}
	return access, nil
}

func resolveAccess(ctx context.Context, name string) (iface.Binding, iface.LCPPair, error) {
	backend, binding, err := iface.ResolveBackend("vpp", name)
	if err != nil {
		return iface.Binding{}, iface.LCPPair{}, err
	}
	reader, ok := backend.(iface.LCPPairReader)
	if !ok {
		return iface.Binding{}, iface.LCPPair{}, errors.New("selected VPP interface backend cannot read actual LCP pairs")
	}
	pair, err := reader.GetLCPPair(ctx, binding.OsName)
	if err != nil {
		return iface.Binding{}, iface.LCPPair{}, err
	}
	return binding, pair, nil
}

func prepareAccess(ctx context.Context, name string, ops accessPreparationOps,
	preparations *accessPreparations, maps *controlMapOwners) (dataplane.Access, error) {
	if ctx == nil {
		return nil, errors.New("VPP access preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("VPP access preparation requires a logical interface name")
	}
	connection, err := ops.api(ctx)
	if err != nil {
		return nil, err
	}
	binding, pair, err := ops.resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	indices, err := validateControlPair(binding, pair)
	if err != nil {
		return nil, err
	}
	localMAC, err := net.ParseMAC(binding.OperMAC)
	if err != nil {
		return nil, fmt.Errorf("read VPP access source MAC: %w", err)
	}
	if len(localMAC) != 6 {
		return nil, errors.New("VPP access source MAC is not Ethernet")
	}
	var dataMAC [6]byte
	copy(dataMAC[:], localMAC)
	if dataMAC == [6]byte{} {
		return nil, errors.New("VPP access source MAC is absent")
	}
	if dataMAC[0]&1 != 0 {
		return nil, errors.New("VPP access source MAC is not unicast")
	}
	control, err := ops.control(ctx, pair)
	if err != nil {
		return nil, err
	}
	if control.name != pair.ControlDevice {
		return nil, errors.New("the Linux LCP readback names another control device")
	}
	if control.index != pair.LinuxIndex {
		return nil, errors.New("the Linux LCP readback has another interface index")
	}
	// VPP advertises its DP MAC and preserves that destination on CP ingress.
	// Linux pppoe_rcv drops PACKET_OTHERHOST before looking up the session, so
	// an unequal TAP MAC cannot deliver the advertised PPPoE control channel.
	if control.mac != dataMAC {
		return nil, errors.New("the Linux LCP control MAC differs from the VPP access MAC")
	}
	// The access carries the smaller of the two MTUs, so each must be known:
	// a zero on either side would leave the access no MTU at all.
	if binding.MTU <= 0 {
		return nil, fmt.Errorf("VPP reports MTU %d for data-plane interface %q", binding.MTU, binding.OsName)
	}
	if control.mtu <= 0 {
		return nil, fmt.Errorf("pppoe-vpp: Linux reports MTU %d for LCP control device %q", control.mtu, control.name)
	}
	lease, err := maps.acquire(ctx, connection, indices)
	if err != nil {
		return nil, err
	}
	info := dataplane.AccessInfo{Device: binding.OsName, ControlDevice: control.name,
		Namespace: pair.Namespace, DiscoveryIfIndex: control.index, HWAddr: control.mac,
		MTU: min(binding.MTU, control.mtu)}
	snapshot := preparedSessionAccess{api: connection, dpIndex: indices.dataplane, mtu: info.MTU, localMAC: dataMAC}
	candidate := &accessPreparation{registry: preparations, name: name, info: info,
		snapshot: snapshot, pair: indices, mapping: lease, references: 1}
	access, err := dataplane.NewAccess(info, &accessPreparationLease{preparation: candidate})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, access.Close())
	}
	return preparations.publish(candidate, access)
}

func validateControlPair(binding iface.Binding, pair iface.LCPPair) (controlPair, error) {
	if binding.Ifindex <= 0 {
		return controlPair{}, errors.New("VPP access has no actual data-plane index")
	}
	if uint64(binding.Ifindex) >= uint64(noInterface) {
		return controlPair{}, errors.New("VPP data-plane index exceeds its API range")
	}
	if pair.DataplaneIndex != uint32(binding.Ifindex) {
		return controlPair{}, errors.New("LCP pair does not belong to the resolved VPP access")
	}
	if pair.ControlPlaneIndex == 0 {
		return controlPair{}, errors.New("LCP control-plane index names local0")
	}
	if pair.ControlPlaneIndex == noInterface {
		return controlPair{}, errors.New("LCP control-plane index is invalid")
	}
	if pair.ControlPlaneIndex == pair.DataplaneIndex {
		return controlPair{}, errors.New("LCP data and control interfaces are identical")
	}
	if pair.LinuxIndex <= 0 {
		return controlPair{}, errors.New("LCP pair has no actual Linux index")
	}
	if pair.ControlDevice == "" {
		return controlPair{}, errors.New("LCP pair has no actual Linux device")
	}
	if len(pair.ControlDevice) > 15 {
		return controlPair{}, errors.New("LCP control device exceeds the Linux name limit")
	}
	if strings.ContainsRune(pair.ControlDevice, 0) {
		return controlPair{}, errors.New("LCP control device contains a NUL byte")
	}
	// This raw API field reaches /run/netns before NewAccess validates the
	// published snapshot. Reject paths and parser delimiters before that I/O.
	if len(pair.Namespace) > 255 {
		return controlPair{}, errors.New("LCP namespace exceeds the Linux name limit")
	}
	if strings.ContainsAny(pair.Namespace, "/\\\x00") {
		return controlPair{}, errors.New("LCP namespace contains a path separator or NUL byte")
	}
	for _, character := range pair.Namespace {
		if !unicode.IsPrint(character) {
			return controlPair{}, errors.New("LCP namespace contains a non-printable character")
		}
		if unicode.IsSpace(character) {
			return controlPair{}, errors.New("LCP namespace contains whitespace")
		}
	}
	return controlPair{pair.DataplaneIndex, pair.ControlPlaneIndex}, nil
}

// preparedSessionAccess borrows a preparation's immutable identities and API.
// A successful lookup MUST be paired with release after session construction
// has either published or rolled back. Its per-access read lock prevents Close
// from removing the map while that construction is in flight.
type preparedSessionAccess struct {
	api      *binaryAPI
	dpIndex  uint32
	mtu      int
	localMAC [6]byte
	release  func()
}

type accessPreparations struct {
	mu      sync.Mutex
	records map[string]*accessPreparation
}
type accessPreparation struct {
	mu         sync.RWMutex
	registry   *accessPreparations
	name       string
	info       dataplane.AccessInfo
	snapshot   preparedSessionAccess
	pair       controlPair
	mapping    *controlMapLease
	references int
	closed     bool
}

func (r *accessPreparations) publish(candidate *accessPreparation, access dataplane.Access) (dataplane.Access, error) {
	r.mu.Lock()
	existing := r.records[candidate.name]
	if existing == nil {
		r.records[candidate.name] = candidate
		r.mu.Unlock()
		return access, nil
	}
	r.mu.Unlock()
	existing.mu.Lock()
	if existing.closed {
		existing.mu.Unlock()
		return nil, errors.Join(errors.New("VPP access changed while preparation was publishing; retry after reload"), access.Close())
	}
	if !existing.sameIdentity(candidate) {
		existing.mu.Unlock()
		return nil, errors.Join(errors.New("VPP access already has a lease for another device or connection generation"), access.Close())
	}
	existing.references++
	existing.mu.Unlock()
	if err := access.Close(); err != nil {
		return nil, errors.Join(err, (&accessPreparationLease{preparation: existing}).Close())
	}
	return dataplane.NewAccess(existing.info, &accessPreparationLease{preparation: existing})
}

func (p *accessPreparation) sameIdentity(other *accessPreparation) bool {
	if p.info != other.info {
		return false
	}
	if p.pair != other.pair {
		return false
	}
	if p.snapshot.localMAC != other.snapshot.localMAC {
		return false
	}
	if p.snapshot.api.connector != other.snapshot.api.connector {
		return false
	}
	return p.snapshot.api.generation == other.snapshot.api.generation
}

type accessPreparationLease struct {
	preparation *accessPreparation
	once        sync.Once
	err         error
}

func (l *accessPreparationLease) Close() error {
	l.once.Do(func() { l.err = l.preparation.close() })
	return l.err
}

// Open borrows this lease's original preparation, never a later registry entry.
// The access caller MUST join admission and sessions before closing the lease.
func (l *accessPreparationLease) Open(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
	return openWithOps(ctx, request, factoryOps{
		access: l.preparation.borrow, socket: openControlSocket, counters: captureCounters,
	})
}

func (p *accessPreparation) close() error {
	p.mu.Lock()
	p.references--
	if p.references > 0 {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.registry.mu.Lock()
	if p.registry.records[p.name] == p {
		delete(p.registry.records, p.name)
	}
	p.registry.mu.Unlock()
	p.mu.Unlock()
	return p.mapping.Close()
}

// borrow retains this exact preparation. The caller MUST invoke the returned
// release function after the session-creation attempt completes.
func (p *accessPreparation) borrow(ctx context.Context, request dataplane.Request) (preparedSessionAccess, error) {
	if ctx == nil {
		return preparedSessionAccess{}, errors.New("VPP access lookup requires a context")
	}
	if err := ctx.Err(); err != nil {
		return preparedSessionAccess{}, err
	}
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return preparedSessionAccess{}, errors.New("PPPoE VPP access preparation is closed")
	}
	if request.AccessName != p.name {
		p.mu.RUnlock()
		return preparedSessionAccess{}, errors.New("PPPoE logical access differs from its prepared VPP access")
	}
	if request.Device != p.info.Device {
		p.mu.RUnlock()
		return preparedSessionAccess{}, errors.New("PPPoE session device differs from its prepared VPP access")
	}
	if request.ControlDevice != p.info.ControlDevice {
		p.mu.RUnlock()
		return preparedSessionAccess{}, errors.New("PPPoE control device differs from its prepared VPP access")
	}
	if request.Namespace != p.info.Namespace {
		p.mu.RUnlock()
		return preparedSessionAccess{}, errors.New("PPPoE control namespace differs from its prepared VPP access")
	}
	if err := ctx.Err(); err != nil {
		p.mu.RUnlock()
		return preparedSessionAccess{}, err
	}
	snapshot := p.snapshot
	var once sync.Once
	snapshot.release = func() { once.Do(p.mu.RUnlock) }
	return snapshot, nil
}
