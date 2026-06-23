package dhcpserver

// Regression tests for the `ze install remote` provisioning DHCP path.
//
// A remote install is a two-stage boot: the PXE firmware/iPXE fetches the
// kernel+initrd, then the booted kernel runs `ip=dhcp` (see the generated
// boot.ipxe in internal/plugins/imageserver) to bring up networking so the
// installer can reach the image server. Both stages DHCP against this server.
// These tests pin that the server, built exactly as register.go builds it from
// the generated config, answers the kernel's second-stage SELECTING REQUEST and
// stays correctly silent when a client selects a competing DHCP server.

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
)

// buildKernelDiscover mimics the Linux kernel ip=dhcp client DISCOVER: no PXE
// vendor class, a parameter request list, and the broadcast flag set (the kernel
// has no address yet so it must receive the reply by broadcast).
func buildKernelDiscover(mac net.HardwareAddr, xid uint32) []byte {
	pkt := make([]byte, 576)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	binary.BigEndian.PutUint16(pkt[10:12], 0x8000) // broadcast flag
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	off := 240
	pkt[off] = optMessageType
	pkt[off+1] = 1
	pkt[off+2] = msgDiscover
	off += 3
	pkt[off] = optParamReqList
	pkt[off+1] = 4
	copy(pkt[off+2:], []byte{1, 3, 6, 15})
	off += 6
	pkt[off] = optEnd
	// Real kernels send a fixed-size, zero-padded packet; do not truncate.
	return pkt
}

// buildKernelRequest mimics the Linux kernel ip=dhcp SELECTING REQUEST: the
// kernel echoes back the server-id it learned from the OFFER's option 54, with
// option 54 placed before option 50, and the broadcast flag set.
func buildKernelRequest(mac net.HardwareAddr, xid uint32, serverID, requestedIP netip.Addr) []byte {
	pkt := make([]byte, 576)
	pkt[0] = opRequest
	pkt[1] = htypeEthernet
	pkt[2] = hlenEthernet
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	binary.BigEndian.PutUint16(pkt[10:12], 0x8000)
	copy(pkt[28:34], mac)
	binary.BigEndian.PutUint32(pkt[236:240], magicCookie)
	off := 240
	pkt[off] = optMessageType
	pkt[off+1] = 1
	pkt[off+2] = msgRequest
	off += 3
	pkt[off] = optServerID
	pkt[off+1] = 4
	s := serverID.As4()
	copy(pkt[off+2:off+6], s[:])
	off += 6
	pkt[off] = optRequestedIP
	pkt[off+1] = 4
	r := requestedIP.As4()
	copy(pkt[off+2:off+6], r[:])
	off += 6
	pkt[off] = optParamReqList
	pkt[off+1] = 4
	copy(pkt[off+2:], []byte{1, 3, 6, 15})
	off += 6
	pkt[off] = optEnd
	return pkt
}

// installHandler builds a handler exactly as register.go does for the config
// generateConfig() emits for `ze install remote`: an unmasked subnet prefix,
// default-router == server IP, pool .2-.254, PXE enabled.
func installHandler(t *testing.T) *dhcpHandler {
	t.Helper()
	sub := subnetConfig{
		Prefix:        netip.MustParsePrefix("198.19.255.1/24"), // unmasked, as generated
		Ranges:        []addressRange{{Name: "pool1", Start: netip.MustParseAddr("198.19.255.2"), Stop: netip.MustParseAddr("198.19.255.254")}},
		LeaseTimeSec:  86400,
		DefaultRouter: netip.MustParseAddr("198.19.255.1"),
	}
	serverIP := sub.DefaultRouter // register.go: default-router wins
	pxe := pxeConfig{
		Enabled:       true,
		TFTPServer:    netip.MustParseAddr("198.19.255.1"),
		BootfileBIOS:  "ipxe.pxe",
		BootfileUEFI:  "ipxe.efi",
		BootScriptURL: "http://198.19.255.1/install/boot/boot.ipxe",
	}
	return newDHCPHandler(sub, serverIP, pxe)
}

