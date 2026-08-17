package runner

// VALIDATES: no two link-creating .ci tests configure a tunnel on the same
//   endpoint pair. The suites share one VM, and the kernel refuses a second
//   tunnel of the same driver on a local/remote pair that is already taken,
//   whatever the new device is named.
// PREVENTS: the failure of 2026-08-11, where test/plugin/iface-tunnel-kinds.ci
//   took the endpoint pairs of test/reload/test-tx-iface-tunnel-create.ci, ran
//   first, and left its links behind. Three reload tests then failed on
//   `file exists` for device names they had never created, and the failure was
//   first diagnosed as a product defect in the reload path.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// tunnelKindBlocks are the encapsulation case names a tunnel block can carry
// (internal/component/iface/tunnel.go, tunnelKindNames).
var tunnelKindBlocks = map[string]bool{
	"gre":       true,
	"gretap":    true,
	"ip6gre":    true,
	"ip6gretap": true,
	"ipip":      true,
	"sit":       true,
	"ip6tnl":    true,
	"ipip6":     true,
	"vxlan":     true,
}

// tunnelUniquenessDomain names the kernel driver that owns a kind's endpoint
// table. Two tunnels collide only inside one domain, so the check groups by it
// rather than by the ze kind: ip6tnl and ipip6 are both the ip6_tunnel driver
// and share one endpoint table, while gre and gretap are separate device types
// and do not collide with each other.
func tunnelUniquenessDomain(kind string) string {
	switch kind {
	case "ip6tnl", "ipip6":
		return "ip6_tunnel"
	default:
		return kind
	}
}

// tunnelEndpointClaim is one tunnel stanza's claim on the kernel's endpoint
// table: the driver, the local endpoint, the remote endpoint, and the GRE key.
//
// local is empty for a stanza that names no local address. That is a legal and
// common shape: only remote/ip is `mandatory true` in
// internal/component/iface/yang/ze-iface-conf.yang, `local { interface eth0; }`
// is the other way to give a source, and a stanza can give neither. The kernel
// keys such a tunnel on the wildcard source, so two of them sharing a remote
// and a key collide exactly as two with equal local addresses do. Claims with
// no local address were DROPPED until 2026-08-11, which made this check blind
// to every interface-sourced tunnel in the corpus.
//
// The key is part of the claim because the ip_tunnel and ip6_gre drivers
// compare it alongside the addresses. The evidence is this repository's own
// corpus rather than a kernel read: test-tx-iface-tunnel-create.ci (key 42),
// test-tx-iface-tunnel-modify-key.ci (keys 1 and 2) and
// test-tx-iface-tunnel-remove.ci (no key) all create a gre tunnel on
// 192.0.2.1 -> 198.51.100.1 in the same VM and all three pass, while
// iface-tunnel-kinds.ci taking that pair with no key collided with the third.
// Kinds that carry no key leaf simply claim the empty one.
type tunnelEndpointClaim struct {
	domain string
	local  string
	remote string
	key    string
	name   string // the ze interface name, for the failure message
}

// tunnelEndpointClaims returns every tunnel endpoint claim a .ci file makes.
// It reads the raw file rather than the parsed Record because the config sits
// in a stdin= or tmpfs= section either way, and the block structure is what
// this needs.
func tunnelEndpointClaims(raw string) []tunnelEndpointClaim {
	var claims []tunnelEndpointClaim
	for _, block := range namedBlocks(raw, "tunnel") {
		encap := firstBlock(block.body, "encapsulation")
		if encap == "" {
			continue
		}
		for kind := range tunnelKindBlocks {
			body := firstBlock(encap, kind)
			if body == "" {
				continue
			}
			// remote/ip is `mandatory true`, so its absence means the scanner
			// failed to read the stanza rather than that the config omitted
			// it. The blindness guard in the test below turns that into a
			// failure. local/ip is optional and its absence is recorded.
			remote := leafInBlock(body, "remote", "ip")
			if remote == "" {
				continue
			}
			claims = append(claims, tunnelEndpointClaim{
				domain: tunnelUniquenessDomain(kind),
				local:  leafInBlock(body, "local", "ip"),
				remote: remote,
				key:    leaf(body, "key"),
				name:   block.arg,
			})
		}
	}
	return claims
}

