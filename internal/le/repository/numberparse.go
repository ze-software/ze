// Design: docs/architecture/core-design.md -- the post-verify repository checks
// RFC: rfc/short/rfc6793.md -- the four-octet AS number this check is about
//
// numberparse.go is the sixth check: every 32-bit text-to-integer parse in
// product Go is on an allowlist, so a NEW one is justified rather than
// discovered.
//
// The defect it exists for is recorded over seven review rounds of
// plan/spec-bgp-as-notation.md. An AS number is a 32-bit value an operator
// types. The tree grew a private parser for it in eight places: two AS-path
// readers, four route distinguisher readers, and two `AS<n>` selector readers.
// Each copy read decimal only, so `1.10` was accepted at one entry point and
// refused at the next. The selector copy failed SILENTLY. Its caller turned the
// refusal into a peer name, which matched no peer and reported nothing.
//
// What the check CAN see, in three cases:
//   - a 32-bit parse in a file the allowlist does not name.
//   - a different number of them in a file than the allowlist records.
//   - an allowlist line for a file that now parses none.
//
// What it CANNOT see. Four cases, and each one is a way an AS number still
// reaches strconv.
//
//  1. An ALLOWLISTED parse that was always meant to be asn.Parse. Nothing
//     static separates an AS number from a table index. The evidence for the
//     current population is the per-parser enumeration in
//     plan/spec-bgp-as-notation.md, "Every Text-to-AS-Number Parser In The
//     Tree".
//
//  2. A bit size that crosses a function boundary, so the width is not at the
//     call. Three product files hold one: `validateUint`
//     (component/command/argvalidate.go), `injectParseUint`
//     (plugins/ospf/inject.go) and le/deployment/l2tpdiag.go. The tree has 98
//     such files where this check counts 95.
//
//  3. A parse at another width that still yields an AS number, such as Atoi
//     into a uint32.
//
//  4. A hand-rolled digit loop, which names strconv not at all.
//
// Of those four, `validateUint` is the one to know about. It is the YANG-typed pre-handler
// validator. A command input leaf typed `zt:asn` would therefore refuse a
// dotted AS number BEFORE any handler reached the ASN reader. Nothing is broken
// today, because every AS command input leaf is `zt:asn-notated`. The trap is
// set for whoever types the next one `zt:asn`.
//
// A gate that repeated the sweep's own name matching would inherit the sweep's
// blind spot. That sweep saw neither `s[2:]` after a two-character prefix test
// nor `args[i]` behind a flag name, which is how both misses of round 7
// happened.

package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// NumberParseAllowlistPath is the tree-relative allowlist: one `count path`
// line for each product file that parses a 32-bit integer from text.
const NumberParseAllowlistPath = "internal/le/repository/numberparse-allowlist.txt"

// NumberParsePattern matches a text-to-integer parse whose result is 32 bits
// wide, which is the width of an AS number (RFC 6793). A port is parsed at 16
// and a timer at 64, so this width is where the AS numbers are.
const NumberParsePattern = `strconv\.Parse(?:Uint|Int)\(\s*[^,()]*(?:\([^()]*\))?[^,()]*,\s*10,\s*32\s*\)`

var numberParseRe = regexp.MustCompile(NumberParsePattern)

// numberParseSkipDirs are the trees this check does not judge: vendored
// sources, build outputs and scratch. Test files are skipped by name, because
// a test parses fixtures rather than operator input.
var numberParseSkipDirs = map[string]bool{
	".git": true, "vendor": true, "cache": true, "node_modules": true,
	"tmp": true, "bin": true, rfcDir: true, "gokrazy": true, "backups": true,
	"website": true, "testdata": true,
}

// rfcDir is the RFC text store, which carries no Go and is skipped by name.
const rfcDir = "rfc"

// numberParseCounts walks the product tree and answers how many 32-bit parses
// each file holds. A file with none is absent.
func numberParseCounts(tree string) (map[string]int, error) {
	counts := make(map[string]int)
	err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != tree && (numberParseSkipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path) //nolint:gosec // a Go file of the tree the caller named
		if readErr != nil {
			return readErr
		}
		found := len(numberParseRe.FindAll(data, -1))
		if found == 0 {
			return nil
		}
		rel, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return relErr
		}
		counts[filepath.ToSlash(rel)] = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// readNumberParseAllowlist reads the checked-in allowlist. A missing file is an
// ERROR rather than an empty allowlist: an empty one would pass every file,
// which is the fail-open shape ai/rules/evidence.md bans. A tree that parses
// NOTHING is the one exception, and the caller decides that.
func readNumberParseAllowlist(tree string) (map[string]int, error) {
	data, err := os.ReadFile(filepath.Join(tree, NumberParseAllowlistPath)) //nolint:gosec // the allowlist of the tree the caller named
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]int)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count, path, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		n, convErr := strconv.Atoi(count)
		if convErr != nil {
			continue
		}
		allowed[strings.TrimSpace(path)] = n
	}
	return allowed, nil
}

// checkNumberParseSites is the gate. It reports three things:
//   - a file that parses a 32-bit integer and is not on the allowlist.
//   - a file whose count differs from its line.
//   - a line whose file parses none.
//
// The count is EXACT rather than a ceiling. Under a ceiling, a deleted parse
// leaves a permanent free slot for an unjustified one in the same file. Nothing
// would say the slot had opened.
func checkNumberParseSites(tree string) ([]Finding, error) {
	counts, err := numberParseCounts(tree)
	if err != nil {
		return nil, err
	}
	allowed, err := readNumberParseAllowlist(tree)
	if err != nil {
		if os.IsNotExist(err) && len(counts) == 0 {
			// A fixture checkout with no such parse and no allowlist has
			// nothing to judge either way. A tree that PARSES and carries no
			// allowlist is the error above, so the gate still fails closed.
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0, len(counts))
	for path := range counts {
		paths = append(paths, path)
	}
	slices.Sort(paths)

	var findings []Finding
	for _, path := range paths {
		recorded, listed := allowed[path]
		if listed && counts[path] == recorded {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("32-bit text-to-integer parse count is ")
		tb.Int(int64(counts[path]))
		if listed {
			tb.Str(", and ").Str(NumberParseAllowlistPath).Str(" records ").Int(int64(recorded))
		} else {
			tb.Str(", and the file is not on the allowlist")
		}
		tb.Str(": an AS NUMBER is read with asn.Parse (internal/core/bgp/asn), which takes every RFC 5396 spelling. ")
		tb.Str("If this value is not an AS number, correct the file's line in ").Str(NumberParseAllowlistPath)
		findings = append(findings, Finding{Severity: severityIssue, File: path, Message: tb.String()})
	}

	// A line whose file parses none is stale: the file was deleted, or its
	// parse became asn.Parse. Left in place, that line pre-authorizes the next
	// parse written there, so nothing would report it.
	stale := make([]string, 0, len(allowed))
	for path := range allowed {
		if counts[path] == 0 {
			stale = append(stale, path)
		}
	}
	slices.Sort(stale)
	for _, path := range stale {
		var tb textbuf.Buffer
		tb.Str("stale line in ").Str(NumberParseAllowlistPath)
		tb.Str(": this file parses no 32-bit integer, so the line pre-authorizes the next one. Remove it")
		findings = append(findings, Finding{Severity: severityIssue, File: path, Message: tb.String()})
	}
	return findings, nil
}
