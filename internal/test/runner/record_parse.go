// Design: docs/architecture/testing/ci-format.md — CI file parsing and test discovery
// Overview: record.go — Record type definitions and methods
// Related: record_collection.go — Tests container and querying
// Related: accept_only.go — accept-only (weak) predicate over the parsed Records
// Related: record_parse_option.go — the option= arms
// Related: caps.go — needs-linux capability probe and the netns-link skip gate

package runner

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// directiveTypeBGP is the type token of the expect:bgp and reject:bgp
// directives, which name the BGP wire rather than the JSON envelope typeBGP
// names (json.go).
const directiveTypeBGP = "bgp"

var (
	errOptionFileMissingPath            = errors.New("option:file missing path=")
	errOptionAsnMissingValue            = errors.New("option:asn missing value=")
	errOptionBindMissingValue           = errors.New("option:bind missing value=")
	errOptionTimeoutMissingValue        = errors.New("option:timeout missing value=")
	errOptionTcpConnectionsMissingValue = errors.New("option:tcp_connections missing value=")
	errOptionOpenMissingValue           = errors.New("option:open missing value=")
	errOptionUpdateMissingValue         = errors.New("option:update missing value=")
	errOptionEnvMissingVar              = errors.New("option:env missing var=")
	errOptionSkipOsMissingValue         = errors.New("option:skip-os missing value=")
	errOptionSkipEnvMissingVar          = errors.New("option:skip-env missing var=")
	errOptionNetnsLinkMissingName       = errors.New("option:netns-link missing name=")
	errOptionExclusiveMissingGroup      = errors.New("option:exclusive missing group=")
	errExpectBgpMissingHex              = errors.New("expect:bgp missing hex=")
	errExpectJsonMissingJson            = errors.New("expect:json missing json=")
	errExpectExitMissingCode            = errors.New("expect:exit missing code=")
	errExpectFileMissingTarget          = errors.New("expect:file missing path= or glob=")
	errActionSendMissingHex             = errors.New("action:send missing hex=")
	errActionRewriteMissingSource       = errors.New("action:rewrite missing source=")
	errActionRewriteMissingDest         = errors.New("action:rewrite missing dest=")
	errHttpMissingSeq                   = errors.New("http= missing seq=")
	errHttpMissingUrl                   = errors.New("http= missing url=")
	errHttpMissingStatus                = errors.New("http= missing status=")
	errMissingConn                      = errors.New("missing conn=")
	errMissingSeq                       = errors.New("missing seq=")
)

// EncodingTests manages encoding test discovery.
type EncodingTests struct {
	*Tests
	baseDir string
	port    int
}

// NewEncodingTests creates an encoding test manager.
func NewEncodingTests(baseDir string) *EncodingTests {
	return &EncodingTests{
		Tests:   NewTests(),
		baseDir: baseDir,
		port:    1790,
	}
}

