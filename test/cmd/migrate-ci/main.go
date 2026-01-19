// migrate-ci converts .ci files from old format to new key=value format.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: migrate-ci <file.ci> [file2.ci ...]\n")
		os.Exit(1)
	}

	for _, path := range os.Args[1:] {
		if err := migrateFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error migrating %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("✓ %s\n", path)
	}
}

func migrateFile(path string) error {
	f, err := os.Open(path) //nolint:gosec // CLI tool with user-provided paths
	if err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer for long JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Track last seq number and current implicit conn.
	// If number reduces, it's a new connection.
	var lastNum int
	implicitConn := 1
	lastSeq := make(map[int]int)

	for scanner.Scan() {
		line := scanner.Text()
		migrated, num, hasLetter := migrateLine(line, implicitConn, lastSeq)
		// Only track implicit conn changes for lines without explicit letter
		if num > 0 && !hasLetter {
			if num < lastNum {
				implicitConn++
			}
			lastNum = num
		}
		lines = append(lines, migrated)
	}
	if err := scanner.Err(); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()

	// Write back
	out, err := os.Create(path) //nolint:gosec // CLI tool with user-provided paths
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}

	return nil
}

// Regex patterns for old format lines (N:raw:, AN:raw:, N:cmd:, etc).
var (
	rawPattern          = regexp.MustCompile(`^([A-Za-z]?)(\d+):raw:(.+)$`)
	jsonPattern         = regexp.MustCompile(`^([A-Za-z]?)(\d+):json:(.+)$`)
	cmdPattern          = regexp.MustCompile(`^([A-Za-z]?)(\d+):cmd:(.+)$`)
	notificationPattern = regexp.MustCompile(`^([A-Za-z]?)(\d+):notification:(.+)$`)
	// ExaBGP-specific JSON patterns (skip these).
	exabgpJSONPattern = regexp.MustCompile(`^([A-Za-z]?)(\d+):jsv[46]:`)
)

