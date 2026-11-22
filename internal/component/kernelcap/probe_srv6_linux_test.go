//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: probe_srv6_linux.go -- SRv6Encap, SRv6ForwardTTL, SRv6ForwardFrag, and the TUN classifier
//
// Deterministic unit tests for the SRv6 behavioral probe's frame parsing,
// error/policy classification, poll loop, and process-lifetime memoization.

package kernelcap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// buildTestSRv6Frame wraps an IPv4 inner packet inside an IPv6 header and
// single-segment SRH addressed to targetSID.
func buildTestSRv6Frame(targetSID [16]byte, inner []byte) []byte {
	const srhLen = 24 // 8-byte fixed SRH header + 16-byte SID
	frame := make([]byte, ipv6HeaderLen+srhLen+len(inner))
	frame[0] = 0x60
	binary.BigEndian.PutUint16(frame[4:6], uint16(srhLen+len(inner))) //nolint:gosec // small fixed test frame
	frame[6] = unix.IPPROTO_ROUTING
	frame[7] = 64
	copy(frame[8:24], srv6ProbeTUNV6Addr[:])
	copy(frame[24:40], targetSID[:])

	srh := frame[ipv6HeaderLen:]
	srh[0] = unix.IPPROTO_IPIP
	srh[1] = 2 // (2 + 1) * 8 = 24 bytes
	srh[2] = srhRoutingType
	srh[3] = 0 // segments left
	srh[4] = 0 // last entry
	copy(srh[8:24], targetSID[:])

	copy(frame[ipv6HeaderLen+srhLen:], inner)
	return frame
}

