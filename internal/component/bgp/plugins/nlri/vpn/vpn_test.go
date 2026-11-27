// Design: docs/architecture/wire/nlri.md -- VPN NLRI decode stdout error propagation
// Related: vpn.go -- runVPNDecode and handleDecodeNLRI

package vpn

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type vpnFailWriter struct {
	err error
}

func (w vpnFailWriter) Write([]byte) (int, error) {
	return 0, w.err
}

// TestVPNDecodeReturnsExitOneOnWriteError verifies that runVPNDecode returns
// exit code 1 when writing decoded output (JSON or unknown) fails.
//
// VALIDATES: AC-3 (vpn/vpn.go runVPNDecode and handleDecodeNLRI write errors).
// PREVENTS: broken stdout pipes exiting 0 with lost VPN decode output.
func TestVPNDecodeReturnsExitOneOnWriteError(t *testing.T) {
	t.Parallel()
	fw := vpnFailWriter{err: errors.New("broken pipe")}

	// 112 bits = 24-bit label (100, S=1) + 64-bit RD (65001:100) + 24-bit prefix (10.0.0.0/24).
	validVPN := "700006410000fde9000000640a0000"

	t.Run("valid_nlri_write_error", func(t *testing.T) {
		t.Parallel()
		code := runVPNDecode(strings.NewReader("decode nlri ipv4/mpls-vpn "+validVPN+"\n"), fw)
		assert.Equal(t, 1, code)
	})

	t.Run("unknown_hex_write_error", func(t *testing.T) {
		t.Parallel()
		code := runVPNDecode(strings.NewReader("decode nlri ipv4/mpls-vpn not-hex\n"), fw)
		assert.Equal(t, 1, code)
	})

	t.Run("short_line_write_error", func(t *testing.T) {
		t.Parallel()
		code := runVPNDecode(strings.NewReader("decode nlri\n"), fw)
		assert.Equal(t, 1, code)
	})
}
