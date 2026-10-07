// Design: docs/contributing/feature-maturity.md -- the recorded green run Supported requires
// Related: runrecord.go -- the record this writes and the check reads
// Related: check.go -- the S1 criterion that consumes it
//
// The writer records only what it watched pass. It runs every real-path test
// item of one feature through the repository's own runners, refuses the whole
// record when one item fails, when the output does not show that item passing,
// or when the item's file changed while it ran, and writes nothing in each of
// those cases. A run that selected no test exits 0 under `go test -run`, so the
// exit code alone is never the evidence: the item's own PASS line is.

package feature

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testfunctional "github.com/ze-software/ze/internal/le/test/functional"
)

// itemRunDeadline bounds one item's run. An expired deadline is an error, never
// a red and never a pass: an unmeasured answer is not recorded.
const itemRunDeadline = 20 * time.Minute

// excerptOctetsMax bounds the run output quoted in a refusal.
const excerptOctetsMax = 4000

// observation is one run of one real-path test item.
type observation struct {
	exited bool // the command exited 0
	output string
}

// runItem runs one real-path test item and answers what it observed. The error
// is for a run that could not be made or did not finish.
type runItem func(item string) (observation, error)

// RecordRun runs every real-path test item of feature id and, only when every
// one was observed passing, writes features/runs/<id>.json.
func RecordRun(tree, id string) (RunRecord, error) {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return RunRecord{}, err
	}
	commit, err := headCommit(tree)
	if err != nil {
		return RunRecord{}, err
	}
	runner := &repoRunner{tree: tree, toolchain: toolchain}
	defer runner.release()

	return recordRun(tree, id, runner.run, commit, time.Now().UTC().Format(time.DateOnly))
}

// recordRun is RecordRun with its runner, commit and date passed in, so a test
// can drive every refusal without a toolchain.
func recordRun(tree, id string, run runItem, commit, date string) (RunRecord, error) {
	declaration, err := declarationNamed(tree, id)
	if err != nil {
		return RunRecord{}, err
	}
	if len(declaration.RealPathTests) == 0 {
		return RunRecord{}, errors.New("feature " + id + " lists no real-path test, so there is nothing to run")
	}
	record := RunRecord{Feature: id, Runs: make([]TestRun, 0, len(declaration.RealPathTests))}
	for _, item := range declaration.RealPathTests {
		file, _, _ := strings.Cut(item, goTestSeparator)
		before, err := blobID(tree, file)
		if err != nil {
			return RunRecord{}, err
		}
		seen, err := run(item)
		if err != nil {
			return RunRecord{}, errors.New(item + ": " + err.Error() + "; nothing recorded")
		}
		if problem := observedPass(item, seen); problem != "" {
			return RunRecord{}, errors.New(item + ": " + problem + "; nothing recorded:\n" + excerpt(seen.output))
		}
		after, err := blobID(tree, file)
		if err != nil {
			return RunRecord{}, err
		}
		if after != before {
			return RunRecord{}, errors.New(item + ": " + file + " changed while it ran, so the pass belongs " +
				"to neither content; nothing recorded")
		}
		record.Runs = append(record.Runs, TestRun{Test: item, TestBlob: before, Commit: commit, Date: date,
			Result: runResultPass})
	}
	if err := writeRunRecord(tree, &record); err != nil {
		return RunRecord{}, err
	}
	return record, nil
}

// declarationNamed answers the declaration whose id is id, refusing one that
// does not parse: a record for a declaration the check refuses proves nothing.
func declarationNamed(tree, id string) (Declaration, error) {
	declarations, problems, err := Load(tree)
	if err != nil {
		return Declaration{}, err
	}
	for _, problem := range problems {
		if strings.Contains(problem.Error(), declarationDir+"/"+id+".md") {
			return Declaration{}, problem
		}
	}
	for i := range declarations {
		if declarations[i].ID == id {
			return declarations[i], nil
		}
	}
	return Declaration{}, errors.New("no feature declaration " + declarationDir + "/" + id + ".md")
}

