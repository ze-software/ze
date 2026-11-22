// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: apparmor.go -- ProbeAppArmorProfile, the profile these tests read
//
// The profile Ze ships for the kernel probe container grants exactly what the
// probe does, so the grant is read against the probe's own paths here.

package kernelcap

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: AC-20 (D-7) and AC-11 (SRv6 probe grant). The profile names
// itself, grants the three mounts remountSysctlWritable makes and the four
// sysctl writes Ze's throwaway-namespace probes make (MPLS platform_labels and
// the three SRv6 forwarding sysctls), carries neither docker-default's blanket
// mount deny nor a deny that covers those four writes, and keeps
// docker-default's read denies.
// PREVENTS: a profile that still blocks the MPLS or SRv6 probe (a deny outranks
// every allow, so a kept `deny @{PROC}/sys/[^k]** w,` would block the writes),
// and one that grants more than the probes need.
func TestProbeAppArmorProfileGrantsExactlyTheProbe(t *testing.T) {
	profile := ProbeAppArmorProfile()
	lines := strings.Split(profile, "\n")
	has := func(line string) bool {
		return slices.ContainsFunc(lines, func(l string) bool { return strings.TrimSpace(l) == line })
	}

	for _, want := range []string{
		"profile " + ProbeAppArmorProfileName + " flags=(attach_disconnected,mediate_deleted) {",
		"mount options=(rw, rprivate) -> /,",
		"mount options=(rw, bind) /proc/sys/ -> /proc/sys/,",
		"remount options=(rw, bind) /proc/sys/,",
		"deny @{PROC}/kcore rwklx,",
		"deny @{PROC}/sysrq-trigger rwklx,",
		"deny /sys/kernel/security/** rwklx,",
		"deny @{PROC}/sys/[^n]** w,",
		"deny @{PROC}/sys/net/[^im]** w,",
		"deny @{PROC}/sys/net/ipv[^46]** w,",
		"deny @{PROC}/sys/net/ipv4/[^ci]** w,",
		"deny @{PROC}/sys/net/ipv4/ip_forward{?,/}** w,",
		"deny @{PROC}/sys/net/ipv4/conf/all/send_redirects{?,/}** w,",
		"deny @{PROC}/sys/net/ipv6/conf/all/forwarding{?,/}** w,",
		"deny @{PROC}/sys/net/mpls/platform_label[^s]** w,",
		"deny @{PROC}/sys/net/mpls/platform_labels{?,/}** w,",
	} {
		if !has(want) {
			t.Errorf("the profile lacks %q:\n%s", want, profile)
		}
	}
	for _, banned := range []string{"deny mount,", "deny @{PROC}/sys/[^k]** w,", "mount,"} {
		if has(banned) {
			t.Errorf("the profile carries %q, which blocks the probe or grants every mount", banned)
		}
	}
}

