// Design: docs/contributing/committing.md -- change-set checks ./le commit create runs
// Related: register.go -- command and changecheck registration
// Related: actions.go -- the ./le commit secrets check action
//
// Package commitsecrets refuses secrets, private keys, credential files, live
// secret-named environment values, and local home-directory paths added by a
// commit (ai/rules/git-safety.md).
package commitsecrets

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

const (
	// allowMarker exempts one line carrying a deliberate synthetic token or
	// path in a test fixture. Split so this file never exempts itself.
	allowMarker = "secret-scan: " + "allow"
	// maxScanBytes skips binary or oversized payloads (lockfiles and fixtures
	// stay well below this cap).
	maxScanBytes = 4 * 1024 * 1024
	// binaryProbeBytes is the prefix length inspected for a NUL byte to
	// distinguish binary blobs from text sources.
	binaryProbeBytes = 8192
	// minSecretEnvLen avoids matching short non-secret words from environment
	// variables whose names happen to contain a secret keyword.
	minSecretEnvLen = 12
	// minHomePathLen skips non-personal roots such as "/root" or "C:\".
	minHomePathLen = 6
)

// tokenPrefix describes a known credential token prefix and the minimum run of
// token characters that follows a real key.
type tokenPrefix struct {
	prefix  string
	minTail int
}

// knownTokenPrefixes lists API token prefixes split across two literals so this
// source file never triggers itself if scanned as raw text.
var knownTokenPrefixes = [...]tokenPrefix{
	{prefix: "sk-" + "ant-", minTail: 20},
	{prefix: "sk-" + "proj-", minTail: 20},
	{prefix: "sk-" + "or-v1-", minTail: 20},
	{prefix: "AK" + "IA", minTail: 16},
	{prefix: "AS" + "IA", minTail: 16},
	{prefix: "AI" + "za", minTail: 30},
	{prefix: "gh" + "p_", minTail: 30},
	{prefix: "gh" + "o_", minTail: 30},
	{prefix: "gh" + "s_", minTail: 30},
	{prefix: "github_" + "pat_", minTail: 30},
	{prefix: "gl" + "pat-", minTail: 20},
	{prefix: "xo" + "xb-", minTail: 10},
	{prefix: "xo" + "xp-", minTail: 10},
	{prefix: "hf" + "_", minTail: 30},
	{prefix: "r8" + "_", minTail: 30},
	{prefix: "sk_" + "live_", minTail: 20},
}

// privateKeyHeaders lists PEM private-key markers split across literals so this
// file never triggers itself.
var privateKeyHeaders = [...]string{
	"-----BEGIN " + "PRIVATE KEY-----",
	"-----BEGIN " + "RSA PRIVATE KEY-----",
	"-----BEGIN " + "EC PRIVATE KEY-----",
	"-----BEGIN " + "OPENSSH PRIVATE KEY-----",
	"-----BEGIN " + "DSA PRIVATE KEY-----",
}

// forbiddenNames are file basenames that hold credentials or local secrets and
// must never be newly committed.
var forbiddenNames = [...]string{
	".env",
	".netrc",
	".npmrc",
	"id_rsa",
	"id_ed25519",
	"settings.local.json",
	"credentials.json",
}

// forbiddenKeyExts are private-key and keystore extensions refused outside test
// fixture trees.
var forbiddenKeyExts = [...]string{
	".pem",
	".p12",
	".pfx",
	".key",
	".keystore",
	".jks",
}

// secretEnvKeywords mark an environment variable whose value should not appear
// verbatim in committed text.
var secretEnvKeywords = [...]string{
	"KEY",
	"TOKEN",
	"SECRET",
	"PASSWORD",
	"PASSWD",
	"CREDENTIAL",
	"AUTH",
}

// safeEnvNames are public or non-secret environment variables whose names
// contain one of secretEnvKeywords.
var safeEnvNames = map[string]bool{
	"GIT_AUTHOR_NAME":  true,
	"GIT_AUTHOR_EMAIL": true,
	"GIT_AUTHOR_DATE":  true,
	"SSH_AUTH_SOCK":    true,
	"XAUTHORITY":       true,
	"ICEAUTHORITY":     true,
}

// Finding is one secret, credential file, or local home path detected in a
// change.
type Finding struct {
	Path string         `json:"path"`
	Line int            `json:"line,omitempty"`
	Err  *opererr.Error `json:"error"`
}

// envLeak is one live environment value (a secret-named variable or the user's
// home directory) that must not be leaked into a commit.
type envLeak struct {
	name   string
	value  string
	isHome bool
}

