// ci-fix-order reorders path attributes in .ci files to RFC 4271 order.
//
// RFC 4271 Section 5: Path attributes MUST be in ascending type code order.
//
// Usage:
//
//	go run ./test/cmd/ci-fix-order [options] [files...]
//
// Options:
//
//	--dry-run    Show changes without modifying files
//	--all        Process all .ci files in test/data/
//	--verbose    Show detailed output
package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	dryRun  = flag.Bool("dry-run", false, "Show changes without modifying files")
	all     = flag.Bool("all", false, "Process all .ci files")
	verbose = flag.Bool("verbose", false, "Show detailed output")
)

func main() {
	flag.Parse()

	var files []string
	if *all {
		// Find all .ci files
		for _, dir := range []string{"test/data/encode", "test/data/api"} {
			matches, err := filepath.Glob(filepath.Join(dir, "*.ci"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error globbing %s: %v\n", dir, err)
				continue
			}
			files = append(files, matches...)
		}
	} else {
		files = flag.Args()
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: ci-fix-order [--dry-run] [--all] [--verbose] [files...]")
		os.Exit(1)
	}

	totalFixed := 0
	for _, file := range files {
		fixed, err := processFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", file, err)
			continue
		}
		totalFixed += fixed
	}

	fmt.Printf("Total: %d lines fixed\n", totalFixed)
}

func processFile(filename string) (int, error) {
	f, err := os.Open(filename) //nolint:gosec // intentional file path from user
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}

	fixed := 0
	for i, line := range lines {
		newLine, changed, err := processLine(line)
		if err != nil {
			if *verbose {
				fmt.Printf("%s:%d: skip - %v\n", filename, i+1, err)
			}
			continue
		}
		if changed {
			fixed++
			if *verbose {
				fmt.Printf("%s:%d: fixed\n", filename, i+1)
				fmt.Printf("  OLD: %s\n", truncate(line, 100))
				fmt.Printf("  NEW: %s\n", truncate(newLine, 100))
			}
			lines[i] = newLine
		}
	}

	if fixed > 0 {
		fmt.Printf("%s: %d lines fixed\n", filename, fixed)
		if !*dryRun {
			if err := writeFile(filename, lines); err != nil {
				return fixed, err
			}
		}
	}

	return fixed, nil
}