// VALIDATES: the SRv6 probe classifies every outcome honestly: EOPNOTSUPP on
// route add -> all three absent naming CONFIG_IPV6_SEG6_LWTUNNEL; inner TTL
// sent-1 -> encap and forward-ttl present; inner TTL unchanged -> encap
// present, forward-ttl absent naming CONFIG_IPV6_SEG6_FORWARD_TTL and patch
// 0014; unexpected inner TTL -> encap present, forward-ttl unknown; missing
// /dev/net/tun -> unknown naming /dev/net/tun; EACCES under a confining policy
// -> denied naming the policy and the three SRv6 sysctl paths; non-matching or
// truncated frames are ignored and a deadline expiry with no matching frame
// answers unknown.
// PREVENTS: a stock kernel read as patched, a missing /dev/net/tun or policy
// denial read as an absent kernel feature, or an unsolicited IPv6 frame
// panicking or skewing the verdict.
func TestSRv6ProbeClassification(t *testing.T) {
	t.Run("probe packet has valid IPv4 header checksum and marker", func(t *testing.T) {
		marker := []byte(srv6ProbeMarker)
		pkt := buildSRv6ProbePacket(srv6ProbeSentTTL, marker)
		if len(pkt) != ipv4MinHeaderLen+udpHeaderLen+len(marker) {
			t.Fatalf("packet length = %d, want %d", len(pkt), ipv4MinHeaderLen+udpHeaderLen+len(marker))
		}
		if pkt[8] != srv6ProbeSentTTL {
			t.Errorf("packet TTL = %d, want %d", pkt[8], srv6ProbeSentTTL)
		}
		if sum := ipv4HeaderChecksum(pkt[:ipv4MinHeaderLen]); sum != 0 {
			t.Errorf("IPv4 header checksum verification = 0x%04x, want 0", sum)
		}
	})

	t.Run("route add EOPNOTSUPP means all three capabilities absent", func(t *testing.T) {
		encap, ttl, frag := classifySRv6RouteAddError(unix.EOPNOTSUPP)
		if encap.State != StateAbsent {
			t.Errorf("encap state = %v, want absent", encap.State)
		}
		if ttl.State != StateAbsent {
			t.Errorf("forward-ttl state = %v, want absent", ttl.State)
		}
		if frag.State != StateAbsent {
			t.Errorf("forward-frag state = %v, want absent", frag.State)
		}
		if !errors.Is(encap.Reason, unix.EOPNOTSUPP) {
			t.Errorf("encap reason %v does not wrap EOPNOTSUPP", encap.Reason)
		}
		if !errors.Is(frag.Reason, unix.EOPNOTSUPP) {
			t.Errorf("forward-frag reason %v does not wrap EOPNOTSUPP", frag.Reason)
		}
		if !strings.Contains(encap.Reason.Error(), "CONFIG_IPV6_SEG6_LWTUNNEL") {
			t.Errorf("encap reason %q does not name CONFIG_IPV6_SEG6_LWTUNNEL", encap.Reason)
		}
	})

	t.Run("route add other error means all three capabilities unknown", func(t *testing.T) {
		encap, ttl, frag := classifySRv6RouteAddError(unix.EPERM)
		if encap.State != StateUnknown {
			t.Errorf("encap state = %v, want unknown", encap.State)
		}
		if ttl.State != StateUnknown {
			t.Errorf("forward-ttl state = %v, want unknown", ttl.State)
		}
		if frag.State != StateUnknown {
			t.Errorf("forward-frag state = %v, want unknown", frag.State)
		}
	})

	t.Run("inner TTL classification", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			innerTTL    byte
			wantEncap   State
			wantTTL     State
			wantReasons []string
		}{
			{
				name:      "decremented once (patched kernel)",
				innerTTL:  srv6ProbeSentTTL - 1,
				wantEncap: StatePresent,
				wantTTL:   StatePresent,
			},
			{
				name:      "unchanged (stock kernel)",
				innerTTL:  srv6ProbeSentTTL,
				wantEncap: StatePresent,
				wantTTL:   StateAbsent,
				wantReasons: []string{
					"CONFIG_IPV6_SEG6_FORWARD_TTL",
					"gokrazy/kernel/patches/0014-seg6-forward-ttl.patch",
				},
			},
			{
				name:        "unexpected inner TTL",
				innerTTL:    srv6ProbeSentTTL - 2,
				wantEncap:   StatePresent,
				wantTTL:     StateUnknown,
				wantReasons: []string{"62", "64"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				encap, ttl := classifySRv6InnerTTL(tc.innerTTL, srv6ProbeSentTTL)
				if encap.State != tc.wantEncap {
					t.Errorf("encap state = %v (%v), want %v", encap.State, encap.Reason, tc.wantEncap)
				}
				if ttl.State != tc.wantTTL {
					t.Errorf("forward-ttl state = %v (%v), want %v", ttl.State, ttl.Reason, tc.wantTTL)
				}
				if tc.wantTTL == StatePresent {
					if ttl.Reason != nil {
						t.Errorf("present verdict carries a reason: %v", ttl.Reason)
					}
					return
				}
				if ttl.Reason == nil {
					t.Fatalf("state %v carries no reason", ttl.State)
				}
				for _, want := range tc.wantReasons {
					if !strings.Contains(ttl.Reason.Error(), want) {
						t.Errorf("reason %q does not mention %q", ttl.Reason, want)
					}
				}
			})
		}
	})

	t.Run("extractSRv6InnerTTL accepts matching frame and ignores unrelated or truncated frames", func(t *testing.T) {
		marker := []byte(srv6ProbeMarker)
		goodInner := buildSRv6ProbePacket(srv6ProbeSentTTL-1, marker)
		goodFrame := buildTestSRv6Frame(srv6ProbeTargetSID, goodInner)

		ttl, ok := extractSRv6InnerTTL(goodFrame, srv6ProbeTargetSID, marker)
		if !ok {
			t.Fatal("valid SRv6 probe frame was not matched")
		}
		if ttl != srv6ProbeSentTTL-1 {
			t.Errorf("extracted TTL = %d, want %d", ttl, srv6ProbeSentTTL-1)
		}

		wrongSID := srv6ProbeTargetSID
		wrongSID[15] = 0x99

		wrongMarkerInner := buildSRv6ProbePacket(srv6ProbeSentTTL-1, []byte("other-payload-16"))
		wrongNextHdr := append([]byte(nil), goodFrame...)
		wrongNextHdr[6] = unix.IPPROTO_ICMPV6

		wrongVersion := append([]byte(nil), goodFrame...)
		wrongVersion[0] = 0x45

		wrongSRHType := append([]byte(nil), goodFrame...)
		wrongSRHType[ipv6HeaderLen+2] = 0

		wrongSRHNext := append([]byte(nil), goodFrame...)
		wrongSRHNext[ipv6HeaderLen] = unix.IPPROTO_IPV6

		oversizedSRH := append([]byte(nil), goodFrame...)
		oversizedSRH[ipv6HeaderLen+1] = 250

		badInnerIHL := append([]byte(nil), goodFrame...)
		badInnerIHL[ipv6HeaderLen+24] = 0x42 // IHL = 2 (8 bytes < 20)

		badInnerProto := append([]byte(nil), goodFrame...)
		badInnerProto[ipv6HeaderLen+24+9] = unix.IPPROTO_TCP

		for _, tc := range []struct {
			name  string
			frame []byte
		}{
			{"empty", nil},
			{"truncated IPv6 header", goodFrame[:39]},
			{"truncated SRH header", goodFrame[:ipv6HeaderLen+7]},
			{"truncated inner IPv4", goodFrame[:ipv6HeaderLen+24+19]},
			{"truncated payload marker", goodFrame[:len(goodFrame)-1]},
			{"wrong outer IP version", wrongVersion},
			{"ICMPv6 router solicitation", wrongNextHdr},
			{"wrong destination SID", buildTestSRv6Frame(wrongSID, goodInner)},
			{"wrong SRH routing type", wrongSRHType},
			{"wrong SRH next header", wrongSRHNext},
			{"SRH length past frame end", oversizedSRH},
			{"inner IHL below minimum", badInnerIHL},
			{"inner protocol not UDP", badInnerProto},
			{"wrong payload marker", buildTestSRv6Frame(srv6ProbeTargetSID, wrongMarkerInner)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, matched := extractSRv6InnerTTL(tc.frame, srv6ProbeTargetSID, marker); matched {
					t.Errorf("frame %q unexpectedly matched", tc.name)
				}
			})
		}
	})

	t.Run("poll ignores non-matching frame and classifies matching frame", func(t *testing.T) {
		var fds [2]int
		if err := unix.Pipe2(fds[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
			t.Fatalf("pipe2: %v", err)
		}
		t.Cleanup(func() {
			_ = unix.Close(fds[0]) //nolint:errcheck // test pipe cleanup
			_ = unix.Close(fds[1]) //nolint:errcheck // test pipe cleanup
		})

		marker := []byte(srv6ProbeMarker)
		noise := []byte("not-an-ipv6-srv6-packet-at-all-012345678901234567890123456789")
		if _, err := unix.Write(fds[1], noise); err != nil {
			t.Fatalf("write noise frame: %v", err)
		}
		// Short timeout with only noise must return StateUnknown.
		encap, ttl := pollSRv6ProbePacket(fds[0], srv6ProbeTargetSID, marker, srv6ProbeSentTTL, 30*time.Millisecond)
		if encap.State != StateUnknown {
			t.Errorf("timeout encap state = %v, want unknown", encap.State)
		}
		if ttl.State != StateUnknown {
			t.Errorf("timeout forward-ttl state = %v, want unknown", ttl.State)
		}
		if !errors.Is(encap.Reason, errSRv6ProbeTimeout) {
			t.Errorf("timeout reason = %v, want %v", encap.Reason, errSRv6ProbeTimeout)
		}

		// Now write an unrelated IPv6 frame followed by the real probe frame on
		// separate writes (using a SOCK_DGRAM socketpair to preserve datagram
		// boundaries just like a TUN device does).
		sp, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			t.Fatalf("socketpair: %v", err)
		}
		t.Cleanup(func() {
			_ = unix.Close(sp[0]) //nolint:errcheck // test socketpair cleanup
			_ = unix.Close(sp[1]) //nolint:errcheck // test socketpair cleanup
		})

		wrongSID := srv6ProbeTargetSID
		wrongSID[15] = 0x55
		wrongFrame := buildTestSRv6Frame(wrongSID, buildSRv6ProbePacket(srv6ProbeSentTTL, marker))
		goodFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildSRv6ProbePacket(srv6ProbeSentTTL-1, marker))
		if _, err := unix.Write(sp[1], wrongFrame); err != nil {
			t.Fatalf("write wrong frame: %v", err)
		}
		if _, err := unix.Write(sp[1], goodFrame); err != nil {
			t.Fatalf("write good frame: %v", err)
		}
		encap, ttl = pollSRv6ProbePacket(sp[0], srv6ProbeTargetSID, marker, srv6ProbeSentTTL, 200*time.Millisecond)
		if encap.State != StatePresent {
			t.Errorf("matched encap state = %v (%v), want present", encap.State, encap.Reason)
		}
		if ttl.State != StatePresent {
			t.Errorf("matched forward-ttl state = %v (%v), want present", ttl.State, ttl.Reason)
		}
	})

	t.Run("missing /dev/net/tun reports unknown naming /dev/net/tun", func(t *testing.T) {
		for _, openErr := range []error{unix.ENOENT, unix.ENODEV, fs.ErrNotExist} {
			t.Run(openErr.Error(), func(t *testing.T) {
				origRemount, origWrite, origOpen := srv6ProbeWritableSysctl, writeFile, srv6ProbeOpenTUN
				t.Cleanup(func() {
					srv6ProbeWritableSysctl, writeFile, srv6ProbeOpenTUN = origRemount, origWrite, origOpen
				})
				srv6ProbeWritableSysctl = func() error { return nil }
				writeFile = func(string, []byte, fs.FileMode) error { return nil }
				srv6ProbeOpenTUN = func() (int, error) { return -1, openErr }

				encap, ttl, frag := askSRv6InThrowawayNamespace()
				if encap.State != StateUnknown {
					t.Errorf("%v: encap state = %v, want unknown", openErr, encap.State)
				}
				if ttl.State != StateUnknown {
					t.Errorf("%v: forward-ttl state = %v, want unknown", openErr, ttl.State)
				}
				if frag.State != StateUnknown {
					t.Errorf("%v: forward-frag state = %v, want unknown", openErr, frag.State)
				}
				if encap.Reason == nil {
					t.Fatalf("%v: reason is nil, want %s named", openErr, srv6ProbeTUNDevice)
				}
				if !strings.Contains(encap.Reason.Error(), srv6ProbeTUNDevice) {
					t.Errorf("%v: reason %v does not name %s", openErr, encap.Reason, srv6ProbeTUNDevice)
				}
			})
		}
	})

	t.Run("policy denial reports denied naming the policy and SRv6 sysctl paths", func(t *testing.T) {
		const wantAppArmorPolicy = "AppArmor profile docker-default (enforce)"
		appArmor := confinement{module: securityModuleAppArmor, label: "docker-default (enforce)"}
		selinux := classifyLabel("system_u:system_r:container_t:s0:c1,c2")

		for _, tc := range []struct {
			name        string
			remountErr  error
			writeErr    error
			openErr     error
			policy      confinement
			wantState   State
			wantProfile bool
			wantPolicy  string
		}{
			{
				name:        "remount EACCES under AppArmor",
				remountErr:  unix.EACCES,
				policy:      appArmor,
				wantState:   StateDenied,
				wantProfile: true,
				wantPolicy:  wantAppArmorPolicy,
			},
			{
				name:        "sysctl write EACCES under AppArmor",
				writeErr:    unix.EACCES,
				policy:      appArmor,
				wantState:   StateDenied,
				wantProfile: true,
				wantPolicy:  wantAppArmorPolicy,
			},
			{
				name:        "tun open EACCES under AppArmor",
				openErr:     unix.EACCES,
				policy:      appArmor,
				wantState:   StateDenied,
				wantProfile: true,
				wantPolicy:  wantAppArmorPolicy,
			},
			{
				name:        "sysctl write EACCES under SELinux",
				writeErr:    unix.EACCES,
				policy:      selinux,
				wantState:   StateDenied,
				wantProfile: false,
				wantPolicy:  "SELinux context system_u:system_r:container_t:s0:c1,c2",
			},
			{
				name:      "sysctl write EACCES unconfined",
				writeErr:  unix.EACCES,
				policy:    confinement{},
				wantState: StateUnknown,
			},
			{
				name:       "remount EPERM under AppArmor",
				remountErr: unix.EPERM,
				policy:     appArmor,
				wantState:  StateUnknown,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				origRemount, origWrite, origOpen, origPolicy := srv6ProbeWritableSysctl, writeFile, srv6ProbeOpenTUN, confiningPolicy
				t.Cleanup(func() {
					srv6ProbeWritableSysctl, writeFile, srv6ProbeOpenTUN, confiningPolicy = origRemount, origWrite, origOpen, origPolicy
				})
				srv6ProbeWritableSysctl = func() error { return tc.remountErr }
				writeFile = func(string, []byte, fs.FileMode) error { return tc.writeErr }
				srv6ProbeOpenTUN = func() (int, error) { return -1, tc.openErr }
				confiningPolicy = func() confinement { return tc.policy }

				encap, ttl, frag := askSRv6InThrowawayNamespace()
				if encap.State != tc.wantState {
					t.Fatalf("encap state = %v (%v), want %v", encap.State, encap.Reason, tc.wantState)
				}
				if ttl.State != tc.wantState {
					t.Fatalf("forward-ttl state = %v (%v), want %v", ttl.State, ttl.Reason, tc.wantState)
				}
				if frag.State != tc.wantState {
					t.Fatalf("forward-frag state = %v (%v), want %v", frag.State, frag.Reason, tc.wantState)
				}
				if tc.wantState != StateDenied {
					return
				}
				reason := encap.Reason.Error()
				if !strings.Contains(reason, tc.wantPolicy) {
					t.Errorf("denied reason %q does not name %q", reason, tc.wantPolicy)
				}
				for _, path := range srv6ProbeSysctlPaths() {
					if !strings.Contains(reason, path) {
						t.Errorf("denied reason %q does not name SRv6 sysctl %q", reason, path)
					}
				}
				if strings.Contains(reason, MPLSPlatformLabelsPath()) {
					t.Errorf("SRv6 denied reason %q wrongly names MPLS sysctl %q", reason, MPLSPlatformLabelsPath())
				}
				if tc.wantProfile {
					if !strings.Contains(reason, ProbeAppArmorProfileName) {
						t.Errorf("AppArmor denied reason %q does not name %s", reason, ProbeAppArmorProfileName)
					}
				} else {
					if strings.Contains(reason, ProbeAppArmorProfileName) {
						t.Errorf("non-AppArmor denied reason %q wrongly names %s", reason, ProbeAppArmorProfileName)
					}
				}
			})
		}
	})
}