// duplicateTunnelEndpoints reports every claim made by more than one INTERFACE,
// one line per collision, sorted. files maps a display path to file content.
//
// A holder is one interface name in one file, counted once however many stanzas
// name it. One file naming one interface twice on one claim is the reload and
// restart shape: test/reload/test-tx-iface-tunnel-modify-key.ci carries the same
// tunnel in its boot config and in the config it reloads, and a restart test
// runs one config twice. The kernel holds ONE device for it, so there is nothing
// to collide with. Two DIFFERENT names on one claim inside one file is a real
// collision, and it stays reported: the second create gets EEXIST in the same
// run, not merely in a later test.
func duplicateTunnelEndpoints(files map[string]string) []string {
	holders := map[tunnelEndpointClaim]map[string]bool{}
	for path, raw := range files {
		for _, c := range tunnelEndpointClaims(raw) {
			holder := c
			holder.name = ""
			if holders[holder] == nil {
				holders[holder] = map[string]bool{}
			}
			holders[holder][path+" ("+c.name+")"] = true
		}
	}

	var out []string
	for claim, whoSet := range holders {
		if len(whoSet) < 2 {
			continue
		}
		who := make([]string, 0, len(whoSet))
		for label := range whoSet {
			who = append(who, label)
		}
		sort.Strings(who)
		key := claim.key
		if key == "" {
			key = "none"
		}
		out = append(out, fmt.Sprintf("%s %s -> %s key %s claimed by %s",
			claim.domain, claim.local, claim.remote, key, strings.Join(who, ", ")))
	}
	sort.Strings(out)
	return out
}

// TestCITunnelEndpointsAreUniqueAcrossTests walks the .ci corpus and fails when
// two link-creating tests claim one endpoint pair.
//
// Only tests declaring caps=net-admin take part: creating a link needs that
// capability, so a test without it never reaches the kernel's endpoint table.
// That is what keeps the parse-only tests in test/parse, which all validate a
// config on 192.0.2.1 -> 198.51.100.1 and create nothing, out of the check.
func TestCITunnelEndpointsAreUniqueAcrossTests(t *testing.T) {
	root := repoRootForTest(t)
	testDir := filepath.Join(root, "test")

	files := map[string]string{}
	walkErr := filepath.WalkDir(testDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// test/draft/ is invisible to every gate (test/draft/README.md).
		if d.IsDir() && isDraftPath(testDir, p) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".ci") {
			return nil
		}
		raw, readErr := os.ReadFile(p) //nolint:gosec // repo test tree
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(raw), "caps=net-admin") {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		files[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", testDir, walkErr)
	}
	if len(files) == 0 {
		t.Fatal("no link-creating .ci found: the walk or the caps= marker changed, and this check would pass vacuously")
	}

	// A file that configures an encapsulation and yields no claim means the
	// scanner has gone blind, and a blind scanner reports no collision. Checked
	// against the corpus rather than a fixture, because the config dialect is
	// what moves under it.
	for path, raw := range files {
		if !strings.Contains(raw, "encapsulation") {
			continue
		}
		if len(tunnelEndpointClaims(raw)) == 0 {
			t.Errorf("%s configures an encapsulation and this check read no endpoint from it: "+
				"the scanner no longer matches the config dialect", path)
		}
	}

	for _, dup := range duplicateTunnelEndpoints(files) {
		t.Errorf("tunnel endpoint pair shared by two interfaces: %s\n"+
			"the suites share one VM and a link outlives the test that made it, so the second "+
			"interface gets `file exists` for a device it never created; give the new test its own "+
			"block of addresses", dup)
	}
}

// TestCITunnelEndpointLintCatchesTheCollisionItWasWrittenFor proves the check
// reports the 2026-08-11 collision, and stays quiet on the three reload tests
// that legitimately share a pair under different keys.
func TestCITunnelEndpointLintCatchesTheCollisionItWasWrittenFor(t *testing.T) {
	const reloadCreate = `option=needs-linux:caps=net-admin
tmpfs=config2.conf:terminator=EOF
interface {
	tunnel tgre {
		encapsulation {
			gre {
				local { ip 192.0.2.1; }
				remote { ip 198.51.100.1; }
				key 42
			}
		}
	}
}
EOF
`
	const reloadRemove = `option=needs-linux:caps=net-admin
stdin=conf:terminator=EOF
interface {
	tunnel tgrerm0 {
		encapsulation {
			gre {
				local { ip 192.0.2.1; }
				remote { ip 198.51.100.1; }
			}
		}
	}
}
EOF
`
	// The offending file as it first shipped: a different device name on the
	// same pair, with no key, which is what tgrerm0 also claims.
	const kindsBefore = `option=needs-linux:caps=net-admin
stdin=conf:terminator=EOF
interface {
	tunnel tkgre {
		encapsulation {
			gre {
				local { ip 192.0.2.1; }
				remote { ip 198.51.100.1; }
			}
		}
	}
}
EOF
`
	// The fix that landed: its own block of addresses.
	const kindsAfter = `option=needs-linux:caps=net-admin
stdin=conf:terminator=EOF
interface {
	tunnel tkgre {
		encapsulation {
			gre {
				local { ip 192.0.2.201; }
				remote { ip 198.51.100.201; }
			}
		}
	}
}
EOF
`

	clean := map[string]string{"create.ci": reloadCreate, "remove.ci": reloadRemove, "kinds.ci": kindsAfter}
	if dups := duplicateTunnelEndpoints(clean); len(dups) != 0 {
		t.Fatalf("duplicateTunnelEndpoints on the corrected corpus = %v, want none: "+
			"a gre key of its own is what separates the two reload tests", dups)
	}

	broken := map[string]string{"create.ci": reloadCreate, "remove.ci": reloadRemove, "kinds.ci": kindsBefore}
	dups := duplicateTunnelEndpoints(broken)
	if len(dups) != 1 {
		t.Fatalf("duplicateTunnelEndpoints on the colliding corpus = %v, want exactly one collision", dups)
	}
	if !strings.Contains(dups[0], "kinds.ci (tkgre)") || !strings.Contains(dups[0], "remove.ci (tgrerm0)") {
		t.Errorf("collision = %q, want it to name both stanzas that claim the pair", dups[0])
	}
}