// Discover finds all .ci files in the directory.
func (et *EncodingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	slices.Sort(files)

	for _, ciFile := range files {
		// Skip-and-warn on a parse error rather than aborting discovery: one
		// unparseable .ci file must not hide every other test in the directory
		// (it did, suite-wide, before this — see the §1 handover note). The bad
		// file is still recorded as a permanent failure so it fails the suite
		// loudly instead of silently vanishing.
		rec, err := et.parseAndAdd(ciFile)
		if err != nil {
			recordLogger().Warn("unparseable .ci file recorded as failure; continuing discovery",
				"file", filepath.Base(ciFile), "error", err)
			if rec == nil {
				// parseAndAdd failed before a record existed (e.g. tmpfs read
				// error). Create a placeholder so the file still appears in the
				// suite as a failure.
				rec = et.Add(strings.TrimSuffix(filepath.Base(ciFile), ".ci"))
				rec.CIFile = ciFile
				rec.Files = append(rec.Files, ciFile)
			}
			rec.ParseFailed = true
			rec.SetState(StateFail)
			rec.FailureType = failParseError
			rec.Error = err
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test. It returns the record it
// created so a caller can mark it failed on error (the record is added to the
// collection as soon as the file name is known, before line parsing). The
// record is nil only when parsing failed before any record was created.
// Uses new key=value format: action=type:key=value:key=value:...
// Supports Tmpfs blocks for embedded files.
func (et *EncodingTests) parseAndAdd(ciFile string) (*Record, error) {
	// First, try Tmpfs parsing to extract embedded files
	v, err := tmpfs.ReadFrom(ciFile)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ciFile, err)
	}

	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := et.Add(name)
	// The per-test port PREFERENCE, not the port the test binds. Discovery walks
	// the directory in sorted order, so the Nth test of every suite gets the same
	// number here, and it is the number the failure report and the "run this test
	// alone" hint quote.
	//
	// Runner.runTest leases the real pair from this preference when the test
	// starts (LeaseTestPorts). Nothing between discovery and that lease may
	// assume the preference is free: discovery can run minutes ahead of the test,
	// and a second le test process on the same machine numbers its own suite from
	// the same base.
	r.Port = et.port
	et.port += TestPortSpan
	r.CIFile = ciFile
	r.Files = append(r.Files, ciFile)

	// Store Tmpfs files if any
	if len(v.Files) > 0 {
		r.TmpfsFiles = make(map[string]tmpfs.File, len(v.Files))
		for _, f := range v.Files {
			// The whole file, not its bytes. Flattening here is what dropped a
			// declared `mode=<octal>`, because the orchestrated path rebuilds a
			// Tmpfs from this map and had nothing but the path to derive a mode
			// from (runner_exec.go).
			r.TmpfsFiles[f.Path] = *f
		}
	}

	// Store stdin blocks if any
	if len(v.StdinBlocks) > 0 {
		r.StdinBlocks = v.StdinBlocks
		for name, content := range v.StdinBlocks {
			recordLogger().Debug("stdin block loaded", "name", name, "size", len(content), "preview", string(content[:min(100, len(content))]))
		}

	}

	// Parse the non-Tmpfs lines (option:, expect:, cmd:, run=, etc.). The number
	// quoted on a refusal is the line's own number in the file, which is why
	// tmpfs.Line carries it: the blocks and comments this slice dropped are not
	// counted, so its index names a different line.
	for _, line := range v.OtherLines {
		if err := et.parseLine(r, ciFile, line.Text); err != nil {
			return r, fmt.Errorf("line %d: %w", line.Num, err)
		}
	}

	// A peer block that states no AS of its own gets the one ze is configured to
	// expect from it, derived from the configuration this same .ci embeds. Runs
	// before the two guards below so the derived lines are held to the same
	// contract every hand-written line is. See peer_asn.go.
	if err := declarePeerAS(r); err != nil {
		return r, err
	}

	// Every line of every ze-peer stdin block is now claimed by ze-peer or
	// parsed by the runner, and a line neither reads fails the file. Runs after
	// the cmd= lines because the set of blocks that reach a ze-peer is derived
	// from them. See peer_contract.go.
	if err := et.validatePeerBlockDirectives(r, ciFile); err != nil {
		return r, err
	}

	// Reject a check-mode ze-peer that has nothing to check: it exits before
	// binding, so ze dials a dead port for the whole test and nothing about BGP
	// is ever proven. Runs after the cmd= lines are parsed because the guard
	// needs each peer's mode and stdin block. See peer_contract.go.
	if err := validatePeerBlocks(r); err != nil {
		return r, err
	}

	// Verify config exists (for non-Tmpfs configs)
	if configPath, ok := r.Conf["config"].(string); ok {
		// Check if it's a Tmpfs file first
		if r.TmpfsFiles != nil {
			if _, isTmpfs := r.TmpfsFiles[filepath.Base(configPath)]; isTmpfs {
				// Config is in Tmpfs, will be written to temp dir at runtime
				goto generateDecoded
			}
		}
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return r, fmt.Errorf("config not found: %s", configPath)
		}
	}

generateDecoded:
	// Generate decoded strings for messages with Raw
	for i := range r.Messages {
		if len(r.Messages[i].Raw) > 0 {
			if decoded, err := DecodeMessageBytes(r.Messages[i].Raw); err == nil {
				r.Messages[i].Decoded = decoded.String()
			}
		}
	}

	// option=netns-link declares a prerequisite the runner can only satisfy under
	// the per-test netns launch mode, so a test that declared one is skipped here
	// when that mode is off (see applyNetnsLinkGate for why). Applied after all
	// options are parsed so it does not depend on their order in the .ci file.
	applyNetnsLinkGate(r)

	// ZE_QEMU_LINUX_ONLY mode (the `./le test qemu all-tests` tight loop) runs
	// ONLY tests marked option=needs-linux: every other test is skipped so the
	// QEMU VM spends its time on the Linux-only surface, not re-running tests
	// that already pass natively. Applied after all options are parsed so the
	// needs-linux flag is final. Never overrides an existing skip reason.
	if r.SkipReason == "" && !r.NeedsLinux && os.Getenv("ZE_QEMU_LINUX_ONLY") == "1" {
		r.SkipReason = "ZE_QEMU_LINUX_ONLY (not option=needs-linux)"
	}

	return r, nil
}

