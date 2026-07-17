// Design: docs/architecture/testing/ci-format.md — CI file parsing and test discovery
// Overview: record.go — Record type definitions and methods
// Related: record_collection.go — Tests container and querying

package runner

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/test/ci"
	"codeberg.org/thomas-mangin/ze/internal/test/tmpfs"
)

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
	errOptionRequireTagMissingValue     = errors.New("option:require-tag missing value=")
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

	sort.Strings(files)

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
			rec.State = StateFail
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
	// Default per-test port assignment. This is the live allocator for the shared
	// ci_runner.go path: the registerCIRoot suites (ldp, rsvpte, static, policy,
	// traffic, ui, isis, l2tp, firewall, ...; see internal/test/cli/register.go)
	// neither reserve nor rebase, so they consume Record.Port as set here (via
	// $PORT/$PORT2 substitution and the BGP peer --port in runner_exec.go). The
	// bgp suite (cmd_bgp.go, including its parse/plugin modes) and the vpp suite
	// (cmd_vpp.go) instead OVERRIDE every Record.Port from a concurrency-safe
	// range (they call runner.ReservePorts, then reassign rr.Port) because they
	// bind real ze and BGP-peer ports and must not collide across parallel
	// ze-test processes. So this assignment is load-bearing, not vestigial; do
	// not remove it.
	r.Port = et.port
	et.port += 2 // 2 ports per test ($PORT and $PORT2)
	r.CIFile = ciFile
	r.Files = append(r.Files, ciFile)

	// Store Tmpfs files if any
	if len(v.Files) > 0 {
		r.TmpfsFiles = make(map[string][]byte)
		for _, f := range v.Files {
			r.TmpfsFiles[f.Path] = f.Content
		}
	}

	// Store stdin blocks if any
	if len(v.StdinBlocks) > 0 {
		r.StdinBlocks = v.StdinBlocks
		for name, content := range v.StdinBlocks {
			recordLogger().Debug("stdin block loaded", "name", name, "size", len(content), "preview", string(content[:min(100, len(content))]))
		}

		// Also parse "peer" stdin block for expectations (for reporting purposes).
		// The peer block content is passed to ze-peer which parses it, but the
		// test runner also needs to know about expectations for progress/failure reporting.
		//
		// The block also contains ze-peer-consumed directives like
		// option=timeout, option=open, option=update, option=tcp_connections —
		// those are pass-through and must NOT be rejected here.
		//
		// option=env, however, is consumed by the test runner (it seeds proc.Env
		// when spawning ze/ze-peer/helpers), NOT by ze-peer. Placing it inside
		// the peer block means it is silently dropped and the target process
		// never sees it. See plan/learned/545-debug-plugin-test-cluster.md.
		// Reject it with a hard error naming the directive so the author can
		// move it outside the block.
		if peerBlock, ok := v.StdinBlocks["peer"]; ok {
			blockLine := 0
			for line := range strings.SplitSeq(string(peerBlock), "\n") {
				blockLine++
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				if strings.HasPrefix(trimmed, "option=env:") {
					return r, fmt.Errorf("stdin=peer block line %d: %q is consumed by the test runner, not ze-peer, "+
						"so placing it inside a stdin=peer block silently drops it. "+
						"Move it outside (above) the stdin=peer:terminator=... header. "+
						"See plan/learned/545-debug-plugin-test-cluster.md",
						blockLine, trimmed)
				}
				// Parse expect= and action= lines for reporting purposes
				if strings.HasPrefix(trimmed, "expect=") || strings.HasPrefix(trimmed, "action=") {
					if err := et.parseLine(r, ciFile, trimmed); err != nil {
						// Log but don't fail - these are primarily for ze-peer
						recordLogger().Debug("parsing peer block line", "line", trimmed, "error", err)
					}
				}
			}
		}
	}

	// Parse the non-Tmpfs lines (option:, expect:, cmd:, run=, etc.)
	for lineNum, line := range v.OtherLines {
		if err := et.parseLine(r, ciFile, line); err != nil {
			return r, fmt.Errorf("line %d: %w", lineNum+1, err)
		}
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

	// ZE_QEMU_LINUX_ONLY mode (the `ze-qemu-needs-linux-test` tight loop) runs
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
	kvPairs := ci.ParseKVPairs(parts[1:])

	switch action {
	case "option":
		return et.parseOption(r, ciFile, lineType, kvPairs)
	case "expect":
		return et.parseExpect(r, lineType, kvPairs)
	case "reject":
		return et.parseReject(r, lineType, kvPairs)
	case "action":
		return et.parseAction(r, lineType, kvPairs)
	case "cmd":
		return et.parseCmd(r, lineType, kvPairs, line)
	case "await":
		return et.parseAwait(r, lineType, kvPairs)
	case "http":
		return et.parseHTTP(r, lineType, line)
	case engineActionCommand, engineActionStream:
		// Engine steps keep the full raw remainder (colons included), so the
		// generic key=value splitter must not be applied (engine_steps.go).
		return parseEngineCmd(r, action, line)
	default:
		return fmt.Errorf("unknown action %q in %q", action, line)
	}
}

