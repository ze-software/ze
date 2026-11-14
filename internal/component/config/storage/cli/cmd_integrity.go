// Design: docs/architecture/zefs-format.md -- check, repair, and encode CLI commands

package cli

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// cmdCheck prints the integrity verdict `show data check` answers, from the
// same rows: a container fault (FAIL) and each corrupt frame (CORRUPT) on
// stderr, each history finding (warnings on stdout, errors on stderr), then
// the line that closes the verdict. It exits 0 clean, 1 when any row is an
// error, 2 when the check could not run.
func cmdCheck(storePath string, _ []string) int {
	verdict, err := integrityVerdict(storePath)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 2
	}
	for _, finding := range verdict.findings {
		printIntegrityFinding(finding)
	}
	if verdict.containerFault {
		return 1
	}
	var tb textbuf.Buffer
	if verdict.corrupt > 0 {
		tb.Str(verdict.corruptLine()).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 1
	}
	if errorCount := verdict.errorCount(); errorCount > 0 {
		tb.Int(int64(errorCount)).Str(" history errors\n")
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 1
	}
	tb.Str("ok: ").Int(int64(verdict.entries)).Str(" entries, magic ok, container ok\n")
	tb.StdOut() //nolint:errcheck // CLI output
	return 0
}

// The check a finding row comes from, and its grade.
const (
	checkContainer = "container"
	checkFrame     = "frame"
	checkHistory   = "history"

	severityError   = "error"
	severityWarning = "warning"
)

// integrityFinding is one row of a store's integrity verdict: the container
// fault of a blob, a frame that does not decode, or one finding of the history
// walk.
type integrityFinding struct {
	check    string // checkContainer, checkFrame or checkHistory
	severity string // severityError or severityWarning
	kind     string // the frame status, the history finding kind, or checkContainer
	key      string // empty for a container fault
	detail   string
}

// storeVerdict is the integrity verdict of one store, the rows `show data
// check` answers and `ze data check` prints.
type storeVerdict struct {
	path           string
	entries        int  // frames that decode
	corrupt        int  // frames that do not
	containerFault bool // a blob whose container does not parse
	historyChecked bool
	findings       []integrityFinding // the container fault, the frames, then history
}

// integrityVerdict runs the frame check and, over a store whose frames all
// decode, the history walk, and answers the rows. It prints nothing, so the
// daemon can answer it (command.LocalDataHandler). A corrupt frame does not
// read, and the history walk refuses a key it cannot read rather than report
// it absent, so history is not checked then: the frame verdict is the answer,
// and history is checked over the readable frames a repair writes.
func integrityVerdict(storePath string) (storeVerdict, error) {
	report, err := zefs.CheckPath(storePath)
	if err != nil {
		return storeVerdict{}, err
	}
	verdict := storeVerdict{path: storePath, entries: report.TotalEntries, corrupt: report.CorruptEntries}
	if report.ContainerError != "" {
		verdict.containerFault = true
		verdict.findings = append(verdict.findings, integrityFinding{
			check: checkContainer, severity: severityError, kind: checkContainer, detail: report.ContainerError,
		})
	}
	for _, entry := range report.Entries {
		if entry.Status == "ok" {
			continue
		}
		verdict.findings = append(verdict.findings, integrityFinding{
			check: checkFrame, severity: severityError, kind: entry.Status, key: entry.Key, detail: entry.Error,
		})
	}
	if verdict.containerFault {
		return verdict, nil
	}
	if verdict.corrupt > 0 {
		return verdict, nil
	}
	history, err := storeHistory(storePath)
	if err != nil {
		return storeVerdict{}, err
	}
	verdict.historyChecked = true
	for _, finding := range history {
		severity := severityError
		if finding.Severity == storage.HistorySeverityWarning {
			severity = severityWarning
		}
		verdict.findings = append(verdict.findings, integrityFinding{
			check: checkHistory, severity: severity, kind: finding.Kind, key: finding.Key, detail: finding.Detail,
		})
	}
	return verdict, nil
}

// storeHistory runs the history reachability walk over the store at storePath
// through a reader handle and answers its findings.
func storeHistory(storePath string) ([]storage.HistoryFinding, error) {
	s, err := openStore(storePath, false)
	if err != nil {
		return nil, err
	}
	defer s.Close() //nolint:errcheck // read-only check handle.
	return storage.CheckHistory(s)
}

