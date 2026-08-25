// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias tests

package runner

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/testcond"
)

// TestPerProcessSyncWriter verifies that multiple ze-peer processes get
// independent syncWriter instances with independent WaitFor synchronization.
//
// VALIDATES: AC-1 (independent syncWriter per ze-peer), AC-2 (independent WaitFor)
// PREVENTS: Shared syncWriter race where first peer's "listening on" satisfies second peer's WaitFor.
func TestPerProcessSyncWriter(t *testing.T) {
	// Create two independent peerOutput instances (simulating what runOrchestrated does).
	po1 := peerOutput{
		stdout: newSyncWriter(),
		stderr: &lockedBuilder{},
	}
	po2 := peerOutput{
		stdout: newSyncWriter(),
		stderr: &lockedBuilder{},
	}

	// Write "listening on" to first peer only.
	_, err := po1.stdout.Write([]byte("listening on 127.0.0.1:1790\n"))
	require.NoError(t, err)

	// First peer's WaitFor should succeed immediately.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()
	assert.True(t, po1.stdout.waitFor(ctx1), "first peer should find 'listening on'")

	// Second peer's WaitFor should NOT succeed (no output written to it).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	assert.False(t, po2.stdout.waitFor(ctx2), "second peer should not find 'listening on' from first peer")

	// Now write to second peer.
	_, err = po2.stdout.Write([]byte("listening on 127.0.0.2:1790\n"))
	require.NoError(t, err)

	ctx3, cancel3 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel3()
	assert.True(t, po2.stdout.waitFor(ctx3), "second peer should find 'listening on' after its own write")

	// Verify output isolation: each peer's output is independent.
	assert.Contains(t, po1.stdout.String(), "127.0.0.1")
	assert.NotContains(t, po1.stdout.String(), "127.0.0.2")
	assert.Contains(t, po2.stdout.String(), "127.0.0.2")
	assert.NotContains(t, po2.stdout.String(), "127.0.0.1")
}

// TestPerProcessSyncWriterConcurrent verifies the fix for the original race:
// peer1's "listening on" must NOT unblock peer2's WaitFor under concurrency.
//
// VALIDATES: AC-2 (independent WaitFor under concurrent writes)
// PREVENTS: Race where concurrent write to peer1 satisfies peer2's blocking WaitFor.
func TestPerProcessSyncWriterConcurrent(t *testing.T) {
	po1 := peerOutput{stdout: newSyncWriter(), stderr: &lockedBuilder{}}
	po2 := peerOutput{stdout: newSyncWriter(), stderr: &lockedBuilder{}}

	var wg sync.WaitGroup

	// started is closed by the goroutine the instant before it enters WaitFor,
	// so the main goroutine can proceed on a real signal that peer2's blocking
	// read has begun rather than a fixed delay. This is strictly stronger than a
	// sleep: it guarantees the goroutine was scheduled (a bare sleep does not on a
	// loaded machine). The assertion below holds regardless of ordering anyway --
	// nothing is ever written to po2 -- so the remaining sub-poll gap inside
	// WaitFor is immaterial; WaitFor exposes no "now blocking" state to close it.
	started := make(chan struct{})

	// Start WaitFor on peer2 in a goroutine (will block).
	wg.Add(1)
	var po2Found bool
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		close(started)
		po2Found = po2.stdout.waitFor(ctx)
	}()

	// Write "listening on" to peer1 concurrently, once peer2's WaitFor has begun.
	<-started
	_, err := po1.stdout.Write([]byte("listening on 127.0.0.1:1790\n"))
	require.NoError(t, err)

	wg.Wait()
	// peer2's WaitFor should have timed out (peer1's write doesn't affect it).
	assert.False(t, po2Found, "peer1's write must not unblock peer2's WaitFor")
}

// TestSinglePeerUnchanged verifies that single-peer tests still work with
// the per-process output tracking (backward compatibility).
//
// VALIDATES: AC-4 (single peer unchanged behavior)
// PREVENTS: Regression where single-peer tests break due to per-process tracking changes.
func TestSinglePeerUnchanged(t *testing.T) {
	// Single peer: one peerOutput in the slice.
	po := peerOutput{
		stdout: newSyncWriter(),
		stderr: &lockedBuilder{},
	}
	outputs := []peerOutput{po}

	// Write output.
	_, err := outputs[0].stdout.Write([]byte("listening on 127.0.0.1:1790\nsuccessful\n"))
	require.NoError(t, err)
	_, err = outputs[0].stderr.WriteString("some stderr\n")
	require.NoError(t, err)

	// WaitFor works.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.True(t, outputs[0].stdout.waitFor(ctx))

	// Combined output works the same as before.
	var allStdout, allStderr strings.Builder
	for i := range outputs {
		allStdout.WriteString(outputs[i].stdout.String())
		allStderr.WriteString(outputs[i].stderr.String())
	}
	combined := allStdout.String() + allStderr.String()
	assert.Contains(t, combined, "listening on 127.0.0.1:1790")
	assert.Contains(t, combined, "some stderr")
	// Success detection still works on combined output.
	assert.Contains(t, combined, "successful")
}