// parseOption handles option=type:key=value lines.
func (et *EncodingTests) parseOption(r *Record, ciFile, optType string, kv map[string]string) error {
	switch optType {
	case "file":
		configName := kv["path"]
		if configName == "" {
			return errOptionFileMissingPath
		}
		configPath := filepath.Join(filepath.Dir(ciFile), configName)
		absConfig, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
		absTestDir, err := filepath.Abs(filepath.Dir(ciFile))
		if err != nil {
			return fmt.Errorf("invalid test dir: %w", err)
		}
		if !strings.HasPrefix(absConfig, absTestDir+string(filepath.Separator)) && absConfig != absTestDir {
			return fmt.Errorf("config file outside test directory: %s", configName)
		}
		r.Conf["config"] = configPath
		r.ConfigFile = configPath
		r.Files = append(r.Files, configPath)

	case "asn":
		value := kv["value"]
		if value == "" {
			return errOptionAsnMissingValue
		}
		r.Extra["asn"] = value
		r.Options = append(r.Options, "option=asn:value="+value)

	case "bind":
		value := kv["value"]
		if value == "" {
			return errOptionBindMissingValue
		}
		r.Extra["bind"] = value
		r.Options = append(r.Options, "option=bind:value="+value)

	case "timeout":
		value := kv["value"]
		if value == "" {
			return errOptionTimeoutMissingValue
		}
		r.Extra["timeout"] = value

	case "tcp_connections":
		value := kv["value"]
		if value == "" {
			return errOptionTcpConnectionsMissingValue
		}
		r.Options = append(r.Options, "option=tcp_connections:value="+value)

	case "open":
		value := kv["value"]
		if value == "" {
			return errOptionOpenMissingValue
		}
		r.Options = append(r.Options, "option=open:value="+value)

	case "update":
		value := kv["value"]
		if value == "" {
			return errOptionUpdateMissingValue
		}
		r.Options = append(r.Options, "option=update:value="+value)

	case "env":
		varName := kv["var"]
		value := kv["value"]
		if varName == "" {
			return errOptionEnvMissingVar
		}
		// Store as KEY=VALUE for environment setting
		r.EnvVars = append(r.EnvVars, varName+"="+value)

	case "skip-os":
		value := kv["value"]
		if value == "" {
			return errOptionSkipOsMissingValue
		}
		// Record a skip reason when the current GOOS is in the skip list.
		// The .ci format has no build tags, so OS-specific features (e.g.
		// Linux-only IP_RECVTTL for BFD echo) gate at parse time. The runner
		// honors SkipReason regardless of whether the user selected the test
		// by name -- asking for an unsupported test on the wrong OS reports
		// SKIP (with reason), never FAIL. Multiple skip-os options
		// accumulate; any match skips.
		for skipOS := range strings.SplitSeq(value, ",") {
			if strings.TrimSpace(skipOS) == runtime.GOOS {
				var tb textbuf.Buffer
				r.SkipReason = tb.Str("skip-os=").Str(value).Str(" (current GOOS=").Str(runtime.GOOS).Byte(')').String()
				return nil
			}
		}

	case "needs-linux":
		// Marks a .ci test that requires a real Linux kernel (netlink interface
		// management, nftables, kernel sockets, ...) and therefore cannot pass
		// natively on a non-Linux host. On such a host the test is SKIPPED with
		// a reason pointing at the QEMU runner; inside the QEMU Alpine VM
		// (GOOS=linux, via `make ze-qemu-needs-linux-test`) the directive is
		// inert and the test runs normally. This is how Linux-only functional
		// tests are validated automatically via QEMU instead of failing
		// natively. See ai/rules/qemu-testing.md "Linux-only functional tests".
		r.NeedsLinux = true
		if runtime.GOOS != "linux" {
			var tb textbuf.Buffer
			r.SkipReason = tb.Str("needs-linux (run via make ze-qemu-needs-linux-test; current GOOS=").Str(runtime.GOOS).Byte(')').String()
			return nil
		}

	case "skip-env":
		varName := kv["var"]
		if varName == "" {
			return errOptionSkipEnvMissingVar
		}
		expected := kv["value"]
		actual := os.Getenv(varName)
		var tb textbuf.Buffer
		if expected == "" {
			if actual != "" {
				r.SkipReason = tb.Str("skip-env=").Str(varName).Str(" (set to ").Str(actual).Byte(')').String()
				return nil
			}
		} else if actual == expected {
			r.SkipReason = tb.Reset().Str("skip-env=").Str(varName).Byte('=').Str(expected).String()
			return nil
		}

	case "require-tag":
		value := kv["value"]
		if value == "" {
			return errOptionRequireTagMissingValue
		}
		active := TestBuildTags()
		found := false
		for tag := range strings.SplitSeq(active, ",") {
			if strings.TrimSpace(tag) == value {
				found = true
				break
			}
		}
		if !found {
			var tb textbuf.Buffer
			r.SkipReason = tb.Str("require-tag=").Str(value).Str(" (not in build tags: ").Str(active).Byte(')').String()
			return nil
		}

	default:
		return fmt.Errorf("unknown option type %q", optType)
	}
	return nil
}

