// VALIDATES: option=needs-linux:lacks-caps= -- capabilities the daemon MUST NOT
// hold. The tokens are the caps= table's and are refused, typo or empty, on
// every host; the test skips with a reason when the process that will run ze
// holds one, and runs when it holds none; in the per-test netns mode the judge
// is the daemon's grant (netnsDaemonCapBits), not the root runner; and
// NeedsGuest never routes such a test to the QEMU guest, which runs as root.
// PREVENTS: test/plugin/bgp-listen-privileged-port-refused.ci going red for the
// host rather than for ze. As root, or with CAP_NET_BIND_SERVICE, ze binds
// 127.0.0.1:178 and starts, so the refusal the test asserts never happens.
// METHOD: drive hasCaps and netnsActive through their seams, so both
// polarities and both launch modes run on any host.

package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// A lacks-caps token is validated against the caps= table on every host, and
// an empty list or a token in both lists is refused.
func TestNeedsLinuxLacksCapsParseRefusals(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"option=needs-linux:lacks-caps=net-bnd", []string{"lacks-caps", "net-bnd", capsNetBind, capsNetAdmin}},
		{"option=needs-linux:lacks-caps=", []string{"lacks-caps= is empty", capsNetBind}},
		{"option=needs-linux:caps=net-bind:lacks-caps=net-bind", []string{"both caps= and lacks-caps="}},
	} {
		et := &EncodingTests{}
		err := et.parseLine(newRecord("lacks-caps-bad"), "test/plugin/fake.ci", tc.line)
		if err == nil {
			t.Errorf("%q was accepted on %s", tc.line, runtime.GOOS)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%q: error %q does not name %q", tc.line, err, want)
			}
		}
	}

	r := parseNeedsLinux(t, "option=needs-linux:lacks-caps=net-bind")
	if !r.NeedsLinux {
		t.Error("lacks-caps= did not mark the test Linux-only")
	}
	if !slices.Equal(r.LacksCaps, []string{capsNetBind}) {
		t.Errorf("LacksCaps = %v, want [%s]", r.LacksCaps, capsNetBind)
	}
}

// Off netns mode the runner and ze share one credential: holding the
// capability skips with a reason naming it and the remedy, lacking it runs,
// and the probe is asked about the declared token.
func TestNeedsLinuxLacksCapsHostModeBothPolarities(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("the lacks-caps gate runs after the needs-linux GOOS skip, so it is reached only on Linux")
	}
	withNetns(t, false)

	var seen []string
	withCapsRecording(t, true, &seen)
	held := parseNeedsLinux(t, "option=needs-linux:lacks-caps=net-bind")
	if held.SkipReason == "" {
		t.Fatal("lacks-caps=net-bind ran although this process holds CAP_NET_BIND_SERVICE: ze binds the port and the test goes red for the host")
	}
	if !slices.Contains(seen, capsNetBind) {
		t.Errorf("the probe was never asked about %q (seen: %v)", capsNetBind, seen)
	}
	for _, want := range []string{"lacks-caps=net-bind", "cap_net_bind_service", "not root"} {
		if !strings.Contains(held.SkipReason, want) {
			t.Errorf("skip reason %q does not name %q", held.SkipReason, want)
		}
	}
	if strings.Contains(held.SkipReason, guestRouteCommand) {
		t.Errorf("skip reason %q names the QEMU guest, which runs as root and skips the test too", held.SkipReason)
	}

	withCaps(t, false)
	if r := parseNeedsLinux(t, "option=needs-linux:lacks-caps=net-bind"); r.SkipReason != "" {
		t.Fatalf("lacks-caps=net-bind skipped although this process lacks the capability: %q", r.SkipReason)
	}
}

