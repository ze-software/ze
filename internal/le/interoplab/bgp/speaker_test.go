package bgp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var compiledProcessScenarios = []string{
	"as112-community-frr", "as112-origin-as-frr", "bgp-addpath-frr",
	"bgp-community-frr", "bgp-ecmp-frr", "bgp-evpn-frr", "bgp-evpn-gobgp",
	"bgp-extended-community-frr", "bgp-flowspec-frr", "bgp-flowspec-gobgp",
	"bgp-graceful-restart-frr", "bgp-ipv6-ebgp-bird", "bgp-ipv6-ebgp-frr",
	"bgp-ipv6-ebgp-gobgp", "bgp-med-across-as-gobgp", "bgp-multihop-ebgp-bird",
	"bgp-multihop-ebgp-frr", "bgp-multihop-ebgp-gobgp", "bgp-paths-limit-frr",
	"bgp-route-refresh-frr", "bgp-route-withdrawal-frr", "bgp-routes-gobgp",
	"bgp-routes-to-frr", "bgp-vpn-frr", "bgp-vpn-gobgp",
	"bgp-wire-edit-api-origin-bird", "shutdown-cease-frr",
}

func TestAnnouncementPlansPreserveEveryProcessProducer(t *testing.T) {
	for _, scenario := range compiledProcessScenarios {
		plan, err := announcementPlan(scenario)
		if err != nil {
			t.Errorf("%s: %v", scenario, err)
			continue
		}
		if scenario != "bgp-wire-edit-api-origin-bird" && len(plan.updates) == 0 {
			t.Errorf("%s has no ordered updates", scenario)
		}
	}
	if _, err := announcementPlan("not-a-scenario"); err == nil {
		t.Fatal("unknown process helper did not fail closed")
	}
}

func TestAnnouncementPlanOrderAndTimingFixture(t *testing.T) {
	digest := sha256.New()
	for _, scenario := range compiledProcessScenarios {
		plan, err := announcementPlan(scenario)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(digest, "%s|%d|%d|%t\n", scenario, plan.startup, plan.life, plan.stop)
		for _, update := range plan.updates {
			fmt.Fprintf(digest, "%s|%s|%d|%t\n", update.selector, update.command, update.delay, update.quiesce)
		}
	}
	const want = "ca92d5a102f35ce9893b999ef02664d9c03c7ddd0146f0bafd0f438efddfb37a"
	if got := hex.EncodeToString(digest.Sum(nil)); got != want {
		t.Fatalf("compiled announcement order/timing digest = %s, want %s", got, want)
	}
}

