package firewall

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestProtocolNumber pins the single protocol-name -> IANA-number table that
// all firewall backends share.
//
// VALIDATES: ProtocolNumber maps every supported L4 protocol name to its IANA
// number and reports ok=false for unknown names.
// PREVENTS: the nft, VPP-classify, and VPP-NAT backends keeping divergent
// private copies of this table -- the NAT copy previously handled only tcp/udp
// and silently programmed protocol 0 for every other protocol.
func TestProtocolNumber(t *testing.T) {
	want := map[string]uint8{
		"tcp": 6, "udp": 17, "icmp": 1, "icmpv6": 58,
		"sctp": 132, "gre": 47, "esp": 50, "ah": 51,
		"ospf": 89, "vrrp": 112,
	}
	for name, num := range want {
		got, ok := ProtocolNumber(name)
		if !ok || got != num {
			t.Errorf("ProtocolNumber(%q) = (%d, %v), want (%d, true)", name, got, ok, num)
		}
	}
	for _, name := range []string{"", "TCP", "bogus", "ip", "0"} {
		if got, ok := ProtocolNumber(name); ok {
			t.Errorf("ProtocolNumber(%q) = (%d, true), want ok=false", name, got)
		}
	}
}

// TestProtocolNameRoundTripsEveryCanonicalNumber pins the reverse direction of
// the single protocol table.
//
// VALIDATES: ProtocolName returns the canonical name for every number
// ProtocolNumber hands out, and ProtocolNumber accepts that name back.
// PREVENTS: a producer that starts from a wire protocol number keeping its own
// number -> name table, which is how the FlowSpec translator came to know five
// of the ten names and to render the rest as decimal digits.
func TestProtocolNameRoundTripsEveryCanonicalNumber(t *testing.T) {
	for _, name := range ProtocolNames() {
		num, ok := ProtocolNumber(name)
		if !ok {
			t.Fatalf("ProtocolNumber(%q) = ok false, but the name came from ProtocolNames", name)
		}
		back, ok := ProtocolName(num)
		if !ok || back != name {
			t.Errorf("ProtocolName(%d) = (%q, %v), want (%q, true)", num, back, ok, name)
		}
	}
}

// TestProtocolNameRefusesUnnamedNumber covers the boundary rows of the spec's
// numeric table: 0 sits below the lowest canonical number and 133 above the
// highest, and neither has a name.
//
// VALIDATES: ProtocolName reports ok=false rather than inventing a spelling.
// PREVENTS: a MatchProtocol carrying digits, which no backend can lower and
// which aborts the whole firewall reconcile for every owner.
func TestProtocolNameRefusesUnnamedNumber(t *testing.T) {
	for _, num := range []uint8{0, 2, 4, 41, 133, 255} {
		if got, ok := ProtocolName(num); ok {
			t.Errorf("ProtocolName(%d) = (%q, true), want ok=false", num, got)
		}
	}
}

// TestProtocolNamesListsEveryCanonicalName checks the accessor validators and
// error messages use so they never spell the accepted set a second time.
//
// VALIDATES: ProtocolNames returns each name once, sorted.
// PREVENTS: an error message that names a set the backends do not accept.
func TestProtocolNamesListsEveryCanonicalName(t *testing.T) {
	names := ProtocolNames()
	if len(names) != len(ianaProtocolNumbers) {
		t.Fatalf("ProtocolNames returned %d names, table holds %d", len(names), len(ianaProtocolNumbers))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("ProtocolNames is not sorted and deduplicated: %v", names)
		}
	}
	for _, name := range names {
		if _, ok := ianaProtocolNumbers[name]; !ok {
			t.Errorf("ProtocolNames returned %q, which the table does not carry", name)
		}
	}
}