// TestCITunnelEndpointLintSeparatesTheDrivers proves the domain grouping: gre
// and gretap are different device types and never collide, while ip6tnl and
// ipip6 are one driver with one endpoint table and always do.
func TestCITunnelEndpointLintSeparatesTheDrivers(t *testing.T) {
	ci := func(kind, name string) string {
		return "option=needs-linux:caps=net-admin\nstdin=conf:terminator=EOF\ninterface {\n\ttunnel " + name +
			" {\n\t\tencapsulation {\n\t\t\t" + kind +
			" {\n\t\t\t\tlocal { ip 2001:db8::1; }\n\t\t\t\tremote { ip 2001:db8::2; }\n\t\t\t}\n\t\t}\n\t}\n}\nEOF\n"
	}

	if dups := duplicateTunnelEndpoints(map[string]string{
		"a.ci": ci("ip6gre", "t1"), "b.ci": ci("ip6gretap", "t2"),
	}); len(dups) != 0 {
		t.Errorf("ip6gre and ip6gretap on one pair = %v, want no collision: they are separate device types", dups)
	}

	if dups := duplicateTunnelEndpoints(map[string]string{
		"a.ci": ci("ip6tnl", "t1"), "b.ci": ci("ipip6", "t2"),
	}); len(dups) != 1 {
		t.Errorf("ip6tnl and ipip6 on one pair = %v, want one collision: both are the ip6_tunnel driver", dups)
	}
}

// TestCITunnelEndpointLintReadsAClaimWithNoLocalAddress proves a stanza that
// gives no local address is still a claim.
//
// local/ip is optional in internal/component/iface/yang/ze-iface-conf.yang and
// `local { interface eth0; }` is the other legal shape, so dropping such a
// stanza left the check blind to it. The kernel keys a tunnel with no source
// address on the wildcard, so two of them on one remote collide.
func TestCITunnelEndpointLintReadsAClaimWithNoLocalAddress(t *testing.T) {
	const byInterface = `option=needs-linux:caps=net-admin
stdin=conf:terminator=EOF
interface {
	tunnel tif0 {
		encapsulation {
			gre {
				local { interface eth0; }
				remote { ip 198.51.100.9; }
			}
		}
	}
}
EOF
`
	const noLocalAtAll = `option=needs-linux:caps=net-admin
stdin=conf:terminator=EOF
interface {
	tunnel tnl0 {
		encapsulation {
			gre {
				remote { ip 198.51.100.9; }
			}
		}
	}
}
EOF
`

	claims := tunnelEndpointClaims(byInterface)
	if len(claims) != 1 {
		t.Fatalf("tunnelEndpointClaims on an interface-sourced stanza = %v, want one claim", claims)
	}
	if claims[0].local != "" || claims[0].remote != "198.51.100.9" {
		t.Errorf("claim = %+v, want an empty local and the remote address", claims[0])
	}

	dups := duplicateTunnelEndpoints(map[string]string{"a.ci": byInterface, "b.ci": noLocalAtAll})
	if len(dups) != 1 {
		t.Fatalf("two source-less gre tunnels on one remote = %v, want one collision", dups)
	}
}

