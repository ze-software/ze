// Design: docs/guide/l2tp.md -- character limits on disconnect input

package cmd

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
)

// TestSessionTeardownAuditLengthCountsCharacters exercises each documented
// 256-character audit field with multibyte text, preserving the following cause
// and original guard. The next character must refuse before selection.
func TestSessionTeardownAuditLengthCountsCharacters(t *testing.T) {
	const reference = "l2tp-00000000000000000000000000000001"
	for _, keyword := range []string{"actor", "reason"} {
		for _, count := range []int{256, 257} {
			t.Run(fmt.Sprintf("%s/%d", keyword, count), func(t *testing.T) {
				value := strings.Repeat("界", count)
				request, refusal := parseSessionTeardownRequest([]string{
					"42", "expected-id", reference, keyword, value, "cause", "6",
				})
				if count == 257 {
					require.NotNil(t, refusal)
					return
				}
				require.Nil(t, refusal, "256 characters are permitted even when their UTF-8 encoding is longer")
				require.Equal(t, uint16(42), request.selector.SID)
				require.Equal(t, reference, request.selector.Birth.String())
				require.Equal(t, uint32(6), request.cause)
				if keyword == "actor" {
					require.Equal(t, value, request.actor)
					return
				}
				require.Equal(t, value, request.reason)
			})
		}
	}
}

// TestSessionTeardownListenerLengthCountsCharacters binds an allowed multibyte
// configured name to its original endpoint; the 254th character is refused by
// the command's declared 253-character selector bound.
func TestSessionTeardownListenerLengthCountsCharacters(t *testing.T) {
	endpoint := netip.MustParseAddrPort("127.0.0.1:1701")
	for _, count := range []int{253, 254} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			name := strings.Repeat("界", count)
			request, refusal := parseSessionTeardownRequest([]string{"42", "listener", name})
			if count == 254 {
				require.NotNil(t, refusal)
				return
			}
			require.Nil(t, refusal)
			selector, refusal := request.resolveSelector(&fakeService{listeners: []l2tp.ListenerSnapshot{{Name: name, Addr: endpoint}}})
			require.Nil(t, refusal)
			require.Equal(t, uint16(42), selector.SID)
			require.Equal(t, endpoint, selector.Listener)
		})
	}
}
