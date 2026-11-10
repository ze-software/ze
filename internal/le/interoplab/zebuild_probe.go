// Design: docs/architecture/testing/interop.md -- the start probe a staged lab binary passes before any image build
// Related: zebuild.go -- StageBinaries, which runs the probe after each build.
// Related: kernelcheck.go -- stagedBinaryContainer, the one-shot the kernel probe shares.
//
// zebuild_probe.go proves that a staged lab binary starts before any image
// carries it.
//
// On 2026-11-10 every le process panicked at start on a 65-character help
// line (commit c77a8dd2f3). The cross-compile passed, so the preflight passed,
// and every lab helper container that runs le (speaker, relay, injector)
// failed its readiness wait: four BGP scenarios failed one after another, each
// after its image build and its containers, before anyone saw that no scenario
// had reached its checker. The probe runs the binary once, in seconds, and
// refuses the run with the panic in hand.
package interoplab

import (
	"context"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// startProbeTimeout bounds one start probe. A healthy le answers `help` in
// well under a second; the bound covers the first pull of StaticBinaryImage
// on a fresh daemon, which is the same pull the kernel probe bounds at
// kernelProbeTimeout.
const startProbeTimeout = kernelProbeTimeout

// startProbeDirectory is where a probed binary appears inside the probe
// container. The file inside it keeps the binary's declared Name, because
// cmd/ze selects its personality from the name it was started under
// (defaultDispatch): the le build mounted under any other name starts as ze
// and proves nothing about le.
const startProbeDirectory = "/lab"

// startProbeStderrLinesMax is how many leading stderr lines a refusal quotes.
// A Go panic names its cause on the first line and the failing frame within
// the next few; the rest of the trace is the same for every panic at start.
const startProbeStderrLinesMax = 12

// probeBinaryStart runs the start probe binary declares, after its build, and
// writes its verdict and elapsed time to progress. A binary that declares no
// probe is not run: a staged daemon needs a config to start, and DockerKernel
// already runs the staged ze.
func probeBinaryStart(ctx context.Context, docker *Docker, root string, binary LabBinary, progress io.Writer) error {
	if len(binary.StartProbe) == 0 {
		return nil
	}
	// The name becomes the file name inside the container, so a separator in
	// it would mount the binary somewhere its personality name is not.
	if strings.ContainsAny(binary.Name, "/:") {
		var tb textbuf.Buffer
		return errors.New(tb.Str("a lab binary that declares a start probe is mounted under its name, and ").
			Quoted(binary.Name).Str(" holds a path separator or a colon; give it the file name ").
			Str("its personality is selected by, such as le").String())
	}

	target := startProbeDirectory + "/" + binary.Name
	container := stagedBinaryContainer(filepath.Join(root, binary.Output), target, binary.StartProbe, startProbeTimeout)
	started := time.Now()
	result, err := docker.RunOneShot(ctx, container)
	elapsed := time.Since(started).Round(time.Millisecond)

	var line textbuf.Buffer
	line.Str("interop: start probe ").Str(binary.Name).Str(" (`").Str(startProbeArgv(binary)).Str("`): ")
	if err == nil {
		line.Str("pass (").Str(elapsed.String()).Str(")\n")
		writeProgress(progress, &line)
		return nil
	}
	line.Str("FAIL (").Str(elapsed.String()).Str(")\n")
	writeProgress(progress, &line)
	return errors.New(startProbeRefusal(binary, elapsed, result, err))
}

// startProbeArgv answers the probe as a reader would type it: the binary's
// name, then its declared arguments.
func startProbeArgv(binary LabBinary) string {
	return binary.Name + " " + strings.Join(binary.StartProbe, " ")
}

// startProbeLab answers the lab selector a refusal's fix names. A lab stages
// its binaries into its own Docker build context, test/<lab>/, and that
// directory's name is what `./le test integration` takes.
func startProbeLab(binary LabBinary) string {
	return path.Base(path.Dir(filepath.ToSlash(binary.Output)))
}

// startProbeRefusal answers the refusal for a probe that did not exit 0: what
// failed, the exit status or the reason no status exists, the leading stderr
// lines, why it stops the run, and the fix on a line of its own.
func startProbeRefusal(binary LabBinary, elapsed time.Duration, result CommandResult, err error) string {
	var tb textbuf.Buffer
	tb.Str("lab binary ").Str(binary.Name).Str(" (").Str(binary.Output).Str(") did not start: `").
		Str(startProbeArgv(binary)).Str("` in a one-shot ").Str(StaticBinaryImage).Str(" container ")
	if OneShotDidNotExit(err) {
		tb.Str("did not run to an exit after ").Str(elapsed.String()).Str(": ").Err(err).
			Str(".\nWhy: Docker could not run the probe, so whether this binary starts is unproven, ").
			Str("and every container that runs it would meet the same failure.").
			Str("\nFix: correct what the error names (`docker info` shows whether the daemon answers), then run\n  ./le test integration ").
			Str(startProbeLab(binary)).Str(" again")
		return tb.String()
	}
	tb.Str("exited ").Int(int64(result.ExitCode)).Str(" after ").Str(elapsed.String()).
		Str(". Its stderr begins:\n").Str(startProbeStderr(result)).
		Str("\nWhy: every container of this lab that runs ").Str(binary.Name).
		Str(" would fail the same way, so each scenario would fail its readiness wait before it reached its checker.").
		Str("\nFix: correct the start failure the stderr names, then run\n  ./le test integration ").
		Str(startProbeLab(binary)).Str(" again")
	return tb.String()
}

// startProbeStderr answers the first startProbeStderrLinesMax lines of the
// probe's stderr, indented. An empty stderr is named, and stdout is quoted in
// its place, so a refusal never reads as a binary that said nothing wrong.
func startProbeStderr(result CommandResult) string {
	text := strings.TrimSpace(result.Stderr)
	source := "stderr"
	if text == "" {
		text = strings.TrimSpace(result.Stdout)
		source = "stdout, because stderr was empty"
	}
	if text == "" {
		return "  (the binary printed nothing on stderr or stdout)"
	}
	lines := strings.Split(text, "\n")
	var tb textbuf.Buffer
	if source != "stderr" {
		tb.Str("  (").Str(source).Str(")\n")
	}
	for index, line := range lines {
		if index == startProbeStderrLinesMax {
			tb.Str("  ... ").Int(int64(len(lines) - startProbeStderrLinesMax)).Str(" more line(s)\n")
			break
		}
		tb.Str("  ").Str(line).Byte('\n')
	}
	return strings.TrimSuffix(tb.String(), "\n")
}

// writeProgress writes one progress line. A failed write is not a preflight
// failure: the line narrates the run, and the error carries every verdict.
func writeProgress(progress io.Writer, line *textbuf.Buffer) {
	//nolint:errcheck // progress narration; the returned error carries every verdict
	progress.Write(line.Bytes())
}