func writeFile(filename string, lines []string) error {
	f, err := os.Create(filename) //nolint:gosec // intentional file path from user
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for i, line := range lines {
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if i < len(lines)-1 || line != "" {
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

func processLine(line string) (string, bool, error) {
	// Parse: N:raw:MARKER:LENGTH:TYPE:PAYLOAD
	// or:    N:raw:MARKER:LENGTHTYPE:PAYLOAD (merged format for EOR)
	if !strings.Contains(line, ":raw:") {
		return line, false, nil
	}

	parts := strings.SplitN(line, ":", 6)

	var prefix, kind, marker, length, msgType, payloadHex string

	switch len(parts) {
	case 6:
		// Standard format: prefix:kind:marker:length:type:payload
		prefix = parts[0]
		kind = parts[1]
		marker = parts[2]
		length = parts[3]
		msgType = parts[4]
		payloadHex = parts[5]
	case 5:
		// Merged format: prefix:kind:marker:lengthtype:payload
		prefix = parts[0]
		kind = parts[1]
		marker = parts[2]
		lengthType := parts[3]
		payloadHex = parts[4]

		if len(lengthType) >= 6 {
			length = lengthType[:4]
			msgType = lengthType[4:6]
		} else {
			return line, false, nil // Skip malformed
		}
	default:
		return line, false, nil // Skip
	}

	if kind != "raw" {
		return line, false, nil
	}

	// Only process UPDATE messages (type 02)
	if msgType != "02" {
		return line, false, nil
	}

	// Decode payload
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return line, false, fmt.Errorf("invalid hex payload: %w", err)
	}

	// Parse UPDATE message
	newPayload, changed, err := reorderUpdateAttributes(payload)
	if err != nil {
		return line, false, err
	}

	if !changed {
		return line, false, nil
	}

	// Rebuild line
	newPayloadHex := strings.ToUpper(hex.EncodeToString(newPayload))

	// Update length if payload size changed (shouldn't happen for reordering)
	newLen := 19 + len(newPayload) // 16 marker + 2 length + 1 type
	newLengthHex := fmt.Sprintf("%04X", newLen)

	// Handle edge case where original length field might be wrong format
	if len(length) != 4 {
		newLengthHex = length // Keep original if malformed
	}

	newLine := fmt.Sprintf("%s:%s:%s:%s:%s:%s", prefix, kind, marker, newLengthHex, msgType, newPayloadHex)
	return newLine, true, nil
}

// attribute holds a single path attribute.
type attribute struct {
	flags uint8
	code  uint8
	value []byte
}

func (a attribute) pack() []byte {
	extLen := a.flags&0x10 != 0
	if extLen {
		// 4-byte header: flags, code, 2-byte length
		buf := make([]byte, 4+len(a.value))
		buf[0] = a.flags
		buf[1] = a.code
		buf[2] = byte(len(a.value) >> 8)
		buf[3] = byte(len(a.value))
		copy(buf[4:], a.value)
		return buf
	}
	// 3-byte header: flags, code, 1-byte length
	buf := make([]byte, 3+len(a.value))
	buf[0] = a.flags
	buf[1] = a.code
	buf[2] = byte(len(a.value))
	copy(buf[3:], a.value)
	return buf
}

func reorderUpdateAttributes(payload []byte) ([]byte, bool, error) {
	if len(payload) < 4 {
		return nil, false, fmt.Errorf("payload too short")
	}

	// Parse withdrawn routes length
	withdrawnLen := int(payload[0])<<8 | int(payload[1])
	pos := 2

	if pos+withdrawnLen > len(payload) {
		return nil, false, fmt.Errorf("withdrawn length exceeds payload")
	}
	withdrawn := payload[pos : pos+withdrawnLen]
	pos += withdrawnLen

	// Parse path attributes length
	if pos+2 > len(payload) {
		return nil, false, fmt.Errorf("missing path attributes length")
	}
	attrLen := int(payload[pos])<<8 | int(payload[pos+1])
	pos += 2

	if pos+attrLen > len(payload) {
		return nil, false, fmt.Errorf("attributes length exceeds payload")
	}
	attrData := payload[pos : pos+attrLen]
	pos += attrLen

	// Remaining is NLRI
	nlri := payload[pos:]

	// Parse individual attributes
	attrs, err := parseAttributes(attrData)
	if err != nil {
		return nil, false, err
	}

	if len(attrs) < 2 {
		return nil, false, nil // Nothing to reorder
	}

	// Check if already sorted
	sorted := true
	for i := 1; i < len(attrs); i++ {
		if attrs[i].code < attrs[i-1].code {
			sorted = false
			break
		}
	}

	if sorted {
		return nil, false, nil
	}

	// Sort by type code
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].code < attrs[j].code
	})

	// Rebuild attributes
	var newAttrData []byte
	for _, attr := range attrs {
		newAttrData = append(newAttrData, attr.pack()...)
	}

	// Rebuild payload
	newPayload := make([]byte, 0, 2+len(withdrawn)+2+len(newAttrData)+len(nlri))
	newPayload = append(newPayload, byte(withdrawnLen>>8), byte(withdrawnLen))
	newPayload = append(newPayload, withdrawn...)
	newPayload = append(newPayload, byte(len(newAttrData)>>8), byte(len(newAttrData)))
	newPayload = append(newPayload, newAttrData...)
	newPayload = append(newPayload, nlri...)

	return newPayload, true, nil
}

func parseAttributes(data []byte) ([]attribute, error) {
	var attrs []attribute
	pos := 0

	for pos < len(data) {
		if pos+2 > len(data) {
			return nil, fmt.Errorf("truncated attribute header at pos %d", pos)
		}

		flags := data[pos]
		code := data[pos+1]
		pos += 2

		extLen := flags&0x10 != 0
		var attrLen int

		if extLen {
			if pos+2 > len(data) {
				return nil, fmt.Errorf("truncated extended length at pos %d", pos)
			}
			attrLen = int(data[pos])<<8 | int(data[pos+1])
			pos += 2
		} else {
			if pos+1 > len(data) {
				return nil, fmt.Errorf("truncated length at pos %d", pos)
			}
			attrLen = int(data[pos])
			pos++
		}

		if pos+attrLen > len(data) {
			return nil, fmt.Errorf("attribute value exceeds data at pos %d", pos)
		}

		value := make([]byte, attrLen)
		copy(value, data[pos:pos+attrLen])
		pos += attrLen

		attrs = append(attrs, attribute{
			flags: flags,
			code:  code,
			value: value,
		})
	}

	return attrs, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
