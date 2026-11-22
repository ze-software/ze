// Design: docs/research/l2tpv2-ze-integration.md -- original IPv6 socket cleanup recovery
package kernel

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

// Failure leaves this exact fake native socket OPEN. Only its provider can
// retry it after recovery; no raw descriptor or interface lookup is involved.
type recoverableIPv6RA struct {
	*carrierTestRA
	failure  error
	attempts int
}

func (s *recoverableIPv6RA) Close() error {
	s.attempts++
	if s.failure != nil {
		return s.failure
	}
	return s.carrierTestRA.Close()
}
func (s *recoverableIPv6RA) RetryIPv6SocketCleanup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Close()
}
func newRecoverableIPv6RA() *recoverableIPv6RA {
	return &recoverableIPv6RA{carrierTestRA: &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}}
}

// The baseline compiles without any new production declaration: the missing
// capability is observed only AFTER a real existing carrier Close has failed.
func TestIPv6CarrierRecoveryRetriesOnlyItsFailedProviderSocket(t *testing.T) {
	cause := errors.New("owned RA socket cleanup unconfirmed")
	ra := newRecoverableIPv6RA()
	ra.failure = cause
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, 7)
	if err := carrier.Close(); !errors.Is(err, cause) {
		t.Fatalf("actual first Close=%v", err)
	}
	if ra.closed {
		t.Fatal("failed provider Close secretly completed")
	}
	if dhcp.closes != 1 {
		t.Fatal("first Close did not release its independent DHCP socket")
	}
	retry, ok := any(carrier).(interface{ RetryIPv6CarrierCleanup(context.Context) error })
	if !ok {
		t.Fatal("failed carrier has no exact provider recovery capability")
	}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := ra.attempts
	if err := retry.RetryIPv6CarrierCleanup(expired); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if ra.attempts != attempts {
		t.Fatal("expired cleanup reached the failed socket")
	}
	if err := retry.RetryIPv6CarrierCleanup(t.Context()); !errors.Is(err, cause) {
		t.Fatalf("unrepaired provider=%v", err)
	}
	if dhcp.closes != 1 {
		t.Fatal("retry closed an already-successful socket")
	}
	ra.failure = nil
	if err := retry.RetryIPv6CarrierCleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !ra.closed || ra.closes != 1 {
		t.Fatal("recovery did not close the exact original failed RA owner")
	}
	attempts = ra.attempts
	if err := retry.RetryIPv6CarrierCleanup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if ra.attempts != attempts || dhcp.closes != 1 {
		t.Fatal("completed recovery repeated physical cleanup")
	}
	if err := carrier.Close(); !errors.Is(err, cause) {
		t.Fatal("ordinary Close lost its cached first cause")
	}
}

func TestIPv6CarrierPartialAcquisitionRetainsFailedCleanupOwner(t *testing.T) {
	for _, phase := range []string{"RA partial", "DHCP refusal", "DHCP partial"} {
		t.Run(phase, func(t *testing.T) {
			setup := errors.New("socket acquisition refused")
			cause := errors.New("partial RA cleanup unconfirmed")
			ra := newRecoverableIPv6RA()
			ra.failure = cause
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			calls := 0
			carrier, err := acquireIPv6Carrier(t.Context(), 7,
				func(context.Context) (raSocket, error) {
					if phase == "RA partial" {
						return ra, setup
					}
					return ra, nil
				},
				func(context.Context) (dhcpSocket, error) {
					calls++
					if phase == "DHCP partial" {
						return dhcp, setup
					}
					return nil, setup
				})
			if !errors.Is(err, setup) || !errors.Is(err, cause) {
				t.Fatalf("actual acquisition/cleanup failure=%v", err)
			}
			if carrier == nil {
				t.Fatal("failed acquisition discarded its still-owned partial socket")
			}
			if phase == "RA partial" && calls != 0 {
				t.Fatal("DHCP opened after partial RA refusal")
			}
			if err := carrier.ReadRS(t.Context()); err == nil {
				t.Fatal("partial owner borrowed an RA reader")
			}
			if err := carrier.WriteRA(nil, 255); err == nil {
				t.Fatal("partial owner published RA")
			}
			if _, _, err := carrier.ReadDHCPv6(t.Context(), nil); err == nil {
				t.Fatal("partial owner borrowed nonexistent DHCP reader")
			}
			if _, err := carrier.WriteDHCPv6(nil, netip.AddrPort{}); err == nil {
				t.Fatal("partial owner published DHCP")
			}
			retry, ok := carrier.(interface{ RetryIPv6CarrierCleanup(context.Context) error })
			if !ok {
				t.Fatal("partial owner omitted exact recovery capability")
			}
			ra.failure = nil
			if err := retry.RetryIPv6CarrierCleanup(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !ra.closed || ra.closes != 1 {
				t.Fatal("partial recovery did not close original RA")
			}
			wantDHCP := 0
			if phase == "DHCP partial" {
				wantDHCP = 1
			}
			if dhcp.closes != wantDHCP {
				t.Fatal("partial recovery touched absent/successful DHCP ownership")
			}
			if err := carrier.Close(); !errors.Is(err, cause) {
				t.Fatal("partial ordinary Close lost cached failure")
			}
		})
	}
}

// net.ErrClosed from an OPAQUE backend is not the concrete Go/Linux ownership
// proof. Such a failed object remains retained and is never blindly reclosed.
func TestIPv6CarrierOpaqueClosedErrorRemainsUnconfirmed(t *testing.T) {
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	ra.closeErr = net.ErrClosed
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, 7)
	if err := carrier.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
	retry, ok := any(carrier).(interface{ RetryIPv6CarrierCleanup(context.Context) error })
	if !ok {
		t.Fatal("failed opaque carrier has no retained recovery observation")
	}
	if err := retry.RetryIPv6CarrierCleanup(t.Context()); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("opaque failure was invented as confirmed release: %v", err)
	}
	if ra.closes != 1 || dhcp.closes != 1 {
		t.Fatal("opaque or already-successful socket was closed again")
	}
}
