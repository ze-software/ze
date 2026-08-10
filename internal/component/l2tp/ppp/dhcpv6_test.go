package ppp

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func testServerID() DHCPv6DUID {
	return DHCPv6DUID{Type: DUIDTypeEN, EnterpriseNum: 12345, ID: []byte{0x01, 0x02}}
}

func buildSolicit(txnID [3]byte, clientID DHCPv6DUID, iaid uint32) []byte { //nolint:unparam // test helper keeps param for symmetry with buildRenew/buildRelease
	var buf [512]byte
	off := 0

	// Message header: type(1) + txn-id(3)
	buf[off] = DHCPv6Solicit
	copy(buf[off+1:off+4], txnID[:])
	off += 4

	// Client ID option
	duidBytes := encodeDUID(clientID)
	binary.BigEndian.PutUint16(buf[off:], D6OptClientID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(duidBytes)))
	off += 4
	copy(buf[off:], duidBytes)
	off += len(duidBytes)

	// IA_PD option (empty, just IAID + T1 + T2)
	binary.BigEndian.PutUint16(buf[off:], D6OptIAPD)
	binary.BigEndian.PutUint16(buf[off+2:], 12) // len: 4+4+4
	off += 4
	binary.BigEndian.PutUint32(buf[off:], iaid)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0) // T1
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0) // T2
	off += 4

	return buf[:off]
}

func buildRenew(txnID [3]byte, clientID, serverID DHCPv6DUID, iaid uint32) []byte { //nolint:unparam // test helper keeps param for symmetry with buildSolicit/buildRelease
	var buf [512]byte
	off := 0

	buf[off] = DHCPv6Renew
	copy(buf[off+1:off+4], txnID[:])
	off += 4

	duidBytes := encodeDUID(clientID)
	binary.BigEndian.PutUint16(buf[off:], D6OptClientID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(duidBytes)))
	off += 4
	copy(buf[off:], duidBytes)
	off += len(duidBytes)

	srvBytes := encodeDUID(serverID)
	binary.BigEndian.PutUint16(buf[off:], D6OptServerID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(srvBytes)))
	off += 4
	copy(buf[off:], srvBytes)
	off += len(srvBytes)

	binary.BigEndian.PutUint16(buf[off:], D6OptIAPD)
	binary.BigEndian.PutUint16(buf[off+2:], 12)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], iaid)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4

	return buf[:off]
}

func buildRelease(txnID [3]byte, clientID, serverID DHCPv6DUID, iaid uint32) []byte {
	var buf [512]byte
	off := 0

	buf[off] = DHCPv6Release
	copy(buf[off+1:off+4], txnID[:])
	off += 4

	duidBytes := encodeDUID(clientID)
	binary.BigEndian.PutUint16(buf[off:], D6OptClientID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(duidBytes)))
	off += 4
	copy(buf[off:], duidBytes)
	off += len(duidBytes)

	srvBytes := encodeDUID(serverID)
	binary.BigEndian.PutUint16(buf[off:], D6OptServerID)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(len(srvBytes)))
	off += 4
	copy(buf[off:], srvBytes)
	off += len(srvBytes)

	binary.BigEndian.PutUint16(buf[off:], D6OptIAPD)
	binary.BigEndian.PutUint16(buf[off+2:], 12)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], iaid)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], 0)
	off += 4

	return buf[:off]
}

func encodeDUID(d DHCPv6DUID) []byte {
	var buf [64]byte
	binary.BigEndian.PutUint16(buf[0:], d.Type)
	off := 2
	switch d.Type {
	case DUIDTypeEN:
		binary.BigEndian.PutUint32(buf[off:], d.EnterpriseNum)
		off += 4
		copy(buf[off:], d.ID)
		off += len(d.ID)
	case DUIDTypeLL:
		binary.BigEndian.PutUint16(buf[off:], d.HWType)
		off += 2
		copy(buf[off:], d.ID)
		off += len(d.ID)
	}
	return buf[:off]
}

func TestParseDHCPv6Solicit(t *testing.T) {
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	txn := [3]byte{0x12, 0x34, 0x56}
	pkt := buildSolicit(txn, clientDUID, 1)

	msg, err := parseDHCPv6(pkt)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DHCPv6Solicit {
		t.Errorf("type = %d, want %d", msg.Type, DHCPv6Solicit)
	}
	if msg.TransactionID != txn {
		t.Errorf("txn = %x, want %x", msg.TransactionID, txn)
	}
	if msg.ClientID == nil {
		t.Fatal("client ID not parsed")
	}
	if msg.IAPD == nil {
		t.Fatal("IA_PD not parsed")
	}
	if msg.IAPD.IAID != 1 {
		t.Errorf("IAID = %d, want 1", msg.IAPD.IAID)
	}
}

