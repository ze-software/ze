// Design: docs/architecture/testing/interop.md -- collector oracle discrimination.
// Related: check_ospf_bgpls.go -- production scenario checker, not the OSPF producer.
package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

const (
	ospfBGPLSTestNodeLocal     = "000100150300000000000000000100000802030004ac1e0002"
	ospfBGPLSTestNodeRemote    = "000100150300000000000000000100000802030004ac1e0003"
	ospfBGPLSTestLinkLocal     = "000200210300000000000000000100000802030004ac1e00020101000802030004ac1e0003"
	ospfBGPLSTestLinkRemote    = "000200210300000000000000000100000802030004ac1e00030101000802030004ac1e0002"
	ospfBGPLSTestLinkInterAS   = "000200290300000000000000000100000802030004ac1e000201010010020000040000fde902030004cb007109"
	ospfBGPLSTestLocalID       = "04040004ac1e0002"
	ospfBGPLSTestRemoteID      = "04040004ac1e0003"
	ospfBGPLSTestLocalLinkIDs  = "04040004ac1e000204060004ac1e0003"
	ospfBGPLSTestRemoteLinkIDs = "04040004ac1e000304060004ac1e0002"
	ospfBGPLSTestInterASIDs    = "04040004ac1e000204060004cb0071090407001020010db8000000000000000000000009"
)

