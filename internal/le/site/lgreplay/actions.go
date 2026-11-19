// Design: docs/contributing/gh-pages.md -- the looking glass replay: its le action
// Related: record.go -- record, the run this action starts
// Related: register.go -- the registration of the area

package sitelgreplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/gitenv"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/test/fixture"
	"github.com/ze-software/ze/internal/test/runner"
)

const (
	area = "site lg-replay"

	keywordBinary = "binary"
	keywordOutput = "output"
	keywordPort   = "port"
	keywordTrial  = "trial"

	// portDefault is where the search for three free ports starts. The
	// reservation moves forward when they are taken.
	portDefault = 28470
	// runWait bounds a whole run: the network's start, the checks, both reads.
	runWait = 10 * time.Minute
	// gitWait bounds one git query.
	gitWait = 30 * time.Second
	// uncommittedShown bounds the uncommitted paths a refusal names.
	uncommittedShown = 5
)

const recordWhy = "record the looking glass for the website's replay.\n" +
	"\n" +
	"It starts the simulated network (eleven BGP speakers generated from a seeded\n" +
	"topology model) against a real ze, runs the checks the functional test runs,\n" +
	"asks the looking glass the declared tour over HTTP, follows every request the\n" +
	"answers name, and writes " + File + ".\n" +
	"Session uptimes, state-change times and the listener address are replaced by\n" +
	"the fixed values the file's provenance.normalized declares. Every question is\n" +
	"asked twice, ten seconds apart, and an answer that still differs is named in\n" +
	"provenance.varies: an empty list means two recordings of one commit are\n" +
	"byte-identical.\n" +
	"The file names the checkout's commit as the code that answered, so a checkout\n" +
	"with uncommitted changes is refused before anything starts. The trial keyword\n" +
	"records it anyway, to try the recorder: the file then says the tree was dirty,\n" +
	"and ./le site build refuses to publish it.\n" +
	"./le site build publishes the replay from the file. It never records."

var actions = leaction.New(area,
	leaction.Action{
		Verb: "record", Heavy: true, Writes: true,
		Summary: "record the looking glass over the simulated network",
		Why:     recordWhy,
		Parameters: []leaction.Parameter{
			{Keyword: keywordBinary, Kind: leaction.KindPath, Help: "the ze to record; without it, the ze found in PATH", Value: "ze-binary", Requirement: leaction.Optional},
			{Keyword: keywordOutput, Kind: leaction.KindPath, Help: "the file the recording is written to", Value: "json-file", Requirement: leaction.Optional},
			{Keyword: keywordPort, Kind: leaction.KindUint, Help: "the first of the ports the run may listen on", Value: "port", Requirement: leaction.Optional},
			{Keyword: keywordTrial, Kind: leaction.KindSwitch, Help: "record an uncommitted tree; the site refuses to publish it", Requirement: leaction.Optional},
		},
		AnswerArgs: runRecord,
	},
)

// Actions answers every action with its exact writes metadata.
func Actions() leaction.List { return actions.Actions() }

// Subs answers the action names for the area's usage line.
func Subs() string { return actions.Subs() }

// Answer runs one action of the area.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runRecord(arguments leaction.Arguments) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, runWait)
	defer cancel()

	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "Ze cannot find the checkout to record the looking glass in",
			"The recording is written into the checkout's website sources. Run the command from the Ze checkout.",
			"cd <ze-checkout> && "+RecorderCommand))
		return nil, 1
	}
	binary, err := recordedBinary(arguments.One(keywordBinary))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	port := portDefault
	if arguments.Has(keywordPort) {
		port, err = strconv.Atoi(arguments.One(keywordPort))
		if err != nil {
			leaction.ReportError(opererr.Wrap(err, fmt.Sprintf("port %q is not a port number", arguments.One(keywordPort)),
				"The simulated speakers and the looking glass listen on consecutive ports from this one, so it is a whole number",
				RecorderCommand+" "+keywordPort+" "+strconv.Itoa(portDefault)))
			return nil, 2
		}
	}
	output := arguments.One(keywordOutput)
	if output == "" {
		output = filepath.Join(root, filepath.FromSlash(File))
	}
	recorded, err := readCheckout(ctx, root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	// Refused here, before the network starts: a run takes minutes, and its
	// file would name a commit that does not hold the code that answered.
	if err := refuseUncommitted(recorded, arguments.Has(keywordTrial)); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	recording, err := recordNetwork(ctx, binary, port, recorded.provenance)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	unchanged, err := writeRecording(output, recording)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	messages := 0
	if recording.Stream != nil {
		messages = len(recording.Stream.Messages)
	}
	return map[string]any{
		"file":            output,
		"unchanged":       unchanged,
		"trial":           recording.Provenance.TreeDirty,
		"tour-steps":      len(recording.Tour),
		"exchanges":       len(recording.Exchanges),
		"assets":          len(recording.Assets),
		"stream-messages": messages,
		"varies":          recording.Provenance.Varies,
	}, 0
}