// parseLine parses a single .ci line in the action=type:key=value format.
func (et *EncodingTests) parseLine(r *Record, ciFile, line string) error {
	// expect=output / expect=stream carry a contains= needle that may itself
	// hold ':' (e.g. a compact-JSON fragment "rekey-count":1). The generic ':'
	// splitter below would truncate it at the first colon, so parse these
	// colon-preserving with the trailing ':timeout=' as a suffix, mirroring how
	// command=/stream= keep their raw remainder (parseEngineCmd).
	if rest, ok := strings.CutPrefix(line, "expect=output:"); ok {
		return parseEngineExpectContains(r, "output", rest)
	}
	if rest, ok := strings.CutPrefix(line, "expect=stream:"); ok {
		return parseEngineExpectContains(r, engineActionStream, rest)
	}
	if rest, ok := strings.CutPrefix(line, "expect=command-error:"); ok {
		return parseEngineExpectCommandError(r, rest)
	}

	// Parse action=type:key=value:key=value:...
	// First segment is action=type, remaining segments are key=value pairs
	parts := strings.Split(line, ":")
	if len(parts) < 1 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}

	// First segment is action=type
	actionType := strings.SplitN(parts[0], "=", 2)
	if len(actionType) != 2 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}
	action := actionType[0]
	lineType := actionType[1]
	if !slices.Contains(recordActions, action) {
		return unknownDirective("action", action, recordActions)
	}
	// Only the key=value families split the tail. http=, cmd= program lines and
	// the engine steps read the raw line, whose command text may hold a colon
	// and a bare word the strict splitter would refuse.
	// The type is checked before the split, so option=bogus:65000 names the
	// unknown type and the accepted list, not the unkeyed segment.
	kvPairs := func() (map[string]string, error) {
		if err := checkDirectiveType(action, lineType); err != nil {
			return nil, err
		}
		return parseDirectiveKV(parts[0], parts[1:], familyKeys(action, lineType))
	}

	switch action {
	case directiveOption:
		kv, err := kvPairs()
		if err != nil {
			return err
		}
		return et.parseOption(r, ciFile, lineType, kv)
	case directiveExpect:
		kv, err := kvPairs()
		if err != nil {
			return err
		}
		return et.parseExpect(r, lineType, kv)
	case directiveReject:
		kv, err := kvPairs()
		if err != nil {
			return err
		}
		return et.parseReject(r, lineType, kv)
	case directiveAction:
		kv, err := kvPairs()
		if err != nil {
			return err
		}
		return et.parseAction(r, lineType, kv)
	case directiveCmd:
		return et.parseCmd(r, lineType, parts, line)
	case directiveAwait:
		kv, err := kvPairs()
		if err != nil {
			return err
		}
		return et.parseAwait(r, lineType, kv)
	case "http":
		return et.parseHTTP(r, lineType, line)
	case engineActionCommand, engineActionStream:
		// Engine steps keep the full raw remainder (colons included), so the
		// generic key=value splitter must not be applied (engine_steps.go).
		return parseEngineCmd(r, action, line)
	default:
		return errDirectiveUnlisted("action", action)
	}
}