// VALIDATES: the booted kernel's ip=dhcp DISCOVER->REQUEST gets an ACK.
// PREVENTS: a regression where the second-stage SELECTING REQUEST (server-id =
// our OFFER's option 54) is dropped, leaving the installer with no address and
// unable to reach the image server.
func TestInstallKernelIPDHCPSelectingACKs(t *testing.T) {
	h := installHandler(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd5}

	offer := h.handle(buildKernelDiscover(mac, 0xCAFE))
	if offer == nil || getResponseMsgType(offer) != msgOffer {
		t.Fatal("kernel DISCOVER did not produce an OFFER")
	}
	yiaddr := netip.AddrFrom4([4]byte(offer[16:20]))
	siaddr := netip.AddrFrom4([4]byte(offer[20:24]))
	opt54 := getResponseOption(offer, optServerID)
	if len(opt54) != 4 {
		t.Fatalf("OFFER missing option 54 (server-id), got %v", opt54)
	}
	serverID := netip.AddrFrom4([4]byte(opt54))
	if serverID != netip.MustParseAddr("198.19.255.1") || siaddr != serverID {
		t.Fatalf("OFFER server-id=%v siaddr=%v, want both 198.19.255.1", serverID, siaddr)
	}

	ack := h.handle(buildKernelRequest(mac, 0xCAFE, serverID, yiaddr))
	if ack == nil {
		t.Fatalf("kernel REQUEST (server-id=%v requested-ip=%v) got no reply", serverID, yiaddr)
	}
	if getResponseMsgType(ack) != msgAck {
		t.Fatalf("kernel REQUEST reply type = %d, want ACK", getResponseMsgType(ack))
	}
}

// VALIDATES: a REQUEST that selected a different DHCP server is ignored.
// PREVENTS: us hijacking a lease offered by another server (RFC 2131 4.3.2).
// This is the exact shape behind the "no reply (... not our subnet)" symptom
// when a second DHCP server shares the provisioning segment.
func TestInstallForeignServerIDStaysSilent(t *testing.T) {
	h := installHandler(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd5}
	_ = h.handle(buildKernelDiscover(mac, 0xCAFE)) // OFFER .2

	foreign := netip.MustParseAddr("192.168.1.1")
	resp := h.handle(buildKernelRequest(mac, 0xCAFE, foreign, netip.MustParseAddr("198.19.255.2")))
	if resp != nil {
		t.Fatalf("foreign server-id REQUEST got a reply type %d; expected silence", getResponseMsgType(resp))
	}
}

// VALIDATES: the real two-stage boot (PXE firmware round, then kernel round)
// both ACK against one handler instance.
// PREVENTS: a regression where round-1 PXE lease/pool state breaks round-2.
func TestInstallTwoStageBootACKs(t *testing.T) {
	h := installHandler(t)
	defer h.leases.stop()

	mac := net.HardwareAddr{0x60, 0xbe, 0xb4, 0x20, 0xe8, 0xd5}

	// Round 1: UEFI PXE firmware.
	pxeOffer := h.handle(buildPXEDiscover(mac, 0x1111, pxeArchUEFIx64))
	if pxeOffer == nil || getResponseMsgType(pxeOffer) != msgOffer {
		t.Fatal("PXE DISCOVER did not OFFER")
	}
	pxeYiaddr := netip.AddrFrom4([4]byte(pxeOffer[16:20]))
	pxeAck := h.handle(buildPXERequest(mac, 0x1111, pxeYiaddr, netip.MustParseAddr("198.19.255.1"), pxeArchUEFIx64))
	if pxeAck == nil || getResponseMsgType(pxeAck) != msgAck {
		t.Fatal("PXE REQUEST did not ACK")
	}

	// Round 2: booted kernel ip=dhcp.
	offer := h.handle(buildKernelDiscover(mac, 0x2222))
	if offer == nil {
		t.Fatal("kernel DISCOVER got no OFFER")
	}
	yiaddr := netip.AddrFrom4([4]byte(offer[16:20]))
	serverID := netip.AddrFrom4([4]byte(getResponseOption(offer, optServerID)))
	ack := h.handle(buildKernelRequest(mac, 0x2222, serverID, yiaddr))
	if ack == nil || getResponseMsgType(ack) != msgAck {
		t.Fatalf("round-2 kernel REQUEST did not ACK (server-id=%v)", serverID)
	}
}