func TestDHCPv6SolicitReply(t *testing.T) {
	srv := testServerID()
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	txn := [3]byte{0x12, 0x34, 0x56}

	solicit := buildSolicit(txn, clientDUID, 1)
	msg, err := parseDHCPv6(solicit)
	if err != nil {
		t.Fatal(err)
	}

	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	var buf [512]byte
	n := buildDHCPv6Reply(buf[:], dHCPv6ReplyConfig{
		Type:          DHCPv6Advertise,
		TransactionID: msg.TransactionID,
		ServerID:      srv,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  604800,
		ValidLifetime: 2592000,
		T1:            302400,
		T2:            483840,
	})

	reply, err := parseDHCPv6(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != DHCPv6Advertise {
		t.Errorf("reply type = %d, want %d", reply.Type, DHCPv6Advertise)
	}
	if reply.TransactionID != txn {
		t.Errorf("reply txn = %x, want %x", reply.TransactionID, txn)
	}
	if reply.ServerID == nil {
		t.Fatal("server ID missing in reply")
	}
	if reply.IAPD == nil {
		t.Fatal("IA_PD missing in reply")
	}
	if reply.IAPD.IAID != 1 {
		t.Errorf("reply IAID = %d, want 1", reply.IAPD.IAID)
	}
	if reply.IAPD.Prefix == nil {
		t.Fatal("IA_Prefix missing in reply")
	}
	if reply.IAPD.Prefix.Prefix != prefix {
		t.Errorf("prefix = %s, want %s", reply.IAPD.Prefix.Prefix, prefix)
	}
}

func TestDHCPv6NoPrefixAvail(t *testing.T) {
	srv := testServerID()
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	txn := [3]byte{0xab, 0xcd, 0xef}

	var buf [512]byte
	n := buildDHCPv6StatusReply(buf[:], dHCPv6StatusReplyConfig{
		TransactionID: txn,
		ServerID:      srv,
		ClientID:      &clientDUID,
		StatusCode:    D6StatusNoPrefixAvail,
		StatusMessage: "pool exhausted",
	})

	reply, err := parseDHCPv6(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != DHCPv6Reply {
		t.Errorf("type = %d, want %d", reply.Type, DHCPv6Reply)
	}
	if reply.StatusCode == nil {
		t.Fatal("status code missing")
	}
	if *reply.StatusCode != D6StatusNoPrefixAvail {
		t.Errorf("status = %d, want %d", *reply.StatusCode, D6StatusNoPrefixAvail)
	}
}

func TestDHCPv6RenewReply(t *testing.T) {
	srv := testServerID()
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	txn := [3]byte{0x11, 0x22, 0x33}

	renew := buildRenew(txn, clientDUID, srv, 1)
	msg, err := parseDHCPv6(renew)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DHCPv6Renew {
		t.Errorf("type = %d, want %d", msg.Type, DHCPv6Renew)
	}

	prefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	var buf [512]byte
	n := buildDHCPv6Reply(buf[:], dHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: msg.TransactionID,
		ServerID:      srv,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  604800,
		ValidLifetime: 2592000,
		T1:            302400,
		T2:            483840,
	})

	reply, err := parseDHCPv6(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != DHCPv6Reply {
		t.Errorf("type = %d, want %d", reply.Type, DHCPv6Reply)
	}
	if reply.IAPD == nil || reply.IAPD.Prefix == nil {
		t.Fatal("prefix missing in renew reply")
	}
	if reply.IAPD.Prefix.Prefix != prefix {
		t.Errorf("prefix = %s, want %s", reply.IAPD.Prefix.Prefix, prefix)
	}
}

func TestDHCPv6ReleaseHandling(t *testing.T) {
	clientDUID := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}}
	srv := testServerID()
	txn := [3]byte{0x44, 0x55, 0x66}

	release := buildRelease(txn, clientDUID, srv, 1)
	msg, err := parseDHCPv6(release)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Type != DHCPv6Release {
		t.Errorf("type = %d, want %d", msg.Type, DHCPv6Release)
	}
	if msg.IAPD == nil {
		t.Fatal("IA_PD missing in release")
	}
}

func TestParseDHCPv6TooShort(t *testing.T) {
	_, err := parseDHCPv6([]byte{1, 2})
	if err == nil {
		t.Fatal("expected error for too-short packet")
	}
}

func TestParseDHCPv6BadOptionLen(t *testing.T) {
	// Valid header but option length extends beyond packet.
	pkt := []byte{
		DHCPv6Solicit, 0x00, 0x00, 0x01,
		0x00, 0x01, 0xff, 0xff, // option 1, length 65535
	}
	_, err := parseDHCPv6(pkt)
	if err == nil {
		t.Fatal("expected error for option overrun")
	}
}