func TestDispatchDocumentAcceptsObjectAndEncodedObject(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"sessions":1}`),
		[]byte(`"{\"sessions\":1}"`),
	} {
		document, err := decodeDispatchDocument(data)
		if err != nil || recursiveNumber(document, "sessions") != 1 {
			t.Fatalf("dispatch document %s = %#v, %v", data, document, err)
		}
	}
}

func TestDropMEDPreservesExactUpdateSections(t *testing.T) {
	body := []byte{
		0, 0, // withdrawn length
		0, 18, // path attribute length
		0x40, 1, 1, 0, // ORIGIN
		0x80, 4, 4, 0, 0, 0, 100, // MULTI_EXIT_DISC
		0x40, 3, 4, 10, 0, 0, 9, // NEXT_HOP
		24, 10, 99, 0, // NLRI
	}
	got, changed, err := dropMED(body)
	if err != nil || !changed {
		t.Fatalf("drop MED = changed %v, error %v", changed, err)
	}
	want := []byte{0, 0, 0, 11, 0x40, 1, 1, 0, 0x40, 3, 4, 10, 0, 0, 9, 24, 10, 99, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("MED-stripped body = %x, want %x", got, want)
	}
	if unchanged, changed, err := dropMED(got); err != nil || changed || !bytes.Equal(unchanged, got) {
		t.Fatalf("MED-free body changed: %x, %v, %v", unchanged, changed, err)
	}
}

func TestSpeakerWireAndIndependentOracles(t *testing.T) {
	openA, err := speakerOpen(65001, 90, net.ParseIP("1.2.3.4"), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	openB, err := speakerOpen(65001, 90, net.ParseIP("5.6.7.8"), []bgpFamily{{1, 1}, {25, 70}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(openA[24:28], []byte{1, 2, 3, 4}) || !bytes.Equal(openB[24:28], []byte{5, 6, 7, 8}) {
		t.Fatalf("OPEN router IDs = %x and %x", openA[24:28], openB[24:28])
	}
	if !bytes.Contains(openB, []byte{1, 4, 0, 25, 0, 70}) || !bytes.Contains(openB, []byte{69, 8}) {
		t.Fatalf("OPEN omitted EVPN or receive-only ADD-PATH capability: %x", openB)
	}
	if got := speakerEOR(); len(got) != 23 || !bytes.Equal(got[19:], []byte{0, 0, 0, 0}) {
		t.Fatalf("EOR = %x", got)
	}

	duplicate := decodeSpeakerUpdate(buildSpeakerUpdate([]byte{
		0x40, 1, 1, 0,
		0x40, 3, 4, 10, 0, 0, 9,
		0x40, 3, 4, 10, 0, 0, 10,
	}, []byte{24, 10, 0, 0}))
	verdict := &speakerVerdict{plugin: "no-duplicate-attribute"}
	applySpeakerOracle(duplicate, verdict)
	if len(verdict.failures) != 1 || !strings.Contains(verdict.failures[0], "type 3") {
		t.Fatalf("duplicate attribute verdict = %v", verdict.failures)
	}

	value := []byte{0, 25, 70, 0, 0, 99, 2, 0xaa, 0xbb}
	attrs := append([]byte{0x80, mpReach, byte(len(value))}, value...)
	evpn := decodeSpeakerUpdate(buildSpeakerUpdate(attrs, nil))
	verdict = &speakerVerdict{plugin: "no-unrecognized-evpn-type"}
	applySpeakerOracle(evpn, verdict)
	if verdict.evpnNLRI != 1 || len(verdict.failures) != 1 || !strings.Contains(verdict.failures[0], "route type 99") {
		t.Fatalf("EVPN verdict = count %d failures %v", verdict.evpnNLRI, verdict.failures)
	}
}

func TestSpeakerDecodeSectionsAndExtendedAttributes(t *testing.T) {
	attributes := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 0,
		0x40, 3, 4, 10, 0, 0, 9,
		0x90, mpReach, 0, 2, 0xaa, 0xbb,
	}
	update := decodeSpeakerUpdate(buildSpeakerUpdate(attributes, []byte{24, 10, 0, 0}))
	if len(update.attributes) != 4 || update.attributes[0].code != 1 ||
		update.attributes[3].flags != 0x90 || !bytes.Equal(update.attributes[3].value, []byte{0xaa, 0xbb}) {
		t.Fatalf("decoded attributes = %+v", update.attributes)
	}
	if !bytes.Equal(update.nlri, []byte{24, 10, 0, 0}) || len(update.withdrawn) != 0 {
		t.Fatalf("decoded sections = withdrawn %x nlri %x", update.withdrawn, update.nlri)
	}
	clean := &speakerVerdict{plugin: "no-duplicate-attribute"}
	applySpeakerOracle(update, clean)
	if len(clean.failures) != 0 {
		t.Fatalf("clean update failed strict oracle: %v", clean.failures)
	}
}

func TestSpeakerMultiprotocolRouteAndFamilyBoundaries(t *testing.T) {
	mpReachValue := []byte{0, 25, 70, 0, 0, 2, 2, 0xaa, 0xbb}
	mpReachAttribute := append([]byte{0x80, mpReach, byte(len(mpReachValue))}, mpReachValue...)
	if !speakerCarriesRoutes(decodeSpeakerUpdate(buildSpeakerUpdate(mpReachAttribute, nil))) {
		t.Fatal("MP_REACH-only UPDATE was not route-bearing")
	}
	mpUnreachAttribute := []byte{0x80, mpUnreach, 7, 0, 25, 70, 2, 2, 0xaa, 0xbb}
	if !speakerCarriesRoutes(decodeSpeakerUpdate(buildSpeakerUpdate(mpUnreachAttribute, nil))) {
		t.Fatal("MP_UNREACH with NLRI was not route-bearing")
	}

	options, err := parseSpeakerOptions([]string{
		"--connect", "127.0.0.1:179", "--asn", "65001", "--router-id", "1.2.3.4",
		"--test", "no-duplicate-attribute", "--family", "1:1", "--family", "25:70",
	})
	if err != nil || len(options.families) != 2 || options.families[1] != (bgpFamily{25, 70}) {
		t.Fatalf("family parsing = %+v, %v", options.families, err)
	}
	for _, malformed := range []string{"25", "", "l2vpn/evpn"} {
		_, err := parseSpeakerOptions([]string{
			"--connect", "127.0.0.1:179", "--asn", "65001", "--router-id", "1.2.3.4",
			"--test", "no-duplicate-attribute", "--family", malformed,
		})
		if err == nil {
			t.Errorf("malformed family %q passed", malformed)
		}
	}
}

func TestEVPNOracleAcceptsAssignedTypesAndIgnoresOtherFamilies(t *testing.T) {
	evpnNLRI := make([]byte, 0, 20)
	for routeType := byte(1); routeType <= 5; routeType++ {
		evpnNLRI = append(evpnNLRI, routeType, 2, 0xaa, 0xbb)
	}
	evpnValue := append([]byte{0, 25, 70, 0, 0}, evpnNLRI...)
	evpnAttribute := append([]byte{0x80, mpReach, byte(len(evpnValue))}, evpnValue...)
	verdict := &speakerVerdict{plugin: "no-unrecognized-evpn-type"}
	applySpeakerOracle(decodeSpeakerUpdate(buildSpeakerUpdate(evpnAttribute, nil)), verdict)
	if len(verdict.failures) != 0 || verdict.evpnNLRI != 5 {
		t.Fatalf("assigned EVPN types = count %d failures %v", verdict.evpnNLRI, verdict.failures)
	}

	ipv4Value := []byte{0, 1, 1, 4, 10, 0, 0, 1, 0, 24, 10, 0, 0}
	ipv4Attribute := append([]byte{0x80, mpReach, byte(len(ipv4Value))}, ipv4Value...)
	verdict = &speakerVerdict{plugin: "no-unrecognized-evpn-type"}
	applySpeakerOracle(decodeSpeakerUpdate(buildSpeakerUpdate(ipv4Attribute, nil)), verdict)
	if verdict.evpnNLRI != 0 || len(verdict.failures) != 0 {
		t.Fatalf("IPv4 MP_REACH was treated as EVPN: %+v", verdict)
	}
}

type scriptedSpeakerRead struct {
	data []byte
	err  error
}

type scriptedSpeakerConn struct {
	reads []scriptedSpeakerRead
}

func (c *scriptedSpeakerConn) Read(buffer []byte) (int, error) {
	if len(c.reads) == 0 {
		return 0, errors.New("closed")
	}
	current := c.reads[0]
	c.reads = c.reads[1:]
	return copy(buffer, current.data), current.err
}

func (*scriptedSpeakerConn) Write([]byte) (int, error)        { return 0, errors.New("unused") }
func (*scriptedSpeakerConn) Close() error                     { return nil }
func (*scriptedSpeakerConn) LocalAddr() net.Addr              { return nil }
func (*scriptedSpeakerConn) RemoteAddr() net.Addr             { return nil }
func (*scriptedSpeakerConn) SetDeadline(time.Time) error      { return nil }
func (*scriptedSpeakerConn) SetReadDeadline(time.Time) error  { return nil }
func (*scriptedSpeakerConn) SetWriteDeadline(time.Time) error { return nil }

type speakerTimeoutError struct{}

func (speakerTimeoutError) Error() string   { return "timeout" }
func (speakerTimeoutError) Timeout() bool   { return true }
func (speakerTimeoutError) Temporary() bool { return true }

func TestSpeakerReadDistinguishesIdleFromMidMessageTimeout(t *testing.T) {
	idle := &scriptedSpeakerConn{reads: []scriptedSpeakerRead{{err: speakerTimeoutError{}}}}
	if _, timedOut, err := readSpeakerExact(idle, 4); err != nil || !timedOut {
		t.Fatalf("idle read = timeout %v, error %v", timedOut, err)
	}
	split := &scriptedSpeakerConn{reads: []scriptedSpeakerRead{
		{data: []byte{0xab}},
		{err: speakerTimeoutError{}},
		{data: []byte{0xcd, 0xef, 0x01}},
	}}
	data, timedOut, err := readSpeakerExact(split, 4)
	if err != nil || timedOut || !bytes.Equal(data, []byte{0xab, 0xcd, 0xef, 0x01}) {
		t.Fatalf("split read = %x, timeout %v, error %v", data, timedOut, err)
	}
}

func TestSpeakerDecodeIsBoundedAndEORIsNotRouteBearing(t *testing.T) {
	for _, body := range [][]byte{nil, {0}, {0xff, 0xff}, {0, 10, 0}, {0, 0, 0xff, 0xff}} {
		_ = decodeSpeakerUpdate(body)
	}
	if speakerCarriesRoutes(decodeSpeakerUpdate([]byte{0, 0, 0, 0})) {
		t.Fatal("empty IPv4 UPDATE counted as route-bearing")
	}
	mpEOR := []byte{0, 0, 0, 6, 0x80, mpUnreach, 3, 0, 25, 70}
	if speakerCarriesRoutes(decodeSpeakerUpdate(mpEOR)) {
		t.Fatal("multiprotocol EOR counted as route-bearing")
	}
}
func TestBMPCollectorRecordsWireTypesAndStaysAlive(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(t.TempDir(), "status.json")
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- runBMPCollector(ctx, address, statusPath) }()

	var connection net.Conn
	for range 20 {
		connection, err = net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("dial collector: %v", err)
	}
	for _, messageType := range []byte{3, 0, 4} {
		if _, err := connection.Write([]byte{3, 0, 0, 0, 6, messageType}); err != nil {
			t.Fatal(err)
		}
	}
	var document struct {
		Types []uint8 `json:"types"`
	}
	for range 20 {
		data, readErr := os.ReadFile(statusPath)
		if readErr == nil && json.Unmarshal(data, &document) == nil && len(document.Types) == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := document.Types; !bytes.Equal(got, []byte{3, 0, 4}) {
		t.Fatalf("BMP message types = %v", got)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("collector exit = %v, want context cancellation after retained status", err)
	}
	_ = connection.Close()
}

func buildSpeakerUpdate(attributes, nlri []byte) []byte {
	body := []byte{0, 0, byte(len(attributes) >> 8), byte(len(attributes))}
	body = append(body, attributes...)
	return append(body, nlri...)
}