// VALIDATES: the write denies leave exactly the four probe sysctl paths open:
// the profile's /proc/sys deny rules match the exact trie over the four kept
// paths (every prefix excludes only the exact valid next byte(s), every leaf
// denies any trailing byte), all four kept paths match zero deny rules, and
// every single-byte divergence (0x01..0xff) at every position, every trailing
// byte (0x01..0xff), and every unrelated /proc/sys path matches at least one
// deny rule.
// PREVENTS: a trie bug or hand-edited chain that widens a negated character
// class, skips a position, opens an unrelated sysctl, or accidentally denies
// one of the four kept paths.
func TestProbeAppArmorProfileWriteDeniesFollowThePath(t *testing.T) {
	keeps := probeSysctlKeepPaths()
	wantKeeps := []string{
		"net/ipv4/conf/all/send_redirects",
		"net/ipv4/ip_forward",
		"net/ipv6/conf/all/forwarding",
		"net/mpls/platform_labels",
	}
	if !slices.Equal(keeps, wantKeeps) {
		t.Fatalf("probeSysctlKeepPaths() = %v, want %v", keeps, wantKeeps)
	}

	rules := extractProcSysWriteDenyPatterns(t, ProbeAppArmorProfile())
	if len(rules) == 0 {
		t.Fatal("the profile carries no /proc/sys write deny rules")
	}

	// Exact structural check: at every strict prefix shared by one or more kept
	// paths, the profile must emit one deny rule whose negated class excludes
	// only the exact set of valid next bytes, and at every kept leaf it must
	// deny any longer path.
	wantNextByPrefix := make(map[string]map[byte]bool)
	for _, keep := range keeps {
		for pos := range len(keep) {
			prefix := keep[:pos]
			if wantNextByPrefix[prefix] == nil {
				wantNextByPrefix[prefix] = make(map[byte]bool)
			}
			wantNextByPrefix[prefix][keep[pos]] = true
		}
	}
	wantRules := make([]string, 0, len(wantNextByPrefix)+len(keeps))
	for prefix, nextSet := range wantNextByPrefix {
		allowed := make([]byte, 0, len(nextSet))
		for b := range nextSet {
			allowed = append(allowed, b)
		}
		slices.Sort(allowed)
		wantRules = append(wantRules, prefix+"[^"+string(allowed)+"]**")
	}
	for _, keep := range keeps {
		wantRules = append(wantRules, keep+"{?,/}**")
	}
	slices.Sort(wantRules)
	gotRules := slices.Clone(rules)
	slices.Sort(gotRules)
	if !slices.Equal(gotRules, wantRules) {
		t.Errorf("emitted /proc/sys deny rules differ from exact trie over %v:\ngot:  %v\nwant: %v", keeps, gotRules, wantRules)
	}

	// Exhaustive behavioral check over all non-NUL bytes (0x01..0xff) at every
	// position and every leaf suffix.
	for _, keep := range keeps {
		if matched, pattern := anyAppArmorRuleMatches(rules, keep); matched {
			t.Errorf("kept path %q is blocked by deny rule %q", keep, pattern)
		}
		for b := 1; b <= 0xff; b++ {
			extended := keep + string(byte(b))
			if matched, _ := anyAppArmorRuleMatches(rules, extended); !matched {
				t.Errorf("extended path %q (trailing byte 0x%02x) is not denied by any /proc/sys rule", extended, b)
			}
		}
		for pos := range len(keep) {
			prefix := keep[:pos]
			allowed := wantNextByPrefix[prefix]
			for b := 1; b <= 0xff; b++ {
				replacement := byte(b)
				if allowed[replacement] {
					continue
				}
				shortCandidate := prefix + string(replacement)
				if matched, _ := anyAppArmorRuleMatches(rules, shortCandidate); !matched {
					t.Errorf("diverged prefix %q (pos %d, byte 0x%02x) is not denied", shortCandidate, pos, b)
				}
				fullCandidate := shortCandidate + keep[pos+1:]
				if matched, _ := anyAppArmorRuleMatches(rules, fullCandidate); !matched {
					t.Errorf("mutated path %q (from %q at pos %d, byte 0x%02x) is not denied", fullCandidate, keep, pos, b)
				}
			}
		}
	}

	unrelated := []string{
		"kernel/core_pattern",
		"vm/drop_caches",
		"net/core/somaxconn",
		"net/mpls/conf/lo/input",
		"net/mpls/platform_labels2",
		"net/ipv4/ip_forward_use_pmtu",
		"net/ipv4/conf/all/forwarding",
		"net/ipv4/conf/default/send_redirects",
		"net/ipv6/conf/all/send_redirects",
		"net/ipv6/conf/default/forwarding",
		"net/ipv6/conf/all/seg6_enabled",
	}
	for _, path := range unrelated {
		if matched, _ := anyAppArmorRuleMatches(rules, path); !matched {
			t.Errorf("unrelated sysctl %q is not denied by any /proc/sys rule", path)
		}
	}
}

