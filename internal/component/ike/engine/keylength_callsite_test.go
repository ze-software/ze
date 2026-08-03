package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// kcsCaptureLog returns a logger that records what a call site writes, plus the buffer.
func kcsCaptureLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})), &buf
}

// kcsAssertReported checks that a call site stated the key it accepted above its policy.
func kcsAssertReported(t *testing.T, where string, buf *bytes.Buffer) {
	t.Helper()
	got := buf.String()
	if !strings.Contains(got, "configured-bits=128") || !strings.Contains(got, "accepted-bits=256") {
		t.Errorf("%s logged %q, want the configured and the accepted key lengths", where, got)
	}
}

// VALIDATES: the IKE_SA_INIT responder reports an encryption key it accepts above its own
// policy, and stays silent when the accepted key is the configured one.
// PREVENTS: the report existing only as a helper nobody calls. Deleting the
// logKeyLengthUpgrade line from responder.go left every test green, because the one test
// that covered the message called the helper directly with a hand-built proposal
// (ai/rules/evidence.md: drive the guard from its entry point).
func TestKcsSAInitResponderReportsTheAcceptedKey(t *testing.T) {
	// The initiator offers aes256. The responder policy names aes128, and RFC 7296
	// Section 3.3.5 lets it accept the longer key.
	upgraded, buf := kcsRunSAInit(t, testIKEGroup(), klnIKEGroup())
	if upgraded.State != StateSAInitReceived {
		t.Fatalf("the responder state is %v, want the offer accepted", upgraded.State)
	}
	kcsAssertReported(t, "handleSAInitRequest", buf)

	// The same policy on both sides accepts the configured key and says nothing.
	same, quiet := kcsRunSAInit(t, klnIKEGroup(), klnIKEGroup())
	if same.State != StateSAInitReceived {
		t.Fatalf("the responder state is %v, want the offer accepted", same.State)
	}
	if quiet.Len() != 0 {
		t.Errorf("an accepted key of the configured length logged %q, want silence", quiet.String())
	}
}

// kcsRunSAInit runs one IKE_SA_INIT with the given initiator offer and responder policy,
// and returns the responder SA plus what it logged.
func kcsRunSAInit(t *testing.T, offer, policy ipsec.IKEGroup) (*SA, *bytes.Buffer) {
	t.Helper()
	log, buf := kcsCaptureLog()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "keylength-psk")

	ini, err := newInitiatorSA("ze", iniPeer, offer, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	req := buildSAInitRequest(ini, offer)

	resp, err := newResponderSA("ze", respPeer, policy, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	return resp, buf
}

// VALIDATES: the IKE SA rekey responder reports an encryption key it accepts above its
// own policy, and stays silent when the accepted key is the configured one.
// PREVENTS: the same silent deletion on the rekey call site. respondIKERekey runs its own
// NegotiateIKE, so it needs its own proof that the result reaches an operator.
func TestKcsRekeyResponderReportsTheAcceptedKey(t *testing.T) {
	upgraded := kcsRunIKERekey(t, testIKEGroup(), klnIKEGroup())
	kcsAssertReported(t, "respondIKERekey", upgraded)

	quiet := kcsRunIKERekey(t, klnIKEGroup(), klnIKEGroup())
	if quiet.Len() != 0 {
		t.Errorf("an accepted key of the configured length logged %q, want silence", quiet.String())
	}
}

// kcsRunIKERekey establishes a session, then rekeys the IKE SA with the given offer
// against the given responder policy. It returns what the responder logged.
func kcsRunIKERekey(t *testing.T, offer, policy ipsec.IKEGroup) *bytes.Buffer {
	t.Helper()
	ini, resp, _ := establishPSK(t)

	req, pending, err := initiateIKERekey(ini, offer)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	t.Cleanup(pending.clear)

	reqInner, err := decryptAndParse(resp, parseMsg(t, req), req)
	if err != nil {
		t.Fatalf("responder decrypt of the rekey request: %v", err)
	}

	// The responder answers from its own policy, which is what NegotiateIKE compares the
	// offer against.
	resp.IKEGroup = policy

	log, buf := kcsCaptureLog()
	if _, _, err := respondIKERekey(resp, reqInner, pending.messageID, log); err != nil {
		t.Fatalf("respondIKERekey: %v", err)
	}
	return buf
}