// In netns mode ze is dropped to ZE_TEST_UID with exactly netnsDaemonCapBits,
// so the grant decides in both directions, whatever the runner probe answers.
func TestNeedsLinuxLacksCapsInNetnsModeJudgeTheDaemonGrant(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("the lacks-caps gate runs after the needs-linux GOOS skip, so it is reached only on Linux")
	}
	withNetns(t, true)

	withCaps(t, false)
	granted := parseNeedsLinux(t, "option=needs-linux:lacks-caps=net-bind")
	if !strings.Contains(granted.SkipReason, "cap_net_bind_service") {
		t.Errorf("netns mode ran lacks-caps=net-bind although the daemon grant carries it: skip reason %q", granted.SkipReason)
	}
	if !strings.Contains(granted.SkipReason, "netnsDaemonCapBits") {
		t.Errorf("skip reason %q does not name the grant that holds the capability", granted.SkipReason)
	}

	withCaps(t, true)
	if r := parseNeedsLinux(t, "option=needs-linux:lacks-caps=sys-time"); r.SkipReason != "" {
		t.Errorf("netns mode skipped lacks-caps=sys-time, which the daemon grant does not carry, on the root runner's answer: %q", r.SkipReason)
	}
}

// daemonHeldCaps names each held capability once, nothing for a set the
// daemon lacks, and refuses an unknown token rather than answering empty.
func TestDaemonHeldCapsAgainstTheGrant(t *testing.T) {
	withNetns(t, true)
	for _, tc := range []struct {
		tokens []string
		want   []string
	}{
		{[]string{capsNetBind}, []string{"cap_net_bind_service"}},
		{[]string{capsNetBind, capsNetBind, capsSysTime}, []string{"cap_net_bind_service"}},
		{[]string{capsSysTime, capsSysAdmin}, nil},
	} {
		got, err := daemonHeldCaps(tc.tokens)
		if err != nil {
			t.Fatalf("daemonHeldCaps(%v): %v", tc.tokens, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("daemonHeldCaps(%v) = %v, want %v", tc.tokens, got, tc.want)
		}
	}
	if _, err := daemonHeldCaps([]string{"net-bnd"}); err == nil {
		t.Error("daemonHeldCaps accepted an unknown token, answering an empty held list for it")
	}
}

// Off Linux the reason names an unprivileged Linux host, never the guest; and
// a test with caps= and lacks-caps= is told to grant caps= alone.
func TestLacksCapsAdviceNamesTheUnprivilegedHost(t *testing.T) {
	advice := needsLinuxRunAdvice("test/plugin/fake.ci", nil, []string{capsNetBind})
	if strings.Contains(advice, guestRouteCommand) {
		t.Errorf("advice %q routes a lacks-caps test to the QEMU guest, which runs as root", advice)
	}
	if !strings.Contains(advice, "lacks-caps=net-bind") || !strings.Contains(advice, "not root") {
		t.Errorf("advice %q does not name the capability or the unprivileged user", advice)
	}
	both := needsLinuxRunAdvice("test/plugin/fake.ci", []string{capsNetAdmin}, []string{capsNetBind})
	if !strings.Contains(both, "caps=net-admin alone") {
		t.Errorf("advice %q does not say to grant caps= alone", both)
	}
	if plain := needsLinuxRunAdvice("", nil, nil); !strings.Contains(plain, guestRouteCommand) {
		t.Errorf("advice %q for a test without lacks-caps lost the guest route", plain)
	}
}

// NeedsGuest never sends a lacks-caps test to the guest, even where this host
// skips it: the guest holds the capability too.
func TestNeedsGuestKeepsALacksCapsTestOnTheHost(t *testing.T) {
	withNetns(t, false)
	withCaps(t, true)
	ci := filepath.Join(t.TempDir(), "lacks.ci")
	body := "option=needs-linux:lacks-caps=net-bind\ncmd=foreground:seq=1:exec=ze\nexpect=exit:code=0\n"
	if err := os.WriteFile(ci, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	guest, err := NeedsGuest(ci)
	if err != nil {
		t.Fatalf("NeedsGuest: %v", err)
	}
	if guest {
		t.Error("NeedsGuest sent a lacks-caps test to the QEMU guest, which runs as root and skips it too")
	}
}