// parseExpect handles expect:type:... lines.
func (et *EncodingTests) parseExpect(r *Record, expType string, kv map[string]string) error {
	switch expType {
	case "bgp":
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
		codeStr := kv["code"]
		if codeStr == "" {
			return errExpectExitMissingCode
		}
		code, err := strconv.Atoi(codeStr)
		if err != nil {
			return fmt.Errorf("expect:exit invalid code=%q: %w", codeStr, err)
		}
		r.ExpectExitCode = &code

	case "stderr":
		// Support both pattern= (regex) and contains= (substring)
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("expect=stderr:pattern= must not be empty (an empty regex matches everything)")
			}
			r.ExpectStderr = append(r.ExpectStderr, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			r.ExpectStderrMatch = append(r.ExpectStderrMatch, contains)
		}

	case "stdout":
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("expect=stdout:pattern= must not be empty (an empty regex matches everything)")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid expect=stdout pattern %q: %w", pattern, err)
			}
			r.ExpectStdoutRegex = append(r.ExpectStdoutRegex, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			r.ExpectStdoutMatch = append(r.ExpectStdoutMatch, contains)
		}
		if notContains := kv["!contains"]; notContains != "" {
			r.ExpectStdoutNotMatch = append(r.ExpectStdoutNotMatch, notContains)
		}

	case "syslog":
		pattern := kv["pattern"]
		if pattern == "" {
			return errors.New("expect=syslog:pattern= must not be empty (an empty regex matches everything)")
		}
		r.ExpectSyslog = append(r.ExpectSyslog, pattern)

	case "file":
		check, err := parseFileCheck(kv)
		if err != nil {
			return err
		}
		r.FileChecks = append(r.FileChecks, check)

	case "event":
		// expect=output / expect=stream are intercepted in parseLine before the
		// generic ':' split (their contains= needle may hold ':'); only event
		// reaches here.
		return parseEngineExpectEvent(r, kv)

	default:
		return fmt.Errorf("unknown expect type %q", expType)
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
	switch rejType {
	case "stderr":
		pattern := kv["pattern"]
		if pattern == "" {
			return errors.New("reject=stderr:pattern= must not be empty (an empty regex matches everything)")
		}
		r.RejectStderr = append(r.RejectStderr, pattern)

	case "syslog":
		pattern := kv["pattern"]
		if pattern == "" {
			return errors.New("reject=syslog:pattern= must not be empty (an empty regex matches everything)")
		}
		r.RejectSyslog = append(r.RejectSyslog, pattern)

	case "stdout":
		if pattern, ok := kv["pattern"]; ok {
			if pattern == "" {
				return errors.New("reject=stdout:pattern= must not be empty (an empty regex matches everything)")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid reject=stdout pattern %q: %w", pattern, err)
			}
			r.RejectStdoutRegex = append(r.RejectStdoutRegex, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			r.ExpectStdoutNotMatch = append(r.ExpectStdoutNotMatch, contains)
		}

	default:
		return fmt.Errorf("unknown reject type %q", rejType)
	}
	return nil
}