// recordedBinary answers the ze a run starts: the one at binaryPath, or the
// one PATH holds when binaryPath is empty.
func recordedBinary(binaryPath string) (string, error) {
	if binaryPath != "" {
		absolute, err := filepath.Abs(binaryPath)
		if err != nil {
			return "", opererr.Wrap(err, "The path of the ze to record, "+binaryPath+", cannot be resolved",
				"The path is relative and the current directory cannot be read, so the recorder cannot find the binary",
				RecorderCommand+" binary <absolute-path-to-ze>")
		}
		if _, err := os.Stat(absolute); err != nil {
			return "", opererr.Wrap(err, "The ze to record does not exist",
				"The recorder starts a real ze and asks its looking glass.",
				RecorderCommand+" binary <path-to-ze>")
		}
		return absolute, nil
	}
	found, err := exec.LookPath("ze")
	if err != nil {
		return "", opererr.Wrap(err, "No ze was found to record",
			"The recorder starts a real ze and asks its looking glass, and PATH holds none.",
			RecorderCommand+" binary <path-to-ze>")
	}
	return found, nil
}

// recordStage is the part of a run that failed. Each part has a failure of its
// own, because each has a fix of its own (stageFailure).
type recordStage uint8

const (
	// recordStageUnspecified is the zero value and never a part of a run.
	recordStageUnspecified recordStage = iota
	// recordStageWorkDirectory: the directory ze runs in could not be made.
	recordStageWorkDirectory
	// recordStagePorts: no free ports were found for the run.
	recordStagePorts
	// recordStageNetwork: the simulated network did not come up, or one of the
	// functional test's checks failed, so the looking glass was never asked.
	recordStageNetwork
	// recordStageSubjects: the network holds no route the tour can show.
	recordStageSubjects
	// recordStageTour: the looking glass was asked and one answer failed.
	recordStageTour
)

// stageFailure answers the operator error of a run that failed at stage: what
// failed, why no file was written, and the command that fixes that failure.
// binary is the ze the run started. port is the first port the run listened
// on, or the one it asked for when none was reserved.
func stageFailure(stage recordStage, cause error, binary string, port int) *opererr.Error {
	switch stage {
	case recordStageWorkDirectory:
		return opererr.Wrap(cause, "The recorder cannot create its work directory in "+os.TempDir(),
			"ze runs in a directory of its own, removed with the run, so nothing it writes lands in the checkout",
			"TMPDIR=<writable-directory> "+RecorderCommand)
	case recordStagePorts:
		return opererr.Wrap(cause, fmt.Sprintf("The recorder found no %d free ports from port %d", runner.TestPortSpan, port),
			"The simulated speakers and the looking glass listen on ports of this machine, and other programs hold these",
			RecorderCommand+" "+keywordPort+" <first-free-port>")
	case recordStageNetwork:
		return opererr.Wrap(cause, "The simulated network did not pass its checks against "+binary,
			"The looking glass is asked only after every speaker holds its session and every check of the functional test has passed, "+
				"so no file was written. The fixture below runs the same network alone and names the session or the check that failed",
			fmt.Sprintf("./le test fixture plugin/lg-simulated-network %d %d", port, port+1))
	case recordStageSubjects:
		return opererr.Wrap(cause, "The simulated network holds no route the tour can show",
			"The tour shows one prefix that an external neighbor announces and whose trace crosses "+strconv.Itoa(traceRouters)+
				" routers of the AS. The generated network has none, so no file was written. "+
				"Change the network's parameters in internal/test/fixture/lg_simulated_network.go so that it has one, then record again",
			RecorderCommand)
	case recordStageTour:
		return opererr.Wrap(cause, "The looking glass did not answer the whole tour",
			"A recording holds every answer of the tour and of the requests its pages make, each read twice, so no file was written. "+
				"Correct the question in internal/le/site/lgreplay/tour.go, or the looking glass handler that answers it, then record again. "+
				"The request that failed",
			RecorderCommand)
	case recordStageUnspecified:
		// Only a defect of this package reaches a failure with no stage: the BUG below.
	}
	panic("BUG: sitelgreplay.stageFailure: a run failed at no stage")
}