// VALIDATES: procSysWriteDeniesMulti agrees byte-for-byte with
// ProcSysWriteDenies on a single path, deduplicates and sorts its input, and
// denies all of /proc/sys when given no paths.
// PREVENTS: a regression in the multi-path trie generator or fail-open on an
// empty keep list.
func TestProcSysWriteDeniesMultiMatchesSinglePathAndEdgeCases(t *testing.T) {
	single := "net/mpls/platform_labels"
	if got, want := procSysWriteDeniesMulti([]string{single}), ProcSysWriteDenies(single); got != want {
		t.Errorf("single-path multi denies differ from ProcSysWriteDenies:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if got, want := procSysWriteDeniesMulti(nil), "  deny @{PROC}/sys/** w,\n"; got != want {
		t.Errorf("empty keeps = %q, want %q", got, want)
	}
	a := procSysWriteDeniesMulti([]string{"net/ipv6/conf/all/forwarding", "net/ipv4/ip_forward", "net/ipv4/ip_forward"})
	b := procSysWriteDeniesMulti([]string{"net/ipv4/ip_forward", "net/ipv6/conf/all/forwarding"})
	if a != b {
		t.Errorf("unsorted/duplicate keeps produced different rules:\n%s\nvs\n%s", a, b)
	}
}

type appArmorTokenKind uint8

const (
	appArmorTokUnspecified appArmorTokenKind = iota
	appArmorTokLiteral
	appArmorTokAnyChar
	appArmorTokAnyByte
	appArmorTokStar
	appArmorTokDoubleStar
	appArmorTokClass
	appArmorTokNegClass
)

type appArmorToken struct {
	kind  appArmorTokenKind
	ch    byte
	chars string
}

func (tok appArmorToken) matchesByte(b byte) bool {
	switch tok.kind {
	case appArmorTokLiteral:
		return b == tok.ch
	case appArmorTokAnyChar:
		return b != '/'
	case appArmorTokAnyByte:
		return true
	case appArmorTokClass:
		return strings.IndexByte(tok.chars, b) >= 0
	case appArmorTokNegClass:
		return strings.IndexByte(tok.chars, b) < 0
	case appArmorTokStar, appArmorTokDoubleStar:
		return false
	case appArmorTokUnspecified:
		panic("BUG: unspecified AppArmor glob token")
	default:
		panic("BUG: unknown AppArmor glob token")
	}
}

func extractProcSysWriteDenyPatterns(t *testing.T, profile string) []string {
	t.Helper()
	var patterns []string
	for line := range strings.SplitSeq(profile, "\n") {
		trimmed := strings.TrimSpace(line)
		body, ok := strings.CutPrefix(trimmed, "deny @{PROC}/sys/")
		if !ok {
			continue
		}
		pattern, ok := strings.CutSuffix(body, " w,")
		if !ok {
			t.Fatalf("unexpected /proc/sys deny line %q", trimmed)
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

func anyAppArmorRuleMatches(patterns []string, path string) (bool, string) {
	for _, pattern := range patterns {
		if appArmorGlobMatches(pattern, path) {
			return true, pattern
		}
	}
	return false, ""
}

// appArmorGlobMatches evaluates an AppArmor path glob pattern (supporting
// literals, ?, {?,/}, *, **, [chars], and [^chars]) against path.
func appArmorGlobMatches(pattern, path string) bool {
	tokens := parseAppArmorGlob(pattern)
	dp := make([]bool, len(path)+1)
	dp[0] = true
	for _, tok := range tokens {
		next := make([]bool, len(path)+1)
		switch tok.kind {
		case appArmorTokDoubleStar:
			seen := false
			for j := range len(path) + 1 {
				if dp[j] {
					seen = true
				}
				if seen {
					next[j] = true
				}
			}
		case appArmorTokStar:
			for j := range len(path) + 1 {
				if !dp[j] {
					continue
				}
				next[j] = true
				for k := j; k < len(path); k++ {
					if path[k] == '/' {
						break
					}
					next[k+1] = true
				}
			}
		case appArmorTokLiteral, appArmorTokAnyChar, appArmorTokAnyByte, appArmorTokClass, appArmorTokNegClass:
			for j := range len(path) {
				if !dp[j] {
					continue
				}
				if tok.matchesByte(path[j]) {
					next[j+1] = true
				}
			}
		case appArmorTokUnspecified:
			panic("BUG: unspecified AppArmor glob token")
		}
		dp = next
	}
	return dp[len(path)]
}

func parseAppArmorGlob(pattern string) []appArmorToken {
	var tokens []appArmorToken
	for i := 0; i < len(pattern); {
		if strings.HasPrefix(pattern[i:], "**") {
			tokens = append(tokens, appArmorToken{kind: appArmorTokDoubleStar})
			i += 2
			continue
		}
		if strings.HasPrefix(pattern[i:], "{?,/}") {
			tokens = append(tokens, appArmorToken{kind: appArmorTokAnyByte})
			i += len("{?,/}")
			continue
		}
		if pattern[i] == '*' {
			tokens = append(tokens, appArmorToken{kind: appArmorTokStar})
			i++
			continue
		}
		if pattern[i] == '?' {
			tokens = append(tokens, appArmorToken{kind: appArmorTokAnyChar})
			i++
			continue
		}
		if pattern[i] == '[' {
			end := strings.IndexByte(pattern[i:], ']')
			if end < 0 {
				panic("BUG: unclosed character class in AppArmor pattern " + pattern)
			}
			body := pattern[i+1 : i+end]
			if strings.HasPrefix(body, "^") {
				tokens = append(tokens, appArmorToken{kind: appArmorTokNegClass, chars: body[1:]})
			} else {
				tokens = append(tokens, appArmorToken{kind: appArmorTokClass, chars: body})
			}
			i += end + 1
			continue
		}
		tokens = append(tokens, appArmorToken{kind: appArmorTokLiteral, ch: pattern[i]})
		i++
	}
	return tokens
}

// VALIDATES: D-7. The /proc/sys write chain takes a character class as one
// position and a trailing slash as a directory grant: every name that differs
// from the kept prefix at some position is denied, and nothing under the kept
// directory is.
// PREVENTS: a lab profile that grants more of /proc/sys than its keep names,
// or one whose class position denies the very write it keeps.
func TestProcSysWriteDeniesKeepAClassAndADirectory(t *testing.T) {
	want := strings.Join([]string{
		"  deny @{PROC}/sys/[^n]** w,",
		"  deny @{PROC}/sys/n[^e]** w,",
		"  deny @{PROC}/sys/ne[^t]** w,",
		"  deny @{PROC}/sys/net[^/]** w,",
		"  deny @{PROC}/sys/net/[^i]** w,",
		"  deny @{PROC}/sys/net/i[^p]** w,",
		"  deny @{PROC}/sys/net/ip[^v]** w,",
		"  deny @{PROC}/sys/net/ipv[^46]** w,",
		"  deny @{PROC}/sys/net/ipv[46][^/]** w,",
		"  deny @{PROC}/sys/net/ipv[46]/[^c]** w,",
		"  deny @{PROC}/sys/net/ipv[46]/c[^o]** w,",
		"  deny @{PROC}/sys/net/ipv[46]/co[^n]** w,",
		"  deny @{PROC}/sys/net/ipv[46]/con[^f]** w,",
		"  deny @{PROC}/sys/net/ipv[46]/conf[^/]** w,",
	}, "\n") + "\n"
	if got := ProcSysWriteDenies("net/ipv[46]/conf/"); got != want {
		t.Errorf("chain:\n%s\nwant:\n%s", got, want)
	}
}

// VALIDATES: D-7. A profile derived from docker-default carries its name in
// the header, the signal and the ptrace peers, the /proc/sys rules and the
// mount rules it was given, and docker-default's other denies.
// PREVENTS: a lab profile that silently drops one of docker-default's denies.
func TestDockerDefaultProfileKeepsTheRest(t *testing.T) {
	profile := DockerDefaultProfile("ze-lab", "  deny @{PROC}/sys/** w,\n", "  deny mount,\n")
	for _, want := range []string{
		"profile ze-lab flags=(attach_disconnected,mediate_deleted) {",
		"signal (send,receive) peer=ze-lab,",
		"ptrace (trace,tracedby,read,readby) peer=ze-lab,",
		"  deny @{PROC}/sys/** w,\n",
		"  deny mount,\n",
		"deny @{PROC}/sysrq-trigger rwklx,",
		"deny /sys/kernel/security/** rwklx,",
	} {
		if !strings.Contains(profile, want) {
			t.Errorf("the profile lacks %q:\n%s", want, profile)
		}
	}
	if strings.Contains(profile, ProbeAppArmorProfileName) {
		t.Errorf("the profile names the probe's profile:\n%s", profile)
	}
}
