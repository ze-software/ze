// Design: docs/architecture/container/network.md -- addressing: the gateway, set and assigned addresses, the record
// Related: apply.go -- reads and writes the record through the engine client
// Related: verify.go -- the address rules a config is refused by

package network

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
)

// assignment is what the record keeps for one attached instance: one address
// per prefix of the network, in the network's prefix order, and the slot that
// numbers its host end on the bridge.
type assignment struct {
	Addresses []netip.Addr `json:"addresses"`
	Slot      uint16       `json:"slot"`
}

// addressRecord is the value of one network's state key: every attached
// instance's assignment, written whole. It is read tolerantly: an unknown
// field is ignored and an entry that no longer fits the network is assigned
// again, so a record an older image wrote still reads.
type addressRecord struct {
	Instances map[string]assignment `json:"instances"`
}

// slotsMax bounds the slot numbers of one network: a host end is named
// zc-INDEX-SLOT, and the interface component holds at most 1024 links.
const slotsMax = attachmentsMax

// defaultGateway is the network address plus one: the first host address.
func defaultGateway(prefix netip.Prefix) netip.Addr {
	return prefix.Addr().Next()
}

// broadcastAddress answers the broadcast address of an IPv4 prefix, its
// highest address. prefix MUST be an IPv4 prefix: IPv6 has no broadcast.
func broadcastAddress(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Addr().As4()
	value := binary.BigEndian.Uint32(bytes[:])
	value |= uint32(math.MaxUint32) >> prefix.Bits()
	binary.BigEndian.PutUint32(bytes[:], value)
	return netip.AddrFrom4(bytes)
}

// isHostAddress answers whether address can be held on prefix's network: it is
// inside the prefix, it is not the network address, and in IPv4 it is not the
// broadcast address. IPv6 has no broadcast address.
func isHostAddress(prefix netip.Prefix, address netip.Addr) bool {
	if !prefix.Contains(address) {
		return false
	}
	if address == prefix.Addr() {
		return false
	}
	if prefix.Addr().Is4() {
		return address != broadcastAddress(prefix)
	}
	return true
}

// hostAddressCount answers how many host addresses prefix holds, capped at
// math.MaxInt: an IPv6 prefix can hold more than an int counts, and every count
// above the attachment bound answers the same question.
func hostAddressCount(prefix netip.Prefix) int {
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	if hostBits >= 32 {
		return math.MaxInt
	}
	total := 1 << hostBits
	if prefix.Addr().Is4() {
		return total - 2
	}
	return total - 1
}

// freeAddressCount answers how many addresses of prefix an instance can be
// assigned: the host addresses less the gateway and the reserved addresses
// operators set.
func freeAddressCount(prefix netip.Prefix, reserved int) int {
	count := hostAddressCount(prefix)
	if count == math.MaxInt {
		return count
	}
	return count - 1 - reserved
}

// attached is one instance attached to the network being assigned, with the
// addresses its operator set, one per family at most.
type attached struct {
	instance string
	set      []netip.Addr
}

// assign answers the record of network after this config: each attached
// instance keeps the address its operator set, else the address stored for
// it when that still fits, else the lowest free host address that is not
// the gateway. An instance that left the network is not in the answer, which
// releases its addresses. held are the addresses links of instances no longer
// attached still carry: they stay taken until those containers end.
func assign(network Network, members []attached, stored addressRecord, held []netip.Addr) (addressRecord, error) {
	taken := make(map[netip.Addr]bool, len(members)+len(held)+len(network.Gateways))
	for _, gateway := range network.Gateways {
		taken[gateway] = true
	}
	for _, address := range held {
		taken[address] = true
	}
	for _, member := range members {
		for _, address := range member.set {
			taken[address] = true
		}
	}

	result := addressRecord{Instances: make(map[string]assignment, len(members))}
	slots := make(map[uint16]bool, len(members))
	// The first pass keeps what is stored, so a container keeps its address
	// and slot whatever the order of the newcomers.
	for _, member := range members {
		previous, found := stored.Instances[member.instance]
		current := assignment{Addresses: keepStored(network, member, previous, taken), Slot: slotUnset}
		if found {
			current.Slot = keepSlot(previous.Slot, slots)
		}
		result.Instances[member.instance] = current
	}
	// The second pass fills what is missing, in instance name order.
	for _, member := range members {
		current := result.Instances[member.instance]
		for index, prefix := range network.Prefixes {
			if current.Addresses[index].IsValid() {
				continue
			}
			address, found := lowestFree(prefix, taken)
			if !found {
				return addressRecord{}, refuseExhausted(network, prefix, member.instance)
			}
			taken[address] = true
			current.Addresses[index] = address
		}
		if current.Slot == slotUnset {
			slot, found := lowestFreeSlot(slots)
			if !found {
				return addressRecord{}, refuseSlots(network, member.instance)
			}
			slots[slot] = true
			current.Slot = slot
		}
		result.Instances[member.instance] = current
	}
	return result, nil
}