// VALIDATES: A-6. Definitive SRv6 probe verdicts (present/present/present,
// present/present/absent, present/absent/absent, absent/absent/absent) are
// memoized for the process life so SRv6Encap, SRv6ForwardTTL, and
// SRv6ForwardFrag share one throwaway-namespace run; non-definitive verdicts
// (unknown, denied, or partial present/unknown) are never cached so a later
// call retries the probe.
// PREVENTS: spawning a fresh namespace and TUN probe on every doctor, start,
// reload, and validate call, or caching a transient unknown/denied failure as
// permanent.
func TestSRv6ProbeMemoizesDefinitiveVerdict(t *testing.T) {
	t.Run("definitive verdicts run once across all three getters", func(t *testing.T) {
		noSeg6Err := errors.New("no seg6")
		for _, tc := range []struct {
			name        string
			encap       Result
			forwardTTL  Result
			forwardFrag Result
		}{
			{
				name:        "all present",
				encap:       Result{State: StatePresent},
				forwardTTL:  Result{State: StatePresent},
				forwardFrag: Result{State: StatePresent},
			},
			{
				name:        "encap and forward-ttl present, forward-frag absent",
				encap:       Result{State: StatePresent},
				forwardTTL:  Result{State: StatePresent},
				forwardFrag: Result{State: StateAbsent, Reason: errSRv6ForwardFragDropped},
			},
			{
				name:        "encap present, forward-ttl and forward-frag absent",
				encap:       Result{State: StatePresent},
				forwardTTL:  Result{State: StateAbsent, Reason: errSRv6ForwardTTLUnchanged},
				forwardFrag: Result{State: StateAbsent, Reason: errSRv6ForwardFragDropped},
			},
			{
				name:        "all absent",
				encap:       Result{State: StateAbsent, Reason: noSeg6Err},
				forwardTTL:  Result{State: StateAbsent, Reason: noSeg6Err},
				forwardFrag: Result{State: StateAbsent, Reason: noSeg6Err},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				origRunner := srv6ProbeRunner
				t.Cleanup(func() {
					srv6ProbeRunner = origRunner
					resetSRv6ProbeMemo()
				})
				resetSRv6ProbeMemo()

				runs := 0
				srv6ProbeRunner = func() (Result, Result, Result) {
					runs++
					return tc.encap, tc.forwardTTL, tc.forwardFrag
				}

				gotEncap1 := SRv6Encap()
				if gotEncap1.State != tc.encap.State {
					t.Fatalf("first SRv6Encap = %v, want %v", gotEncap1.State, tc.encap.State)
				}
				gotTTL1 := SRv6ForwardTTL()
				if gotTTL1.State != tc.forwardTTL.State {
					t.Fatalf("first SRv6ForwardTTL = %v, want %v", gotTTL1.State, tc.forwardTTL.State)
				}
				gotFrag1 := SRv6ForwardFrag()
				if gotFrag1.State != tc.forwardFrag.State {
					t.Fatalf("first SRv6ForwardFrag = %v, want %v", gotFrag1.State, tc.forwardFrag.State)
				}
				gotEncap2 := SRv6Encap()
				if gotEncap2.State != tc.encap.State {
					t.Fatalf("second SRv6Encap = %v, want %v", gotEncap2.State, tc.encap.State)
				}
				gotTTL2 := SRv6ForwardTTL()
				if gotTTL2.State != tc.forwardTTL.State {
					t.Fatalf("second SRv6ForwardTTL = %v, want %v", gotTTL2.State, tc.forwardTTL.State)
				}
				gotFrag2 := SRv6ForwardFrag()
				if gotFrag2.State != tc.forwardFrag.State {
					t.Fatalf("second SRv6ForwardFrag = %v, want %v", gotFrag2.State, tc.forwardFrag.State)
				}
				if runs != 1 {
					t.Errorf("probe ran %d times, want 1", runs)
				}
			})
		}
	})

	t.Run("non-definitive verdicts are not memoized and recover on definitive verdict", func(t *testing.T) {
		deniedErr := errors.New("denied")
		for _, tc := range []struct {
			name        string
			encap       Result
			forwardTTL  Result
			forwardFrag Result
		}{
			{
				name:        "all unknown",
				encap:       Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
				forwardTTL:  Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
				forwardFrag: Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
			},
			{
				name:        "all denied",
				encap:       Result{State: StateDenied, Reason: deniedErr},
				forwardTTL:  Result{State: StateDenied, Reason: deniedErr},
				forwardFrag: Result{State: StateDenied, Reason: deniedErr},
			},
			{
				name:        "encap present, forward-ttl unknown, forward-frag absent",
				encap:       Result{State: StatePresent},
				forwardTTL:  Result{State: StateUnknown, Reason: errors.New("unexpected ttl")},
				forwardFrag: Result{State: StateAbsent, Reason: errSRv6ForwardFragDropped},
			},
			{
				name:        "encap present, forward-ttl present, forward-frag unknown",
				encap:       Result{State: StatePresent},
				forwardTTL:  Result{State: StatePresent},
				forwardFrag: Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				origRunner := srv6ProbeRunner
				t.Cleanup(func() {
					srv6ProbeRunner = origRunner
					resetSRv6ProbeMemo()
				})
				resetSRv6ProbeMemo()

				runs := 0
				srv6ProbeRunner = func() (Result, Result, Result) {
					runs++
					if runs <= 3 {
						return tc.encap, tc.forwardTTL, tc.forwardFrag
					}
					return Result{State: StatePresent}, Result{State: StatePresent}, Result{State: StatePresent}
				}

				gotEncap1 := SRv6Encap()
				if gotEncap1.State != tc.encap.State {
					t.Fatalf("call 1 SRv6Encap = %v, want %v", gotEncap1.State, tc.encap.State)
				}
				gotTTL2 := SRv6ForwardTTL()
				if gotTTL2.State != tc.forwardTTL.State {
					t.Fatalf("call 2 SRv6ForwardTTL = %v, want %v", gotTTL2.State, tc.forwardTTL.State)
				}
				gotFrag3 := SRv6ForwardFrag()
				if gotFrag3.State != tc.forwardFrag.State {
					t.Fatalf("call 3 SRv6ForwardFrag = %v, want %v", gotFrag3.State, tc.forwardFrag.State)
				}
				if runs != 3 {
					t.Fatalf("after three non-definitive calls runs = %d, want 3", runs)
				}

				// Call 4 returns a definitive verdict and memoizes it across all getters.
				gotEncap4 := SRv6Encap()
				if gotEncap4.State != StatePresent {
					t.Fatalf("call 4 SRv6Encap = %v, want present", gotEncap4.State)
				}
				gotTTL5 := SRv6ForwardTTL()
				if gotTTL5.State != StatePresent {
					t.Fatalf("call 5 SRv6ForwardTTL = %v, want present", gotTTL5.State)
				}
				gotFrag6 := SRv6ForwardFrag()
				if gotFrag6.State != StatePresent {
					t.Fatalf("call 6 SRv6ForwardFrag = %v, want present", gotFrag6.State)
				}
				if runs != 4 {
					t.Errorf("after definitive verdict runs = %d, want 4", runs)
				}
			})
		}
	})
}