// migrateLine returns (migrated line, prefix number, has explicit letter).
func migrateLine(line string, implicitConn int, lastSeq map[int]int) (string, int, bool) {
	trimmed := strings.TrimSpace(line)

	// Skip empty lines and comments.
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return line, 0, false
	}

	// Skip ExaBGP-specific JSON lines (jsv4, jsv6) - comment them out.
	if exabgpJSONPattern.MatchString(trimmed) {
		return "# " + line, 0, false
	}

	// Option migrations.
	switch {
	case strings.HasPrefix(trimmed, "option:file:"):
		value := strings.TrimPrefix(trimmed, "option:file:")
		return "option=file:path=" + value, 0, false

	case strings.HasPrefix(trimmed, "option:asn:"):
		value := strings.TrimPrefix(trimmed, "option:asn:")
		return "option=asn:value=" + value, 0, false

	case strings.HasPrefix(trimmed, "option:bind:"):
		value := strings.TrimPrefix(trimmed, "option:bind:")
		return "option=bind:value=" + value, 0, false

	case strings.HasPrefix(trimmed, "option:timeout:"):
		value := strings.TrimPrefix(trimmed, "option:timeout:")
		return "option=timeout:value=" + value, 0, false

	case strings.HasPrefix(trimmed, "option:tcp_connections:"):
		value := strings.TrimPrefix(trimmed, "option:tcp_connections:")
		return "option=tcp_connections:value=" + value, 0, false

	case strings.HasPrefix(trimmed, "option:env:"):
		envPart := strings.TrimPrefix(trimmed, "option:env:")
		if idx := strings.Index(envPart, "="); idx != -1 {
			varName := envPart[:idx]
			varValue := envPart[idx+1:]
			return fmt.Sprintf("option=env:var=%s:value=%s", varName, varValue), 0, false
		}
		return "option=env:var=" + envPart + ":value=", 0, false

	case trimmed == "option:open:send-unknown-capability":
		return "option=open:value=send-unknown-capability", 0, false
	case trimmed == "option:open:inspect-open-message":
		return "option=open:value=inspect-open-message", 0, false
	case trimmed == "option:open:send-unknown-message":
		return "option=open:value=send-unknown-message", 0, false
	case trimmed == "option:update:send-default-route":
		return "option=update:value=send-default-route", 0, false

	case strings.HasPrefix(trimmed, "expect:stderr:"):
		pattern := strings.TrimPrefix(trimmed, "expect:stderr:")
		return "expect=stderr:pattern=" + pattern, 0, false

	case strings.HasPrefix(trimmed, "expect:syslog:"):
		pattern := strings.TrimPrefix(trimmed, "expect:syslog:")
		return "expect=syslog:pattern=" + pattern, 0, false

	case strings.HasPrefix(trimmed, "reject:stderr:"):
		pattern := strings.TrimPrefix(trimmed, "reject:stderr:")
		return "reject=stderr:pattern=" + pattern, 0, false

	case strings.HasPrefix(trimmed, "reject:syslog:"):
		pattern := strings.TrimPrefix(trimmed, "reject:syslog:")
		return "reject=syslog:pattern=" + pattern, 0, false
	}

	// Raw message pattern: [A-Z]?N:raw:HEX
	if m := rawPattern.FindStringSubmatch(trimmed); m != nil {
		letter := m[1]
		seq, _ := strconv.Atoi(m[2])
		if seq == 0 {
			seq = 1
		}
		conn := getConn(letter, implicitConn)
		lastSeq[conn] = seq

		hexData := strings.ReplaceAll(m[3], ":", "")
		return fmt.Sprintf("expect=bgp:conn=%d:seq=%d:hex=%s", conn, seq, hexData), seq, letter != ""
	}

	// JSON pattern: [A-Z]?N:json:{...}
	// JSON lines use lastSeq[conn], and return 0 for num to avoid affecting connection tracking.
	// Old format used arbitrary numbers for JSON documentation, not sequence ordering.
	if m := jsonPattern.FindStringSubmatch(trimmed); m != nil {
		letter := m[1]
		baseSeq, _ := strconv.Atoi(m[2])
		if baseSeq == 0 {
			baseSeq = 1
		}
		conn := getConn(letter, implicitConn)
		seq := lastSeq[conn]
		if seq == 0 {
			seq = baseSeq
		}

		jsonData := m[3]
		// Return 0 for num so JSON lines don't affect connection tracking
		return fmt.Sprintf("expect=json:conn=%d:seq=%d:json=%s", conn, seq, jsonData), 0, letter != ""
	}

	// Cmd pattern: [A-Z]?N:cmd:text
	if m := cmdPattern.FindStringSubmatch(trimmed); m != nil {
		letter := m[1]
		seq, _ := strconv.Atoi(m[2])
		if seq == 0 {
			seq = 1
		}
		conn := getConn(letter, implicitConn)

		cmdText := m[3]
		return fmt.Sprintf("cmd=api:conn=%d:seq=%d:text=%s", conn, seq, cmdText), seq, letter != ""
	}

	// Notification pattern: [A-Z]?N:notification:text
	if m := notificationPattern.FindStringSubmatch(trimmed); m != nil {
		letter := m[1]
		seq, _ := strconv.Atoi(m[2])
		if seq == 0 {
			seq = 1
		}
		conn := getConn(letter, implicitConn)
		lastSeq[conn] = seq

		text := m[3]
		return fmt.Sprintf("action=notification:conn=%d:seq=%d:text=%s", conn, seq, text), seq, letter != ""
	}

	// Unknown line - return as-is.
	return line, 0, false
}

// getConn returns connection number from letter, or implicitConn if no letter.
func getConn(letter string, implicitConn int) int {
	if letter == "" {
		return implicitConn
	}
	c := letter[0]
	if c >= 'A' && c <= 'Z' {
		return int(c-'A') + 1
	}
	if c >= 'a' && c <= 'z' {
		return int(c-'a') + 1
	}
	return implicitConn
}
