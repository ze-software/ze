package ipsec

import (
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the retransmit schedule the checker budgets against is the one Ze's
// engine runs. It parses internal/component/ike/engine/fsm.go and compares the three
// constants, and the two expressions retransmitBackoff is built from, with the copies.
// PREVENTS: an engine schedule change that leaves the checker lifting its hold after
// Ze's last retransmission, which would read as a Ze defect.
func TestZeRetransmitScheduleMatchesEngine(t *testing.T) {
	source := filepath.Join("..", "..", "..", "component", "ike", "engine", "fsm.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, source, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"retransmitBase":     "500 * time.Millisecond",
		"retransmitMax":      "60 * time.Second",
		"maxRetransmissions": "7",
	}
	got := map[string]string{}
	var backoffBody string
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			for i, name := range typed.Names {
				if _, wanted := want[name.Name]; wanted && i < len(typed.Values) {
					got[name.Name] = types.ExprString(typed.Values[i])
				}
			}
		case *ast.FuncDecl:
			if typed.Name.Name == "retransmitBackoff" && typed.Body != nil {
				for _, statement := range typed.Body.List {
					assign, isAssign := statement.(*ast.AssignStmt)
					if !isAssign {
						continue
					}
					for _, value := range assign.Rhs {
						backoffBody += types.ExprString(value) + "\n"
					}
				}
			}
		}
		return true
	})
	for name, expression := range want {
		if got[name] != expression {
			t.Errorf("fsm.go %s = %q, the checker's copy assumes %q", name, got[name], expression)
		}
	}
	for _, expression := range []string{"min(attempt - 1, 7)", "retransmitBase * time.Duration(1 << uint(shift))"} {
		if !strings.Contains(backoffBody, expression) {
			t.Errorf("fsm.go retransmitBackoff no longer computes %q; it assigns:\n%s", expression, backoffBody)
		}
	}
	if zeRetransmitBase != 500*time.Millisecond || zeRetransmitMax != 60*time.Second || zeMaxRetransmissions != 7 {
		t.Errorf("the checker's copies drifted from the values above")
	}
}

// VALIDATES: the bounds the checker names: Ze's seventh IKE_AUTH retransmission leaves
// 32 s after the request, and Ze gives the handshake up at 64 s.
// PREVENTS: an off-by-one in the schedule walk that budgets one backoff too many.
func TestZeAuthRetransmitWindow(t *testing.T) {
	if got := zeLastAuthRetransmitOffset(); got != 32*time.Second {
		t.Errorf("last retransmission at %s, want 32s", got)
	}
	if got := zeAuthGiveUpOffset(); got != 64*time.Second {
		t.Errorf("give-up at %s, want 64s", got)
	}
}

// VALIDATES: the forged response is an IKE_AUTH response at Message ID 1 under the
// given SPIs, its UDP length is the one the hold rule exempts, and the octet the hold
// rule reads at authHoldExchangeType is its Exchange Type.
// PREVENTS: a hold that drops the forgeries too, or one that reads another octet and
// holds nothing.
func TestForgedAuthResponseShape(t *testing.T) {
	const initiatorSPI, responderSPI = "0011223344556677", "8899aabbccddeeff"
	forged, err := unprotForgedAuthHex(initiatorSPI, responderSPI,
		unprotFlagsResponderResponse, authRespForgedMessageID, unprotInnerIDr)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := hex.DecodeString(strings.TrimPrefix(forged, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	const udpHeaderOctets, markerOctets = 8, 4
	if len(payload)+udpHeaderOctets != unprotForgedAuthUDPOctets {
		t.Fatalf("UDP length %d, the hold exempts %d", len(payload)+udpHeaderOctets, unprotForgedAuthUDPOctets)
	}
	ike := payload[markerOctets:]
	if hex.EncodeToString(ike[0:16]) != initiatorSPI+responderSPI {
		t.Errorf("SPIs %x, want %s%s", ike[0:16], initiatorSPI, responderSPI)
	}
	if ike[18] != 35 || ike[19] != 0x20 || ike[23] != authRespForgedMessageID || ike[28] != unprotInnerIDr {
		t.Errorf("header exchange %d flags %#x message %d inner %d, want 35 0x20 1 36", ike[18], ike[19], ike[23], ike[28])
	}
	if length := int(ike[24])<<24 | int(ike[25])<<16 | int(ike[26])<<8 | int(ike[27]); length != len(ike) {
		t.Errorf("IKE Length %d, message is %d octets", length, len(ike))
	}
	fields := strings.Split(strings.TrimPrefix(authHoldExchangeType, "@th,"), ",")
	bit, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	octet := bit/8 - udpHeaderOctets
	if strconv.Itoa(int(payload[octet])) != ikeExchangeIKEAuth {
		t.Errorf("the hold reads octet %d = %d, want the Exchange Type %s", octet, payload[octet], ikeExchangeIKEAuth)
	}
}

// VALIDATES: the held handshake's SPI pair is the one listed SA that is not the
// control's, and a listing with none or two such SAs is an error.
// PREVENTS: forging under the control SA's SPIs, which Ze no longer holds, so the
// count would rise for the wrong reason or not at all.
func TestHeldSPIPair(t *testing.T) {
	const controlLine = "ze: #1, DELETING, IKEv2, 1111111111111111_i 2222222222222222_r*\n"
	const heldLine = "ze: #2, ESTABLISHED, IKEv2, 3333333333333333_i 4444444444444444_r*\n"
	const otherLine = "ze: #3, ESTABLISHED, IKEv2, 5555555555555555_i 6666666666666666_r*\n"
	control := ikeSPIPair{initiator: "1111111111111111", responder: "2222222222222222"}

	held, err := heldSPIPair(controlLine+heldLine, control)
	if err != nil {
		t.Fatal(err)
	}
	if held.initiator != "3333333333333333" || held.responder != "4444444444444444" {
		t.Errorf("held pair %+v", held)
	}
	for _, refused := range []string{controlLine, controlLine + heldLine + otherLine, ""} {
		if _, err := heldSPIPair(refused, control); err == nil {
			t.Errorf("listing %q was read as one held SA", refused)
		}
	}
}

// VALIDATES: Ze's established initiator SA is read with its SPI, a listing with none
// established reads as not established, and two established SAs or a responder SA
// are errors.
// PREVENTS: a new SA, started after the forgeries ended the first, being read as
// the one they targeted.
func TestZeEstablishedInitiatorSA(t *testing.T) {
	const established = `[{"peer-name":"swan","state":"established","is-initiator":true,"initiator-spi":"3333333333333333"}]`
	reading, err := zeEstablishedInitiatorSA(established)
	if err != nil {
		t.Fatal(err)
	}
	if !reading.established || reading.initiatorSPI != "3333333333333333" {
		t.Errorf("reading %+v", reading)
	}
	pending, err := zeEstablishedInitiatorSA(`[{"peer-name":"swan","state":"auth-sent","is-initiator":true,"initiator-spi":"3333333333333333"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if pending.established {
		t.Errorf("a handshake in progress read as established")
	}
	for _, refused := range []string{
		`[{"peer-name":"swan","state":"established","is-initiator":false,"initiator-spi":"3333333333333333"}]`,
		strings.TrimSuffix(established, "]") + "," + strings.TrimPrefix(established, "["),
		"show vpn ipsec sa: no daemon",
	} {
		if _, err := zeEstablishedInitiatorSA(refused); err == nil {
			t.Errorf("answer %q was read", refused)
		}
	}
}
