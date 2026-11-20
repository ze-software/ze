// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP session API ownership
// Related: api_routes.go -- owned subscriber routes; session.go -- lifetime
// API: https://github.com/FDio/vpp, src/plugins/pppoe/pppoe.api and
// src/vnet/interface.api; GoVPP v0.13.0, pinned patched VPP 25.10.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/memclnt"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"

	"go.fd.io/govpp/binapi/ethernet_types"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

const apiTimeout = 5 * time.Second
const noInterface = ^uint32(0)

// binaryAPI owns no persistent channel. Every call opens a context-bound stream
// and closes it before returning; callers MUST retain the same VPP connector
// generation for the whole session. No operation reconnects to another process.
type binaryAPI struct {
	connector  *vppcomp.Connector
	generation uint64
	open       func(context.Context) (api.Stream, error)
	retired    atomic.Bool // exact supervised process proof, never API loss
}

func captureAPI(ctx context.Context) (*binaryAPI, error) {
	if ctx == nil {
		return nil, errors.New("pppoe-vpp: API capture requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connector := vppcomp.GetActiveConnector()
	if connector == nil {
		return nil, errors.New("pppoe-vpp: VPP connector is unavailable")
	}
	if err := connector.WaitConnected(ctx, apiTimeout); err != nil {
		return nil, err
	}
	generation, err := connector.ConnectionGeneration()
	if err != nil {
		return nil, err
	}
	return &binaryAPI{connector: connector, generation: generation, open: func(ctx context.Context) (api.Stream, error) {
		return connector.NewStreamForGeneration(ctx, generation)
	}}, nil
}

func (a *binaryAPI) exchange(ctx context.Context, request api.Message) (reply api.Message, err error) {
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	if err := bounded.Err(); err != nil {
		return nil, &apiRequestNotSentError{cause: err}
	}
	if a.processExited() {
		return nil, &apiRequestNotSentError{cause: errors.New("pppoe-vpp: API belongs to a retired VPP process")}
	}
	stream, err := a.open(bounded)
	if err != nil {
		return nil, &apiRequestNotSentError{cause: err}
	}
	if stream == nil {
		return nil, &apiRequestNotSentError{cause: errors.New("pppoe-vpp: API returned no stream")}
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	if err := stream.SendMsg(request); err != nil {
		return nil, err
	}
	reply, err = stream.RecvMsg()
	if err != nil {
		return nil, err
	}
	if err := bounded.Err(); err != nil {
		return reply, err
	}
	return reply, nil
}

// dump bounds externally supplied replies and requires the ping completion.
// The caller chooses a bound derived from the queried table's native limits.
func (a *binaryAPI) dump(ctx context.Context, request api.Message, limit int, visit func(api.Message) error) (err error) {
	bounded, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	if err := bounded.Err(); err != nil {
		return err
	}
	if a.processExited() {
		return errors.New("pppoe-vpp: API belongs to a retired VPP process")
	}
	stream, err := a.open(bounded)
	if err != nil {
		return err
	}
	if stream == nil {
		return errors.New("pppoe-vpp: API returned no dump stream")
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	if err := stream.SendMsg(request); err != nil {
		return err
	}
	if err := stream.SendMsg(&memclnt.ControlPing{}); err != nil {
		return err
	}
	for range limit + 1 {
		message, readErr := stream.RecvMsg()
		if readErr != nil {
			return readErr
		}
		if ping, ok := message.(*memclnt.ControlPingReply); ok {
			if ping == nil {
				return errors.New("pppoe-vpp: nil dump completion")
			}
			if ping.Retval != 0 {
				return api.RetvalToVPPApiError(ping.Retval)
			}
			return bounded.Err()
		}
		if err := visit(message); err != nil {
			return err
		}
	}
	return fmt.Errorf("pppoe-vpp: %s exceeded %d replies", request.GetMessageName(), limit)
}

// creation is a raw API receipt before a validated owner is constructed. A
// successful mutation remains confirmed even when stream cleanup fails, so
// the factory retains authority to remove exactly its SID/MAC on rollback.
type creation struct {
	index     uint32
	confirmed bool
	uncertain bool
}

func (a *binaryAPI) create(ctx context.Context, sid uint16, mac [6]byte, table uint32) (creation, error) {
	message, err := a.exchange(ctx, &pppoeapi.PppoeAddDelSession{IsAdd: true, SessionID: sid, ClientMac: ethernet_types.MacAddress(mac), DecapVrfID: table, DisableFib: true})
	if requestWasNotSent(err) {
		return creation{}, err
	}
	if message == nil {
		return creation{uncertain: true}, errors.Join(err, errors.New("pppoe-vpp: create outcome is unknown; VPP supplied no reply"))
	}
	reply, ok := message.(*pppoeapi.PppoeAddDelSessionReply)
	if !ok {
		return creation{uncertain: true}, errors.Join(err, fmt.Errorf("pppoe-vpp: create reply %T", message))
	}
	if reply == nil {
		return creation{uncertain: true}, errors.Join(err, errors.New("pppoe-vpp: nil create reply"))
	}
	if reply.Retval != 0 {
		return creation{}, errors.Join(err, api.RetvalToVPPApiError(reply.Retval))
	}
	index := uint32(reply.SwIfIndex)
	receipt := creation{index: index, confirmed: true}
	if index == 0 {
		return receipt, errors.Join(err, errors.New("pppoe-vpp: created interface is local0"))
	}
	if index == noInterface {
		return receipt, errors.Join(err, errors.New("pppoe-vpp: created interface index is invalid"))
	}
	return receipt, err
}

func (a *binaryAPI) remove(ctx context.Context, sid uint16, mac [6]byte, table uint32) error {
	message, err := a.exchange(ctx, &pppoeapi.PppoeAddDelSession{SessionID: sid, ClientMac: ethernet_types.MacAddress(mac), DecapVrfID: table, DisableFib: true})
	if err != nil {
		return err
	}
	reply, ok := message.(*pppoeapi.PppoeAddDelSessionReply)
	if !ok {
		return fmt.Errorf("pppoe-vpp: delete reply %T", message)
	}
	if reply == nil {
		return errors.New("pppoe-vpp: nil delete reply")
	}
	return api.RetvalToVPPApiError(reply.Retval)
}

func (a *binaryAPI) interfaceName(ctx context.Context, index uint32) (string, error) {
	var name string
	err := a.dump(ctx, &interfaces.SwInterfaceDump{SwIfIndex: interface_types.InterfaceIndex(index)}, 1, func(message api.Message) error {
		reply, ok := message.(*interfaces.SwInterfaceDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: interface readback %T", message)
		}
		if reply == nil {
			return errors.New("pppoe-vpp: nil interface readback")
		}
		if uint32(reply.SwIfIndex) != index {
			return errors.New("pppoe-vpp: interface readback names another index")
		}
		if name != "" {
			return errors.New("pppoe-vpp: duplicate interface readback")
		}
		name = strings.TrimRight(reply.InterfaceName, "\x00")
		if name == "" {
			return errors.New("pppoe-vpp: created interface has no name")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", errors.New("pppoe-vpp: created interface absent from readback")
	}
	return name, nil
}

// The MAC-only upstream link table can change between discovery and creation.
// Read back the created session before publication and refuse any different
// ingress device or MAC; a successful add is still ours to roll back.
func (a *binaryAPI) verifySession(ctx context.Context, index, access, table uint32, sid uint16, peer, local [6]byte) error {
	found := false
	err := a.dump(ctx, &pppoeapi.PppoeSessionDump{SwIfIndex: interface_types.InterfaceIndex(index)}, 1, func(message api.Message) error {
		reply, ok := message.(*pppoeapi.PppoeSessionDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: session readback %T", message)
		}
		if reply == nil {
			return errors.New("pppoe-vpp: nil session readback")
		}
		if found {
			return errors.New("pppoe-vpp: duplicate session readback")
		}
		found = true
		if uint32(reply.SwIfIndex) != index {
			return errors.New("pppoe-vpp: session readback names another interface")
		}
		if uint32(reply.EncapIfIndex) != access {
			return errors.New("pppoe-vpp: client MAC was learned on another access interface")
		}
		if reply.SessionID != sid {
			return errors.New("pppoe-vpp: session readback has another discovery ID")
		}
		if [6]byte(reply.ClientMac) != peer {
			return errors.New("pppoe-vpp: session readback has another client MAC")
		}
		if [6]byte(reply.LocalMac) != local {
			return errors.New("pppoe-vpp: session readback has another data-plane source MAC")
		}
		if reply.DecapVrfID != table {
			return errors.New("pppoe-vpp: session readback has another decapsulation table")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return errors.New("pppoe-vpp: created session absent from readback")
	}
	return nil
}

func (a *binaryAPI) tables(ctx context.Context, index uint32) ([2]uint32, error) {
	var tables [2]uint32
	for family := range tables {
		message, err := a.exchange(ctx, &interfaces.SwInterfaceGetTable{SwIfIndex: interface_types.InterfaceIndex(index), IsIPv6: family == 1})
		if err != nil {
			return tables, err
		}
		reply, ok := message.(*interfaces.SwInterfaceGetTableReply)
		if !ok {
			return tables, fmt.Errorf("pppoe-vpp: table reply %T", message)
		}
		if reply == nil {
			return tables, errors.New("pppoe-vpp: nil table reply")
		}
		if reply.Retval != 0 {
			return tables, api.RetvalToVPPApiError(reply.Retval)
		}
		tables[family] = reply.VrfID
	}
	return tables, nil
}

func (a *binaryAPI) setTable(ctx context.Context, index, table uint32, ipv6 bool) error {
	message, err := a.exchange(ctx, &interfaces.SwInterfaceSetTable{SwIfIndex: interface_types.InterfaceIndex(index), VrfID: table, IsIPv6: ipv6})
	if err != nil {
		return err
	}
	reply, ok := message.(*interfaces.SwInterfaceSetTableReply)
	if !ok {
		return fmt.Errorf("pppoe-vpp: set-table reply %T", message)
	}
	if reply == nil {
		return errors.New("pppoe-vpp: nil set-table reply")
	}
	return api.RetvalToVPPApiError(reply.Retval)
}

func (a *binaryAPI) setMTU(ctx context.Context, index uint32, mtu int) error {
	message, err := a.exchange(ctx, &interfaces.SwInterfaceSetMtu{SwIfIndex: interface_types.InterfaceIndex(index), Mtu: []uint32{uint32(mtu), uint32(mtu), uint32(mtu), 0}})
	if err != nil {
		return err
	}
	reply, ok := message.(*interfaces.SwInterfaceSetMtuReply)
	if !ok {
		return fmt.Errorf("pppoe-vpp: set-MTU reply %T", message)
	}
	if reply == nil {
		return errors.New("pppoe-vpp: nil set-MTU reply")
	}
	return api.RetvalToVPPApiError(reply.Retval)
}

func (a *binaryAPI) setAdmin(ctx context.Context, index uint32, up bool) error {
	flags := interface_types.IfStatusFlags(0)
	if up {
		flags = interface_types.IF_STATUS_API_FLAG_ADMIN_UP
	}
	message, err := a.exchange(ctx, &interfaces.SwInterfaceSetFlags{SwIfIndex: interface_types.InterfaceIndex(index), Flags: flags})
	if err != nil {
		return err
	}
	reply, ok := message.(*interfaces.SwInterfaceSetFlagsReply)
	if !ok {
		return fmt.Errorf("pppoe-vpp: set-admin reply %T", message)
	}
	if reply == nil {
		return errors.New("pppoe-vpp: nil set-admin reply")
	}
	return api.RetvalToVPPApiError(reply.Retval)
}

func (a *binaryAPI) setIP(ctx context.Context, index uint32, ipv6, enable bool) error {
	prefix := "ip4"
	if ipv6 {
		prefix = "ip6"
	}
	var result error
	for _, suffix := range []string{"-unicast", "-multicast"} {
		arc, node := prefix+suffix, prefix+"-not-enabled"
		closed, err := a.ipGate(ctx, index, arc, node)
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		if closed == !enable {
			continue
		}
		// Pinned VPP config.c adds duplicate features on repeated enables.
		// Read before mutation, and confirm after even when its receipt is lost.
		message, mutationErr := a.exchange(ctx, &feature.FeatureEnableDisable{SwIfIndex: interface_types.InterfaceIndex(index), Enable: !enable, ArcName: arc, FeatureName: node})
		reply, ok := message.(*feature.FeatureEnableDisableReply)
		if !ok || reply == nil {
			mutationErr = errors.Join(mutationErr, fmt.Errorf("pppoe-vpp: feature mutation reply %T", message))
		} else {
			mutationErr = errors.Join(mutationErr, api.RetvalToVPPApiError(reply.Retval))
		}
		closed, readErr := a.ipGate(ctx, index, arc, node)
		if readErr == nil && closed != !enable {
			readErr = fmt.Errorf("pppoe-vpp: %s gate did not reach requested state", arc)
		}
		result = errors.Join(result, mutationErr, readErr)
	}
	return result
}

func (a *binaryAPI) ipGate(ctx context.Context, index uint32, arc, node string) (bool, error) {
	message, err := a.exchange(ctx, &feature.FeatureIsEnabled{SwIfIndex: interface_types.InterfaceIndex(index), ArcName: arc, FeatureName: node})
	if err != nil {
		return false, err
	}
	reply, ok := message.(*feature.FeatureIsEnabledReply)
	if !ok {
		return false, fmt.Errorf("pppoe-vpp: feature readback reply %T", message)
	}
	if reply == nil {
		return false, errors.New("pppoe-vpp: nil feature readback")
	}
	if reply.Retval != 0 {
		return false, api.RetvalToVPPApiError(reply.Retval)
	}
	return reply.IsEnabled, nil
}
