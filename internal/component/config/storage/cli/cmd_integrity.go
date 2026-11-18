// Design: docs/architecture/zefs-format.md -- check, repair, and encode CLI commands

package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// cmdCheck prints the integrity verdict `show data check` answers, from the
// same rows (printVerdict). It exits 0 when no row is an error, 1 when any row
// is, 2 when the check could not run.
func cmdCheck(storePath string, _ []string) int {
	verdict, err := integrityVerdict(storePath)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 2
	}
	return printVerdict(verdict)
}

// printVerdict prints verdict and answers its exit code: a container fault
// (FAIL) and each corrupt frame (CORRUPT) on stderr, each history finding
// (warnings on stdout, errors on stderr), one line on stdout naming the keys a
// writer removed while the check ran, then the line that closes the verdict.
// A removed key is not an error, since a live store removes keys, but it was
// not read, so the closing line counts it apart from the entries read.
func printVerdict(verdict storeVerdict) int {
	for _, finding := range verdict.findings {
		if finding.kind == kindRemoved {
			continue // one line for every removed key, below
		}
		printIntegrityFinding(finding)
	}
	removed := verdict.removedKeys()
	if len(removed) > 0 {
		var tb textbuf.Buffer
		tb.Str("removed while the check ran, not read (").Int(int64(len(removed))).Str("): ")
		for i, key := range removed {
			if i > 0 {
				tb.Str(", ")
			}
			tb.Str(key)
		}
		tb.Byte('\n')
		tb.StdOut() //nolint:errcheck // CLI output
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
	tb.Str("ok: ").Int(int64(verdict.entries)).Str(" entries")
	if len(removed) > 0 {
		tb.Str(" read, ").Int(int64(len(removed))).Str(" removed during the check not read")
	}
	tb.Str(", magic ok, container ok\n")
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

// kindRemoved is the kind of a row naming a key a writer removed while a walk
// ran, so the walk did not read it. The frame rows take the history walk's
// kind, so one `| match removed-during-check` selects the rows of both walks.
const kindRemoved = storage.HistoryRemovedDuringCheck

// frameRemovedDetail is the detail of a frame row whose key was removed.
const frameRemovedDetail = "removed while the frame check ran, not read"

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
	findings       []integrityFinding // the container fault, the frames (corrupt, then removed), then history
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
	return reportVerdict(storePath, report, storeHistory)
}

// reportVerdict turns the frame report of the store at storePath into rows,
// one per container fault, corrupt frame and key the frame walk lists in
// Vanished, and adds the findings of history, the history walk
// (storeHistory), when every frame decodes. history is a parameter so a test
// can answer the walk a writer raced.
func reportVerdict(storePath string, report *zefs.CheckReport, history func(string) ([]storage.HistoryFinding, error)) (storeVerdict, error) {
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
	for _, key := range report.Vanished {
		verdict.findings = append(verdict.findings, integrityFinding{
			check: checkFrame, severity: severityWarning, kind: kindRemoved, key: key, detail: frameRemovedDetail,
		})
	}
	if verdict.containerFault {
		return verdict, nil
	}
	if verdict.corrupt > 0 {
		return verdict, nil
	}
	findings, err := history(storePath)
	if err != nil {
		return storeVerdict{}, err
	}
	verdict.historyChecked = true
	for _, finding := range findings {
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

// removedKeys answers, in row order, the keys a writer removed while the frame
// or the history walk ran, which that walk did not read.
func (v storeVerdict) removedKeys() []string {
	var keys []string
	for _, finding := range v.findings {
		if finding.kind == kindRemoved {
			keys = append(keys, finding.key)
		}
	}
	return keys
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
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 2
	}
	return printRepair(report, outputPath)
}

// printRepair prints the salvage report, repairs the history of the output
// (salvageHistory), prints the summary line and answers the shared verdict: a
// `recovered:` line per key on stdout, a `skipped:` line per entry that did
// not read on stderr, a `removed:` line on stdout per key a writer removed
// from the source while the walk ran, which is not in the output, then the
// history rows and the counts. A removed key does not change the exit code:
// the store no longer holds it, so the salvage lost nothing the store holds.
func printRepair(report *zefs.RepairReport, outputPath string) int {
	var tb textbuf.Buffer
	for _, key := range report.Recovered {
		tb.Reset().Str("recovered: ").Str(key).Byte('\n')
		tb.StdOut() //nolint:errcheck // CLI output
	}
	for _, entry := range report.Skipped {
		tb.Reset().Str("skipped: ").Str(entry.Key).Str(": ").Str(entry.Error).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output
	}
	for _, key := range report.Vanished {
		tb.Reset().Str("removed: ").Str(key).Byte('\n')
		tb.StdOut() //nolint:errcheck // CLI output
	}
	findings, err := salvageHistory(outputPath)
	if err != nil {
		tb.Reset().Str("error: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // CLI output; the exit code carries the verdict
		return 2
	}
	verdict := newRepairVerdict(report, findings)
	verdict.printHistory()

	tb.Reset().Int(int64(report.RecoveredCount)).Str(" recovered, ").Int(int64(report.SkippedCount)).
		Str(" skipped, ").Int(int64(len(report.Vanished))).Str(" removed while the repair ran -> ").Str(outputPath).Byte('\n')
	tb.StdOut() //nolint:errcheck // CLI output
	return verdict.exitCode()
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
	part := netcapFull
	capOverride := -1
	var dataArgs []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--crc":
			part = netcapCRC
		case "--header":
			part = netcapHeader
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

	encoded, err := encodeNetcap(data, cap_, part)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	os.Stdout.Write(encoded) //nolint:errcheck // CLI output
	return 0
}