// TestCITunnelEndpointLintCountsInterfacesNotStanzas proves the check separates
// the two shapes a repeated claim inside ONE file can take.
//
// A reload or restart test names one interface in two configs and the kernel
// holds one device for it, so that is not a collision. Two different names on
// one claim in one file is: the second create gets EEXIST inside that run.
func TestCITunnelEndpointLintCountsInterfacesNotStanzas(t *testing.T) {
	stanza := func(name string) string {
		return "\ttunnel " + name + " {\n\t\tencapsulation {\n\t\t\tgre {\n" +
			"\t\t\t\tlocal { ip 192.0.2.210; }\n\t\t\t\tremote { ip 198.51.100.210; }\n" +
			"\t\t\t}\n\t\t}\n\t}\n"
	}
	file := func(first, second string) string {
		return "option=needs-linux:caps=net-admin\nstdin=boot:terminator=EOF\ninterface {\n" +
			stanza(first) + "}\nEOF\nstdin=reload:terminator=EOF2\ninterface {\n" +
			stanza(second) + "}\nEOF2\n"
	}

	if dups := duplicateTunnelEndpoints(map[string]string{"restart.ci": file("trs0", "trs0")}); len(dups) != 0 {
		t.Errorf("one interface named in two configs of one file = %v, want no collision: "+
			"that is the reload and restart shape and the kernel holds one device for it", dups)
	}

	dups := duplicateTunnelEndpoints(map[string]string{"twins.ci": file("ta0", "tb0")})
	if len(dups) != 1 {
		t.Fatalf("two interfaces on one claim in one file = %v, want one collision", dups)
	}
	if !strings.Contains(dups[0], "twins.ci (ta0)") || !strings.Contains(dups[0], "twins.ci (tb0)") {
		t.Errorf("collision = %q, want it to name both interfaces", dups[0])
	}
}

// ciBlock is one `<keyword> [arg] { ... }` block: its argument and its body.
type ciBlock struct {
	arg  string
	body string
}

// namedBlocks returns every top-of-line `keyword arg {` block in raw, with the
// body between its braces. Nested blocks of the same keyword are not expected
// in a config and are not searched for.
func namedBlocks(raw, keyword string) []ciBlock {
	var out []ciBlock
	for i := 0; i < len(raw); {
		idx := strings.Index(raw[i:], keyword+" ")
		if idx < 0 {
			return out
		}
		start := i + idx
		if !atTokenStart(raw, start) {
			i = start + len(keyword)
			continue
		}
		head := start + len(keyword)
		open := strings.IndexByte(raw[head:], '{')
		if open < 0 {
			return out
		}
		arg := strings.TrimSpace(raw[head : head+open])
		body, end := braceBody(raw, head+open)
		if end < 0 || strings.ContainsAny(arg, "\n{}") {
			i = head
			continue
		}
		out = append(out, ciBlock{arg: arg, body: body})
		i = end
	}
	return out
}

// firstBlock returns the body of the first `keyword {` block in raw, or "".
func firstBlock(raw, keyword string) string {
	for i := 0; i < len(raw); {
		idx := strings.Index(raw[i:], keyword)
		if idx < 0 {
			return ""
		}
		start := i + idx
		rest := strings.TrimLeft(raw[start+len(keyword):], " \t")
		if !atTokenStart(raw, start) || !strings.HasPrefix(rest, "{") {
			i = start + len(keyword)
			continue
		}
		open := strings.IndexByte(raw[start:], '{')
		body, end := braceBody(raw, start+open)
		if end < 0 {
			return ""
		}
		return body
	}
	return ""
}

// leafInBlock returns the value of leaf inside the named sub-block, e.g. the
// `ip` leaf of `local { ip 192.0.2.1; }`.
func leafInBlock(raw, block, name string) string {
	return leaf(firstBlock(raw, block), name)
}

// leaf returns the value of a `name value` statement, without its terminator.
func leaf(raw, name string) string {
	for i := 0; i < len(raw); {
		idx := strings.Index(raw[i:], name+" ")
		if idx < 0 {
			return ""
		}
		start := i + idx
		if !atTokenStart(raw, start) {
			i = start + len(name)
			continue
		}
		rest := raw[start+len(name):]
		if cut := strings.IndexAny(rest, "\n;}"); cut >= 0 {
			rest = rest[:cut]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

// atTokenStart reports whether position i starts a token rather than ending
// another one, so `gre` does not match inside `ip6gre`.
func atTokenStart(raw string, i int) bool {
	if i == 0 {
		return true
	}
	switch raw[i-1] {
	case ' ', '\t', '\n', '\r', '{', '}', ';':
		return true
	}
	return false
}

// braceBody returns the text between the brace at open and its match, plus the
// index just past the closing brace. end is -1 when the braces do not balance.
func braceBody(raw string, open int) (body string, end int) {
	depth := 0
	for i := open; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[open+1 : i], i + 1
			}
		}
	}
	return "", -1
}