// ze point: git-safety/directives/never-commit-secrets-or-home-paths
//
// judgeChange is the commit-time change-set check registered into changecheck.
func judgeChange(change changecheck.Change) (string, error) {
	findings, err := ScanChange(change, os.Environ())
	if err != nil {
		return "", err
	}
	return FormatFindings(findings), nil
}

// ScanChange inspects every carried path in change against its parent and
// returns findings for newly added forbidden files and newly added lines
// holding API tokens, private keys, live secret env values, or home paths.
func ScanChange(change changecheck.Change, environ []string) ([]Finding, error) {
	leaks := collectEnvLeaks(environ)
	var findings []Finding
	for _, rel := range change.Paths {
		beforeText, hadBefore, err := change.Before(rel)
		if err != nil {
			return nil, err
		}
		if !hadBefore {
			if finding, bad := forbiddenPathFinding(rel); bad {
				findings = append(findings, finding)
			}
		}
		afterText, hasAfter, err := change.After(rel)
		if err != nil {
			return nil, err
		}
		if !hasAfter {
			continue
		}
		if len(afterText) > maxScanBytes {
			continue
		}
		if looksBinary(afterText) {
			continue
		}
		added := addedLines(beforeText, hadBefore, afterText)
		fixture := isTestFixturePath(rel)
		for _, entry := range added {
			if strings.Contains(entry.text, allowMarker) {
				continue
			}
			lineFindings := scanLine(rel, entry.line, entry.text, fixture, leaks)
			findings = append(findings, lineFindings...)
		}
	}
	return findings, nil
}

// FormatFindings renders findings into the refusal message `./le commit create`
// prints, or "" when findings is empty.
func FormatFindings(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("commit adds secrets, credential files, or local home-directory paths:\n")
	for _, item := range findings {
		tb.Str("  ").Str(item.Err.Error()).Byte('\n')
	}
	tb.Str("See ai/rules/git-safety.md. For a deliberate synthetic fixture, mark the line with `").
		Str(allowMarker).Str("`.\n")
	return tb.String()
}

type numberedLine struct {
	line int
	text string
}

// addedLines returns every line in afterText that was not already present at
// the same multiplicity in beforeText.
func addedLines(beforeText string, hadBefore bool, afterText string) []numberedLine {
	beforeCounts := make(map[string]int)
	if hadBefore {
		for line := range strings.SplitSeq(beforeText, "\n") {
			beforeCounts[line]++
		}
	}
	var added []numberedLine
	lineNo := 0
	for line := range strings.SplitSeq(afterText, "\n") {
		lineNo++
		if beforeCounts[line] > 0 {
			beforeCounts[line]--
			continue
		}
		added = append(added, numberedLine{line: lineNo, text: line})
	}
	return added
}

func forbiddenPathFinding(rel string) (Finding, bool) {
	base := path.Base(rel)
	for _, name := range forbiddenNames {
		if base == name {
			return Finding{
				Path: rel,
				Err: opererr.New(
					rel+": credential or local settings file",
					"file holds local credentials or session settings and must not be committed",
					"omit "+rel+" from `./le commit create` and add it to `.gitignore` if local",
				),
			}, true
		}
		if name == ".env" && strings.HasPrefix(base, ".env.") {
			return Finding{
				Path: rel,
				Err: opererr.New(
					rel+": local environment file",
					"environment file holds local secrets and must not be committed",
					"omit "+rel+" from `./le commit create` and keep secrets in environment variables",
				),
			}, true
		}
	}
	if isTestFixturePath(rel) {
		return Finding{}, false
	}
	ext := strings.ToLower(path.Ext(base))
	for _, badExt := range forbiddenKeyExts {
		if ext == badExt {
			return Finding{
				Path: rel,
				Err: opererr.New(
					rel+": private key or keystore file outside test fixtures",
					"key or certificate material outside test fixtures must not be committed",
					"omit "+rel+" from `./le commit create`, or place synthetic test material under `testdata/` or `test/`",
				),
			}, true
		}
	}
	return Finding{}, false
}

// isTestFixturePath reports whether rel sits inside a test fixture tree where
// synthetic PKI certificates and keys are legitimately committed.
func isTestFixturePath(rel string) bool {
	if strings.HasPrefix(rel, "test/") {
		return true
	}
	if strings.HasPrefix(rel, "testdata/") {
		return true
	}
	return strings.Contains(rel, "/testdata/")
}

func looksBinary(text string) bool {
	limit := min(len(text), binaryProbeBytes)
	return bytes.IndexByte([]byte(text[:limit]), 0) >= 0
}

func locationLabel(rel string, line int) string {
	var tb textbuf.Buffer
	return tb.Str(rel).Byte(':').Int(int64(line)).String()
}