// endedRun answers the operator error of a run whose context ended, and nil
// for a context still alive. It is asked before stageFailure: a stage that
// fails because the run was stopped has the stop as its cause, and the stage's
// own fix would not help.
func endedRun(ctx context.Context) *opererr.Error {
	cause := ctx.Err()
	if cause == nil {
		return nil
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return opererr.Wrap(cause, "The recording did not finish within "+runWait.String(),
			"A run is bounded so a network that never settles cannot hold the machine, and a machine busy with another heavy job is the usual cause. "+
				"No file was written. Run it again when that job has ended",
			RecorderCommand)
	}
	return opererr.Wrap(cause, "The recording was stopped before it finished",
		"The run received an interrupt, stopped the simulated network and wrote no file",
		RecorderCommand)
}

// recordNetwork runs the simulated network against binary and records its
// looking glass. Every failure it answers is an operator error that names the
// part of the run that failed and that part's fix.
func recordNetwork(ctx context.Context, binary string, port int, provenance Provenance) (Recording, error) {
	// ze runs in a directory of its own, removed with the run, so nothing it
	// writes lands in the checkout.
	workDir, err := os.MkdirTemp("", "ze-lg-replay-")
	if err != nil {
		return Recording{}, stageFailure(recordStageWorkDirectory, err, binary, port)
	}
	defer func() {
		if removeErr := os.RemoveAll(workDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "lg-replay: remove %s: %v\n", workDir, removeErr)
		}
	}()
	ports, _, err := runner.ReservePorts(port, runner.TestPortSpan)
	if err != nil {
		return Recording{}, stageFailure(recordStagePorts, err, binary, port)
	}
	defer ports.Release()

	var recording Recording
	// stage is the part of the run in progress, so a failure names the right
	// one: the network until the fixture hands over, then the part the
	// hand-over reached.
	stage := recordStageNetwork
	options := fixture.LGSimulatedNetworkOptions{
		Binary: binary, WorkDir: workDir, PortFirst: ports.Start, LGPort: ports.Start + 1,
	}
	err = fixture.RunLGSimulatedNetwork(ctx, options, func(ctx context.Context, network fixture.LGSimulatedNetwork) error {
		stage = recordStageSubjects
		values, subjectErr := subjects(network.Topology)
		if subjectErr != nil {
			return subjectErr
		}
		provenance.Seed = network.Params.Seed
		provenance.Speakers = len(network.Topology.Speakers())
		stage = recordStageTour
		var recordErr error
		recording, recordErr = record(ctx, lookingGlass{base: network.Base, wait: waitFor}, resolve(tour, values), provenance)
		return recordErr
	})
	if err != nil {
		if ended := endedRun(ctx); ended != nil {
			return Recording{}, ended
		}
		return Recording{}, stageFailure(stage, err, binary, ports.Start)
	}
	return recording, nil
}

// checkout is what git knows about the checkout a recording is made from.
type checkout struct {
	provenance Provenance
	// uncommitted holds one `git status --porcelain` line for each path that
	// differs from the commit. It is empty exactly when provenance.TreeDirty
	// is false.
	uncommitted []string
}

