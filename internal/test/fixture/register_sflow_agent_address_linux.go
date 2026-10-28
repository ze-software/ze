//go:build linux

// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- daemon agent-address advice proof.
package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("flow-export/sflow-agent-address-daemon", sflowAgentAddressDaemon)
	Register("flow-export/sflow-agent-address-receiver", sflowAgentAddressReceiver)
}

// sflowAgentAddressDaemon supplies real routed interfaces before Ze configures
// the exporter. A dummy pair would incorrectly exercise the loopback branch.
// The two .ci callers MUST hold the sflow-agent-address-topology exclusive group.
func sflowAgentAddressDaemon(ctx context.Context, _ []string) (result error) {
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "sfproof0"}, PeerName: "sfproof1"}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("create owned sFlow proof veth pair: %w", err)
	}
	defer func() {
		result = errors.Join(result, netlink.LinkDel(link))
	}()
	for _, endpoint := range []struct {
		name   string
		prefix string
	}{
		{"sfproof0", "192.0.2.41/32"},
		{"sfproof1", "198.51.100.41/32"},
	} {
		device, err := netlink.LinkByName(endpoint.name)
		if err != nil {
			return fmt.Errorf("find proof interface %s: %w", endpoint.name, err)
		}
		address, err := netlink.ParseAddr(endpoint.prefix)
		if err != nil {
			return err
		}
		if err := netlink.AddrAdd(device, address); err != nil {
			return fmt.Errorf("address proof interface %s: %w", endpoint.name, err)
		}
		if err := netlink.LinkSetUp(device); err != nil {
			return fmt.Errorf("enable proof interface %s: %w", endpoint.name, err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// sFlow v5 Section 4.3: reach advice through the real daemon's config apply,
	// not a fixture call to its classifier or advisor registry.
	command := exec.CommandContext(ctx, "ze", "-") //nolint:gosec // the functional runner supplies the isolated daemon on PATH
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("sFlow proof daemon: %w", err)
	}
	return nil
}

// sflowAgentAddressReceiver verifies the kernel stimulus and then observes the
// daemon's UDP output. It never prints a warning or invokes the warning producer.
func sflowAgentAddressReceiver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("sFlow agent-address receiver needs collector port and expected agent address")
	}
	agent, err := netip.ParseAddr(args[1])
	if err != nil {
		return err
	}
	if !agent.Is4() {
		return errors.New("sFlow agent-address proof expects an IPv4 agent")
	}
	for _, endpoint := range []struct {
		name    string
		address netip.Addr
	}{
		{"sfproof0", netip.MustParseAddr("192.0.2.41")},
		{"sfproof1", netip.MustParseAddr("198.51.100.41")},
	} {
		link, err := netlink.LinkByName(endpoint.name)
		if err != nil {
			return fmt.Errorf("read back proof interface %s: %w", endpoint.name, err)
		}
		if link.Type() != "veth" {
			return fmt.Errorf("proof interface %s has type %s, want veth", endpoint.name, link.Type())
		}
		if link.Attrs().Flags&net.FlagUp == 0 {
			return fmt.Errorf("proof interface %s is down", endpoint.name)
		}
		addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return fmt.Errorf("read back addresses on %s: %w", endpoint.name, err)
		}
		found := false
		for _, address := range addresses {
			got, ok := netip.AddrFromSlice(address.IP)
			if ok && got.Unmap() == endpoint.address {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("proof interface %s does not hold %s", endpoint.name, endpoint.address)
		}
	}
	link, err := netlink.LinkByName("sfproof0")
	if err != nil {
		return err
	}
	port, err := parsePort(args, 0)
	if err != nil {
		return err
	}
	sock, err := listenUDP(port)
	if err != nil {
		return err
	}
	defer sock.Close() //nolint:errcheck // fixture teardown
	return Observe(ctx, "sflow-warning-receiver", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		deadline := time.Now().Add(8 * time.Second)
		if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
			deadline = limit
		}
		if err := sock.SetReadDeadline(deadline); err != nil {
			return err
		}
		var data [65535]byte
		// An exporter may split the interface snapshot across datagrams.
		// Bound both the read window and the number of inspected datagrams.
		for range 128 {
			n, _, err := sock.ReadFromUDP(data[:])
			if err != nil {
				return fmt.Errorf("receive sFlow counter for real interface %d: %w", link.Attrs().Index, err)
			}
			// sFlow v5 Sections 4.3 and 5: require the configured agent and
			// a counter sample, not merely a version-only header.
			found, err := sflowAgentAddressDatagram(data[:n], agent, uint32(link.Attrs().Index))
			if err != nil {
				return err
			}
			if found {
				return nil
			}
		}
		return fmt.Errorf("128 sFlow datagrams had no counter sample for real interface %d", link.Attrs().Index)
	})
}

// sflowAgentAddressDatagram checks the IPv4 datagram and expanded counter sample
// layouts from sFlow v5 Section 5. The header's agent_address is described as
// "IP address of sampling agent, sFlowAgentAddress."
// Header offsets: version=0, address_type=4, agent=8, sub_agent=12,
// sequence=16, uptime=20, sample_count=24; samples start at byte 28.
func sflowAgentAddressDatagram(data []byte, agent netip.Addr, index uint32) (bool, error) {
	if len(data) < 28 {
		return false, fmt.Errorf("sFlow datagram has %d bytes, need IPv4 header", len(data))
	}
	be := binary.BigEndian
	if be.Uint32(data) != 5 {
		return false, errors.New("sFlow proof received a non-v5 datagram")
	}
	if be.Uint32(data[4:]) != 1 {
		return false, errors.New("sFlow proof received a non-IPv4 agent address")
	}
	if got := netip.AddrFrom4([4]byte(data[8:12])); got != agent {
		return false, fmt.Errorf("exported agent address %s, want configured %s", got, agent)
	}
	if be.Uint32(data[16:]) == 0 {
		return false, errors.New("sFlow export has no datagram sequence")
	}
	found := false
	offset := 28
	for remaining := be.Uint32(data[24:]); remaining > 0; remaining-- {
		if len(data)-offset < 8 {
			return false, errors.New("truncated sFlow counter sample header")
		}
		format := be.Uint32(data[offset:])
		size := uint64(be.Uint32(data[offset+4:]))
		offset += 8
		if size > uint64(len(data)-offset) {
			return false, errors.New("truncated sFlow counter sample body")
		}
		sample := data[offset : offset+int(size)]
		offset += int(size)
		// Expanded counter sample: sequence=0, source_type=4, source_index=8,
		// record_count=12, record_format=16, record_length=20, ifIndex=24.
		if format != 4 {
			return false, fmt.Errorf("sFlow sample format %d, want expanded counters (4)", format)
		}
		if len(sample) != 112 {
			return false, fmt.Errorf("sFlow counter sample has %d bytes, want 112", len(sample))
		}
		if be.Uint32(sample) == 0 {
			return false, errors.New("sFlow counter sample has no sequence")
		}
		for _, field := range []struct {
			offset int
			want   uint32
		}{{4, 0}, {12, 1}, {16, 1}, {20, 88}} {
			if got := be.Uint32(sample[field.offset:]); got != field.want {
				return false, fmt.Errorf("sFlow counter field at %d = %d, want %d", field.offset, got, field.want)
			}
		}
		if be.Uint32(sample[8:]) == index {
			if be.Uint32(sample[24:]) != index {
				return false, errors.New("sFlow counter record does not identify its data source")
			}
			found = true
		}
	}
	if offset != len(data) {
		return false, errors.New("sFlow datagram has trailing bytes outside its sample count")
	}
	return found, nil
}