// newTestSocketpair opens a nonblocking AF_UNIX SOCK_DGRAM socketpair (matching
// TUN datagram boundaries) and closes both ends on test cleanup.
func newTestSocketpair(t *testing.T) [2]int {
	t.Helper()
	sp, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open test socketpair: %v", err)
	}
	t.Cleanup(func() {
		_ = unix.Close(sp[0]) //nolint:errcheck // test socketpair cleanup
		_ = unix.Close(sp[1]) //nolint:errcheck // test socketpair cleanup
	})
	return sp
}

func writeTestDatagram(t *testing.T, fd int, frame []byte) {
	t.Helper()
	_, err := unix.Write(fd, frame)
	if err != nil {
		t.Fatalf("write test datagram (%d bytes): %v", len(frame), err)
	}
}

// buildTestIPv4Fragment constructs one inner IPv4 fragment packet with the
// probe endpoints, UDP protocol, decremented TTL, fragID, byte offset (a
// multiple of 8), more-fragments flag, and payload, with DF clear and a valid
// IPv4 header checksum.
func buildTestIPv4Fragment(fragID uint16, offset int, more bool, payload []byte) []byte {
	totalLen := ipv4MinHeaderLen + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen)) //nolint:gosec // test fragment fits well within uint16
	binary.BigEndian.PutUint16(pkt[4:6], fragID)
	flagsFrag := uint16(offset / 8) //nolint:gosec // test fragment offset fits in 13 bits
	if more {
		flagsFrag |= ipv4FlagMF
	}
	binary.BigEndian.PutUint16(pkt[6:8], flagsFrag)
	pkt[8] = srv6ProbeSentTTL - 1
	pkt[9] = unix.IPPROTO_UDP
	copy(pkt[12:16], srv6ProbeSrcV4Addr[:])
	copy(pkt[16:20], srv6ProbeDstV4Addr[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:ipv4MinHeaderLen]))
	copy(pkt[ipv4MinHeaderLen:], payload)
	return pkt
}