// parseAction handles action:type:... lines.
func (et *EncodingTests) parseAction(r *Record, actType string, kv map[string]string) error {
	switch actType {
	case "notification":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:notification: %w", err)
		}
		text := kv["text"]
		// Add to Expects for testpeer (new format).
		var eb textbuf.Buffer
		r.Expects = append(r.Expects, eb.Reset().Str("action=notification:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).Str(":text=").Str(text).String())

	case "send":
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

	case "rewrite":
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

	case "sighup":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:sighup: %w", err)
		}
		// Add to Expects for testpeer (new format).
		var eb textbuf.Buffer
		r.Expects = append(r.Expects, eb.Reset().Str("action=sighup:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).String())

	case "sigterm":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:sigterm: %w", err)
		}
		// Add to Expects for testpeer (new format).
		var eb textbuf.Buffer
		r.Expects = append(r.Expects, eb.Reset().Str("action=sigterm:conn=").Int(int64(conn)).Str(":seq=").Int(int64(seq)).String())

	default:
		return fmt.Errorf("unknown action type %q", actType)
	}
	return nil
}

// parseCmd handles cmd:type:... lines.
func (et *EncodingTests) parseCmd(r *Record, cmdType string, kv map[string]string, rawLine string) error {
	switch cmdType {
	case "api":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("cmd:api: %w", err)
		}
		text := kv["text"]
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.Cmd = text

	case "background", "foreground":
		// Use marker-based parsing because exec= values may contain colons
		// (e.g., --web :8000). The standard KV parser splits on colons and
		// would truncate the exec value.
		rc, err := parseCmdExec(cmdType, rawLine)
		if err != nil {
			return err
		}
		r.RunCommands = append(r.RunCommands, rc)

	default:
		return fmt.Errorf("unknown cmd type %q", cmdType)
	}
	return nil
}

// parseCmdExec extracts fields from a cmd=background/foreground line using
// marker-based parsing. This handles exec= values containing colons correctly.
//
// Format: cmd=background:seq=N:exec=COMMAND[:stdin=BLOCK][:timeout=DUR][:exit=N].
func parseCmdExec(mode, line string) (RunCommand, error) {
	seqMarker := ":seq="
	execMarker := ":exec="
	stdinMarker := ":stdin="
	timeoutMarker := ":timeout="
	exitMarker := ":exit="

	seqIdx := strings.Index(line, seqMarker)
	execIdx := strings.Index(line, execMarker)

	if seqIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing seq=", mode)
	}
	if execIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	// Extract seq value: from after ":seq=" to the next known marker or end.
	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, execMarker, stdinMarker, timeoutMarker, exitMarker)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("cmd:%s invalid seq=%q", mode, seqStr)
	}

	// Extract exec value: from after ":exec=" to the next known marker or end.
	// This correctly preserves colons inside the exec value.
	execStart := execIdx + len(execMarker)
	execEnd := nextMarker(line, execStart, stdinMarker, timeoutMarker, exitMarker)
	execVal := line[execStart:execEnd]
	if execVal == "" {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	rc := RunCommand{
		Mode: mode,
		Seq:  seq,
		Exec: execVal,
	}

	// Extract optional stdin=, timeout= and exit= values.
	if idx := strings.Index(line, stdinMarker); idx >= 0 {
		start := idx + len(stdinMarker)
		end := nextMarker(line, start, timeoutMarker, exitMarker)
		rc.Stdin = line[start:end]
	}
	if idx := strings.Index(line, timeoutMarker); idx >= 0 {
		start := idx + len(timeoutMarker)
		end := nextMarker(line, start, exitMarker)
		rc.Timeout = line[start:end]
	}
	if idx := strings.Index(line, exitMarker); idx >= 0 {
		start := idx + len(exitMarker)
		end := nextMarker(line, start, stdinMarker, timeoutMarker)
		codeStr := line[start:end]
		code, err := strconv.Atoi(codeStr)
		if err != nil || code < 0 || code > 255 {
			return RunCommand{}, fmt.Errorf("cmd:%s invalid exit=%q (want 0..255)", mode, codeStr)
		}
		rc.ExitCode = &code
	}

	return rc, nil
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

// parseHTTP handles http=method:seq=N:url=URL:status=CODE[:contains=TEXT][:timeout=DUR] lines.
// Uses marker-based parsing (nextMarker) because URLs contain colons that would
// confuse simple colon-splitting. Each marker's value extends to the next known
// marker or end-of-line, so marker order in the input does not matter.
// Method "wait" polls until the condition is met (retries on content mismatch).
func (et *EncodingTests) parseHTTP(r *Record, method, line string) error {
	isWait := method == "wait"
	if !isWait && method != "get" && method != "post" {
		return fmt.Errorf("unsupported HTTP method %q (use get, post, or wait)", method)
	}

	seqMarker := ":seq="
	urlMarker := ":url="
	statusMarker := ":status="
	containsMarker := ":contains="
	bodyfileMarker := ":bodyfile="
	sendfileMarker := ":sendfile="
	contentTypeMarker := ":content-type="
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

	allMarkers := []string{seqMarker, urlMarker, statusMarker, containsMarker, bodyfileMarker, sendfileMarker, contentTypeMarker, insecureTLSMarker, timeoutMarker}

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
