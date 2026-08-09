// Design: docs/architecture/zefs-format.md -- check, repair, and encode CLI commands

package cli

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strconv"

	"github.com/ze-software/ze/pkg/zefs"
)

func cmdCheck(storePath string, _ []string) int {
	report, err := zefs.Check(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if report.ContainerError != "" {
		fmt.Fprintf(os.Stderr, "FAIL: %s\n", report.ContainerError)
		return 1
	}

	for _, e := range report.Entries {
		if e.Status != "ok" {
			fmt.Fprintf(os.Stderr, "CORRUPT: %s: %s\n", e.Key, e.Error)
		}
	}

	if report.CorruptEntries > 0 {
		fmt.Fprintf(os.Stderr, "%d/%d entries corrupt\n", report.CorruptEntries, report.TotalEntries+report.CorruptEntries)
		return 1
	}

	fmt.Fprintf(os.Stdout, "ok: %d entries, magic ok, container ok\n", report.TotalEntries) //nolint:errcheck // CLI output
	return 0
}

func cmdRepair(storePath string, args []string) int {
	outputPath := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output" && i+1 < len(args):
			outputPath = args[i+1]
			i++
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
			fmt.Fprintf(os.Stderr, "usage: ze data repair --output <path>\n")
			return 1
		}
	}

	if outputPath == "" {
		fmt.Fprintf(os.Stderr, "usage: ze data repair --output <path>\n")
		return 1
	}

	report, err := zefs.Repair(storePath, outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	for _, key := range report.Recovered {
		fmt.Fprintf(os.Stdout, "recovered: %s\n", key) //nolint:errcheck // CLI output
	}
	for _, e := range report.Skipped {
		fmt.Fprintf(os.Stderr, "skipped: %s: %s\n", e.Key, e.Error) //nolint:errcheck // CLI output
	}

	fmt.Fprintf(os.Stdout, "%d recovered, %d skipped -> %s\n", report.RecoveredCount, report.SkippedCount, outputPath) //nolint:errcheck // CLI output
	if report.SkippedCount > 0 {
		return 1
	}
	return 0
}

func cmdEncode(_ string, args []string) int {
	mode := "full" // "full", "crc", "header"
	capOverride := -1
	var dataArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--crc":
			mode = "crc"
		case "--header":
			mode = "header"
		case "--cap":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --cap requires a value\n")
				return 1
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v < 0 {
				fmt.Fprintf(os.Stderr, "error: --cap must be a non-negative integer\n")
				return 1
			}
			capOverride = v
		default:
			dataArgs = append(dataArgs, args[i])
		}
	}

	if len(dataArgs) != 1 {
		fmt.Fprintf(os.Stderr, "usage: ze data encode [--crc|--header] [--cap N] <string|->\n")
		return 1
	}

	var data []byte
	if dataArgs[0] == "-" {
		var err error
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read stdin: %v\n", err)
			return 2
		}
	} else {
		data = []byte(dataArgs[0])
	}

	cap_ := len(data) + len(data)/10
	if capOverride >= 0 {
		if capOverride < len(data) {
			fmt.Fprintf(os.Stderr, "error: --cap %d is less than data length %d\n", capOverride, len(data))
			return 1
		}
		cap_ = capOverride
	}

	crc := crc32.Checksum(data, zefs.CRC32cTable)

	switch mode {
	case "crc":
		var b [4]byte
		b[0] = byte(crc >> 24)
		b[1] = byte(crc >> 16)
		b[2] = byte(crc >> 8)
		b[3] = byte(crc)
		fmt.Fprintln(os.Stdout, hex.EncodeToString(b[:])) //nolint:errcheck // CLI output
	case "header":
		encoded, err := zefs.EncodeNetcapstring(data, cap_)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 2
		}
		// Header is everything before the first '\n'
		for i, c := range encoded {
			if c == '\n' {
				os.Stdout.Write(encoded[:i]) //nolint:errcheck // stdout
				fmt.Fprintln(os.Stdout)      //nolint:errcheck // CLI output
				break
			}
		}
	case "full":
		encoded, err := zefs.EncodeNetcapstring(data, cap_)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 2
		}
		os.Stdout.Write(encoded) //nolint:errcheck // stdout
	}

	return 0
}
