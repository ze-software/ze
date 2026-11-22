//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: probe_srv6_linux.go -- SRv6Encap, SRv6ForwardTTL, and the TUN classifier
//
// Deterministic unit tests for the SRv6 behavioral probe's frame parsing,
// error/policy classification, poll loop, and process-lifetime memoization.

package kernelcap

import (
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
// route add -> both absent naming CONFIG_IPV6_SEG6_LWTUNNEL; inner TTL sent-1
// -> both present; inner TTL unchanged -> encap present, forward-ttl absent
// naming CONFIG_IPV6_SEG6_FORWARD_TTL and patch 0014; unexpected inner TTL ->
// encap present, forward-ttl unknown; missing /dev/net/tun -> unknown naming
// /dev/net/tun; EACCES under a confining policy -> denied naming the policy
// and the three SRv6 sysctl paths; non-matching or truncated frames are ignored
// and a deadline expiry with no matching frame answers unknown.
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

	t.Run("route add EOPNOTSUPP means both capabilities absent", func(t *testing.T) {
		encap, ttl := classifySRv6RouteAddError(unix.EOPNOTSUPP)
		if encap.State != StateAbsent {
			t.Errorf("encap state = %v, want absent", encap.State)
		}
		if ttl.State != StateAbsent {
			t.Errorf("forward-ttl state = %v, want absent", ttl.State)
		}
		if !errors.Is(encap.Reason, unix.EOPNOTSUPP) {
			t.Errorf("encap reason %v does not wrap EOPNOTSUPP", encap.Reason)
		}
		if !strings.Contains(encap.Reason.Error(), "CONFIG_IPV6_SEG6_LWTUNNEL") {
			t.Errorf("encap reason %q does not name CONFIG_IPV6_SEG6_LWTUNNEL", encap.Reason)
		}
	})

	t.Run("route add other error means both capabilities unknown", func(t *testing.T) {
		encap, ttl := classifySRv6RouteAddError(unix.EPERM)
		if encap.State != StateUnknown {
			t.Errorf("encap state = %v, want unknown", encap.State)
		}
		if ttl.State != StateUnknown {
			t.Errorf("forward-ttl state = %v, want unknown", ttl.State)
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

				encap, ttl := askSRv6InThrowawayNamespace()
				if encap.State != StateUnknown {
					t.Errorf("%v: encap state = %v, want unknown", openErr, encap.State)
				}
				if ttl.State != StateUnknown {
					t.Errorf("%v: forward-ttl state = %v, want unknown", openErr, ttl.State)
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

				encap, ttl := askSRv6InThrowawayNamespace()
				if encap.State != tc.wantState {
					t.Fatalf("encap state = %v (%v), want %v", encap.State, encap.Reason, tc.wantState)
				}
				if ttl.State != tc.wantState {
					t.Fatalf("forward-ttl state = %v (%v), want %v", ttl.State, ttl.Reason, tc.wantState)
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

// VALIDATES: A-6. Definitive SRv6 probe verdicts (present/present,
// present/absent, absent/absent) are memoized for the process life so
// SRv6Encap and SRv6ForwardTTL share one throwaway-namespace run; non-definitive
// verdicts (unknown, denied, or partial present/unknown) are never cached so a
// later call retries the probe.
// PREVENTS: spawning a fresh namespace and TUN probe on every doctor, start,
// reload, and validate call, or caching a transient unknown/denied failure as
// permanent.
func TestSRv6ProbeMemoizesDefinitiveVerdict(t *testing.T) {
	t.Run("definitive verdicts run once across both getters", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			encap      Result
			forwardTTL Result
		}{
			{
				name:       "both present",
				encap:      Result{State: StatePresent},
				forwardTTL: Result{State: StatePresent},
			},
			{
				name:       "encap present and forward-ttl absent",
				encap:      Result{State: StatePresent},
				forwardTTL: Result{State: StateAbsent, Reason: errSRv6ForwardTTLUnchanged},
			},
			{
				name:       "both absent",
				encap:      Result{State: StateAbsent, Reason: errors.New("no seg6")},
				forwardTTL: Result{State: StateAbsent, Reason: errors.New("no seg6")},
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
				srv6ProbeRunner = func() (Result, Result) {
					runs++
					return tc.encap, tc.forwardTTL
				}

				if got := SRv6Encap(); got.State != tc.encap.State {
					t.Fatalf("first SRv6Encap = %v, want %v", got.State, tc.encap.State)
				}
				if got := SRv6ForwardTTL(); got.State != tc.forwardTTL.State {
					t.Fatalf("first SRv6ForwardTTL = %v, want %v", got.State, tc.forwardTTL.State)
				}
				if got := SRv6Encap(); got.State != tc.encap.State {
					t.Fatalf("second SRv6Encap = %v, want %v", got.State, tc.encap.State)
				}
				if got := SRv6ForwardTTL(); got.State != tc.forwardTTL.State {
					t.Fatalf("second SRv6ForwardTTL = %v, want %v", got.State, tc.forwardTTL.State)
				}
				if runs != 1 {
					t.Errorf("probe ran %d times, want 1", runs)
				}
			})
		}
	})

	t.Run("non-definitive verdicts are not memoized and recover on definitive verdict", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			encap      Result
			forwardTTL Result
		}{
			{
				name:       "both unknown",
				encap:      Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
				forwardTTL: Result{State: StateUnknown, Reason: errSRv6ProbeTimeout},
			},
			{
				name:       "both denied",
				encap:      Result{State: StateDenied, Reason: errors.New("denied")},
				forwardTTL: Result{State: StateDenied, Reason: errors.New("denied")},
			},
			{
				name:       "encap present and forward-ttl unknown",
				encap:      Result{State: StatePresent},
				forwardTTL: Result{State: StateUnknown, Reason: errors.New("unexpected ttl")},
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
				srv6ProbeRunner = func() (Result, Result) {
					runs++
					if runs <= 2 {
						return tc.encap, tc.forwardTTL
					}
					return Result{State: StatePresent}, Result{State: StatePresent}
				}

				if got := SRv6Encap(); got.State != tc.encap.State {
					t.Fatalf("call 1 SRv6Encap = %v, want %v", got.State, tc.encap.State)
				}
				if got := SRv6ForwardTTL(); got.State != tc.forwardTTL.State {
					t.Fatalf("call 2 SRv6ForwardTTL = %v, want %v", got.State, tc.forwardTTL.State)
				}
				if runs != 2 {
					t.Fatalf("after two non-definitive calls runs = %d, want 2", runs)
				}

				// Call 3 returns a definitive verdict and memoizes it.
				if got := SRv6Encap(); got.State != StatePresent {
					t.Fatalf("call 3 SRv6Encap = %v, want present", got.State)
				}
				if got := SRv6ForwardTTL(); got.State != StatePresent {
					t.Fatalf("call 4 SRv6ForwardTTL = %v, want present", got.State)
				}
				if runs != 3 {
					t.Errorf("after definitive verdict runs = %d, want 3", runs)
				}
			})
		}
	})
}
