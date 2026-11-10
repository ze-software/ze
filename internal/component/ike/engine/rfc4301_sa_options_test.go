// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- per-peer SA options the backend must honor
// Related: sa_options.go -- verifyPeerSAOptions, the commit-time refusal under test
// Related: child.go -- isXFRMUnsupported, the install-time half under test
// RFC: rfc/short/rfc4301.md -- Section 5.1.2.1 outer DSCP, Section 8.1 DF, Section 6.2 ICMP errors

package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// saOptionsDP is a backend that declares, as Linux XFRM does, that it copies the outer
// DSCP or writes 0 and nothing else. It records every question config verify asks.
type saOptionsDP struct {
	mockDP
	asked []dataplane.SAOptions
}

func (d *saOptionsDP) SAOptionsSupported(o dataplane.SAOptions) error {
	d.asked = append(d.asked, o)
	if v, fixed := o.OuterDSCP.Fixed(); fixed && v != 0 {
		return fmt.Errorf("test backend: %w: outer DSCP %d", dataplane.ErrSAOptionRefused, v)
	}
	return nil
}

// saOptionRefusingDP refuses every SA install with the option refusal a working backend
// answers for a configuration no verify judged (a configuration the daemon started with).
type saOptionRefusingDP struct {
	mockDP
}

func (d *saOptionRefusingDP) InstallSA(p dataplane.SAParams) error {
	return fmt.Errorf("xfrm: state add spi=%d: %w: outer DSCP 46 cannot be expressed", p.SPI, dataplane.ErrSAOptionRefused)
}

func saOptionsConfig(t *testing.T, dscp uint8) *ipsec.IPsecConfig {
	t.Helper()
	mapped, err := dataplane.FixedOuterDSCP(dscp)
	if err != nil {
		t.Fatal(err)
	}
	return &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{
		"plain":  {Name: "plain"},
		"mapped": {Name: "mapped", OuterDSCP: mapped},
	}}
}

// VALIDATES: config verify asks the loaded backend about every peer's df-bit, dscp-map
// and icmp-error-check, and refuses the commit when the backend cannot apply one,
// naming the peer; a backend that declares nothing is refused anything but defaults.
// PREVENTS: dscp-map 46 committed on Linux XFRM, then refused only at install after IKE
// negotiated the Child SA, the refusal the reviewer found kept as a degraded SA.
// RFC requirement: RFC4301-5.1.2.1-1 negative -- a peer whose outer DSCP mapping the loaded backend declares it cannot write is refused at config verify, naming the peer, with ErrSAOptionRefused; a peer with the defaults passes.
func TestVerifyPeerSAOptionsRefusesWhatTheBackendCannotApply(t *testing.T) {
	dp := &saOptionsDP{}
	err := verifyPeerSAOptions(dp, "options", saOptionsConfig(t, 46))
	if !errors.Is(err, dataplane.ErrSAOptionRefused) {
		t.Fatalf("dscp-map 46 on a backend that writes copy or 0 only: err = %v, want ErrSAOptionRefused", err)
	}
	if !strings.Contains(err.Error(), `"mapped"`) {
		t.Errorf("the refusal does not name the peer: %v", err)
	}

	dp = &saOptionsDP{}
	if err := verifyPeerSAOptions(dp, "options", saOptionsConfig(t, 0)); err != nil {
		t.Fatalf("dscp-map 0 on a backend that writes 0: %v", err)
	}
	if len(dp.asked) != 2 {
		t.Fatalf("verify asked the backend about %d peers, want 2", len(dp.asked))
	}
	for _, o := range dp.asked {
		if !o.ICMPErrorCheck {
			t.Errorf("a peer that left icmp-error-check unset was asked about without the check: %+v", o)
		}
	}

	if err := verifyPeerSAOptions(&mockDP{}, "mock", saOptionsConfig(t, 0)); err == nil {
		t.Error("a backend that declares nothing accepted a dscp-map")
	} else if !strings.Contains(err.Error(), `"mock" dataplane backend`) {
		t.Errorf("the refusal does not name the backend by its loaded name: %v", err)
	}
	defaults := &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{"plain": {Name: "plain"}}}
	if err := verifyPeerSAOptions(&mockDP{}, "mock", defaults); err != nil {
		t.Errorf("a backend that declares nothing refused a peer with the defaults: %v", err)
	}
}

// VALIDATES: an install a working backend refuses for an option the peer configured
// fails the Child SA, where a platform without XFRM (ErrNotSupported) keeps it degraded.
// PREVENTS: IKE reporting the Child SA up while the outbound ESP state the option
// refused was never installed.
// RFC requirement: RFC4301-5.1.2.1-1 negative -- a Child SA whose state install the backend refuses with ErrSAOptionRefused is refused, never kept as a degraded SA, while an ErrNotSupported refusal still is.
func TestChildSAOptionRefusalIsNotKeptDegraded(t *testing.T) {
	child, err := createFirstChildSA(wideSA("mapped", "10.0.0.2"), testESPGroup(),
		"10.0.0.1", "10.0.0.2", testChildIfID, &saOptionRefusingDP{}, slogutil.DiscardLogger())
	if !errors.Is(err, dataplane.ErrSAOptionRefused) {
		t.Fatalf("option refusal at install: err = %v, want ErrSAOptionRefused", err)
	}
	if child != nil {
		t.Error("an option refusal still produced a Child SA")
	}
	if isXFRMUnsupported(fmt.Errorf("vpp: %w", errors.Join(dataplane.ErrSAOptionRefused, dataplane.ErrNotSupported))) {
		t.Error("a chain carrying the option refusal was read as a platform without XFRM")
	}
	degraded, err := createFirstChildSA(wideSA("degraded", "10.0.0.2"), testESPGroup(),
		"10.0.0.1", "10.0.0.2", testChildIfID, &noXFRMInstallDP{}, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("an ErrNotSupported install refused the Child SA: %v", err)
	}
	if degraded == nil {
		t.Fatal("an ErrNotSupported install produced no Child SA, want it kept degraded")
	}
	if degraded.ESPInstalled {
		t.Error("a Child SA kept degraded reads ESPInstalled true")
	}
}

// noXFRMInstallDP answers every state install as a platform without XFRM, the
// ErrNotSupported refusal createFirstChildSA keeps as a degraded Child SA.
type noXFRMInstallDP struct {
	mockDP
}

func (d *noXFRMInstallDP) InstallSA(p dataplane.SAParams) error {
	return fmt.Errorf("vpp: state add spi=%d: %w", p.SPI, dataplane.ErrNotSupported)
}