func scanLine(rel string, line int, text string, fixture bool, leaks []envLeak) []Finding {
	loc := locationLabel(rel, line)
	var findings []Finding
	for _, rule := range knownTokenPrefixes {
		if hasTokenPrefix(text, rule.prefix, rule.minTail) {
			var why textbuf.Buffer
			why.Str("line contains a token matching prefix ").Quoted(rule.prefix)
			findings = append(findings, Finding{
				Path: rel,
				Line: line,
				Err: opererr.New(
					loc+": API token prefix detected",
					why.String(),
					"remove the token and rotate it if real, or replace it with an obvious fake",
				),
			})
		}
	}
	if !fixture {
		for _, header := range privateKeyHeaders {
			if strings.Contains(text, header) {
				findings = append(findings, Finding{
					Path: rel,
					Line: line,
					Err: opererr.New(
						loc+": PEM private key header outside test fixtures",
						"line contains a PEM private key header outside test fixtures",
						"remove the private key or move synthetic test material under `testdata/` or `test/`",
					),
				})
				break
			}
		}
	}
	for _, leak := range leaks {
		if leak.isHome {
			if containsHomePath(text, leak.value) {
				var why textbuf.Buffer
				why.Str("line contains the local ").Str(leak.name).Str(" directory path")
				findings = append(findings, Finding{
					Path: rel,
					Line: line,
					Err: opererr.New(
						loc+": local home directory path detected",
						why.String(),
						"replace the local home path with a repo-relative path or placeholder",
					),
				})
			}
			continue
		}
		if strings.Contains(text, leak.value) {
			var why textbuf.Buffer
			why.Str("line contains the live value of environment variable ").Str(leak.name)
			findings = append(findings, Finding{
				Path: rel,
				Line: line,
				Err: opererr.New(
					loc+": live environment secret detected",
					why.String(),
					"remove the secret value and reference the environment variable by name only",
				),
			})
		}
	}
	return findings
}

func hasTokenPrefix(text, prefix string, minTail int) bool {
	rest := text
	for {
		idx := strings.Index(rest, prefix)
		if idx < 0 {
			return false
		}
		if idx > 0 && isTokenChar(rest[idx-1]) {
			rest = rest[idx+len(prefix):]
			continue
		}
		tail := rest[idx+len(prefix):]
		run := 0
		for run < len(tail) && isTokenChar(tail[run]) {
			run++
		}
		if run >= minTail {
			return true
		}
		rest = tail[run:]
	}
}

func isTokenChar(ch byte) bool {
	if ch >= 'a' && ch <= 'z' {
		return true
	}
	if ch >= 'A' && ch <= 'Z' {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '_' || ch == '-'
}

func containsHomePath(text, home string) bool {
	rest := text
	for {
		idx := strings.Index(rest, home)
		if idx < 0 {
			return false
		}
		end := idx + len(home)
		if end == len(rest) {
			return true
		}
		if isPathBoundary(rest[end]) {
			return true
		}
		rest = rest[end:]
	}
}

func isPathBoundary(ch byte) bool {
	switch ch {
	case '/', '\\', '"', '\'', '`', ' ', '\t', ':', ',', ')', ']':
		return true
	default:
		return false
	}
}

func collectEnvLeaks(environ []string) []envLeak {
	var leaks []envLeak
	seen := make(map[string]bool)
	add := func(leak envLeak) {
		if seen[leak.value] {
			return
		}
		seen[leak.value] = true
		leaks = append(leaks, leak)
	}
	for _, entry := range environ {
		key, val, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if val == "" {
			continue
		}
		upper := strings.ToUpper(key)
		switch upper {
		case "HOME", "USERPROFILE":
			clean := strings.TrimRight(val, "/\\")
			if len(clean) <= minHomePathLen {
				continue
			}
			if clean == "/root" {
				continue
			}
			add(envLeak{name: key, value: clean, isHome: true})
			slash := filepath.ToSlash(clean)
			if slash != clean {
				add(envLeak{name: key, value: slash, isHome: true})
			}
			continue
		}
		if safeEnvNames[upper] {
			continue
		}
		if !hasSecretEnvKeyword(upper) {
			continue
		}
		if len(val) < minSecretEnvLen {
			continue
		}
		if strings.ContainsAny(val, "/\\ \t\r\n") {
			continue
		}
		add(envLeak{name: key, value: val})
	}
	return leaks
}

func hasSecretEnvKeyword(upper string) bool {
	for _, word := range secretEnvKeywords {
		if strings.Contains(upper, word) {
			return true
		}
	}
	return false
}