// readCheckout answers what git knows about the checkout at root.
func readCheckout(ctx context.Context, root string) (checkout, error) {
	commit, err := gitAnswer(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return checkout{}, gitFailure(err, root)
	}
	date, err := gitAnswer(ctx, root, "show", "--no-patch", "--format=%cI", "HEAD")
	if err != nil {
		return checkout{}, gitFailure(err, root)
	}
	// The recording file is this action's own product, not code that answered:
	// a file left by an earlier run does not make the tree uncommitted.
	status, err := gitAnswer(ctx, root, "status", "--porcelain", "--", ".", ":(exclude)"+File)
	if err != nil {
		return checkout{}, gitFailure(err, root)
	}
	if commit == "" {
		return checkout{}, opererr.New("The checkout at "+root+" holds no commit",
			"A recording names the commit as the code that answered, and there is none to name. Commit the tree, then record",
			"git -C "+root+" log -1")
	}
	var uncommitted []string
	if status != "" {
		uncommitted = strings.Split(status, "\n")
	}
	return checkout{
		provenance:  Provenance{Recorder: RecorderCommand, Commit: commit, CommitDate: date, TreeDirty: len(uncommitted) > 0},
		uncommitted: uncommitted,
	}, nil
}

// gitFailure answers the operator error of a git query that failed in root.
// The fix is the query a reader runs to see what git says about the checkout.
func gitFailure(cause error, root string) *opererr.Error {
	return opererr.Wrap(cause, "git could not describe the checkout at "+root,
		"A recording names the checkout's commit and refuses uncommitted changes, and both are read from git. Nothing was started",
		"git -C "+root+" status")
}

// writeFailure answers the operator error of a recording that could not be
// written to path. The run itself succeeded, so the fix names another file.
func writeFailure(cause error, path string) *opererr.Error {
	return opererr.Wrap(cause, "The recording could not be written to "+path,
		"The run finished and its answers exist only in memory, so they are lost and the file is unchanged",
		RecorderCommand+" "+keywordOutput+" <writable-json-file>")
}

// refuseUncommitted refuses to record a checkout that holds uncommitted
// changes, unless the line asked for a trial. A recording names its commit as
// the code that answered, and with uncommitted changes that commit did not.
func refuseUncommitted(recorded checkout, trial bool) error {
	if len(recorded.uncommitted) == 0 {
		return nil
	}
	if trial {
		return nil
	}
	shown := recorded.uncommitted
	more := ""
	if len(shown) > uncommittedShown {
		more = fmt.Sprintf(", and %d more", len(shown)-uncommittedShown)
		shown = shown[:uncommittedShown]
	}
	commit := recorded.provenance.Commit
	if len(commit) > 10 {
		commit = commit[:10]
	}
	return opererr.New(
		fmt.Sprintf("The checkout holds %d uncommitted changes (%s%s), so nothing was recorded",
			len(recorded.uncommitted), strings.Join(shown, "; "), more),
		"A recording names commit "+commit+" as the code that answered, and the website publishes that claim. "+
			"Commit the changes and record again. To try the recorder on this tree instead, record a trial: "+
			"its file says the tree was dirty, and ./le site build refuses to publish it",
		RecorderCommand+" "+keywordTrial)
}

// gitAnswer runs one read-only git query in root and answers its output
// without the surrounding space. The query is bounded by gitWait.
func gitAnswer(ctx context.Context, root string, args ...string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, gitWait)
	defer cancel()
	query := exec.CommandContext(queryCtx, "git", args...) //nolint:gosec // the arguments are literals of this package; root is the checkout lepath.Root found
	query.Dir = root
	// A git status killed at its deadline while it refreshes the index leaves a
	// lock that blocks every session, so the query takes no optional lock.
	query.Env = gitenv.ReadOnly(os.Environ())
	var stderr bytes.Buffer
	query.Stderr = &stderr
	answer, err := query.Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", strings.Join(args, " "), root, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(answer)), nil
}

// writeRecording writes the recording to path and answers whether the file
// already held the same bytes.
func writeRecording(path string, recording Recording) (bool, error) {
	content, err := recording.Encode()
	if err != nil {
		return false, writeFailure(err, path)
	}
	existing, err := os.ReadFile(path) //nolint:gosec // the recording file the caller named
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, writeFailure(err, path)
		}
	}
	if err == nil {
		if bytes.Equal(existing, content) {
			return true, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // a website source directory of the checkout
		return false, writeFailure(err, path)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // a committed website source file
		return false, writeFailure(err, path)
	}
	return false, nil
}