// errorCount answers how many rows are errors.
func (v storeVerdict) errorCount() int {
	count := 0
	for _, finding := range v.findings {
		if finding.severity == severityError {
			count++
		}
	}
	return count
}

// corruptLine is the line that closes a verdict with corrupt frames.
func (v storeVerdict) corruptLine() string {
	var tb textbuf.Buffer
	tb.Int(int64(v.corrupt)).Byte('/').Int(int64(v.entries + v.corrupt)).
		Str(" entries corrupt; history not checked, run ze data repair --output <path>")
	return tb.String()
}

// printIntegrityFinding prints one row the way ze data check always printed
// it: errors on stderr, a history warning on stdout.
func printIntegrityFinding(finding integrityFinding) {
	var tb textbuf.Buffer
	switch finding.check {
	case checkContainer:
		tb.Str("FAIL: ").Str(finding.detail).Byte('\n')
	case checkFrame:
		tb.Str("CORRUPT: ").Str(finding.key).Str(": ").Str(finding.detail).Byte('\n')
	case checkHistory:
		tb.Str(finding.severity).Str(": ").Str(finding.kind).Str(": ").Str(finding.key).Str(": ").Str(finding.detail).Byte('\n')
		if finding.severity == severityWarning {
			tb.StdOut() //nolint:errcheck // CLI output
			return
		}
	default:
		panic("BUG: integrity finding from an unknown check " + finding.check)
	}
	tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
}

// printHistoryFindings prints one row per finding, `<kind>: <key>: <detail>`,
// errors on stderr and warnings on stdout, and returns the error count.
func printHistoryFindings(findings []storage.HistoryFinding) int {
	errorCount := 0
	for _, finding := range findings {
		if finding.Severity == storage.HistorySeverityWarning {
			fmt.Fprintf(os.Stdout, "warning: %s: %s: %s\n", finding.Kind, finding.Key, finding.Detail) //nolint:errcheck // CLI output
			continue
		}
		errorCount++
		fmt.Fprintf(os.Stderr, "error: %s: %s: %s\n", finding.Kind, finding.Key, finding.Detail) //nolint:errcheck // CLI output
	}
	return errorCount
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

	report, err := zefs.RepairPath(storePath, outputPath)
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
	historyIssues, err := repairStoreHistory(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	fmt.Fprintf(os.Stdout, "%d recovered, %d skipped -> %s\n", report.RecoveredCount, report.SkippedCount, outputPath) //nolint:errcheck // CLI output
	if report.SkippedCount > 0 {
		return 1
	}
	if historyIssues > 0 {
		return 1
	}
	return 0
}

// repairStoreHistory repairs the history of the repaired output
// (salvageHistory), reports each finding, and returns how many dropped keys
// and remaining errors there are.
func repairStoreHistory(outputPath string) (int, error) {
	findings, err := salvageHistory(outputPath)
	if err != nil {
		return 0, err
	}
	unresolved := 0
	var remaining []storage.HistoryFinding
	for _, finding := range findings {
		if !historyDropped(finding) {
			remaining = append(remaining, finding)
			continue
		}
		unresolved++
		var tb textbuf.Buffer
		tb.Str("dropped: ").Str(finding.Kind).Str(": ").Str(finding.Key).Str(": ").Str(finding.Detail).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output
	}
	return unresolved + printHistoryFindings(remaining), nil
}

// salvageHistory drops the dangling and malformed entries and wrong-hash
// objects of the salvaged store at outputPath and answers every finding of the
// walk, printing nothing, so `ze data repair` and `request data repair` share
// one history step. A pointer left naming a dropped entry is an error the
// operator resolves; it is never retargeted.
func salvageHistory(outputPath string) ([]storage.HistoryFinding, error) {
	s, err := openStore(outputPath, true)
	if err != nil {
		return nil, err
	}
	findings, err := storage.RepairHistory(s)
	closeErr := s.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return findings, nil
}

// historyDropped reports whether the history repair dropped the key of
// finding, rather than only reporting it.
func historyDropped(finding storage.HistoryFinding) bool {
	switch finding.Kind {
	case storage.HistoryWrongHashObject, storage.HistoryDanglingEntry, storage.HistoryMalformedEntry:
		return true
	}
	return false
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