func TestOSPFRouterIDCollectorRequiresExactFinalInventory(t *testing.T) {
	ordinary := []string{
		ospfBGPLSTestUpdate(t, ospfBGPLSTestNodeLocal, ospfBGPLSTestLocalID, false),
		ospfBGPLSTestUpdate(t, ospfBGPLSTestNodeRemote, ospfBGPLSTestRemoteID, false),
		ospfBGPLSTestUpdate(t, ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalLinkIDs, false),
		ospfBGPLSTestUpdate(t, ospfBGPLSTestLinkRemote, ospfBGPLSTestRemoteLinkIDs, false),
	}
	interAS := []string{
		ordinary[0],
		ospfBGPLSTestUpdate(t, ospfBGPLSTestLinkInterAS, ospfBGPLSTestInterASIDs, false),
	}
	for _, tc := range []struct {
		name    string
		updates []string
		interAS bool
	}{
		{"ordinary-both-directions", ordinary, false},
		{"inter-as-ipv4-ipv6", interAS, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ospfBGPLSRouterIDVerdict(ospfBGPLSTestLog(tc.updates), "172.30.0.2", "172.30.0.3", tc.interAS); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct {
		name       string
		nlri       string
		attributes string
		withdraw   bool
		interAS    bool
	}{
		{"local-node-id-dropped", ospfBGPLSTestNodeLocal, "", false, false},
		{"remote-node-id-dropped", ospfBGPLSTestNodeRemote, "", false, false},
		{"local-link-id-dropped", ospfBGPLSTestLinkLocal, "04060004ac1e0003", false, false},
		{"remote-link-id-dropped", ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalID, false, false},
		{"local-remote-swapped", ospfBGPLSTestLinkLocal, ospfBGPLSTestRemoteLinkIDs, false, false},
		{"reverse-direction-wrong", ospfBGPLSTestLinkRemote, ospfBGPLSTestLocalLinkIDs, false, false},
		{"duplicate-local-id", ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalID + ospfBGPLSTestLocalLinkIDs, false, false},
		{"invented-local-ipv6", ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalLinkIDs + "0405001020010db8000000000000000000000009", false, false},
		{"withdraw-node-after-match", ospfBGPLSTestNodeLocal, "", true, false},
		{"withdraw-link-after-match", ospfBGPLSTestLinkLocal, "", true, false},
		{"malformed-extra-tlv", ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalLinkIDs + "00", false, false},
		{"remote-ipv6-dropped", ospfBGPLSTestLinkInterAS, "04040004ac1e000204060004cb007109", false, true},
		{"remote-ipv6-mislabeled-local", ospfBGPLSTestLinkInterAS, "04040004ac1e00020405001020010db800000000000000000000000904060004cb007109", false, true},
		{"wrong-remote-ipv4", ospfBGPLSTestLinkInterAS, strings.Replace(ospfBGPLSTestInterASIDs, "cb007109", "cb007108", 1), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := ordinary
			if tc.interAS {
				initial = interAS
			}
			logs := ospfBGPLSTestLog(initial) + ospfBGPLSTestUpdate(t, tc.nlri, tc.attributes, tc.withdraw)
			if err := ospfBGPLSRouterIDVerdict(logs, "172.30.0.2", "172.30.0.3", tc.interAS); err == nil {
				t.Fatal("accepted final inventory violating the exact router-ID claim")
			}
		})
	}
	for _, logs := range []string{
		ospfBGPLSTestLog(nil),
		"result: PASS\n" + strings.Join(ordinary, ""),
		strings.Replace(ospfBGPLSTestLog(ordinary), "result: PASS", "result: FAIL", 1),
		strings.Replace(ospfBGPLSTestLog(ordinary), "capture-complete: yes", "capture-complete: no", 1),
		strings.Replace(ospfBGPLSTestLog(ordinary), "note: capture-complete: yes\n", "", 1),
		ospfBGPLSTestLog(ordinary) + "note: received NOTIFICATION: 0600\n",
		ospfBGPLSTestLog(ordinary[:3]),
		ospfBGPLSTestLog(ordinary) + "note: update-hex: 0000000100\n",
	} {
		if err := ospfBGPLSRouterIDVerdict(logs, "172.30.0.2", "172.30.0.3", false); err == nil {
			t.Fatalf("accepted incomplete or malformed collector evidence: %q", logs)
		}
	}
}

// The final withdrawal begins inside the capture window but is truncated when
// the peer closes after that window. Elapsed time must not certify stale routes.
func TestOSPFRouterIDCollectorRejectsPartialWithdrawalPastDeadline(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := listener.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	const capture = time.Second
	verdict := &speakerVerdict{plugin: speakerOracleNoDuplicateAttribute}
	result := make(chan error, 1)
	go func() {
		result <- runSpeakerSession(speakerOptions{
			connect: listener.Addr().String(), asn: 65001, routerID: "172.30.0.10",
			holdTime: 90, duration: capture, families: familyFlags{{afi: 16388, safi: 71}},
		}, verdict)
	}()
	connection, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if kind, _, _, err := readSpeakerMessage(connection, time.Now().Add(10*time.Second)); err != nil || kind != bgpOpen {
		t.Fatalf("collector OPEN: kind=%d err=%v", kind, err)
	}
	open, err := speakerOpen(65001, 90, net.ParseIP("172.30.0.2"), familyFlags{{afi: 16388, safi: 71}}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write(append(open, speakerKeepalive()...)); err != nil {
		t.Fatal(err)
	}
	for {
		kind, _, idle, err := readSpeakerMessage(connection, time.Now().Add(10*time.Second))
		if err != nil || idle {
			t.Fatalf("collector establishment: idle=%t err=%v", idle, err)
		}
		if kind == bgpUpdate {
			break // The collector's EOR follows its Established transition.
		}
	}
	var packets []byte
	for _, entry := range [][2]string{
		{ospfBGPLSTestNodeLocal, ospfBGPLSTestLocalID},
		{ospfBGPLSTestNodeRemote, ospfBGPLSTestRemoteID},
		{ospfBGPLSTestLinkLocal, ospfBGPLSTestLocalLinkIDs},
		{ospfBGPLSTestLinkRemote, ospfBGPLSTestRemoteLinkIDs},
	} {
		text := speakerUpdateHexes(ospfBGPLSTestUpdate(t, entry[0], entry[1], false))[0]
		body, err := hex.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, speakerMessage(bgpUpdate, body)...)
	}
	text := speakerUpdateHexes(ospfBGPLSTestUpdate(t, ospfBGPLSTestLinkLocal, "", true))[0]
	withdrawal, err := hex.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	partial := speakerMessage(bgpUpdate, withdrawal)
	packets = append(packets, partial[:bgpHeaderLength+2]...)
	if _, err := connection.Write(packets); err != nil {
		t.Fatal(err)
	}
	// Cross the actual capture deadline while keeping its partial frame open.
	// A channel barrier alone cannot exercise the elapsed-time completion bug.
	timer := time.NewTimer(2 * capture)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal("test canceled before crossing capture deadline")
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	var runErr error
	select {
	case runErr = <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("collector did not stop after truncated withdrawal EOF")
	}
	logs := "result: PASS\n"
	if runErr != nil || len(verdict.failures) != 0 {
		logs = "result: FAIL\n"
	}
	for _, note := range verdict.notes {
		logs += "note: " + note + "\n"
	}
	if got := len(speakerUpdateHexes(logs)); got != 4 {
		t.Fatalf("fixture did not deliver its complete initial inventory: updates=%d, logs=%s", got, logs)
	}
	if complete, _ := logField(logs, "capture-complete"); complete == logValueYes {
		t.Errorf("partial withdrawal followed by EOF certified a complete capture: %s", logs)
	}
	if err := ospfBGPLSRouterIDVerdict(logs, "172.30.0.2", "172.30.0.3", false); err == nil {
		t.Error("collector accepted stale inventory after partial withdrawal EOF beyond deadline")
	}
}

func ospfBGPLSTestLog(updates []string) string {
	return "result: PASS\nnote: established: yes\nnote: capture-complete: yes\n" + strings.Join(updates, "")
}

// Deliberately literal wire fixtures, independent of the OSPF producer and
// exporter. These test the collector's judgement; they are not producer proof.
func ospfBGPLSTestUpdate(t *testing.T, nlri, ids string, withdraw bool) string {
	t.Helper()
	route, err := hex.DecodeString(nlri)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := hex.DecodeString(ids)
	if err != nil {
		t.Fatal(err)
	}
	mp := []byte{0x40, 4, 71, 4, 172, 30, 0, 2, 0}
	code := byte(14)
	var path []byte
	if withdraw {
		mp = mp[:3]
		code = 15
	} else {
		path = append([]byte{0x80, 29, byte(len(attributes))}, attributes...)
	}
	mp = append(mp, route...)
	path = append(path, 0x80, code, byte(len(mp)))
	path = append(path, mp...)
	body := make([]byte, 4, 4+len(path))
	binary.BigEndian.PutUint16(body[2:], uint16(len(path)))
	body = append(body, path...)
	return fmt.Sprintf("note: update-hex: %x\n", body)
}
