// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP subscriber route ownership
// Related: api.go -- bounded streams; session_routes.go -- retained routes
// API: https://github.com/FDio/vpp, src/vnet/fib/fib.api and src/vnet/ip/ip.api,
// pinned VPP 25.10; GoVPP v0.13.0. Ze owns source ze-pppoe, never FIB_SOURCE_API.
package vpp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

const routeSourceName = "ze-pppoe"
const routeSourcePriority = 0x80

// VPP allocates a new source even when its name already exists. Serialize
// lookup-and-add across this process's subscriber factories, with cancellation
// while waiting; no cached ID can survive a VPP process generation.
var routeSourceRegistration = make(chan struct{}, 1)

func (a *binaryAPI) routeSource(ctx context.Context) (uint8, error) {
	select {
	case routeSourceRegistration <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	defer func() { <-routeSourceRegistration }()
	var source uint8
	found := false
	err := a.dump(ctx, &fib.FibSourceDump{}, 256, func(message api.Message) error {
		reply, ok := message.(*fib.FibSourceDetails)
		if !ok {
			return fmt.Errorf("pppoe-vpp: source readback %T", message)
		}
		if reply == nil {
			return errors.New("pppoe-vpp: nil source readback")
		}
		if reply.Src.Name != routeSourceName {
			return nil
		}
		if found {
			return errors.New("pppoe-vpp: duplicate subscriber FIB source name")
		}
		if reply.Src.Priority != routeSourcePriority {
			return errors.New("pppoe-vpp: subscriber FIB source has another priority")
		}
		source = reply.Src.ID
		found = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if found {
		if source == 0 {
			return 0, errors.New("pppoe-vpp: subscriber FIB source is the shared API source")
		}
		return source, nil
	}
	message, err := a.exchange(ctx, &fib.FibSourceAdd{Src: fib.FibSource{Name: routeSourceName, Priority: routeSourcePriority}})
	if err != nil {
		return 0, err
	}
	reply, ok := message.(*fib.FibSourceAddReply)
	if !ok {
		return 0, fmt.Errorf("pppoe-vpp: add source reply %T", message)
	}
	if reply == nil {
		return 0, errors.New("pppoe-vpp: nil add source reply")
	}
	if reply.Retval != 0 {
		return 0, api.RetvalToVPPApiError(reply.Retval)
	}
	if reply.ID == 0 {
		return 0, errors.New("pppoe-vpp: VPP returned the shared API FIB source")
	}
	return reply.ID, nil
}

// A missing mutation reply is not a refusal. Its exact path stays owned until
// a confirmed withdrawal; a successful reply survives stream-close failure.
type routeReceipt uint8

const (
	routeUnknown routeReceipt = iota
	routeConfirmed
	routeRefused
)

func (a *binaryAPI) route(ctx context.Context, add bool, index, table uint32, source uint8, destination netip.Prefix, gateway netip.Addr, metric int) (routeReceipt, error) {
	path := fib_types.FibPath{SwIfIndex: index, Weight: 1, Preference: uint8(metric), Type: fib_types.FIB_API_PATH_TYPE_NORMAL, Proto: fib_types.FIB_API_PATH_NH_PROTO_IP4}
	if destination.Addr().Is6() {
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP6
	}
	if gateway.IsValid() {
		path.Nh.Address = vppAddress(gateway).Un
	}
	message, err := a.exchange(ctx, &ip.IPRouteAddDelV2{IsAdd: add, IsMultipath: true, Route: ip.IPRouteV2{
		TableID: table, Src: source, Prefix: ip_types.Prefix{Address: vppAddress(destination.Addr()), Len: uint8(destination.Bits())}, NPaths: 1, Paths: []fib_types.FibPath{path},
	}})
	if requestWasNotSent(err) {
		return routeRefused, err
	}
	reply, ok := message.(*ip.IPRouteAddDelV2Reply)
	if !ok {
		return routeUnknown, errors.Join(err, fmt.Errorf("pppoe-vpp: route reply %T", message))
	}
	if reply == nil {
		return routeUnknown, errors.Join(err, errors.New("pppoe-vpp: nil route reply"))
	}
	if reply.Retval != 0 {
		return routeRefused, errors.Join(err, api.RetvalToVPPApiError(reply.Retval))
	}
	return routeConfirmed, err
}

func vppAddress(address netip.Addr) ip_types.Address {
	if address.Is4() {
		return ip_types.Address{Af: ip_types.ADDRESS_IP4, Un: ip_types.AddressUnionIP4(ip_types.IP4Address(address.As4()))}
	}
	return ip_types.Address{Af: ip_types.ADDRESS_IP6, Un: ip_types.AddressUnionIP6(ip_types.IP6Address(address.As16()))}
}
