package runner

import (
	"errors"
	"fmt"
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

// TestPeerASDerivationReachesEveryCIFile runs the AS derivation over every native
// encoding-runner `.ci`, refuses any parse failure, and checks declarations for
// known peer shapes.
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
// Decode and ExaBGP compatibility files have different execution parsers:
// DecodingTests.parseCIFile and cli.parseExaBGPCI, respectively. They never call
// declarePeerAS. Exclude those root populations before native discovery, not by
// accepting errors afterwards: every native parse failure MUST fail this gate,
// whether it occurs before, during, or after AS derivation.
// The shared draft boundary also keeps unpromoted fixtures outside repo gates.
func TestPeerASDerivationReachesEveryCIFile(t *testing.T) {
	derived, err := peerASCorpus(filepath.Join("..", "..", "..", "test"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("parsed %d native .ci records without discovery failures", len(derived))

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

// peerASCorpus selects the native runner's population before parsing. A foreign
// dialect is not a native parse failure; a native parse failure is never omitted.
func peerASCorpus(root string) (map[string][]string, error) {
	decodeRoot := filepath.Join(root, "decode")
	compatRoot := filepath.Join(root, "exabgp-compat")
	dirs := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if isDraftPath(root, path) {
				return filepath.SkipDir
			}
			if path == decodeRoot {
				return filepath.SkipDir
			}
			if path == compatRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".ci") {
			dirs[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk native .ci corpus: %w", err)
	}
	if len(dirs) == 0 {
		return nil, errors.New("no native .ci directories found")
	}

	derived := map[string][]string{}
	for dir := range dirs {
		ResetNickCounter()
		tests := NewEncodingTests(filepath.Dir(root))
		if err := tests.Discover(dir); err != nil {
			return nil, fmt.Errorf("discover %s: %w", dir, err)
		}
		for _, rec := range tests.Registered() {
			if rec.ParseFailed {
				return nil, fmt.Errorf("%s: native .ci parse failed: %w", rec.CIFile, rec.Error)
			}
			derived[filepath.ToSlash(rec.CIFile)] = declaredASLines(rec)
		}
	}
	if len(derived) == 0 {
		return nil, errors.New("no native .ci records parsed")
	}
	return derived, nil
}

// TestPeerASCorpusRejectsNativeParseFailures proves that discovery cannot hide
// failures before derivation or inside it, even beneath foreign-looking names.
func TestPeerASCorpusRejectsNativeParseFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		body string
		want error
	}{
		{
			name: "missing native option path",
			path: "plugin/bad.ci",
			body: "option=file:wrong.conf\n",
			want: errOptionFileMissingPath,
		},
		{
			name: "nested decode remains native",
			path: "plugin/decode/bad.ci",
			body: "option=file:wrong.conf\n",
			want: errOptionFileMissingPath,
		},
		{
			name: "nested compatibility name remains native",
			path: "plugin/exabgp-compat/bad.ci",
			body: "option=file:wrong.conf\n",
			want: errOptionFileMissingPath,
		},
		{
			name: "nested draft name remains native",
			path: "plugin/draft/bad.ci",
			body: "option=file:wrong.conf\n",
			want: errOptionFileMissingPath,
		},
		{
			name: "malformed consumed config",
			path: "plugin/bad.ci",
			body: "stdin=peer:terminator=EOF_PEER\n" +
				"expect=bgp:conn=1:seq=1:hex=FFFF001304\nEOF_PEER\n" +
				"stdin=ze-bgp:terminator=EOF_CONF\nbgp { peer 'broken\nEOF_CONF\n" +
				"cmd=background:seq=1:exec=le test peer --port 1179:stdin=peer\n" +
				"cmd=foreground:seq=2:exec=ze -:stdin=ze-bgp\n",
			want: errASDerivation,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, tc.path)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := peerASCorpus(root)
			if !errors.Is(err, tc.want) {
				t.Fatalf("corpus error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestPeerASCorpusUsesExecutionParsers keeps foreign dialects and unpromoted
// drafts outside this oracle while retaining intentional native config rejection.
func TestPeerASCorpusUsesExecutionParsers(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		"exabgp-compat/encoding/conf.ci": "option=file:conf.conf\n",
		"decode/message.ci":              "expect=json:json={}\n",
		"draft/plugin/unfinished.ci":     "option=file:wrong.conf\n",
		"parse/reject-config.ci": "stdin=bad:terminator=EOF_BAD\n" +
			"bgp { peer 'broken\nEOF_BAD\n" +
			"cmd=foreground:seq=1:exec=ze config validate -:stdin=bad\n" +
			"expect=exit:code=1\n",
	} {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	derived, err := peerASCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(derived) != 1 {
		t.Fatalf("native population = %v, want only intentional config rejection", derived)
	}
	if _, exists := lookupDerived(derived, "parse/reject-config.ci"); !exists {
		t.Fatal("intentional native config rejection was omitted")
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