// observedPass answers why seen is not a pass of item, or "".
func observedPass(item string, seen observation) string {
	if !seen.exited {
		return "the run failed"
	}
	file, function, named := strings.Cut(item, goTestSeparator)
	if named {
		// `go test -v` prints this line for a test that ran and passed; a -run
		// pattern that matched nothing exits 0 and prints none.
		if strings.Contains(seen.output, "--- PASS: "+function+" (") {
			return ""
		}
		return "the output shows no '--- PASS: " + function + "' line, so the run did not prove it passed"
	}
	stem := strings.TrimSuffix(path.Base(file), path.Ext(file))
	for line := range strings.Lines(seen.output) {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[len(fields)-1] != stem {
			continue
		}
		if fields[len(fields)-3] == "PASS" {
			return ""
		}
	}
	return "the output shows no PASS line for " + stem + ", so the run did not prove it passed"
}

func writeRunRecord(tree string, record *RunRecord) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	full := filepath.Join(tree, filepath.FromSlash(runRecordRel(record.Feature)))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	return os.WriteFile(full, append(raw, '\n'), 0o600)
}

func excerpt(output string) string {
	if len(output) <= excerptOctetsMax {
		return output
	}
	return output[len(output)-excerptOctetsMax:]
}

// repoRunner runs an item through the runner that owns its kind: `go test` for
// a Go test, the functional runner for a .ci. The isolated binary set a .ci
// runs against is built once, on the first .ci, and released by release.
// Not safe for concurrent use.
type repoRunner struct {
	tree      string
	toolchain gotoolchain.Toolchain
	set       testfunctional.BinarySet
	prepared  bool
}

func (r *repoRunner) run(item string) (observation, error) {
	file, function, named := strings.Cut(item, goTestSeparator)
	if named {
		var tb textbuf.Buffer
		argv := r.toolchain.GoTest(gotoolchain.TestOptions{}, "-run", tb.Byte('^').Str(function).Byte('$').String(),
			"-count=1", "-v", "./"+path.Dir(file))
		return r.exec(argv, r.toolchain.Environment(gotoolchain.EnvOptions{Test: true, Procs: true}))
	}
	suite, problem := functionalRunnerOf(file)
	if problem != "" {
		return observation{}, errors.New(problem)
	}
	if !r.prepared {
		set, err := testfunctional.Prepare(r.toolchain, "feature-record-run")
		if err != nil {
			return observation{}, errors.New("cannot build the isolated binaries a .ci runs against: " + err.Error())
		}
		r.set, r.prepared = set, true
	}
	argv := []string{filepath.Join(r.set.Dir, testfunctional.LE), "test"}
	for _, arg := range suite.Args {
		if arg == testfunctional.AllTests {
			continue
		}
		argv = append(argv, arg)
	}
	argv = append(argv, strings.TrimSuffix(path.Base(file), path.Ext(file)))
	return r.exec(argv, r.set.Environment(r.toolchain))
}

func (r *repoRunner) release() {
	if r.prepared {
		testfunctional.Release(r.set)
	}
}

func (r *repoRunner) exec(argv, environ []string) (observation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), itemRunDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv is built from a declaration item that passed the check's path resolution
	cmd.Dir = r.tree
	cmd.Env = environ
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return observation{}, errors.New("did not finish inside " + itemRunDeadline.String() +
			", so whether it passes is unknown")
	}
	return observation{exited: runErr == nil, output: out.String()}, nil
}

// functionalRunnerOf answers the suite whose runner discovers the .ci at rel,
// or why none does. The functional runner walks test/<suite>/ for the suite
// named by that directory, the same mapping internal/le/rfc's carriers use.
func functionalRunnerOf(rel string) (testfunctional.Suite, string) {
	if !strings.HasSuffix(rel, ".ci") {
		return testfunctional.Suite{}, "'" + rel + "' is neither a Go test (file.go::TestName) nor a .ci"
	}
	parts := strings.Split(rel, "/")
	if len(parts) != 3 {
		return testfunctional.Suite{}, "'" + rel + "' is not test/<suite>/<name>.ci, which is where the functional runner looks"
	}
	if parts[0] != "test" {
		return testfunctional.Suite{}, "'" + rel + "' is not test/<suite>/<name>.ci, which is where the functional runner looks"
	}
	suite, held := testfunctional.SuiteNamed(parts[1])
	if !held {
		return testfunctional.Suite{}, "'" + rel + "': no functional suite named '" + parts[1] + "' runs test/" + parts[1] + "/"
	}
	return suite, ""
}

// headCommit answers HEAD, which a record carries for a reader.
func headCommit(tree string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
	cmd.Dir = tree
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("cannot read HEAD: " + err.Error())
	}
	return strings.TrimSpace(string(out)), nil
}
