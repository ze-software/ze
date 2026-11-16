// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- direct RADIUS authentication requires subscriber identity

package l2tpauthradius

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestRADIUSAuthRequiresSubscriberIdentity calls the real provider directly.
// Missing context is denied synchronously before no-auth admission; a valid
// opaque ID keeps no-auth admission usable without invoking the async callback.
func TestRADIUSAuthRequiresSubscriberIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		accept bool
	}{
		{name: "missing"},
		{name: "opaque", id: "direct-opaque-subscriber", accept: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := newRADIUSAuth()
			callbacks := 0
			result := provider.handle(ppp.EventAuthRequest{
				SubscriberID: tc.id, Method: ppp.AuthMethodNone,
			}, func(bool, string, []byte) error {
				callbacks++
				return nil
			})
			if result.Accept != tc.accept {
				t.Fatalf("direct RADIUS admission accepted=%t, want %t; reason: %s", result.Accept, tc.accept, result.Message)
			}
			if result.Handled {
				t.Fatal("direct no-auth decision became asynchronous")
			}
			if callbacks != 0 {
				t.Fatalf("synchronous decision invoked callback %d times", callbacks)
			}
			if tc.id == "" {
				if result.Message == "" {
					t.Fatal("missing subscriber identity refusal has no reason")
				}
				if !strings.Contains(result.Message, "identity") {
					t.Fatalf("refusal does not identify the missing context: %s", result.Message)
				}
			}
		})
	}
}
