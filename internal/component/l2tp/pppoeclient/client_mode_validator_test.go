// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- fixture validation fidelity.
// Related: client_protocol_fixture_test.go -- actual invalid-mode validator.

package pppoeclient

import (
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// TestLogicalClientModeUsesRealValidator makes validation deterministic before
// Close can win the existing race. Exact errors distinguish kernel validation
// from invalid-descriptor/platform refusal and fixture admission.
func TestLogicalClientModeUsesRealValidator(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode pppkernel.NPMode
		want string
	}{
		{
			name: "unspecified",
			mode: pppkernel.NPModeUnspecified,
			want: "ppp: IPv4 mode is unspecified; select NPModePass or NPModeDrop",
		},
		{
			name: "unnamed255",
			mode: pppkernel.NPMode(255),
			want: "ppp: invalid IPv4 mode 255; select NPModePass or NPModeDrop",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			link := &sessionLink{client: testClientProtocol(newConnectedExchange()), stopped: make(chan struct{})}
			err := link.setIPv4Mode(tc.mode)
			if err == nil {
				t.Fatal("invalid mode admitted before shutdown")
			}
			if err.Error() != tc.want {
				t.Fatalf("mode %d refusal = %q, want %q", tc.mode, err.Error(), tc.want)
			}
		})
	}
}

// TestLogicalClientModeAcceptsKnownModes preserves useful legal mode decisions
// in pure protocol fixtures without asking a descriptor to authorize an ioctl.
func TestLogicalClientModeAcceptsKnownModes(t *testing.T) {
	link := &sessionLink{client: testClientProtocol(newConnectedExchange()), stopped: make(chan struct{})}
	for _, mode := range []pppkernel.NPMode{pppkernel.NPModePass, pppkernel.NPModeDrop} {
		if err := link.setIPv4Mode(mode); err != nil {
			t.Fatalf("legal logical mode %d: %v", mode, err)
		}
	}
}
