// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA volume lifetimes and SA counter sampling
// Related: rekey.go -- lifetimeState, newChildLifetimeState, volumeSoftThreshold, softExpired, hardExpiryReason
// Related: established.go -- sampleChildVolume and maintainSA 1-second volume polling
// Related: config_verify_commit.go -- verifyVolumeLifetimes commit-time backend refusal
// RFC: rfc/short/rfc4301.md -- Section 4.4.2.1 simultaneous time and volume lifetimes, out-of-sync counters

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// counterCheckingDP is a fake dataplane whose SACountersSupported returns counterErr.
type counterCheckingDP struct {
	mockDP
	counterErr error
}

func (d *counterCheckingDP) SACountersSupported() error {
	return d.counterErr
}

// volumeSampleDP is a thread-safe fake dataplane that serves configurable SAInfo
// counters through ListSAs and records how many times ListSAs was called and which
// SPIs were removed.
type volumeSampleDP struct {
	mu        sync.Mutex
	sas       []dataplane.SAInfo
	listErr   error
	installed []dataplane.SAParams
	removed   []uint32
	listCalls atomic.Int64
	lastIfID  atomic.Uint32
}

func (d *volumeSampleDP) RequiresForwardPolicy() bool { return false }

func (d *volumeSampleDP) InstallSA(p dataplane.SAParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.installed = append(d.installed, p)
	return nil
}

func (d *volumeSampleDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removed = append(d.removed, spi)
	return nil
}

func (d *volumeSampleDP) InstallPolicy(_ dataplane.SPParams) error              { return nil }
func (d *volumeSampleDP) RemovePolicy(_, _ *net.IPNet, _ dataplane.SADir) error { return nil }
func (d *volumeSampleDP) RemovePolicyParams(_ dataplane.SPParams) error         { return nil }
func (d *volumeSampleDP) ListPolicies() ([]dataplane.PolicyInfo, error)         { return nil, nil }
func (d *volumeSampleDP) Close() error                                          { return nil }
func (d *volumeSampleDP) SACountersSupported() error                            { return nil }

func (d *volumeSampleDP) ListSAs(ifID uint32) ([]dataplane.SAInfo, error) {
	d.listCalls.Add(1)
	d.lastIfID.Store(ifID)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.listErr != nil {
		return nil, d.listErr
	}
	return slices.Clone(d.sas), nil
}

func (d *volumeSampleDP) setSAs(sas []dataplane.SAInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sas = slices.Clone(sas)
}

func (d *volumeSampleDP) setListErr(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listErr = err
}

func (d *volumeSampleDP) removedSPIs() []uint32 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.removed)
}