// parseExpect handles expect:type:... lines.
func (et *EncodingTests) parseExpect(r *Record, expType string, kv map[string]string) error {
	// The gate refuses with recordExpectTypes, not the arms alone: the three raw
	// spellings parseLine intercepts are writable, so an author who mistyped one
	// must see it in the accepted set.
	if err := checkDirectiveType(directiveExpect, expType); err != nil {
		return err
	}
	switch expType {
	case directiveTypeBGP:
		if err := checkKeys("expect=bgp", kv, keyConn, keySeq, "hex"); err != nil {
			return err
		}
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:bgp: %w", err)
		}
		hexData := kv["hex"]
		if hexData == "" {
			return errExpectBgpMissingHex
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.RawHex = strings.ReplaceAll(hexData, ":", "")
		if rawBytes, err := hex.DecodeString(msg.RawHex); err == nil {
			msg.Raw = rawBytes
		}
		// Add to Expects for testpeer (new format).
		var eb textbuf.Buffer
		r.Expects = append(r.Expects, eb.Reset().Str("expect=bgp:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).Str(":hex=").Str(hexData).String())

	case "json":
		if err := checkKeys("expect=json", kv, keyConn, keySeq, "json"); err != nil {
			return err
		}
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:json: %w", err)
		}
		jsonData := kv["json"]
		if jsonData == "" {
			return errExpectJsonMissingJson
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.JSON = jsonData

	case "exit":
		if err := checkKeys("expect=exit", kv, "code"); err != nil {
			return err
		}
		codeStr := kv["code"]
		if codeStr == "" {
			return errExpectExitMissingCode
		}
		code, err := strconv.Atoi(codeStr)
		if err != nil {
			return fmt.Errorf("expect:exit invalid code=%q: %w", codeStr, err)
		}
		r.ExpectExitCode = &code

	case directiveTypeStderr:
		// Support both pattern= (regex) and contains= (substring). Stream
		// NON-containment is reject=stderr:, never a negative key here
		// (record_parse_keys.go).
		if err := checkKeys("expect=stderr", kv, "pattern", "contains"); err != nil {
			return err
		}
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("expect=stderr:pattern= must not be empty (an empty regex matches everything)")
			}
			r.ExpectStderr = append(r.ExpectStderr, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			target, err := r.assertionTarget("expect=stderr:contains=")
			if err != nil {
				return err
			}
			target.ExpectStderrHas = append(target.ExpectStderrHas, contains)
		}

	case directiveTypeStdout:
		// Stream non-containment is reject=stdout:contains=, never a negative
		// key here (record_parse_keys.go).
		if err := checkKeys("expect=stdout", kv, "pattern", "contains"); err != nil {
			return err
		}
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("expect=stdout:pattern= must not be empty (an empty regex matches everything)")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid expect=stdout pattern %q: %w", pattern, err)
			}
			target, err := r.assertionTarget("expect=stdout:pattern=")
			if err != nil {
				return err
			}
			target.ExpectStdoutRe = append(target.ExpectStdoutRe, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			target, err := r.assertionTarget("expect=stdout:contains=")
			if err != nil {
				return err
			}
			target.ExpectStdout = append(target.ExpectStdout, contains)
		}

	case directiveTypeSyslog:
		if err := checkKeys("expect=syslog", kv, "pattern"); err != nil {
			return err
		}
		pattern := kv["pattern"]
		if pattern == "" {
			return errors.New("expect=syslog:pattern= must not be empty (an empty regex matches everything)")
		}
		r.ExpectSyslog = append(r.ExpectSyslog, pattern)

	case directiveTypeFile, directiveTypeKey:
		if err := checkKeys("expect="+expType, kv,
			"path", "glob", "contains", "not-contains", "exists", "absent", "count"); err != nil {
			return err
		}
		check, err := parseFileCheck(kv)
		if err != nil {
			return err
		}
		check.Key = expType == directiveTypeKey
		r.FileChecks = append(r.FileChecks, check)

	case "event":
		// expect=output / expect=stream are intercepted in parseLine before the
		// generic ':' split (their contains= needle may hold ':'); only event
		// reaches here.
		if err := checkKeys("expect=event", kv, "namespace", "name", "timeout"); err != nil {
			return err
		}
		return parseEngineExpectEvent(r, kv)

	default:
		return errDirectiveUnlisted("expect type", expType)
	}
	return nil
}

func parseFileCheck(kv map[string]string) (fileCheck, error) {
	check := fileCheck{
		Path:        kv["path"],
		Glob:        kv["glob"],
		Contains:    kv["contains"],
		NotContains: kv["not-contains"],
		Exists:      isTruthy(kv["exists"]),
		Absent:      isTruthy(kv["absent"]),
	}
	if check.Path == "" && check.Glob == "" {
		return fileCheck{}, errExpectFileMissingTarget
	}
	if check.Path != "" && check.Glob != "" {
		return fileCheck{}, errors.New("expect:file must use only one of path= or glob=")
	}
	if countText := kv["count"]; countText != "" {
		count, err := strconv.Atoi(countText)
		if err != nil || count < 0 {
			return fileCheck{}, fmt.Errorf("expect:file invalid count=%q", countText)
		}
		check.Count = &count
	}
	return check, nil
}

func isTruthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// parseReject handles reject:type:... lines.
func (et *EncodingTests) parseReject(r *Record, rejType string, kv map[string]string) error {
	if err := checkDirectiveType(directiveReject, rejType); err != nil {
		return err
	}
	switch rejType {
	case directiveTypeStderr:
		// pattern= is a regex over the daemon's stderr alone (validateLogging).
		// contains= is a substring over the same accumulated stdout+stderr
		// buffer its expect=stderr:contains= mirror reads, so the two answer one
		// question with opposite polarity (runner_exec.go).
		if err := checkKeys("reject=stderr", kv, "pattern", "contains"); err != nil {
			return err
		}
		if len(kv) == 0 {
			return errors.New("reject=stderr needs pattern=<regex> or contains=<text>")
		}
		if contains := kv["contains"]; contains != "" {
			target, err := r.assertionTarget("reject=stderr:contains=")
			if err != nil {
				return err
			}
			target.RejectStderrHas = append(target.RejectStderrHas, contains)
		}
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("reject=stderr:pattern= must not be empty (an empty regex matches everything)")
			}
			r.RejectStderr = append(r.RejectStderr, pattern)
		}

	case directiveTypeSyslog:
		if err := checkKeys("reject=syslog", kv, "pattern"); err != nil {
			return err
		}
		pattern := kv["pattern"]
		if pattern == "" {
			return errors.New("reject=syslog:pattern= must not be empty (an empty regex matches everything)")
		}
		r.RejectSyslog = append(r.RejectSyslog, pattern)

	case directiveTypeStdout:
		if err := checkKeys("reject=stdout", kv, "pattern", "contains"); err != nil {
			return err
		}
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("reject=stdout:pattern= must not be empty (an empty regex matches everything)")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid reject=stdout pattern %q: %w", pattern, err)
			}
			target, err := r.assertionTarget("reject=stdout:pattern=")
			if err != nil {
				return err
			}
			target.RejectStdoutRe = append(target.RejectStdoutRe, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			target, err := r.assertionTarget("reject=stdout:contains=")
			if err != nil {
				return err
			}
			target.RejectStdout = append(target.RejectStdout, contains)
		}

	case directiveTypeBGP:
		// Only ze-peer sees the wire, so reject=bgp is the one reject type the
		// runner cannot answer. Named here rather than left to the default arm:
		// the directive is real, and "unknown reject type" would send its author
		// looking for a typo instead of for the peer block it belongs in.
		return errors.New("reject=bgp:conn=N:pattern=<hex> is read by ze-peer, " +
			"so it belongs inside that peer's stdin block, beside its expect=bgp lines")

	default:
		return errDirectiveUnlisted("reject type", rejType)
	}
	return nil
}

// parseAction handles action=type:... lines, through recordActionArms
// (record_parse_vocabulary.go): an unknown type and an unknown key are both
// refused before any arm runs.
func (et *EncodingTests) parseAction(r *Record, actType string, kv map[string]string) error {
	if err := checkDirectiveType(directiveAction, actType); err != nil {
		return err
	}
	arm := recordActionArms[actType]
	if err := checkKeys("action="+actType, kv, arm.keys...); err != nil {
		return err
	}
	return arm.parse(r, kv)
}