// TestEnsureLoopbackAlias verifies that ensureLoopbackAlias succeeds for
// loopback addresses on all platforms.
//
// VALIDATES: AC-5 (loopback alias)
// PREVENTS: ensureLoopbackAlias failing for loopback addresses.
func TestEnsureLoopbackAlias(t *testing.T) {
	testcond.RequireOS(t, "linux")

	// 127.0.0.1 is always available on all platforms.
	err := ensureLoopbackAlias(net.ParseIP("127.0.0.1"))
	assert.NoError(t, err)

	// 127.0.0.2 -- on Linux this is a no-op (127.0.0.0/8 routes to lo).
	// On macOS/FreeBSD this requires root (SIOCAIFADDR ioctl).
	err = ensureLoopbackAlias(net.ParseIP("127.0.0.2"))
	assert.NoError(t, err) // Linux: always passes. macOS: passes if root.

	// Verify 127.0.0.2 is actually usable after the call.
	var lc net.ListenConfig
	ln, listenErr := lc.Listen(context.Background(), "tcp", "127.0.0.2:0")
	if listenErr == nil {
		require.NoError(t, ln.Close())
	}
	assert.NoError(t, listenErr, "127.0.0.2 should be bindable after ensureLoopbackAlias")

	// Idempotent: calling twice for the same IP must not error.
	err = ensureLoopbackAlias(net.ParseIP("127.0.0.2"))
	assert.NoError(t, err)
}

// absentIPv6 is an address no host carries. RFC 3849 reserves 2001:db8::/32 for
// documentation, so it is never assigned to an interface and never routed. A
// host that did carry it would redden the test below rather than hide it.
const absentIPv6 = "2001:db8::1"

// TestEnsureLoopbackAliasAcceptsPresentIPv6 verifies that an IPv6 address the
// host already carries is accepted, not rejected for not being IPv4.
//
// VALIDATES: an IPv6 --bind address is usable when it is on the interface.
// PREVENTS: the IPv4-only guard erroring on ::1, which reported every IPv6
// fixture as an alias failure before the address was ever probed.
func TestEnsureLoopbackAliasAcceptsPresentIPv6(t *testing.T) {
	// ::1 is on every host this suite runs on: the .ci fixtures bind it, so a
	// host without it cannot run them either. No skip -- a red here is real.
	require.NoError(t, ensureLoopbackAlias(net.ParseIP("::1")))
}

// TestEnsureLoopbackAliasMissingIPv6NamesTheFix verifies that an absent IPv6
// address is reported with the command that adds it, and is NOT added here.
//
// VALIDATES: the error names the address, `./le setup`, and the exact
// platform command, so an operator can act on it without reading the source.
// PREVENTS: the silent path this replaced -- a warning, then a bind failure or a
// whole-test timeout naming neither the address nor the fix.
func TestEnsureLoopbackAliasMissingIPv6NamesTheFix(t *testing.T) {
	ip := net.ParseIP(absentIPv6)

	err := ensureLoopbackAlias(ip)
	require.Error(t, err)
	assert.Contains(t, err.Error(), absentIPv6, "the operator must be told WHICH address is missing")
	assert.Contains(t, err.Error(), "./le setup", "the supported route must be named")

	var want string
	switch runtime.GOOS {
	case "linux":
		want = "sudo ip -6 addr add 2001:db8::1/128 dev lo"
	default: // darwin, freebsd
		want = "sudo ifconfig lo0 inet6 2001:db8::1/128 alias"
	}
	assert.Contains(t, err.Error(), want, "the by-hand command must be copy-pasteable")

	// Reporting is the whole contract: the runner has no privilege to add an
	// IPv6 address, so the address must still be absent afterwards.
	assert.False(t, loopbackBindable(ip), "ensureLoopbackAlias must not add an IPv6 address")
}