// VALIDATES: verifyVolumeLifetimes refuses non-zero life-bytes or life-packets on a
// backend whose SACountersSupported returns ErrNotSupported (such as VPP) or on a
// backend that does not implement SACounterChecker, returning an opererr.Error naming
// the esp-group, backend, reason, and delete fix command; accepts 0 on any backend and
// accepts non-zero values on Linux XFRM (SACountersSupported == nil) or nil dp.
// PREVENTS: committing volume lifetimes on VPP where ListSAs returns ErrNotSupported
// and volume rekeys would never fire.
func TestVerifyVolumeLifetimesRefusesUnsupportedBackend(t *testing.T) {
	vppLike := &counterCheckingDP{
		counterErr: fmt.Errorf("%w: the vpp backend does not expose SA byte or packet counters", dataplane.ErrNotSupported),
	}
	xfrmLike := &counterCheckingDP{counterErr: nil}
	undeclared := &mockDP{}

	bytesCfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"esp-vol": {Name: "esp-vol", Lifetime: 3600, LifeBytes: 131072},
		},
	}
	packetsCfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"esp-pkt": {Name: "esp-pkt", Lifetime: 3600, LifePackets: 500},
		},
	}
	bothCfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"esp-both": {Name: "esp-both", Lifetime: 3600, LifeBytes: 131072, LifePackets: 500},
		},
	}
	timeOnlyCfg := &ipsec.IPsecConfig{
		ESPGroups: map[string]ipsec.ESPGroup{
			"esp-time": {Name: "esp-time", Lifetime: 3600},
		},
	}

	t.Run("vpp refuses life-bytes with opererr", func(t *testing.T) {
		err := verifyVolumeLifetimes(vppLike, "vpp", bytesCfg)
		if !errors.Is(err, dataplane.ErrNotSupported) {
			t.Fatalf("err = %v, want ErrNotSupported", err)
		}
		if _, ok := errors.AsType[*opererr.Error](err); !ok {
			t.Fatalf("err = %T, want *opererr.Error", err)
		}
		for _, want := range []string{`"esp-vol"`, `"vpp"`, "delete vpn ipsec esp-group esp-vol life-bytes"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})

	t.Run("vpp refuses life-packets with opererr", func(t *testing.T) {
		err := verifyVolumeLifetimes(vppLike, "vpp", packetsCfg)
		if !errors.Is(err, dataplane.ErrNotSupported) {
			t.Fatalf("err = %v, want ErrNotSupported", err)
		}
		for _, want := range []string{`"esp-pkt"`, `"vpp"`, "delete vpn ipsec esp-group esp-pkt life-packets"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})

	t.Run("vpp refuses both leaves with both delete commands", func(t *testing.T) {
		err := verifyVolumeLifetimes(vppLike, "vpp", bothCfg)
		if !errors.Is(err, dataplane.ErrNotSupported) {
			t.Fatalf("err = %v, want ErrNotSupported", err)
		}
		for _, want := range []string{
			`"esp-both"`,
			`"vpp"`,
			"delete vpn ipsec esp-group esp-both life-bytes",
			"delete vpn ipsec esp-group esp-both life-packets",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})

	t.Run("undeclared backend fails closed", func(t *testing.T) {
		err := verifyVolumeLifetimes(undeclared, "custom", bytesCfg)
		if err == nil {
			t.Fatal("backend not implementing SACounterChecker accepted life-bytes")
		}
		if _, ok := errors.AsType[*opererr.Error](err); !ok {
			t.Fatalf("err = %T, want *opererr.Error", err)
		}
		for _, want := range []string{`"esp-vol"`, `"custom"`, "delete vpn ipsec esp-group esp-vol life-bytes"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		}
	})

	t.Run("time-only config accepted on counterless and undeclared backends", func(t *testing.T) {
		if err := verifyVolumeLifetimes(vppLike, "vpp", timeOnlyCfg); err != nil {
			t.Errorf("vpp refused time-only esp-group: %v", err)
		}
		if err := verifyVolumeLifetimes(undeclared, "custom", timeOnlyCfg); err != nil {
			t.Errorf("undeclared backend refused time-only esp-group: %v", err)
		}
	})

	t.Run("xfrm and nil dataplane accept volume lifetimes", func(t *testing.T) {
		if err := verifyVolumeLifetimes(xfrmLike, "xfrm", bothCfg); err != nil {
			t.Errorf("xfrm refused volume lifetimes: %v", err)
		}
		if err := verifyVolumeLifetimes(nil, "", bothCfg); err != nil {
			t.Errorf("nil dataplane refused volume lifetimes: %v", err)
		}
	})
}

// VALIDATES: volumeSoftThreshold produces values strictly below hard in the [90%, 95%]
// band across the full uint64 range including math.MaxInt64+1 and math.MaxUint64 without
// signed-integer wrap or panic.
// PREVENTS: big.NewInt(int64(span)) wrapping negative for ceilings above math.MaxInt64
// and panicking in crypto/rand.Int.
func TestVolumeSoftThresholdNearUint64Max(t *testing.T) {
	ceilings := []uint64{
		100,
		65536,
		1073741824,
		uint64(math.MaxInt64) + 1,
		math.MaxUint64 - 1,
		math.MaxUint64,
	}
	for _, hard := range ceilings {
		minSoft := hard - hard/10
		maxSoft := hard - hard/20
		for range 32 {
			soft := volumeSoftThreshold(hard)
			if soft >= hard {
				t.Fatalf("volumeSoftThreshold(%d) = %d, want strictly < hard", hard, soft)
			}
			if soft < minSoft {
				t.Fatalf("volumeSoftThreshold(%d) = %d, below 90%% floor %d", hard, soft, minSoft)
			}
			if soft > maxSoft {
				t.Fatalf("volumeSoftThreshold(%d) = %d, above 95%% ceiling %d", hard, soft, maxSoft)
			}
		}
	}

	ls := newChildLifetimeState(ipsec.ESPGroup{
		Name:        "max-u64",
		Lifetime:    0,
		LifeBytes:   math.MaxUint64,
		LifePackets: math.MaxUint64,
	})
	if ls == nil {
		t.Fatal("newChildLifetimeState returned nil for uint64 max volume limits")
	}
	if ls.hardBytes != math.MaxUint64 {
		t.Errorf("hardBytes = %d, want %d", ls.hardBytes, uint64(math.MaxUint64))
	}
	if ls.softBytes >= ls.hardBytes {
		t.Errorf("softBytes = %d >= hardBytes = %d", ls.softBytes, ls.hardBytes)
	}
	if ls.hardPackets != math.MaxUint64 {
		t.Errorf("hardPackets = %d, want %d", ls.hardPackets, uint64(math.MaxUint64))
	}
	if ls.softPackets >= ls.hardPackets {
		t.Errorf("softPackets = %d >= hardPackets = %d", ls.softPackets, ls.hardPackets)
	}
}

// VALIDATES: newChildLifetimeState builds byte-only lifetimes (lifetime 0, life-bytes > 0)
// with softBytes in [90%, 95%] of hardBytes, triggers softExpired at softBytes and
// hardExpired/hardExpiryReason("bytes") at hardBytes, and returns nil only when all
// three limits are zero.
// PREVENTS: byte-only configurations being ignored when time lifetime is 0, or missing
// hard byte enforcement.
func TestLifetimeStateBytesThresholds(t *testing.T) {
	if got := newChildLifetimeState(ipsec.ESPGroup{Name: "all-zero"}); got != nil {
		t.Fatalf("newChildLifetimeState(0,0,0) = %+v, want nil", got)
	}

	const hardBytes uint64 = 131072
	ls := newChildLifetimeState(ipsec.ESPGroup{
		Name:      "bytes-only",
		Lifetime:  0,
		LifeBytes: hardBytes,
	})
	if ls == nil {
		t.Fatal("newChildLifetimeState returned nil for byte-only group")
	}
	if !ls.hasVolumeLimits() {
		t.Fatal("hasVolumeLimits() = false, want true")
	}
	if !ls.softTime.IsZero() {
		t.Errorf("softTime = %v, want zero for lifetime 0", ls.softTime)
	}
	if !ls.hardTime.IsZero() {
		t.Errorf("hardTime = %v, want zero for lifetime 0", ls.hardTime)
	}
	minSoft := hardBytes - hardBytes/10
	maxSoft := hardBytes - hardBytes/20
	if ls.softBytes < minSoft {
		t.Fatalf("softBytes = %d < %d", ls.softBytes, minSoft)
	}
	if ls.softBytes > maxSoft {
		t.Fatalf("softBytes = %d > %d", ls.softBytes, maxSoft)
	}

	now := time.Now()
	ls.recordVolume(ls.softBytes-1, 0)
	if ls.softExpired(now) {
		t.Error("softExpired true one byte below softBytes")
	}
	if ls.hardExpired(now) {
		t.Error("hardExpired true below softBytes")
	}

	ls.recordVolume(ls.softBytes, 0)
	if !ls.softExpired(now) {
		t.Error("softExpired false at softBytes")
	}
	if ls.hardExpired(now) {
		t.Error("hardExpired true between softBytes and hardBytes")
	}

	ls.recordVolume(hardBytes, 0)
	if !ls.hardExpired(now) {
		t.Error("hardExpired false at hardBytes")
	}
	if got := ls.hardExpiryReason(now); got != "bytes" {
		t.Errorf("hardExpiryReason = %q, want \"bytes\"", got)
	}
}

// VALIDATES: newChildLifetimeState builds packet-only lifetimes (lifetime 0,
// life-packets > 0) with softPackets in [90%, 95%] of hardPackets, triggering
// softExpired at softPackets and hardExpired/hardExpiryReason("packets") at hardPackets.
// PREVENTS: packet-based thresholds being left unenforced or misreported.
func TestLifetimeStatePacketsThresholds(t *testing.T) {
	const hardPackets uint64 = 1000
	ls := newChildLifetimeState(ipsec.ESPGroup{
		Name:        "packets-only",
		Lifetime:    0,
		LifePackets: hardPackets,
	})
	if ls == nil {
		t.Fatal("newChildLifetimeState returned nil for packet-only group")
	}
	if !ls.hasVolumeLimits() {
		t.Fatal("hasVolumeLimits() = false, want true")
	}
	minSoft := hardPackets - hardPackets/10
	maxSoft := hardPackets - hardPackets/20
	if ls.softPackets < minSoft {
		t.Fatalf("softPackets = %d < %d", ls.softPackets, minSoft)
	}
	if ls.softPackets > maxSoft {
		t.Fatalf("softPackets = %d > %d", ls.softPackets, maxSoft)
	}

	now := time.Now()
	ls.recordVolume(0, ls.softPackets-1)
	if ls.softExpired(now) {
		t.Error("softExpired true one packet below softPackets")
	}
	if ls.hardExpired(now) {
		t.Error("hardExpired true below softPackets")
	}

	ls.recordVolume(0, ls.softPackets)
	if !ls.softExpired(now) {
		t.Error("softExpired false at softPackets")
	}
	if ls.hardExpired(now) {
		t.Error("hardExpired true between softPackets and hardPackets")
	}

	ls.recordVolume(0, hardPackets)
	if !ls.hardExpired(now) {
		t.Error("hardExpired false at hardPackets")
	}
	if got := ls.hardExpiryReason(now); got != "packets" {
		t.Errorf("hardExpiryReason = %q, want \"packets\"", got)
	}
}

// VALIDATES: simultaneous time, byte, and packet lifetimes on a single Child SA:
// whichever enabled soft limit is reached first triggers CREATE_CHILD_SA rekey, and
// whichever enabled hard limit is reached first terminates the Child SA unconditionally.
// PREVENTS: time lifetime masking an earlier byte/packet lifetime or volume limits
// disabling time lifetime enforcement.
//
// RFC requirement: RFC4301-4.4.2.1-2 positive -- newChildLifetimeState supports
// standalone time-only (lifetime 3600, life-bytes 0, life-packets 0), standalone
// byte-only (lifetime 0, life-bytes 131072), and simultaneous time (lifetime 3600),
// byte (life-bytes 131072), and packet (life-packets 1000) lifetimes where whichever
// soft threshold is reached first fires softExpired (and drives maintainSA to send
// CREATE_CHILD_SA on byte expiry while 3600s remain) and whichever hard ceiling is
// reached first fires hardExpired with its exact reason (and drives maintainSA to
// remove the Child SA and return errTimeout on hard byte expiry).
// RFC requirement: RFC4301-4.4.2.1-2 negative -- when all three simultaneous limits are
// active and time, byte count, and packet count all sit below their soft thresholds,
// neither softExpired nor hardExpired fires.
func TestLifetimeSimultaneousFirstLimitWins(t *testing.T) {
	esp := ipsec.ESPGroup{
		Name:        "esp-simul",
		Lifetime:    3600,
		LifeBytes:   131072,
		LifePackets: 1000,
	}

	t.Run("standalone time-only lifetime works with volume disabled", func(t *testing.T) {
		ls := newChildLifetimeState(ipsec.ESPGroup{
			Name:     "esp-time-only",
			Lifetime: 3600,
		})
		if ls == nil {
			t.Fatal("newChildLifetimeState returned nil for time-only group")
		}
		if ls.hasVolumeLimits() {
			t.Error("hasVolumeLimits() = true, want false for time-only group")
		}
		if ls.softExpired(ls.softTime.Add(-time.Second)) {
			t.Error("softExpired true before softTime")
		}
		if !ls.softExpired(ls.softTime) {
			t.Error("softExpired false at softTime")
		}
		if got := ls.hardExpiryReason(ls.hardTime); got != "time" {
			t.Errorf("hardExpiryReason = %q, want \"time\"", got)
		}
	})

	t.Run("standalone byte-only lifetime works with time disabled", func(t *testing.T) {
		ls := newChildLifetimeState(ipsec.ESPGroup{
			Name:      "esp-bytes-only",
			Lifetime:  0,
			LifeBytes: 131072,
		})
		if ls == nil {
			t.Fatal("newChildLifetimeState returned nil for byte-only group")
		}
		if !ls.hasVolumeLimits() {
			t.Error("hasVolumeLimits() = false, want true for byte-only group")
		}
		if !ls.softTime.IsZero() {
			t.Errorf("softTime = %v, want zero for byte-only group", ls.softTime)
		}
		now := time.Now()
		ls.recordVolume(ls.softBytes, 0)
		if !ls.softExpired(now) {
			t.Error("softExpired false at softBytes")
		}
		ls.recordVolume(ls.hardBytes, 0)
		if got := ls.hardExpiryReason(now); got != "bytes" {
			t.Errorf("hardExpiryReason = %q, want \"bytes\"", got)
		}
	})

	t.Run("below all three thresholds fires neither soft nor hard", func(t *testing.T) {
		ls := newChildLifetimeState(esp)
		now := time.Now()
		ls.recordVolume(ls.softBytes-1, ls.softPackets-1)
		if ls.softExpired(now) {
			t.Error("softExpired true while time, bytes, and packets are all below soft thresholds")
		}
		if ls.hardExpired(now) {
			t.Error("hardExpired true while time, bytes, and packets are all below hard ceilings")
		}
	})

	t.Run("bytes expire first while time and packets remain", func(t *testing.T) {
		ls := newChildLifetimeState(esp)
		now := time.Now()
		ls.recordVolume(ls.softBytes, 1)
		if !ls.softExpired(now) {
			t.Error("softExpired false when bytes reached softBytes ahead of time and packets")
		}
		if ls.hardExpired(now) {
			t.Error("hardExpired true at softBytes")
		}
		ls.recordVolume(ls.hardBytes, 1)
		if !ls.hardExpired(now) {
			t.Error("hardExpired false when bytes reached hardBytes")
		}
		if got := ls.hardExpiryReason(now); got != "bytes" {
			t.Errorf("hardExpiryReason = %q, want \"bytes\"", got)
		}
	})

	t.Run("packets expire first while time and bytes remain", func(t *testing.T) {
		ls := newChildLifetimeState(esp)
		now := time.Now()
		ls.recordVolume(1, ls.softPackets)
		if !ls.softExpired(now) {
			t.Error("softExpired false when packets reached softPackets ahead of time and bytes")
		}
		if ls.hardExpired(now) {
			t.Error("hardExpired true at softPackets")
		}
		ls.recordVolume(1, ls.hardPackets)
		if !ls.hardExpired(now) {
			t.Error("hardExpired false when packets reached hardPackets")
		}
		if got := ls.hardExpiryReason(now); got != "packets" {
			t.Errorf("hardExpiryReason = %q, want \"packets\"", got)
		}
	})

	t.Run("time expires first while bytes and packets remain zero", func(t *testing.T) {
		ls := newChildLifetimeState(esp)
		if !ls.softExpired(ls.softTime) {
			t.Error("softExpired false at softTime while volume counters are zero")
		}
		if ls.hardExpired(ls.softTime) {
			t.Error("hardExpired true at softTime")
		}
		if !ls.hardExpired(ls.hardTime) {
			t.Error("hardExpired false at hardTime while volume counters are zero")
		}
		if got := ls.hardExpiryReason(ls.hardTime); got != "time" {
			t.Errorf("hardExpiryReason = %q, want \"time\"", got)
		}
	})

	t.Run("maintainSA triggers CREATE_CHILD_SA when byte soft limit beats time", func(t *testing.T) {
		log := slogutil.DiscardLogger()
		_, sa, ps := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t, sa)
		sa.PeerCfg.RemoteAddress = "127.0.0.1"

		dp := &volumeSampleDP{}
		group := testESPGroup()
		group.Lifetime = 3600
		group.LifeBytes = 131072
		old, err := createFirstChildSA(sa, group, "10.0.0.2", "10.0.0.1", 7, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		ps.setChildSA(old)
		ps.stopCh = make(chan struct{})
		ps.supersede = make(chan struct{}, 1)

		childLT := newChildLifetimeState(group)
		dp.setSAs([]dataplane.SAInfo{{
			SPI:          old.OutboundSPI,
			Dst:          old.RemoteAddr,
			Proto:        dataplane.ProtoESP,
			IfID:         old.IfID,
			BytesCurrent: childLT.softBytes,
		}})

		done := make(chan error, 1)
		go func() {
			done <- ps.maintainSA(sa, nil, childLT, nil, testIKEGroup(), NewSATable(), dp, myTr, nil, log)
		}()

		req := rtxRecv(t, peerTr)
		if req == nil {
			t.Fatal("volume soft expiry produced no CREATE_CHILD_SA request before time expiry")
		}
		close(ps.stopCh)
		var loopErr error
		select {
		case loopErr = <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("maintainSA did not stop")
		}
		defer ps.pendingRekey.clear()

		if loopErr != nil {
			t.Fatalf("maintainSA ended with %v before stop; soft byte expiry must rekey, not terminate", loopErr)
		}
		if hdr := parseMsg(t, req).Header; hdr.ExchangeType != wire.ExchangeCreateChildSA {
			t.Errorf("exchange = %d, want CREATE_CHILD_SA (%d)", hdr.ExchangeType, wire.ExchangeCreateChildSA)
		}
		if dp.listCalls.Load() == 0 {
			t.Error("maintainSA never polled ListSAs for a volume-limited Child SA")
		}
	})

	t.Run("maintainSA tears down Child SA unconditionally on hard byte ceiling", func(t *testing.T) {
		log := slogutil.DiscardLogger()
		_, sa, ps := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t, sa)
		sa.PeerCfg.RemoteAddress = "127.0.0.1"
		remote := sa.remoteUDPAddr()
		if remote == nil {
			t.Fatal("no resolvable peer address")
		}

		dp := &volumeSampleDP{}
		group := testESPGroup()
		group.Lifetime = 3600
		group.LifeBytes = 131072
		old, err := createFirstChildSA(sa, group, "10.0.0.2", "10.0.0.1", 7, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		ps.setChildSA(old)
		ps.stopCh = make(chan struct{})
		ps.supersede = make(chan struct{}, 1)

		// Put hardBytes already exceeded while softBytes is far above so only hard expiry fires,
		// proving hard byte expiry terminates unconditionally even when time lifetime is 3600s.
		childLT := &lifetimeState{
			softTime:  time.Now().Add(time.Hour),
			hardTime:  time.Now().Add(2 * time.Hour),
			softBytes: group.LifeBytes + 1000,
			hardBytes: group.LifeBytes,
		}
		dp.setSAs([]dataplane.SAInfo{{
			SPI:          old.InboundSPI,
			Dst:          old.LocalAddr,
			Proto:        dataplane.ProtoESP,
			IfID:         old.IfID,
			BytesCurrent: group.LifeBytes,
		}})

		done := make(chan error, 1)
		go func() {
			done <- ps.maintainSA(sa, nil, childLT, nil, testIKEGroup(), NewSATable(), dp, myTr, nil, log)
		}()

		var loopErr error
		select {
		case loopErr = <-done:
		case <-time.After(5 * time.Second):
			close(ps.stopCh)
			<-done //nolint:errcheck // Join after forced shutdown; the timeout below already fails this test.
			t.Fatal("maintainSA kept a Child SA alive past its hard byte ceiling")
		}

		if !errors.Is(loopErr, errTimeout) {
			t.Errorf("loopErr = %v, want errTimeout", loopErr)
		}
		if ps.getChildSA() != nil {
			t.Error("hard-byte-expired Child SA is still installed on the session")
		}
		if !slices.Contains(dp.removedSPIs(), old.InboundSPI) {
			t.Errorf("removed SPIs %#x missing inbound SPI %#x", dp.removedSPIs(), old.InboundSPI)
		}
		rtxExpectSilence(t, peerTr, myTr, remote, "hard-byte-expired Child SA")
	})
}

// VALIDATES: sampleChildVolume queries dp.ListSAs(child.IfID), matches only the active
// Child SA's outbound and inbound identities via IdentityOf, updates lifetimeState with
// max(out, in) per counter monotonically, handles ErrNotSupported and transient errors
// safely, and time-only maintainSA makes zero ListSAs calls.
// PREVENTS: summing bidirectional counters (halving per-key limits / risking uint64
// overflow), reading another peer's SA counters, or adding netlink dump overhead to
// time-only tunnels.
func TestSampleChildVolumeUsesMaxDirection(t *testing.T) {
	log := slogutil.DiscardLogger()
	ps := &PeerSession{peerName: "vol-peer"}
	child := &ChildSA{
		InboundSPI:  0x11111111,
		OutboundSPI: 0x22222222,
		LocalAddr:   net.ParseIP("192.0.2.10"),
		RemoteAddr:  net.ParseIP("192.0.2.20"),
		IfID:        42,
	}
	ps.setChildSA(child)

	ls := newChildLifetimeState(ipsec.ESPGroup{
		Name:        "esp-vol",
		Lifetime:    3600,
		LifeBytes:   1048576,
		LifePackets: 10000,
	})
	dp := &volumeSampleDP{}

	// Unrelated SA with huge counts must be ignored; outbound bytes > inbound bytes,
	// inbound packets > outbound packets -> max per counter independently.
	dp.setSAs([]dataplane.SAInfo{
		{
			SPI:            0x99999999,
			Dst:            net.ParseIP("198.51.100.9"),
			Proto:          dataplane.ProtoESP,
			IfID:           42,
			BytesCurrent:   999999999,
			PacketsCurrent: 999999999,
		},
		{
			SPI:            child.OutboundSPI,
			Dst:            child.RemoteAddr,
			Proto:          dataplane.ProtoESP,
			IfID:           child.IfID,
			BytesCurrent:   50000,
			PacketsCurrent: 120,
		},
		{
			SPI:            child.InboundSPI,
			Dst:            child.LocalAddr,
			Proto:          dataplane.ProtoESP,
			IfID:           child.IfID,
			BytesCurrent:   30000,
			PacketsCurrent: 250,
		},
	})
	ps.sampleChildVolume(ls, dp, log)
	if dp.lastIfID.Load() != 42 {
		t.Errorf("ListSAs ifID = %d, want 42", dp.lastIfID.Load())
	}
	if ls.byteCount != 50000 {
		t.Errorf("byteCount = %d, want 50000 (max of out=50000, in=30000)", ls.byteCount)
	}
	if ls.packetCount != 250 {
		t.Errorf("packetCount = %d, want 250 (max of out=120, in=250)", ls.packetCount)
	}

	// Inbound overtakes outbound bytes; monotonic update raises byteCount to inbound.
	dp.setSAs([]dataplane.SAInfo{
		{
			SPI:            child.OutboundSPI,
			Dst:            child.RemoteAddr,
			Proto:          dataplane.ProtoESP,
			IfID:           child.IfID,
			BytesCurrent:   50000,
			PacketsCurrent: 120,
		},
		{
			SPI:            child.InboundSPI,
			Dst:            child.LocalAddr,
			Proto:          dataplane.ProtoESP,
			IfID:           child.IfID,
			BytesCurrent:   75000,
			PacketsCurrent: 260,
		},
	})
	ps.sampleChildVolume(ls, dp, log)
	if ls.byteCount != 75000 {
		t.Errorf("byteCount = %d, want 75000", ls.byteCount)
	}
	if ls.packetCount != 260 {
		t.Errorf("packetCount = %d, want 260", ls.packetCount)
	}

	// Transient empty dump must never move observed volume backward.
	dp.setSAs(nil)
	ps.sampleChildVolume(ls, dp, log)
	if ls.byteCount != 75000 {
		t.Errorf("byteCount moved backward to %d on empty dump", ls.byteCount)
	}
	if ls.packetCount != 260 {
		t.Errorf("packetCount moved backward to %d on empty dump", ls.packetCount)
	}

	// ErrNotSupported logs once (counterReadWarned) and preserves counts without panic.
	dp.setListErr(dataplane.ErrNotSupported)
	ps.sampleChildVolume(ls, dp, log)
	if !ls.counterReadWarned {
		t.Error("counterReadWarned = false after ErrNotSupported")
	}
	ps.sampleChildVolume(ls, dp, log)
	if ls.byteCount != 75000 {
		t.Errorf("byteCount changed to %d on ErrNotSupported", ls.byteCount)
	}

	// Transient error preserves counts without panic.
	dp.setListErr(errors.New("transient netlink dump error"))
	ps.sampleChildVolume(ls, dp, log)
	if ls.byteCount != 75000 {
		t.Errorf("byteCount changed to %d on transient error", ls.byteCount)
	}

	// Time-only maintainSA executes ZERO ListSAs calls across a real 1-second tick.
	t.Run("time-only maintainSA makes zero ListSAs calls", func(t *testing.T) {
		_, sa, sess := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t, sa)
		sa.PeerCfg.RemoteAddress = "127.0.0.1"

		timeDP := &volumeSampleDP{}
		old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.2", "10.0.0.1", 1, timeDP, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		sess.setChildSA(old)
		sess.stopCh = make(chan struct{})
		sess.supersede = make(chan struct{}, 1)

		done := make(chan error, 1)
		go func() {
			done <- sess.maintainSA(sa, nil, winSoftExpired(), nil, testIKEGroup(), NewSATable(), timeDP, myTr, nil, log)
		}()

		// Receiving the soft-time CREATE_CHILD_SA proves at least one 1-second tick ran.
		if req := rtxRecv(t, peerTr); req == nil {
			t.Fatal("time-only soft-expired Child SA produced no tick request")
		}
		close(sess.stopCh)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("maintainSA did not stop")
		}
		defer sess.pendingRekey.clear()

		if calls := timeDP.listCalls.Load(); calls != 0 {
			t.Errorf("time-only maintainSA called ListSAs %d times, want 0", calls)
		}
	})
}