// VALIDATES: the SRv6 DF-clear fragmentation probe packet builder and
// classifier: buildSRv6FragProbePacket emits a 1460-byte DF-clear IPv4 UDP
// packet with valid header checksum, fixed ID, and marker; two valid
// unfragmented outer IPv6+SRH frames carrying inner IPv4 fragments (1416B +
// 24B payload) followed by the sentinel classify StatePresent; the sentinel
// arriving with zero fragments or after an incomplete fragment classifies
// StateAbsent immediately naming CONFIG_IPV6_SEG6_FORWARD_FRAG and patch 0015;
// an incomplete fragment without sentinel times out StateUnknown; and
// truncated, unsolicited, outer-IPv6-fragment (NH=44), wrong-ID,
// wrong-payload, bad-checksum, DF-set, unfragmented, misaligned, or oversized
// outer frames are safely ignored without panicking or blocking valid
// fragments that follow.
// PREVENTS: a stock or 0014-only kernel being read as supporting DF-clear
// fragmentation, a 1-second timeout penalty on unpatched kernels, a single
// partial or corrupted fragment passing as reassembled, or malformed frames on
// the TUN fd panicking the probe.
func TestSRv6ProbeDFClearFragClassification(t *testing.T) {
	fragMarker := []byte(srv6ProbeFragMarker)
	sentinelMarker := []byte(srv6ProbeSentinelMarker)
	fragPacket := buildSRv6FragProbePacket(srv6ProbeSentTTL, srv6ProbeFragID, fragMarker)
	expectedPayload := fragPacket[ipv4MinHeaderLen:]

	frag1Inner := buildTestIPv4Fragment(srv6ProbeFragID, 0, true, expectedPayload[:1416])
	frag2Inner := buildTestIPv4Fragment(srv6ProbeFragID, 1416, false, expectedPayload[1416:])
	frag1Frame := buildTestSRv6Frame(srv6ProbeTargetSID, frag1Inner)
	frag2Frame := buildTestSRv6Frame(srv6ProbeTargetSID, frag2Inner)
	sentinelFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildSRv6ProbePacket(srv6ProbeSentTTL-1, sentinelMarker))

	t.Run("DF-clear probe packet has 1460B length, DF clear, valid checksum, ID, and marker", func(t *testing.T) {
		if len(fragPacket) != srv6ProbeFragSize {
			t.Fatalf("frag packet length = %d, want %d", len(fragPacket), srv6ProbeFragSize)
		}
		if fragPacket[0] != 0x45 {
			t.Errorf("version/IHL = 0x%02x, want 0x45", fragPacket[0])
		}
		gotTotalLen := binary.BigEndian.Uint16(fragPacket[2:4])
		if gotTotalLen != srv6ProbeFragSize {
			t.Errorf("IPv4 total length = %d, want %d", gotTotalLen, srv6ProbeFragSize)
		}
		gotID := binary.BigEndian.Uint16(fragPacket[4:6])
		if gotID != srv6ProbeFragID {
			t.Errorf("IPv4 ID = 0x%04x, want 0x%04x", gotID, srv6ProbeFragID)
		}
		flagsFrag := binary.BigEndian.Uint16(fragPacket[6:8])
		if flagsFrag&ipv4FlagDF != 0 {
			t.Errorf("flags/frag = 0x%04x, want DF clear", flagsFrag)
		}
		if fragPacket[8] != srv6ProbeSentTTL {
			t.Errorf("TTL = %d, want %d", fragPacket[8], srv6ProbeSentTTL)
		}
		if fragPacket[9] != unix.IPPROTO_UDP {
			t.Errorf("protocol = %d, want UDP (%d)", fragPacket[9], unix.IPPROTO_UDP)
		}
		sum := ipv4HeaderChecksum(fragPacket[:ipv4MinHeaderLen])
		if sum != 0 {
			t.Errorf("IPv4 header checksum verification = 0x%04x, want 0", sum)
		}
		udpPayload := fragPacket[ipv4MinHeaderLen+udpHeaderLen:]
		if !bytes.Equal(udpPayload[:len(fragMarker)], fragMarker) {
			t.Errorf("UDP payload marker = %q, want %q", udpPayload[:len(fragMarker)], fragMarker)
		}
		if len(frag1Frame) != srv6ProbeMaxOuterLen {
			t.Errorf("test fragment 1 outer frame length = %d, want %d", len(frag1Frame), srv6ProbeMaxOuterLen)
		}
	})

	t.Run("two valid inner IPv4 fragments followed by sentinel classify present", func(t *testing.T) {
		sp := newTestSocketpair(t)
		writeTestDatagram(t, sp[1], frag1Frame)
		writeTestDatagram(t, sp[1], frag2Frame)
		writeTestDatagram(t, sp[1], sentinelFrame)

		res := pollSRv6FragProbe(sp[0], srv6ProbeTargetSID, expectedPayload, sentinelMarker, 500*time.Millisecond)
		if res.State != StatePresent {
			t.Fatalf("state = %v (%v), want present", res.State, res.Reason)
		}
		if res.Reason != nil {
			t.Errorf("present verdict carries a reason: %v", res.Reason)
		}
	})

	t.Run("sentinel with zero fragments classifies absent promptly", func(t *testing.T) {
		sp := newTestSocketpair(t)
		writeTestDatagram(t, sp[1], sentinelFrame)

		start := time.Now()
		res := pollSRv6FragProbe(sp[0], srv6ProbeTargetSID, expectedPayload, sentinelMarker, srv6ProbeDeadline)
		elapsed := time.Since(start)

		if res.State != StateAbsent {
			t.Fatalf("state = %v (%v), want absent", res.State, res.Reason)
		}
		if !errors.Is(res.Reason, errSRv6ForwardFragDropped) {
			t.Errorf("reason = %v, want %v", res.Reason, errSRv6ForwardFragDropped)
		}
		reasonText := res.Reason.Error()
		if !strings.Contains(reasonText, "CONFIG_IPV6_SEG6_FORWARD_FRAG") {
			t.Errorf("reason %q does not name CONFIG_IPV6_SEG6_FORWARD_FRAG", reasonText)
		}
		if !strings.Contains(reasonText, "gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch") {
			t.Errorf("reason %q does not name patch 0015", reasonText)
		}
		if elapsed >= 250*time.Millisecond {
			t.Errorf("sentinel termination took %v, want well under %v", elapsed, srv6ProbeDeadline)
		}
	})

	t.Run("incomplete single fragment followed by sentinel classifies absent", func(t *testing.T) {
		sp := newTestSocketpair(t)
		writeTestDatagram(t, sp[1], frag1Frame)
		writeTestDatagram(t, sp[1], sentinelFrame)

		res := pollSRv6FragProbe(sp[0], srv6ProbeTargetSID, expectedPayload, sentinelMarker, srv6ProbeDeadline)
		if res.State != StateAbsent {
			t.Fatalf("state = %v (%v), want absent", res.State, res.Reason)
		}
		if !errors.Is(res.Reason, errSRv6ForwardFragDropped) {
			t.Errorf("reason = %v, want %v", res.Reason, errSRv6ForwardFragDropped)
		}
	})

	t.Run("incomplete single fragment with no sentinel times out unknown", func(t *testing.T) {
		sp := newTestSocketpair(t)
		writeTestDatagram(t, sp[1], frag1Frame)

		res := pollSRv6FragProbe(sp[0], srv6ProbeTargetSID, expectedPayload, sentinelMarker, 40*time.Millisecond)
		if res.State != StateUnknown {
			t.Fatalf("state = %v (%v), want unknown", res.State, res.Reason)
		}
		if !errors.Is(res.Reason, errSRv6ProbeTimeout) {
			t.Errorf("reason = %v, want %v", res.Reason, errSRv6ProbeTimeout)
		}
	})

	t.Run("extractSRv6InnerIPv4Fragment ignores malformed and non-matching frames", func(t *testing.T) {
		outerFrag := append([]byte(nil), frag1Frame...)
		outerFrag[6] = unix.IPPROTO_FRAGMENT

		wrongIDFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID+1, 0, true, expectedPayload[:1416]))

		badChecksum := append([]byte(nil), frag1Frame...)
		badChecksum[ipv6HeaderLen+24+10] ^= 0xff

		oversizedPayload := make([]byte, 1424)
		oversizedOuter := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID, 0, true, oversizedPayload))

		dfSetInner := append([]byte(nil), frag1Inner...)
		dfSetFlags := binary.BigEndian.Uint16(dfSetInner[6:8]) | ipv4FlagDF
		binary.BigEndian.PutUint16(dfSetInner[6:8], dfSetFlags)
		binary.BigEndian.PutUint16(dfSetInner[10:12], 0)
		binary.BigEndian.PutUint16(dfSetInner[10:12], ipv4HeaderChecksum(dfSetInner[:ipv4MinHeaderLen]))
		dfSetFrame := buildTestSRv6Frame(srv6ProbeTargetSID, dfSetInner)

		unfragmentedFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID, 0, false, expectedPayload[:64]))
		misalignedFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID, 0, true, expectedPayload[:101]))

		badIHLInner := append([]byte(nil), frag1Inner...)
		badIHLInner[0] = 0x42
		badIHLFrame := buildTestSRv6Frame(srv6ProbeTargetSID, badIHLInner)

		badTotalLenInner := append([]byte(nil), frag1Inner...)
		binary.BigEndian.PutUint16(badTotalLenInner[2:4], 2000)
		badTotalLenFrame := buildTestSRv6Frame(srv6ProbeTargetSID, badTotalLenInner)

		for _, tc := range []struct {
			name  string
			frame []byte
		}{
			{"empty", nil},
			{"truncated IPv6 header", frag1Frame[:39]},
			{"truncated SRH", frag1Frame[:ipv6HeaderLen+7]},
			{"truncated inner IPv4", frag1Frame[:ipv6HeaderLen+24+19]},
			{"unsolicited non-IPv6", []byte("not-an-ipv6-srv6-packet")},
			{"outer IPv6 fragment NH=44", outerFrag},
			{"wrong IPv4 ID", wrongIDFrame},
			{"bad inner IPv4 checksum", badChecksum},
			{"oversized outer frame", oversizedOuter},
			{"DF set on inner IPv4", dfSetFrame},
			{"unfragmented single packet", unfragmentedFrame},
			{"non-8-byte-aligned non-final fragment", misalignedFrame},
			{"bad inner IHL", badIHLFrame},
			{"inner totalLen past frame end", badTotalLenFrame},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, ok := extractSRv6InnerIPv4Fragment(tc.frame, srv6ProbeTargetSID, srv6ProbeFragID)
				if ok {
					t.Errorf("frame %q unexpectedly accepted as valid fragment", tc.name)
				}
			})
		}
	})

	t.Run("poll ignores bad frames and still classifies subsequent valid fragments present", func(t *testing.T) {
		sp := newTestSocketpair(t)

		outerFrag := append([]byte(nil), frag1Frame...)
		outerFrag[6] = unix.IPPROTO_FRAGMENT

		wrongIDFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID+1, 0, true, expectedPayload[:1416]))

		wrongPayload := append([]byte(nil), expectedPayload[:1416]...)
		wrongPayload[0] ^= 0xff
		wrongPayloadFrame := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID, 0, true, wrongPayload))

		badChecksum := append([]byte(nil), frag1Frame...)
		badChecksum[ipv6HeaderLen+24+10] ^= 0xff

		oversizedOuter := buildTestSRv6Frame(srv6ProbeTargetSID, buildTestIPv4Fragment(srv6ProbeFragID, 0, true, make([]byte, 1424)))

		// Write 7 bad frames followed by the 2 valid fragments (9 datagrams total,
		// staying within Linux's default net.unix.max_dgram_qlen of 10).
		frames := [][]byte{
			frag1Frame[:ipv6HeaderLen+24+10],
			[]byte("unsolicited-non-ipv6-frame"),
			outerFrag,
			wrongIDFrame,
			wrongPayloadFrame,
			badChecksum,
			oversizedOuter,
			frag1Frame,
			frag2Frame,
		}
		for _, frame := range frames {
			writeTestDatagram(t, sp[1], frame)
		}

		res := pollSRv6FragProbe(sp[0], srv6ProbeTargetSID, expectedPayload, sentinelMarker, 500*time.Millisecond)
		if res.State != StatePresent {
			t.Fatalf("state = %v (%v), want present after ignoring bad frames", res.State, res.Reason)
		}
	})
}
