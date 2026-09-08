package runner

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// derivedASExpectation is one `.ci` whose derived declaration is pinned here.
//
// The set is small on purpose and covers one shape each: an unkeyed line from a
// single configured AS, a keyed line per session where several ASNs are
// configured, and an AS declared on a `group` rather than on the peer.
var derivedASExpectations = []struct {
	file  string
	lines []string
	why   string
}{
	{
		file:  "plugin/peer-local-port-listener.ci",
		lines: []string{"option=asn:value=65534"},
		why:   "one eBGP peer, so one unkeyed line carrying the AS ze expects from it",
	},
	{
		file: "plugin/rfc4271-partial-unknown-transitive.ci",
		lines: []string{
			"option=asn:peer=127.0.0.1:value=65001",
			"option=asn:peer=127.0.0.2:value=65002",
		},
		why: "one ze-peer process serving two of ze's peers, so one keyed line each",
	},
	{
		file:  "encode/group-encode.ci",
		lines: []string{"option=asn:value=65533"},
		why:   "the AS is declared on the group, and the peer inside it names none",
	},
}

// TestPeerASDerivationReachesEveryCIFile runs the AS derivation over every `.ci`
// in the tree, asserts it refuses none, and asserts it actually DERIVED the
// declaration a known set of files needs.
//
// VALIDATES: two halves that fail differently. `declarePeerAS` fails closed
// without failing any file that exists, and it puts the right `option=asn` line
// into the right block for each shape the suite uses.
// PREVENTS: two failures a unit test cannot see, and one this test used to miss
// itself. A guard tightened until it refuses working files turns a green suite
// red for a reason no `.ci` names. A reader gap left open leaves an eBGP peer
// opening with ze's AS in a file nobody wrote a case for. And an
// absence-only assertion passes over `declarePeerAS` returning nil immediately,
// borrowing every bit of its discrimination from the guard it is meant to be
// independent of.
//
// It lives here rather than beside the parse gate in `internal/test/cli` because
// this package is the one that owns the derivation, and because the `cli`
// package links the whole daemon: a build failure anywhere in it takes this
// coverage down with it, which is exactly what happened on 2026-09-08.
//
// **The corpus has a hole this test cannot close.** A `.ci` written in a dialect
// this parser does not read fails at its first line, so `Discover` marks it
// ParseFailed before `declarePeerAS` is ever called and the derivation never
// sees it. `test/exabgp-compat/` is the largest group of them, each refused at
// line 1 with `option:file missing path=`. The count is logged rather than left
// silent, so a reader knows how much of the tree this gate did not reach.
func TestPeerASDerivationReachesEveryCIFile(t *testing.T) {
	root := filepath.Join("..", "..", "..", "test")
	dirs := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && strings.HasSuffix(path, ".ci") {
			dirs[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(dirs) == 0 {
		// Deliberately fatal, not a skip. A gate that disappears when its input
		// moves reads green forever (ai/rules/evidence.md).
		t.Fatalf("no .ci directories found under %s", root)
	}

	derived := map[string][]string{}
	checked, unreached := 0, 0
	for dir := range dirs {
		ResetNickCounter()
		tests := NewEncodingTests(filepath.Join("..", "..", ".."))
		if discErr := tests.Discover(dir); discErr != nil {
			t.Errorf("%s: discover: %v", dir, discErr)
			continue
		}
		for _, rec := range tests.Registered() {
			checked++
			if rec.ParseFailed {
				unreached++
				// errors.Is, not a list of message substrings. The list was a
				// second declaration of every refusal message, and the first
				// message added after it was written went uncounted: a file
				// refused that way was recorded as never reaching the derivation
				// and the gate passed over it.
				if errors.Is(rec.Error, errASDerivation) {
					t.Errorf("%s: the AS derivation refused this file: %v", rec.CIFile, rec.Error)
				}
				continue
			}
			derived[filepath.ToSlash(rec.CIFile)] = declaredASLines(rec)
		}
	}
	if checked == 0 {
		t.Fatal("no .ci records parsed; the gate covered nothing")
	}
	t.Logf("parsed %d .ci; %d never reached the derivation because they failed to parse first", checked, unreached)

	for _, want := range derivedASExpectations {
		lines, parsed := lookupDerived(derived, want.file)
		if !parsed {
			t.Errorf("%s: not among the parsed records, so its derivation was never checked", want.file)
			continue
		}
		if len(lines) != len(want.lines) {
			t.Errorf("%s: derived %v, want %v (%s)", want.file, lines, want.lines, want.why)
			continue
		}
		for _, line := range want.lines {
			if !containsLine(lines, line) {
				t.Errorf("%s: derived %v, want it to carry %q (%s)", want.file, lines, line, want.why)
			}
		}
	}
}

// declaredASLines is every option=asn line the peer blocks of a record carry
// after the derivation ran.
func declaredASLines(rec *Record) []string {
	var out []string
	for _, name := range peerBlockNames(rec) {
		for line := range strings.SplitSeq(string(rec.StdinBlocks[name]), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "option=asn:") {
				out = append(out, strings.TrimSpace(line))
			}
		}
	}
	return out
}

// lookupDerived finds a record by the tail of its path, because Discover records
// an absolute path and the expectations name a suite-relative one.
func lookupDerived(derived map[string][]string, suffix string) ([]string, bool) {
	for path, lines := range derived {
		if strings.HasSuffix(path, "/"+suffix) {
			return lines, true
		}
	}
	return nil, false
}

func containsLine(lines []string, want string) bool {
	return slices.Contains(lines, want)
}