// VALIDATES: two ends of an SA with out-of-sync volume counters interwork cleanly:
// (1) when Peer A hits its soft byte threshold while Peer B's counter is far below,
// Peer A initiates CREATE_CHILD_SA, Peer B accepts it via respondChildRekey, installs
// the replacement SA, and resets its childLT counters to zero;
// (2) when Peer B rekeys first while Peer A is below threshold, Peer A's maintainSA
// accepts the peer-initiated rekey and resets Peer A's childLT to fresh zero counters;
// (3) when both ends reach their soft byte limits concurrently, RFC 7296 Section 2.8.1
// lowest-nonce tie-breaking resolves the collision cleanly.
// PREVENTS: stalling or rejecting a peer's earlier volume rekey when counters drift due
// to packet loss or asymmetric configurations.
//
// RFC requirement: RFC4301-4.4.2.1-3 positive -- when two ends hold disagreeing byte
// counters (one at its soft threshold and the other at zero or a lower count), the
// earlier end's CREATE_CHILD_SA rekey is accepted by the other end via respondChildRekey,
// installs matching replacement SPIs on both sides, and resets the receiving end's
// childLT counters to zero (in both initiator-first and responder-first directions).
// RFC requirement: RFC4301-4.4.2.1-3 negative -- when both ends trigger volume rekeys
// simultaneously with different nonces, the end carrying the lower nonce abandons its
// own pending rekey and accepts the peer's, while the end carrying the higher nonce
// keeps its pending rekey and ignores the peer's request (no duplicate SA stall).
func TestVolumeRekeyOutOfSyncCounters(t *testing.T) {
	log := slogutil.DiscardLogger()

	t.Run("peer A hits softBytes while peer B counter is zero", func(t *testing.T) {
		iniSA, respSA, respPS := establishPSK(t)
		espA := testESPGroup()
		espA.LifeBytes = 131072
		espB := testESPGroup()
		espB.LifeBytes = 1048576

		dpA := &volumeSampleDP{}
		dpB := &volumeSampleDP{}

		childA, err := createFirstChildSA(iniSA, espA, "10.0.0.1", "10.0.0.2", 1, dpA, log)
		if err != nil {
			t.Fatalf("createFirstChildSA(A): %v", err)
		}
		childB := respPS.getChildSA()
		if childB == nil {
			childB, err = createFirstChildSA(respSA, espB, "10.0.0.2", "10.0.0.1", 1, dpB, log)
			if err != nil {
				t.Fatalf("createFirstChildSA(B): %v", err)
			}
			respPS.setChildSA(childB)
		}
		childB.ESPGroup = espB
		respPS.espGroup = espB

		ltA := newChildLifetimeState(espA)
		ltB := newChildLifetimeState(espB)
		now := time.Now()

		// Out-of-sync counters: A reaches its soft limit; B sees only 1000 bytes.
		ltA.recordVolume(ltA.softBytes, 0)
		ltB.recordVolume(1000, 0)
		if !ltA.softExpired(now) {
			t.Fatal("peer A should be soft-expired at softBytes")
		}
		if ltB.softExpired(now) {
			t.Fatal("peer B should NOT be soft-expired at 1000 bytes")
		}

		// Peer A initiates CREATE_CHILD_SA; Peer B accepts via respondChildRekey.
		reqBytes, pendingA, err := initiateChildRekey(iniSA, childA)
		if err != nil {
			t.Fatalf("initiateChildRekey(A): %v", err)
		}
		defer pendingA.clear()

		reqMsg := parseMsg(t, reqBytes)
		reqInner, _, err := openProtected(respSA, reqMsg, reqBytes, "test", log)
		if err != nil {
			t.Fatalf("openProtected(req on B): %v", err)
		}
		respBytes, newChildB, err := respondChildRekey(respSA, reqInner, childB, reqMsg.Header.MessageID, dpB, log)
		if err != nil {
			t.Fatalf("respondChildRekey(B) refused earlier rekey from A: %v", err)
		}
		if newChildB == nil {
			t.Fatal("respondChildRekey(B) installed no replacement Child SA")
		}
		if newChildB.InboundSPI == childB.InboundSPI {
			t.Error("peer B did not rotate inbound SPI")
		}

		// Peer A applies B's response; both sides hold cross-matching replacement SPIs.
		respMsg := parseMsg(t, respBytes)
		respInner, _, err := openProtected(iniSA, respMsg, respBytes, "test", log)
		if err != nil {
			t.Fatalf("openProtected(resp on A): %v", err)
		}
		newChildA, err := applyChildRekeyResponse(iniSA, pendingA, respInner, dpA, log)
		if err != nil {
			t.Fatalf("applyChildRekeyResponse(A): %v", err)
		}
		if newChildA.InboundSPI != newChildB.OutboundSPI {
			t.Errorf("A.InboundSPI (%#x) != B.OutboundSPI (%#x)", newChildA.InboundSPI, newChildB.OutboundSPI)
		}
		if newChildA.OutboundSPI != newChildB.InboundSPI {
			t.Errorf("A.OutboundSPI (%#x) != B.InboundSPI (%#x)", newChildA.OutboundSPI, newChildB.InboundSPI)
		}
	})

	t.Run("peer B rekeys first while peer A maintainSA is below threshold and resets childLT", func(t *testing.T) {
		iniSA, respSA, respPS := establishPSK(t)
		peerTr, myTr := rtxPeerLink(t, respSA)
		respSA.PeerCfg.RemoteAddress = "127.0.0.1"

		espGroup := testESPGroup()
		espGroup.Lifetime = 3600
		espGroup.LifeBytes = 131072
		respPS.espGroup = espGroup

		dp := &volumeSampleDP{}
		oldChild, err := createFirstChildSA(respSA, espGroup, "10.0.0.2", "10.0.0.1", 5, dp, log)
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		respPS.setChildSA(oldChild)
		respPS.stopCh = make(chan struct{})
		respPS.supersede = make(chan struct{}, 1)
		respPS.inbound = make(chan transport.Packet, 4)

		// Local maintainSA starts with a lower softBytes (1000) and byteCount=999 (below
		// its threshold, out-of-sync with the peer). Once the peer's earlier rekey
		// installs out.newChild, maintainSA MUST replace childLT with a fresh
		// newChildLifetimeState(espGroup) whose softBytes is in [117964, 124518] and
		// byteCount starts at 0.
		initialLT := &lifetimeState{
			softTime:  time.Now().Add(time.Hour),
			hardTime:  time.Now().Add(2 * time.Hour),
			softBytes: 1000,
			hardBytes: 2000,
			byteCount: 999,
		}
		if initialLT.softExpired(time.Now()) {
			t.Fatal("setup: initialLT should sit just below softBytes")
		}

		// Peer initiates CREATE_CHILD_SA first; build it from iniSA against oldChild's peer view.
		peerChild := &ChildSA{
			InboundSPI:          oldChild.OutboundSPI,
			OutboundSPI:         oldChild.InboundSPI,
			LocalAddr:           oldChild.RemoteAddr,
			RemoteAddr:          oldChild.LocalAddr,
			IfID:                oldChild.IfID,
			ESPGroup:            espGroup,
			Selectors:           oldChild.Selectors,
			SelectorsLocalIsTSi: !oldChild.SelectorsLocalIsTSi,
			Mode:                oldChild.Mode,
		}
		peerReqBytes, peerPending, err := initiateChildRekey(iniSA, peerChild)
		if err != nil {
			t.Fatalf("initiateChildRekey(peer): %v", err)
		}
		defer peerPending.clear()

		done := make(chan error, 1)
		go func() {
			done <- respPS.maintainSA(respSA, nil, initialLT, nil, testIKEGroup(), NewSATable(), dp, myTr, nil, log)
		}()

		// Deliver the peer's earlier CREATE_CHILD_SA to maintainSA.
		respPS.inbound <- transport.Packet{
			Data:       peerReqBytes,
			RemoteAddr: respSA.remoteUDPAddr(),
		}

		reply := rtxRecv(t, peerTr)
		if reply == nil {
			close(respPS.stopCh)
			<-done //nolint:errcheck // Join before failing.
			t.Fatal("maintainSA did not answer peer-initiated volume rekey")
		}

		// Wait for maintainSA to finish the out.newChild step (childLT reset + incRekeyCount).
		deadline := time.Now().Add(2 * time.Second)
		rekeyed := false
		for time.Now().Before(deadline) {
			respPS.mu.Lock()
			count := respPS.rekeyCount
			respPS.mu.Unlock()
			if count > 0 {
				rekeyed = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !rekeyed {
			close(respPS.stopCh)
			<-done //nolint:errcheck // Join before failing.
			t.Fatal("maintainSA did not complete out.newChild accounting after peer-initiated rekey")
		}

		newChild := respPS.getChildSA()
		if newChild == nil {
			close(respPS.stopCh)
			<-done //nolint:errcheck // Join before failing.
			t.Fatal("maintainSA left Child SA nil after peer-initiated rekey")
		}
		if newChild.InboundSPI == oldChild.InboundSPI {
			close(respPS.stopCh)
			<-done //nolint:errcheck // Join before failing.
			t.Fatalf("maintainSA did not rotate Child SA SPI on peer-initiated rekey: %#x", newChild.InboundSPI)
		}

		// Feed 1500 bytes for newChild: above initialLT.softBytes (1000) so a missed
		// childLT reset would trigger a spurious rekey on the next tick, yet far below
		// the fresh childLT.softBytes (>= 117964).
		dp.setSAs([]dataplane.SAInfo{{
			SPI:          newChild.OutboundSPI,
			Dst:          newChild.RemoteAddr,
			Proto:        dataplane.ProtoESP,
			IfID:         newChild.IfID,
			BytesCurrent: 1500,
		}})
		callsBefore := dp.listCalls.Load()
		tickDeadline := time.Now().Add(3 * time.Second)
		polled := false
		for time.Now().Before(tickDeadline) {
			if dp.listCalls.Load() > callsBefore {
				polled = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !polled {
			close(respPS.stopCh)
			<-done //nolint:errcheck // Join before failing.
			t.Fatal("maintainSA did not poll ListSAs on the fresh volume-limited childLT")
		}

		rtxExpectSilence(t, peerTr, myTr, respSA.remoteUDPAddr(), "fresh childLT after peer-initiated rekey")

		close(respPS.stopCh)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("maintainSA returned %v on clean stop", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("maintainSA did not stop")
		}
	})

	t.Run("simultaneous volume rekey collision resolves by lowest nonce", func(t *testing.T) {
		collide := func(localFill, peerFill byte) (*SA, *PeerSession, ownedOutcome) {
			sa, old, dp, lg := peerRekeyFixture(t)
			if !sa.reserveRequestWindow() {
				t.Fatal("request window already held")
			}
			ps := &PeerSession{peerName: "vol-collide"}
			ps.setChildSA(old)
			ps.pendingRekey = &pendingRekey{
				kind:       rekeyChild,
				localNonce: bytes.Repeat([]byte{localFill}, nonceLen),
				messageID:  1,
				oldChild:   old,
			}
			inner := peerRekeyRequest(t, "10.2.0.0/24", "10.1.0.0/24")
			// Replace nonce with deterministic collision nonce and add REKEY_SA notify.
			for i := range inner {
				if n, ok := inner[i].Payload.(*wire.PayloadNonce); ok {
					n.NonceData = bytes.Repeat([]byte{peerFill}, nonceLen)
				}
			}
			inner = append([]wire.PayloadEntry{{
				Payload: &wire.PayloadNotify{
					ProtocolID:    wire.ProtocolESP,
					SPISize:       4,
					NotifyMsgType: wire.NotifyRekeySA,
					SPI:           []byte{0, 0, 0, 1},
				},
			}}, inner...)
			peerTr, myTr := rtxPeerLink(t, sa)
			_ = peerTr
			sa.PeerCfg.RemoteAddress = "127.0.0.1"
			out := ps.handleCreateChildSAOwned(sa, &wire.Message{Header: wire.Header{MessageID: 5}}, inner, false, myTr, dp, lg)
			return sa, ps, out
		}

		// Lower local nonce abandons our exchange and installs peer's replacement SA.
		saLow, psLow, outLow := collide(0x10, 0xF0)
		if psLow.pendingRekey != nil {
			t.Error("lower local nonce must abandon pending volume rekey")
		}
		if saLow.requestOutstanding {
			t.Error("abandoned exchange must release the request window")
		}
		if outLow.newChild == nil {
			t.Error("lower local nonce must accept the peer's CREATE_CHILD_SA and install out.newChild")
		}

		// Higher local nonce keeps our exchange and ignores peer's colliding request.
		saHigh, psHigh, outHigh := collide(0xF0, 0x10)
		if psHigh.pendingRekey == nil {
			t.Error("higher local nonce must keep pending volume rekey")
		}
		if !saHigh.requestOutstanding {
			t.Error("surviving exchange must keep holding the request window")
		}
		if outHigh.newChild != nil {
			t.Error("higher local nonce must not install the peer's colliding Child SA")
		}
	})
}
