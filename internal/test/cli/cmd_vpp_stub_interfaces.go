// Design: docs/functional-tests.md -- VPP stub interface state readback.
// Related: cmd_vpp_stub.go -- acknowledged mutations and framed dump dispatch.
// Upstream: https://github.com/FDio/vpp/blob/v26.06/src/vnet/interface.api
// and src/vnet/ip/ip.api; generated GoVPP messages carry Ze's requests/replies.
// Native loopback names: src/vnet/ethernet/interface.c,
// format_simulated_ethernet_name and loopback_instance_alloc.

package cli

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// The stub bounds retained test state independently of a client's lifetime.
const (
	vppStubInterfacesMax = 4096
	vppStubAddressesMax  = 1024
)

// vppStubInterfaceTable is safe for concurrent use. Its zero value is an empty
// table. It retains the interfaces this stub created and their addresses.
type vppStubInterfaceTable struct {
	mu    sync.Mutex
	links map[interface_types.InterfaceIndex]*vppStubInterface
}

type vppStubInterface struct {
	details   interfaces.SwInterfaceDetails
	addresses []ip_types.AddressWithPrefix
}

// mutate applies the four interface operations used by the functional suite.
// Generated decoders need a complete body: validate each size before decoding.
func (table *vppStubInterfaceTable) mutate(name string, body []byte) (map[string]any, []byte, error) {
	table.mu.Lock()
	defer table.mu.Unlock()
	fields := make(map[string]any)
	switch name {
	case "create_loopback":
		var request interfaces.CreateLoopback
		if len(body) < request.Size() {
			return nil, nil, fmt.Errorf("short create_loopback body: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return nil, nil, err
		}
		reply := interfaces.CreateLoopbackReply{Retval: int32(api.LIMIT_EXCEEDED), SwIfIndex: ^interface_types.InterfaceIndex(0)}
		// With only loopbacks in this table, the first free slot supplies both
		// a reusable software index and the first free native loop instance.
		for index := interface_types.InterfaceIndex(1); index <= vppStubInterfacesMax; index++ {
			if _, exists := table.links[index]; exists {
				continue
			}
			if table.links == nil {
				table.links = make(map[interface_types.InterfaceIndex]*vppStubInterface)
			}
			details := interfaces.SwInterfaceDetails{
				SwIfIndex: index, SupSwIfIndex: uint32(index),
				L2Address:        request.MacAddress,
				InterfaceName:    "loop" + strconv.FormatUint(uint64(index-1), 10),
				InterfaceDevType: "Loopback",
			}
			if details.L2Address == [6]uint8{} {
				details.L2Address[0], details.L2Address[1] = 0xde, 0xad
				details.L2Address[5] = byte(index - 1) //nolint:gosec // VPP uses the low instance octet in its default MAC.
			}
			table.links[index] = &vppStubInterface{details: details}
			reply.Retval, reply.SwIfIndex = 0, index
			break
		}
		fields["mac_address"] = net.HardwareAddr(request.MacAddress[:]).String()
		fields["sw_if_index"] = reply.SwIfIndex
		encoded, err := reply.Marshal(nil)
		return fields, encoded, err
	case "delete_loopback":
		var request interfaces.DeleteLoopback
		if len(body) < request.Size() {
			return nil, nil, fmt.Errorf("short delete_loopback body: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return nil, nil, err
		}
		reply := interfaces.DeleteLoopbackReply{}
		if _, exists := table.links[request.SwIfIndex]; exists {
			delete(table.links, request.SwIfIndex)
		} else {
			reply.Retval = int32(api.INVALID_SW_IF_INDEX)
		}
		fields["sw_if_index"] = request.SwIfIndex
		encoded, err := reply.Marshal(nil)
		return fields, encoded, err
	case "sw_interface_set_flags":
		var request interfaces.SwInterfaceSetFlags
		if len(body) < request.Size() {
			return nil, nil, fmt.Errorf("short sw_interface_set_flags body: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return nil, nil, err
		}
		reply := interfaces.SwInterfaceSetFlagsReply{}
		if link, exists := table.links[request.SwIfIndex]; exists {
			link.details.Flags = request.Flags & interface_types.IF_STATUS_API_FLAG_ADMIN_UP
			if link.details.Flags != 0 {
				link.details.Flags |= interface_types.IF_STATUS_API_FLAG_LINK_UP
			}
		} else {
			reply.Retval = int32(api.INVALID_SW_IF_INDEX)
		}
		fields["sw_if_index"], fields["flags"] = request.SwIfIndex, request.Flags
		encoded, err := reply.Marshal(nil)
		return fields, encoded, err
	case "sw_interface_add_del_address":
		var request interfaces.SwInterfaceAddDelAddress
		if len(body) < request.Size() {
			return nil, nil, fmt.Errorf("short sw_interface_add_del_address body: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return nil, nil, err
		}
		reply := interfaces.SwInterfaceAddDelAddressReply{}
		if link, exists := table.links[request.SwIfIndex]; exists {
			reply.Retval = int32(link.address(request))
		} else {
			reply.Retval = int32(api.INVALID_SW_IF_INDEX)
		}
		fields["sw_if_index"], fields["is_add"] = request.SwIfIndex, request.IsAdd
		fields["del_all"] = request.DelAll
		encoded, err := reply.Marshal(nil)
		return fields, encoded, err
	default:
		return nil, nil, fmt.Errorf("unsupported stub interface mutation %q", name)
	}
}

// address runs under the table lock. Duplicate adds retain one address and
// deletion removes only that interface's exact host address and prefix length.
func (link *vppStubInterface) address(request interfaces.SwInterfaceAddDelAddress) api.VPPApiError {
	if request.DelAll {
		link.addresses = nil
		return 0
	}
	switch request.Prefix.Address.Af {
	case ip_types.ADDRESS_IP4:
		if request.Prefix.Len > 32 {
			return api.INVALID_VALUE
		}
	case ip_types.ADDRESS_IP6:
		if request.Prefix.Len > 128 {
			return api.INVALID_VALUE
		}
	default: // Address families come from the open VPP API.
		return api.INVALID_VALUE
	}
	index := slices.Index(link.addresses, request.Prefix)
	if request.IsAdd {
		if index >= 0 {
			return 0
		}
		if len(link.addresses) >= vppStubAddressesMax {
			return api.LIMIT_EXCEEDED
		}
		link.addresses = append(link.addresses, request.Prefix)
		return 0
	}
	if index < 0 {
		return api.NO_SUCH_ENTRY
	}
	link.addresses = slices.Delete(link.addresses, index, index+1)
	return 0
}

// serveInterfaceDump sends only detail frames. The ordinary control_ping reply
// terminates GoVPP's multipart request, as it does for the MPLS dump.
func (state *vppStubState) serveInterfaceDump(writer io.Writer, name string, contextID uint32, body []byte) error {
	table := &state.interfaces
	table.mu.Lock()
	defer table.mu.Unlock()
	if err := state.writeLog(name, contextID, map[string]any{}); err != nil {
		return err
	}
	switch name {
	case "sw_interface_dump":
		var request interfaces.SwInterfaceDump
		if len(body) < request.Size() {
			return fmt.Errorf("short sw_interface_dump body: %d", len(body))
		}
		if uint64(binary.BigEndian.Uint32(body[5:9])) > uint64(len(body)-9) {
			return fmt.Errorf("short sw_interface_dump name filter: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return err
		}
		// This stub creates only lowercase ASCII loop names.
		filter := strings.ToLower(request.NameFilter)
		// Iterate bounded slots in software-index order; name filtering takes
		// precedence over the index, as interface.api specifies.
		for index := interface_types.InterfaceIndex(1); index <= vppStubInterfacesMax; index++ {
			link, exists := table.links[index]
			if !exists {
				continue
			}
			if request.NameFilterValid {
				if !strings.Contains(link.details.InterfaceName, filter) {
					continue
				}
			} else {
				switch request.SwIfIndex {
				case 0, ^interface_types.InterfaceIndex(0):
					// Both sentinel values request the whole inventory.
				default:
					if request.SwIfIndex != index {
						continue
					}
				}
			}
			encoded, err := link.details.Marshal(nil)
			if err != nil {
				return err
			}
			frame, err := state.reply("sw_interface_details", contextID, encoded)
			if err != nil {
				return err
			}
			if err := writeVPPFrame(writer, frame); err != nil {
				return err
			}
		}
	case "ip_address_dump":
		var request ip.IPAddressDump
		if len(body) < request.Size() {
			return fmt.Errorf("short ip_address_dump body: %d", len(body))
		}
		if err := request.Unmarshal(body); err != nil {
			return err
		}
		link, exists := table.links[request.SwIfIndex]
		if !exists {
			return nil // A valid dump of an absent index has no detail frames.
		}
		for _, prefix := range link.addresses {
			if (prefix.Address.Af == ip_types.ADDRESS_IP6) != request.IsIPv6 {
				continue
			}
			details := ip.IPAddressDetails{SwIfIndex: request.SwIfIndex, Prefix: prefix}
			encoded, err := details.Marshal(nil)
			if err != nil {
				return err
			}
			frame, err := state.reply("ip_address_details", contextID, encoded)
			if err != nil {
				return err
			}
			if err := writeVPPFrame(writer, frame); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported stub interface dump %q", name)
	}
	return nil
}
