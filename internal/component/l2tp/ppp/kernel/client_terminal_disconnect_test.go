// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- exact terminal pair
// Related: client_pair_test.go -- consuming logical pair fixture

package kernel

import (
	"errors"
	"reflect"
	"testing"
)

// TestClientTerminalDisconnectPrecedesChannelClose drives the actual client
// split shutdown with both an attached and never-attached pair. It observes
// exact pair identity, one disconnect before physical channel close, retention
// of every cause, unit survival until explicit cleanup and idempotence.
func TestClientTerminalDisconnectPrecedesChannelClose(t *testing.T) {
	for _, attached := range []bool{false, true} {
		name := "never attached"
		if attached {
			name = "attached"
		}
		t.Run(name, func(t *testing.T) {
			cfg, ops, closers, released, _ := clientConfigurationInputs(t)
			client, err := newClientSession(cfg, ops, closers)
			if err != nil {
				t.Fatal(err)
			}
			pair := client.pair
			cause := errors.New("exact original pair disconnect refused")
			t.Cleanup(func() {
				err := client.Close()
				if attached {
					if !errors.Is(err, cause) {
						t.Errorf("final cleanup lost disconnect cause: %v", err)
					}
					return
				}
				if err != nil {
					t.Error(err)
				}
			})
			if attached {
				if err := client.Connect(); err != nil {
					t.Fatal(err)
				}
			}
			var operations []string
			client.ops.disconnect = func(got *claimedSessionPair) error {
				if got != pair {
					t.Fatal("disconnect changed original pair")
				}
				if pair.channelClosed {
					t.Fatal("disconnect followed physical channel close")
				}
				if pair.unitClosed {
					t.Fatal("disconnect followed unit release")
				}
				operations = append(operations, "disconnect")
				return cause
			}
			for range 2 {
				err := client.CloseChannel()
				if attached {
					if !errors.Is(err, cause) {
						t.Fatalf("disconnect cause lost: %v", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			want := []string(nil)
			if attached {
				want = []string{"disconnect"}
			}
			if !reflect.DeepEqual(operations, want) {
				t.Fatalf("terminal operations = %v", operations)
			}
			if !reflect.DeepEqual(*released, []int{31}) {
				t.Fatalf("early unit release: %v", *released)
			}
			if err := client.CloseUnit(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(*released, []int{31, 32}) {
				t.Fatalf("terminal release = %v", *released)
			}
		})
	}
}