// TestProtocolTableIsSingleSource holds the protocol table to one file.
//
// VALIDATES: no Go file that produces or consumes a MatchProtocol spells a
// canonical protocol name beside that protocol's IANA number. protocol.go is
// the only file that maps the two, so a producer that starts from a wire
// number reaches the name through ProtocolName, and a backend that must
// program a number reaches it through ProtocolNumber.
// PREVENTS: a private copy drifting from the canonical table. The FlowSpec
// translator kept one that knew five of the ten names and rendered every other
// number as decimal digits. No backend can lower digits, and Apply returns a
// lowering error before its single Flush, so one such rule from one peer left
// every owner's ruleset unapplied in the kernel.
//
// The scope is derived, not listed: a file is read when it names
// MatchProtocol, which is what makes it part of this vocabulary. A protocol
// name that belongs to a DIFFERENT vocabulary stays unread. The FlowSpec NLRI
// text codec is one such vocabulary. It knows igmp and the icmp6 alias, which
// no firewall backend accepts.
//
// This test owns the structural half of the single-source claim. The
// behavioral half is one test per consumer, each looping over ProtocolNames so
// a name the table gains reaches every backend: TestLowerProtoMatchAcceptsEveryCanonicalName,
// TestTranslateTermEveryCanonicalProtocol, TestBuildDropTermCoversEveryCanonicalProtocol,
// TestComponentToMatchEveryCanonicalNumber and TestProtocolNameEnumMatchesCanonicalTable.
func TestProtocolTableIsSingleSource(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, "internal", "component", "firewall", "protocol.go")

	for _, tree := range []string{"internal", "pkg", "cmd"} {
		walkErr := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			// A test that pins the mapping is the intended pin, and
			// TestProtocolNumber above is one. Only a non-test file can ship a
			// private table into the daemon.
			if strings.HasSuffix(path, "_test.go") || path == canonical {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(source)
			if !strings.Contains(text, "MatchProtocol") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			for _, pair := range protocolTablePairs(strings.Split(text, "\n")) {
				t.Errorf("%s:%d spells protocol %q beside its IANA number %d, which is a private "+
					"protocol table: resolve the name through ProtocolName or the number through "+
					"ProtocolNumber instead", rel, pair.line, pair.name, pair.number)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walking %s: %v", tree, walkErr)
		}
	}
}

// protocolTablePair is one canonical name found close to its own IANA number,
// which is the shape a private protocol table takes in Go source.
type protocolTablePair struct {
	name   string
	number uint8
	line   int
}

// protocolTableWindow is how many lines apart the two halves of one table entry
// can sit. A map entry writes both on one line. A switch writes `case 132:` and
// `return "sctp"` on two, and gofmt can leave a comment between them.
const protocolTableWindow = 3

// protocolTablePairs returns each canonical name in lines that sits within
// protocolTableWindow lines of its own IANA number, at most one pair per name.
// The caller reports every pair, so one drifted file names every protocol it
// spells rather than only the first.
func protocolTablePairs(lines []string) []protocolTablePair {
	var pairs []protocolTablePair
	// ProtocolNames is sorted, so a file with several pairs reports them in the
	// same order on every run.
	for _, name := range ProtocolNames() {
		number := ianaProtocolNumbers[name]
		nameLines := linesHoldingName(lines, name)
		if len(nameLines) == 0 {
			continue
		}
		for _, at := range nameLines {
			if !numberIsNear(lines, number, at) {
				continue
			}
			pairs = append(pairs, protocolTablePair{name: name, number: number, line: at + 1})
			break
		}
	}
	return pairs
}

// linesHoldingName returns the index of every line carrying the name as a Go
// string literal. A bare word is not enough: `ospf` names a plugin in many
// files, and only the quoted form can be a table key or a returned name.
func linesHoldingName(lines []string, name string) []int {
	var at []int
	quoted := `"` + name + `"`
	for i := range lines {
		if strings.Contains(lines[i], quoted) {
			at = append(at, i)
		}
	}
	return at
}

// numberIsNear reports whether number appears as a standalone decimal integer
// within protocolTableWindow lines of the line at index.
func numberIsNear(lines []string, number uint8, index int) bool {
	first := max(index-protocolTableWindow, 0)
	last := min(index+protocolTableWindow, len(lines)-1)
	for i := first; i <= last; i++ {
		if hasIntegerToken(lines[i], number) {
			return true
		}
	}
	return false
}

// hasIntegerToken reports whether line holds want as a standalone decimal
// integer. The bounds keep 6 from matching inside 16, 256, 0x60 or a field
// named buf6, so a small number in unrelated code is not read as a protocol
// number.
func hasIntegerToken(line string, want uint8) bool {
	text := strconv.Itoa(int(want))
	for start := 0; start <= len(line)-len(text); {
		offset := strings.Index(line[start:], text)
		if offset < 0 {
			return false
		}
		at := start + offset
		if !isIdentifierByte(line, at-1) && !isIdentifierByte(line, at+len(text)) {
			return true
		}
		start = at + 1
	}
	return false
}

// isIdentifierByte reports whether the byte at index continues an identifier or
// a number. An index outside the line does not.
func isIdentifierByte(line string, index int) bool {
	if index < 0 || index >= len(line) {
		return false
	}
	c := line[index]
	if c == '_' || c == '.' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	if c >= 'a' && c <= 'z' {
		return true
	}
	return c >= 'A' && c <= 'Z'
}

// repoRoot walks up from the test's working directory to the module root, the
// directory holding go.mod. go test runs in the package directory, so the walk
// is three levels for this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod in any parent of the test's working directory")
		}
		dir = parent
	}
}