// parseActionNotification reads action=notification.
func parseActionNotification(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("action:notification: %w", err)
	}
	text := kv["text"]
	// Add to Expects for testpeer (new format).
	var eb textbuf.Buffer
	r.Expects = append(r.Expects, eb.Reset().Str("action=notification:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).Str(":text=").Str(text).String())
	return nil
}

// parseActionSend reads action=send.
func parseActionSend(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("action:send: %w", err)
	}
	hexData := kv["hex"]
	if hexData == "" {
		return errActionSendMissingHex
	}
	// Add to Expects for testpeer (new format).
	var eb textbuf.Buffer
	r.Expects = append(r.Expects, eb.Reset().Str("action=send:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).Str(":hex=").Str(hexData).String())
	return nil
}

// parseActionRewrite reads action=rewrite.
func parseActionRewrite(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("action:rewrite: %w", err)
	}
	source := kv["source"]
	if source == "" {
		return errActionRewriteMissingSource
	}
	dest := kv["dest"]
	if dest == "" {
		return errActionRewriteMissingDest
	}
	// Add to Expects for testpeer (new format).
	var eb textbuf.Buffer
	r.Expects = append(r.Expects, eb.Reset().Str("action=rewrite:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).Str(":source=").Str(source).Str(":dest=").Str(dest).String())
	return nil
}

// parseActionSighup reads action=sighup.
func parseActionSighup(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("action:sighup: %w", err)
	}
	// Add to Expects for testpeer (new format).
	var eb textbuf.Buffer
	r.Expects = append(r.Expects, eb.Reset().Str("action=sighup:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).String())
	return nil
}

// parseActionSigterm reads action=sigterm.
func parseActionSigterm(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("action:sigterm: %w", err)
	}
	// Add to Expects for testpeer (new format).
	var eb textbuf.Buffer
	r.Expects = append(r.Expects, eb.Reset().Str("action=sigterm:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).String())
	return nil
}

// parseCmd handles cmd=type:... lines. cmd=api reads a key=value map through
// recordCmdArms; the program arms read the raw line by marker, because an exec=
// value may hold a colon, and check their own keys (record_parse_cmd.go).
func (et *EncodingTests) parseCmd(r *Record, cmdType string, parts []string, rawLine string) error {
	if err := checkDirectiveType(directiveCmd, cmdType); err != nil {
		return err
	}
	if arm, ok := recordCmdArms[cmdType]; ok {
		kv, err := parseDirectiveKV(parts[0], parts[1:], arm.keys)
		if err != nil {
			return err
		}
		if err := checkKeys("cmd="+cmdType, kv, arm.keys...); err != nil {
			return err
		}
		return arm.parse(r, kv)
	}
	switch cmdType {
	case "background", "foreground":
		// Use marker-based parsing because exec= values may contain colons
		// (e.g., --web :8000). The standard KV parser splits on colons and
		// would truncate the exec value.
		rc, err := parseCmdExec(cmdType, rawLine)
		if err != nil {
			return err
		}
		r.RunCommands = append(r.RunCommands, rc)

	case "stop":
		// cmd=stop:seq=N:name=NAME[:signal=kill|term] terminates a named
		// background process mid-test. Marker-based like parseCmdExec so it
		// orders relative to other cmd= steps via seq=.
		rc, err := parseCmdStop(rawLine)
		if err != nil {
			return err
		}
		r.RunCommands = append(r.RunCommands, rc)

	default:
		return errDirectiveUnlisted("cmd type", cmdType)
	}
	return nil
}

// parseCmdAPI reads cmd=api: the API command text sent with message conn/seq.
func parseCmdAPI(r *Record, kv map[string]string) error {
	conn, seq, err := parseConnSeq(kv)
	if err != nil {
		return fmt.Errorf("cmd:api: %w", err)
	}
	msg := r.getOrCreateMessage(connSeqToIndex(conn, seq))
	msg.Cmd = kv["text"]
	return nil
}

// assertionTarget answers the command a stream assertion belongs to: the cmd=
// line above it in the file.
//
// Position is the whole grammar here, so a line with nothing above it, or with
// a cmd=stop above it, names no output and is refused at parse time rather than
// checked against something its author did not write. The parse suite has
// always worked this way (parsing.go, ciCommand); the generic runner did not,
// and an assertion it could not place was checked against the whole test's
// accumulated output instead -- which is how test/plugin/kernel-capability-
// unknown-starts.ci came to assert that `ze doctor` printed a diagnostic code
// and then satisfy that assertion with its own `ze show diagnostic <that code>`.
func (r *Record) assertionTarget(directive string) (*RunCommand, error) {
	if len(r.RunCommands) == 0 {
		return nil, fmt.Errorf("%s has no cmd= line above it, so it names no command's output: "+
			"move it below the cmd= line whose output it describes", directive)
	}
	target := &r.RunCommands[len(r.RunCommands)-1]
	if target.Mode == modeStop {
		return nil, fmt.Errorf("%s sits under cmd=stop, which runs no program and writes nothing: "+
			"move it below the cmd= line that produces the output it describes", directive)
	}
	return target, nil
}

// nextMarker returns the index of the earliest occurrence of any marker
// in line starting from offset, or len(line) if none found.
func nextMarker(line string, offset int, markers ...string) int {
	best := len(line)
	for _, m := range markers {
		if idx := strings.Index(line[offset:], m); idx >= 0 {
			if offset+idx < best {
				best = offset + idx
			}
		}
	}
	return best
}

// parseHTTP handles http=method:seq=N:url=URL:status=CODE[:contains=TEXT][:header=NAME: VALUE][:timeout=DUR] lines.
// Uses marker-based parsing (nextMarker) because URLs contain colons that would
// confuse simple colon-splitting. Each marker's value extends to the next known
// marker or end-of-line, so marker order in the input does not matter.
// header= is the only repeatable key. It is scanned for every occurrence, and
// each value is split on its first colon into a field name and value.
// Method "wait" polls until the condition is met (retries on content mismatch).
func (et *EncodingTests) parseHTTP(r *Record, method, line string) error {
	if !slices.Contains(recordHTTPMethods, method) {
		var b textbuf.Buffer
		b.Str("unsupported HTTP method ").Quoted(method).Str(" (accepts ").Join(recordHTTPMethods, ", ").Byte(')')
		return errors.New(b.String())
	}
	isWait := method == "wait"

	seqMarker := ":seq="
	urlMarker := ":url="
	statusMarker := ":status="
	containsMarker := ":contains="
	bodyfileMarker := ":bodyfile="
	sendfileMarker := ":sendfile="
	contentTypeMarker := ":content-type="
	headerMarker := ":header="
	insecureTLSMarker := ":insecure-tls="
	timeoutMarker := ":timeout="

	seqIdx := strings.Index(line, seqMarker)
	urlIdx := strings.Index(line, urlMarker)
	statusIdx := strings.Index(line, statusMarker)
	containsIdx := strings.Index(line, containsMarker)
	bodyfileIdx := strings.Index(line, bodyfileMarker)
	sendfileIdx := strings.Index(line, sendfileMarker)
	contentTypeIdx := strings.Index(line, contentTypeMarker)
	insecureTLSIdx := strings.Index(line, insecureTLSMarker)
	timeoutIdx := strings.Index(line, timeoutMarker)

	if seqIdx < 0 {
		return errHttpMissingSeq
	}
	if urlIdx < 0 {
		return errHttpMissingUrl
	}
	if statusIdx < 0 {
		return errHttpMissingStatus
	}

	// headerMarker is deliberately in this set even though header= has no single
	// index above. It is the one REPEATABLE key, so every other key's value must
	// still end when a header= follows it on the same line.
	allMarkers := []string{seqMarker, urlMarker, statusMarker, containsMarker, bodyfileMarker, sendfileMarker, contentTypeMarker, headerMarker, insecureTLSMarker, timeoutMarker}

	// Each value runs to the next KNOWN marker, so an unknown key would be
	// swallowed into the value before it, and a second seq= or status= would
	// be ignored because only the first is read. header= alone repeats.
	if err := checkMarkerKeys("http="+method, line, allMarkers, headerMarker); err != nil {
		return err
	}

	// Extract seq value: from after ":seq=" to next known marker or end.
	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, allMarkers...)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return fmt.Errorf("http= invalid seq=%q", seqStr)
	}

	// Extract url value: from after ":url=" to next known marker or end.
	urlStart := urlIdx + len(urlMarker)
	urlEnd := nextMarker(line, urlStart, allMarkers...)
	url := line[urlStart:urlEnd]

	// Extract status value: from after ":status=" to next known marker or end.
	statusStart := statusIdx + len(statusMarker)
	statusEnd := nextMarker(line, statusStart, allMarkers...)
	statusStr := line[statusStart:statusEnd]
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		return fmt.Errorf("http= invalid status=%q", statusStr)
	}

	// Extract optional contains value.
	var contains string
	if containsIdx >= 0 {
		containsStart := containsIdx + len(containsMarker)
		containsEnd := nextMarker(line, containsStart, allMarkers...)
		contains = line[containsStart:containsEnd]
	}

	// Extract optional bodyfile value (path to expected body content).
	var bodyfile string
	if bodyfileIdx >= 0 {
		bodyfileStart := bodyfileIdx + len(bodyfileMarker)
		bodyfileEnd := nextMarker(line, bodyfileStart, allMarkers...)
		bodyfile = line[bodyfileStart:bodyfileEnd]
	}

	// Extract optional sendfile value (path to request body for POST).
	var sendfile string
	if sendfileIdx >= 0 {
		sendfileStart := sendfileIdx + len(sendfileMarker)
		sendfileEnd := nextMarker(line, sendfileStart, allMarkers...)
		sendfile = line[sendfileStart:sendfileEnd]
	}

	// Extract optional content-type value for sendfile POST bodies.
	var contentType string
	if contentTypeIdx >= 0 {
		contentTypeStart := contentTypeIdx + len(contentTypeMarker)
		contentTypeEnd := nextMarker(line, contentTypeStart, allMarkers...)
		contentType = line[contentTypeStart:contentTypeEnd]
	}

	// Extract optional request headers. Unlike every other key, header= is
	// repeatable, so scan for ALL occurrences instead of a single strings.Index.
	// Each value runs to the next known marker (including the next header=), so a
	// header value can itself contain colons. "Referer: http://host:8080/" is one
	// header, split on its FIRST colon only.
	var headers []httpHeader
	for from := 0; ; {
		idx := strings.Index(line[from:], headerMarker)
		if idx < 0 {
			break
		}
		start := from + idx + len(headerMarker)
		end := nextMarker(line, start, allMarkers...)
		raw := line[start:end]
		name, value, found := strings.Cut(raw, ":")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return fmt.Errorf("http= invalid header=%q (want header=Name: Value)", raw)
		}
		headers = append(headers, httpHeader{Name: name, Value: strings.TrimSpace(value)})
		from = end
	}

	// Extract optional insecure TLS flag for self-signed HTTPS test servers.
	var insecureTLS bool
	if insecureTLSIdx >= 0 {
		insecureTLSStart := insecureTLSIdx + len(insecureTLSMarker)
		insecureTLSEnd := nextMarker(line, insecureTLSStart, allMarkers...)
		insecureTLSStr := line[insecureTLSStart:insecureTLSEnd]
		insecureTLS, err = strconv.ParseBool(insecureTLSStr)
		if err != nil {
			return fmt.Errorf("http= invalid insecure-tls=%q", insecureTLSStr)
		}
	}

	// Extract optional timeout value (wait only).
	var timeout string
	if timeoutIdx >= 0 {
		timeoutStart := timeoutIdx + len(timeoutMarker)
		timeoutEnd := nextMarker(line, timeoutStart, allMarkers...)
		timeout = line[timeoutStart:timeoutEnd]
		if _, parseErr := time.ParseDuration(timeout); parseErr != nil {
			return fmt.Errorf("http= invalid timeout=%q", timeout)
		}
	}

	chk := httpCheck{
		Seq:         seq,
		Method:      method,
		URL:         url,
		Status:      status,
		Contains:    contains,
		BodyFile:    bodyfile,
		SendFile:    sendfile,
		ContentType: contentType,
		Headers:     headers,
		InsecureTLS: insecureTLS,
		Timeout:     timeout,
	}
	if isWait {
		chk.Method = "get" // wait polls with GET
		r.HTTPWaits = append(r.HTTPWaits, chk)
	} else {
		r.HTTPChecks = append(r.HTTPChecks, chk)
	}
	return nil
}

// parseConnSeq extracts conn and seq from key-value pairs.
// Validates: conn and seq must both be >= 1.
func parseConnSeq(kv map[string]string) (conn, seq int, err error) {
	connStr := kv["conn"]
	seqStr := kv["seq"]

	if connStr == "" {
		return 0, 0, errMissingConn
	}
	if seqStr == "" {
		return 0, 0, errMissingSeq
	}

	conn, err = strconv.Atoi(connStr)
	if err != nil || conn < 1 {
		return 0, 0, fmt.Errorf("invalid conn=%q (must be >= 1)", connStr)
	}
	seq, err = strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return 0, 0, fmt.Errorf("invalid seq=%q (must be >= 1)", seqStr)
	}

	return conn, seq, nil
}

// connSeqToIndex converts conn+seq to a unique message index.
// conn=1:seq=1 -> 101, conn=1:seq=2 -> 102, conn=2:seq=1 -> 201, etc.
func connSeqToIndex(conn, seq int) int {
	return conn*100 + seq
}