// slotUnset marks an assignment whose slot the second pass gives. It is
// outside every slot lowestFreeSlot answers.
const slotUnset = math.MaxUint16

// keepSlot answers the stored slot when no other attachment holds it and it
// is inside the bound, marking it held, else slotUnset.
func keepSlot(slot uint16, slots map[uint16]bool) uint16 {
	if slot >= slotsMax {
		return slotUnset
	}
	if slots[slot] {
		return slotUnset
	}
	slots[slot] = true
	return slot
}

// keepStored answers member's addresses after the first pass: the set ones,
// and the stored ones that still fit, each marked taken. A family with
// neither holds the invalid address, which the second pass fills.
func keepStored(network Network, member attached, previous assignment, taken map[netip.Addr]bool) []netip.Addr {
	addresses := make([]netip.Addr, len(network.Prefixes))
	for index, prefix := range network.Prefixes {
		if address, found := addressOfFamily(member.set, prefix); found {
			addresses[index] = address
			continue
		}
		stored, found := addressOfFamily(previous.Addresses, prefix)
		if !found {
			continue
		}
		if !isHostAddress(prefix, stored) {
			continue
		}
		if taken[stored] {
			continue
		}
		taken[stored] = true
		addresses[index] = stored
	}
	return addresses
}

// addressOfFamily answers the address of addresses inside prefix, if any.
func addressOfFamily(addresses []netip.Addr, prefix netip.Prefix) (netip.Addr, bool) {
	for _, address := range addresses {
		if prefix.Contains(address) {
			return address, true
		}
	}
	return netip.Addr{}, false
}

// lowestFree answers the lowest host address of prefix that is not taken. The
// walk skips only taken addresses, so it ends within len(taken)+1 steps.
func lowestFree(prefix netip.Prefix, taken map[netip.Addr]bool) (netip.Addr, bool) {
	for address := prefix.Addr().Next(); isHostAddress(prefix, address); address = address.Next() {
		if !taken[address] {
			return address, true
		}
	}
	return netip.Addr{}, false
}

// lowestFreeSlot answers the lowest slot no other attachment of the network
// holds.
func lowestFreeSlot(slots map[uint16]bool) (uint16, bool) {
	for slot := range uint16(slotsMax) {
		if !slots[slot] {
			return slot, true
		}
	}
	return 0, false
}

// decodeRecord reads a stored record tolerantly: a value that is not a
// record reads as an empty one, so every instance is assigned again rather
// than the apply refused for a record nobody can repair from the config. The
// empty record is a decision, not a lost answer: the next apply writes the
// new assignments over the unreadable value.
func decodeRecord(value []byte) addressRecord {
	var record addressRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return addressRecord{}
	}
	return record
}

// sameRecord answers whether two records hold the same assignments, so an
// unchanged record is not written again.
func sameRecord(a, b addressRecord) bool {
	if len(a.Instances) != len(b.Instances) {
		return false
	}
	for name, left := range a.Instances {
		right, found := b.Instances[name]
		if !found {
			return false
		}
		if left.Slot != right.Slot {
			return false
		}
		if !slices.Equal(left.Addresses, right.Addresses) {
			return false
		}
	}
	return true
}

func refuseExhausted(network Network, prefix netip.Prefix, instance string) error {
	return opererr.New(
		"network "+network.Name+": prefix "+prefix.String()+" has no free address left for container "+instance,
		"each attachment needs an address, and the addresses of the prefix are held by the gateway, by other containers or by containers that are still stopping",
		"set container network "+network.Name+" prefix with a larger prefix, or delete container instance "+instance+" network "+network.Name,
	)
}

func refuseSlots(network Network, instance string) error {
	return opererr.New(
		"network "+network.Name+": container "+instance+" cannot be given a link slot: "+strconv.Itoa(slotsMax)+" are in use",
		"each attachment's host end is numbered by its slot, and a network holds at most "+strconv.Itoa(slotsMax),
		"delete container instance "+instance+" network "+network.Name,
	)
}
