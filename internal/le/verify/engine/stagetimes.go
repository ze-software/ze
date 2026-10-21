// Design: docs/architecture/testing/verify-freshness-scope.md -- stage-time history and cost order
// Related: run.go -- the stage loop that reads the order and appends the times
package verifyengine

import (
	"bufio"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// StageTimesPath is the stage-time history every run appends to, relative to
// the checkout that asked for the run. A worktree run appends to the checkout's
// file, never to its own, because the worktree is removed when the run ends.
const StageTimesPath = "tmp/ze-verify-stage-times.tsv"

// stageTimeFields is the column count of one history line: stage, seconds,
// exit, mode, commit, start.
const stageTimeFields = 6

// stageTimeLineMax bounds one history line the reader accepts. A stage name is
// a short action path and the other five columns are fixed width, so a longer
// line is not one the engine wrote.
const stageTimeLineMax = 4096

// Plan is how one run orders and ends its stages.
type Plan struct {
	// History is the checkout whose stage-time history the run reads its cost
	// order from and appends its stage times to. Empty means no history: the
	// run keeps the declared order and records nothing.
	History string
	// FirstRed stops the run after the first stage that exits non-zero. Every
	// stage it did not start is recorded as not reached.
	FirstRed bool
}

// StageCosts holds the latest recorded seconds of each stage in the history,
// and how many lines the reader refused.
type StageCosts struct {
	Seconds   map[string]float64
	Malformed int
}

// ReadStageCosts reads the history under checkout. The latest line for a stage
// wins, because the file is appended in run order. A line that does not parse
// as six columns with finite non-negative seconds, an integer exit and an
// RFC 3339 start is skipped and counted: the file is shared by every session
// on the machine, so it is untrusted text. A missing history is no costs and
// no error.
func ReadStageCosts(checkout string) (StageCosts, error) {
	costs := StageCosts{Seconds: map[string]float64{}}
	file, err := os.Open(filepath.Join(checkout, filepath.FromSlash(StageTimesPath))) //nolint:gosec // the path is a verify artifact under the checkout root
	if errors.Is(err, os.ErrNotExist) {
		return costs, nil
	}
	if err != nil {
		return costs, err
	}
	defer file.Close() //nolint:errcheck // read-only file

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, stageTimeLineMax), stageTimeLineMax)
	for scanner.Scan() {
		stage, seconds, ok := parseStageTime(scanner.Text())
		if !ok {
			costs.Malformed++
			continue
		}
		costs.Seconds[stage] = seconds
	}
	if err := scanner.Err(); err != nil {
		return costs, err
	}
	return costs, nil
}

func parseStageTime(line string) (string, float64, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) != stageTimeFields {
		return "", 0, false
	}
	if fields[0] == "" {
		return "", 0, false
	}
	seconds, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return "", 0, false
	}
	if math.IsNaN(seconds) {
		return "", 0, false
	}
	if math.IsInf(seconds, 0) {
		return "", 0, false
	}
	if seconds < 0 {
		return "", 0, false
	}
	if _, err := strconv.Atoi(fields[2]); err != nil {
		return "", 0, false
	}
	if _, err := time.Parse(time.RFC3339, fields[5]); err != nil {
		return "", 0, false
	}
	return fields[0], seconds, true
}

// costOrder answers stages in run order: every stage with no recorded time
// first, in declared order, so a stage is measured the first time it exists,
// then every recorded stage by ascending latest seconds, ties in declared
// order. A history name that is not in stages adds nothing.
func costOrder(stages []Stage, costs StageCosts) []Stage {
	ordered := slices.Clone(stages)
	slices.SortStableFunc(ordered, func(first, second Stage) int {
		firstSeconds, firstRecorded := costs.Seconds[first.Identity.Name]
		secondSeconds, secondRecorded := costs.Seconds[second.Identity.Name]
		if firstRecorded != secondRecorded {
			if firstRecorded {
				return 1
			}
			return -1
		}
		if !firstRecorded {
			return 0
		}
		switch {
		case firstSeconds < secondSeconds:
			return -1
		case firstSeconds > secondSeconds:
			return 1
		default:
			return 0
		}
	})
	return ordered
}

// AppendStageTimes appends one line per reached stage of report to the history
// under checkout. Every line goes out in ONE write on a file opened O_APPEND,
// so two sessions appending at once never interleave inside a line. A stage
// whose name holds a tab or a newline cannot be one column and is not written.
func AppendStageTimes(checkout string, report Report) error {
	if len(report.Stages) == 0 {
		return nil
	}
	var lines textbuf.Buffer
	for index := range report.Stages {
		stage := &report.Stages[index]
		if strings.ContainsAny(stage.Identity.Name, "\t\n\r") {
			continue
		}
		lines.Str(stage.Identity.Name).Byte('\t').
			Str(strconv.FormatFloat(stage.Seconds, 'f', 3, 64)).Byte('\t').
			Int(int64(stage.Code)).Byte('\t').
			Str(report.Mode).Byte('\t').
			Str(report.Commit).Byte('\t').
			Str(stage.Start.UTC().Format(time.RFC3339)).Byte('\n')
	}
	path := filepath.Join(checkout, filepath.FromSlash(StageTimesPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600) //nolint:gosec // the path is a verify artifact under the checkout root
	if err != nil {
		return err
	}
	_, writeErr := file.Write(lines.Bytes())
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