// TestExtractBindAddresses verifies the --bind address extraction that
// ensureBindAddresses performs before any command in a test is launched.
//
// The cases are driven through the real function, not through a copy of its
// parsing loop. Extraction is observed by its consequence: absentIPv6 is not on
// the host, so a case that extracts it MUST error and a case that does not
// extract it MUST NOT. A rewritten parser that dropped `--bind` handling would
// turn the first case green, which is what an equality check against a
// locally-recomputed list could not see.
//
// VALIDATES: correct extraction of --bind IPs from command strings, and the
// refusal to run a test whose bind address the host does not carry.
// PREVENTS: missed or incorrect bind address extraction; a fixture starting
// against an address that cannot be bound.
func TestExtractBindAddresses(t *testing.T) {
	tests := []struct {
		name    string
		exec    string
		wantErr bool // true = the address was extracted and found absent
	}{
		{"peer_with_bind", "ze-peer --bind " + absentIPv6 + " --mode sink --port 1790", true},
		{"peer_no_bind", "ze-peer --port 1790", false},
		{"non_peer_with_bind", "ze --bind " + absentIPv6, false}, // only ze-peer commands
		{"bind_truncated", "ze-peer --bind", false},              // --bind without value
		{"bind_invalid_ip", "ze-peer --bind not-an-ip --port 1790", false},
		{"bind_default_loopback", "ze-peer --bind 127.0.0.1 --port 1790", false},
		// The wildcard binds every address the host has and needs no alias. It
		// is outside 127.0.0.0/8, so the IPv4 range check would reject it, and
		// the reject now fails the test rather than logging a warning.
		{"bind_wildcard_v4", "ze-peer --bind 0.0.0.0 --port 1790", false},
		{"bind_wildcard_v6", "ze-peer --bind :: --port 1790", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureBindAddresses(&Record{RunCommands: []RunCommand{{Exec: tt.exec}}})
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), absentIPv6, "the failing address must be named")
		})
	}
}

// absentULA is a unique-local IPv6 address (RFC 4193, fc00::/7) that no host
// carries.
//
// It has to be unique-local rather than the documentation prefix absentIPv6
// uses: a config-declared local address reaches the probe only when this host is
// meant to carry it (loopbackCandidate), and a globally scoped address is
// skipped. The suite's own second IPv6 address is fd00::2
// (scripts/le/application/setup.py), so this one is absent both on a host that has run
// `./le setup` and on one that has not. A host that did carry it would redden
// the cases below rather than hide them.
const absentULA = "fd00:7e57:c0de::1"

// bgpConfigWithLocal is one peer whose two session ends differ, which is the
// shape the RFC 4271 Section 5.1.3 fixtures carry. The `asn { local ... }` leaf
// is not decoration: every fixture has one, and it is the token an address scan
// must not read.
func bgpConfigWithLocal(local string) string {
	return `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip ` + local + `
				accept false
			}
		}
		session {
			asn {
				local 65000
				remote 65001
			}
			router-id 1.2.3.4
		}
	}
}
`
}

// asnLocalTrapConfig carries both spellings of `local` in one file, and puts an
// absent address everywhere EXCEPT a connection's own local block.
//
// The AS number alone proves nothing: a scan that read `65000` as an address
// gets nil from net.ParseIP and skips it, so a correct parser and a broken one
// agree. The second peer is what discriminates. A scan that treats `local` as a
// marker and takes the next `ip` value reaches past the `asn` block into that
// peer's REMOTE address, which Ze never binds, and fails a test the host can
// run.
const asnLocalTrapConfig = `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 65000
				remote 65001
			}
		}
	}
	peer peer2 {
		connection {
			remote {
				ip ` + absentULA + `
			}
			local {
				ip ::1
			}
		}
	}
}
`

