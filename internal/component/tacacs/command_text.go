// Design: docs/architecture/aaa-tacacs.md -- printable command display on the wire.
package tacacs

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
)

// asciiCommandValue is a reversible display encoding, not an execution string.
// Printable text without quotes or backslashes is unchanged. Otherwise Go's
// quoted-string escapes (without the surrounding quotes) preserve the exact
// bytes, including malformed UTF-8, using only printable ASCII.
func asciiCommandValue(value string) string {
	for i := range len(value) {
		if value[i] < 0x20 || value[i] > 0x7e || value[i] == '\\' || value[i] == '"' {
			quoted := strconv.QuoteToASCII(value)
			return quoted[1 : len(quoted)-1]
		}
	}
	return value
}

// accountingArguments bounds the already-redacted command display, not the
// executed command. Two slots hold task/time; an oversized display additionally
// uses two accounting metadata slots before service/cmd. Its SHA-256 covers the
// complete redacted AV list with uint64 byte-length framing, not credentials or
// an ambiguous space-joined command. START and STOP use the same display digest.
// Authorization never calls this truncating path: its policy must be lossless.
func accountingArguments(taskID, timestamp string, commandArgs []string) []string {
	truncated := len(commandArgs) > 253
	for _, arg := range commandArgs {
		if len(arg) > 255 {
			truncated = true
			break
		}
	}
	limit := 253
	metadata := 0
	if truncated {
		limit = 251
		metadata = 2
	}
	args := make([]string, 0, 2+metadata+min(len(commandArgs), limit))
	args = append(args, "task_id="+taskID, timestamp)
	if truncated {
		hash := sha256.New()
		var length [8]byte
		for _, arg := range commandArgs {
			binary.BigEndian.PutUint64(length[:], uint64(len(arg)))
			_, _ = hash.Write(length[:])
			_, _ = hash.Write([]byte(arg))
		}
		var digest [sha256.Size]byte
		hash.Sum(digest[:0])
		args = append(args, "ze-command-truncated=1", "ze-command-sha256="+hex.EncodeToString(digest[:]))
	}
	for _, arg := range commandArgs[:min(len(commandArgs), limit)] {
		if len(arg) > 255 {
			arg = truncateCommandArgument(arg)
		}
		args = append(args, arg)
	}
	return args
}

// truncateCommandArgument retains complete display escapes and marks the
// shortened field. The accompanying metadata distinguishes a shortened display
// from a literal command ending in dots and identifies the complete display.
func truncateCommandArgument(arg string) string {
	end := 0
	for end < len(arg) {
		size := 1
		if arg[end] == '\\' {
			switch arg[end+1] {
			case 'x':
				size = 4
			case 'u':
				size = 6
			case 'U':
				size = 10
			default:
				size = 2
			}
		}
		if end+size > 252 {
			break
		}
		end += size
	}
	return arg[:end] + "..."
}