// TestConfigLocalBindAddresses verifies the `connection { local { ip } }`
// extraction that ensureBindAddresses performs before a fixture is launched.
//
// Ze binds that address: the dialer sends from it and the listener opens on it
// (internal/component/bgp/reactor/session.go, reactor_peers.go). The cases run
// through the real function and are observed by consequence, as the --bind cases
// above are: absentULA is not on this host, so a config that yields it MUST
// error and a config that does not MUST NOT.
//
// VALIDATES: a config-declared local address the host lacks fails the test at
// once, with the command that fixes it; an `asn { local ... }` leaf is never
// read as an address; an address Ze never binds is left alone.
// PREVENTS: the 45-second unexplained timeout the sixteen two-address fixtures
// hit on a host without the loopback aliases; and a hard failure for the
// config-validation fixtures that name a routable local address (192.0.2.1) and
// exit after parsing.
func TestConfigLocalBindAddresses(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool // true = the address was extracted and found absent
	}{
		{"local_absent", bgpConfigWithLocal(absentULA), true},
		{"local_present_v4", bgpConfigWithLocal("127.0.0.1"), false},
		{"local_present_v6", bgpConfigWithLocal("::1"), false},
		// `auto` asks Ze to pick the address, so there is none to check.
		{"local_auto", bgpConfigWithLocal("auto"), false},
		{"local_wildcard", bgpConfigWithLocal("0.0.0.0"), false},
		// The shape test/parse/graceful-restart-llgr.ci carries. `./le setup`
		// could not add this address, so probing it would fail a passing test
		// with an error naming a fix that does not apply.
		{"local_routable_not_probed", bgpConfigWithLocal("192.0.2.1"), false},
		{"local_one_line", "bgp { peer p { connection { local { ip " + absentULA + "; } } } }", true},
		{"local_commented_out", "bgp { peer p { connection { local {\n# ip " + absentULA + "\n} } } }", false},
		// Not a connection's local address: an interface tunnel endpoint is
		// programmed onto a link, never bound as a socket.
		{"tunnel_local_not_probed", "interface { tunnel t { encapsulation { gre { local { ip " + absentULA + "; } } } } }", false},
		{"asn_local_and_connection_local", asnLocalTrapConfig, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &Record{StdinBlocks: map[string][]byte{"ze-bgp": []byte(tt.config)}}
			err := ensureBindAddresses(rec)
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), absentULA, "the failing address must be named")
			assert.Contains(t, err.Error(), "./le setup", "the supported route must be named")
		})
	}
}

// TestConfigLocalBindAddressesFromTmpfs verifies that an embedded `.conf` is
// read for local addresses and an embedded script is not.
//
// A reload fixture keeps its second config in a tmpfs file and swaps it in
// (action=rewrite:source=config2.conf), so that file declares bind addresses
// too. A tmpfs script is not config: its braces belong to another language, and
// reading it would let an unrelated file fail a test.
//
// VALIDATES: both config sources a .ci carries are checked.
// PREVENTS: a rewrite target's local address reaching the daemon unchecked.
func TestConfigLocalBindAddressesFromTmpfs(t *testing.T) {
	conf := &Record{TmpfsFiles: map[string][]byte{"config2.conf": []byte(bgpConfigWithLocal(absentULA))}}
	require.Error(t, ensureBindAddresses(conf), "an embedded .conf declares bind addresses")

	script := &Record{TmpfsFiles: map[string][]byte{"driver.py": []byte(bgpConfigWithLocal(absentULA))}}
	assert.NoError(t, ensureBindAddresses(script), "a tmpfs script is not config")
}

// TestEnsureBindAddressesFromParsedCI drives the guard the way runOrchestrated
// does: from .ci text, through the real parser, to the Record the runner checks
// before it launches anything.
//
// The table above builds the Record by hand, so it proves the parser and not the
// plumbing that fills StdinBlocks. This one starts from the file.
//
// VALIDATES: a fixture whose config names an absent local address fails at the
// pre-flight check, with the address and the fix in the error.
// PREVENTS: a correct parser wired to a field the .ci parser never populates.
func TestEnsureBindAddressesFromParsedCI(t *testing.T) {
	tests := []struct {
		name    string
		local   string
		wantErr bool
	}{
		{"absent_local_fails_before_launch", absentULA, true},
		{"present_local_passes", "127.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetNickCounter()
			dir := t.TempDir()
			ciFile := filepath.Join(dir, "local-address.ci")
			ci := "option=timeout:value=5s\n" +
				"stdin=ze-bgp:terminator=EOF_CONF\n" +
				bgpConfigWithLocal(tt.local) +
				"EOF_CONF\n" +
				"cmd=foreground:seq=1:exec=ze -:stdin=ze-bgp:timeout=5s\n"
			require.NoError(t, os.WriteFile(ciFile, []byte(ci), 0o600))

			et := NewEncodingTests(dir)
			_, err := et.parseAndAdd(ciFile)
			require.NoError(t, err)
			rec := et.GetByNick("1")
			require.NotNil(t, rec)

			err = ensureBindAddresses(rec)
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), absentULA, "the failing address must be named")
			assert.Contains(t, err.Error(), "./le setup", "the supported route must be named")
		})
	}
}
