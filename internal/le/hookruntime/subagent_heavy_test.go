// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: subagent_heavy.go -- bashSubagentHeavy, the check these tests drive through Run
// Related: session_identity_test.go -- runSubagentBash and the agent's identity line
// Related: ../le/root/heavy.go -- HeavyLine, which reads the probe areas registered here

package hookruntime

import (
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// heavyToolLines is one line per heavy tool form of AC-1 and AC-2: each
// executing verb of the Go toolchain, a launcher and an assignment in front
// of one, a loop and a subshell around one, the linters and the formatters.
var heavyToolLines = []string{
	`go test ./x`,
	`go build ./x`,
	`go vet ./x`,
	// Split so the text the contract test refuses in a le test file
	// (TestNoDevelopmentToolTestShellsOutToGoRun) is not in this one.
	"go " + "run ./cmd/x",
	`go generate ./x`,
	`go install ./x`,
	// The toolchain's own formatter and rewriter, which run over packages.
	`go fmt ./x`,
	`go fix ./x`,
	`timeout 60 go test ./x`,
	`GOFLAGS=-count=1 go test ./x`,
	`env GOFLAGS=-mod=mod go build ./x`,
	`for p in a b; do go vet ./$p; done`,
	`( go test ./x )`,
	`cd internal && go test ./x/`,
	`golangci-lint run`,
	`golangci-lint fmt`,
	`staticcheck ./x`,
	`govulncheck ./x`,
	`gofmt -w x.go`,
	`goimports -w x.go`,
	`gofumpt -w x.go`,
}

// heavyActionLines is one line per way an le line is heavy (AC-4): an area
// registered as admitted (`test bgp`, the real registration, linked by
// bash_test.go), and three probes registered by registerHeavyProbe: an action
// row that declares Heavy, an action registered as expensive, and an area
// with no rows registered heavy. The last three stand in for the census
// declarations of the spec's phase 2, which this package's test binary does
// not link, and for `ai hooks unit`, whose package imports this one.
var heavyActionLines = []string{
	`./le test bgp plugin 42`,
	`./le heavy-probe run`,
	`le heavy-probe run`,
	`ze le heavy-probe run`,
	`bin/ze le heavy-probe run`,
	`/srv/checkout/le heavy-probe run`,
	`./le --name probe heavy-probe run`,
	`./le heavy-probe run 2>&1`,
	`./le heavy-probe audit`,
	`./le heavy-probe-area`,
	`./le heavy-probe-area statistics x`,
}

// heavyProbe registers the probe areas once per process: a command
// registration cannot be withdrawn, and a second run of a test in one
// process (-count) would register each twice.
var heavyProbe sync.Once

// registerHeavyProbe registers `heavy-probe`, an area with a heavy row, a
// light row, a light row registered as expensive and two rows that take a
// command as an argv parameter, `haul` heavy and `carry` light, the
// declaration `job run` makes; and `heavy-probe-area`, an area with no rows
// registered heavy. The registrations stay for the process, as
// bash_test.go's `expensive-probe run` does: leroot exports no way to
// withdraw one, and the names are this file's alone.
func registerHeavyProbe() {
	nothing := func() (any, int) { return nil, 0 }
	answer := func([]string) (any, int) { return nil, 0 }
	carried := func(leaction.Arguments) (any, int) { return nil, 0 }
	meta := func(summary string) registry.Meta {
		return registry.Meta{Description: "A probe area for this test.", ShortHelp: summary, Mode: "offline", Section: registry.SectionTest}
	}
	argv := func() []leaction.Parameter {
		return []leaction.Parameter{
			{Keyword: "command", Kind: leaction.KindArgv, Value: "argv", Requirement: leaction.Required,
				Help: "the command to run: every word after this keyword"},
		}
	}
	probe := leaction.New("heavy-probe",
		leaction.Action{Verb: "run", Summary: "start the probe's heavy work", Why: "A heavy row of the probe.", Heavy: true, Answer: nothing},
		leaction.Action{Verb: "list", Summary: "list what the probe holds", Why: "A light row of the probe.", Answer: nothing},
		leaction.Action{Verb: "audit", Summary: "audit the probe", Why: "A light row registered as expensive.", Answer: nothing},
		leaction.Action{Verb: "haul", Summary: "run a command as heavy work", Why: "A heavy row that carries a command.", Heavy: true,
			Parameters: argv(), AnswerArgs: carried},
		leaction.Action{Verb: "carry", Summary: "run a command", Why: "A light row that carries a command.",
			Parameters: argv(), AnswerArgs: carried},
	)
	leroot.Register("heavy-probe", leroot.GroupGate, answer, meta("a probe with a heavy row"))
	leroot.RegisterActions("heavy-probe", probe.Actions)
	leroot.RegisterExpensive("heavy-probe audit")

	leroot.Register("heavy-probe-area", leroot.GroupGate, answer, meta("a probe whose whole line is heavy"))
	leroot.RegisterForwarding("heavy-probe-area", "the heavy probe program")
	leroot.RegisterHeavy("heavy-probe-area")
}

// handedBack reports whether message holds the hand-back for command: the
// header, and the command on the last line of that refusal. A refusal of
// another kind may print after it, so the line is not the message's suffix.
func handedBack(message, command string) bool {
	if !strings.Contains(message, handBackHeader) {
		return false
	}
	return strings.Contains(message, "the main thread runs it:\n  "+command+"\n")
}

// TestSubagentHeavyCommandIsHandedBack is the wiring test of the check: a
// subagent's Bash payload through the pretool-bash action.
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-1 and AC-2. Each
// executing verb of the Go toolchain, alone, behind a launcher or an
// assignment, inside a loop or a subshell, and each linter and formatter is
// refused for a subagent with exit 2 and the hand-back.
// PREVENTS: a subagent starting a test, a build, a lint or a formatter on
// the machine every session shares.
func TestSubagentHeavyCommandIsHandedBack(t *testing.T) {
	for _, command := range heavyToolLines {
		code, output, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output: %s", command, code, output, message)
		}
		if !handedBack(message, command) {
			t.Errorf("%q: the refusal is not the hand-back of the command: %s", command, message)
		}
	}
}

// TestSubagentHeavyRefusalEndsWithTheCommandToHandBack reads the refusal's
// text for one tool and one le action.
//
// VALIDATES: AC-1 and AC-9, the message: it names what was typed, says the
// main thread runs tests, builds, lint and formatters on the shared machine,
// gives the next step, and ends with the command, nothing after it. It names
// neither the admitted form, the stated-reason escape, a Use line nor a
// build output path.
// PREVENTS: a refusal that teaches a route the subagent is refused too
// (plan/journal/guard-message-teaches-the-violation.md), and a next step
// buried above other text.
func TestSubagentHeavyRefusalEndsWithTheCommandToHandBack(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, row := range []struct{ command, named string }{
		{`go test ./x`, "a subagent does not run `go test`"},
		{`gofmt -w x.go`, "a subagent does not run `gofmt`"},
		{`./le test bgp plugin 42`, "a subagent does not run the `./le` action `./le test bgp`"},
		{`./le heavy-probe run`, "a subagent does not run the `./le` action `./le heavy-probe run`"},
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+row.command)
		if code != 2 {
			t.Fatalf("%q: code=%d, want 2: %s", row.command, code, message)
		}
		lines := strings.Split(strings.TrimRight(message, "\n"), "\n")
		if len(lines) != 5 {
			t.Fatalf("%q: the refusal holds %d lines, want the header, three sentences and the command: %s", row.command, len(lines), message)
		}
		if !strings.Contains(lines[0], row.named) {
			t.Errorf("%q: the first line does not name what was typed (%s): %s", row.command, row.named, lines[0])
		}
		if !strings.Contains(lines[1], "The main thread runs tests, builds, lint and formatters") || !strings.Contains(lines[1], "the machine every session shares") {
			t.Errorf("%q: the refusal does not say why: %s", row.command, lines[1])
		}
		if !strings.HasPrefix(lines[3], "  Next: do not run it.") || !strings.Contains(lines[3], "`Owed commands`") {
			t.Errorf("%q: the refusal does not give the hand-back as its next step: %s", row.command, lines[3])
		}
		if lines[4] != "  "+row.command {
			t.Errorf("%q: the last line = %q, want the command", row.command, lines[4])
		}
		for _, banned := range []string{"./le job run", "ZE_ADMIT_RAW", "Use:", "bin/", "script", ".sh"} {
			if strings.Contains(message, banned) {
				t.Errorf("%q: the refusal names %q: %s", row.command, banned, message)
			}
		}
	}
}

// TestSubagentHeavyRefusalNamesTheResolvedAction reads the heading of the
// refusal for le lines that carry words after their command. Its method: the
// probe areas, and the real `job`, `exec` and `test integration` areas this
// test binary links.
//
// VALIDATES: AC-9, the name: the heading names the command le resolved the
// line to (leroot.HeavyCommand), the area and the action's verb, or the area
// alone where it has no row for the next word, and the last line is still
// the command as typed.
// PREVENTS: a heading that counts three words after `./le` whatever they are
// and names a keyword as the action (`./le job run label`).
func TestSubagentHeavyRefusalNamesTheResolvedAction(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, row := range []struct{ command, named string }{
		// An action row typed with a keyword and its value after the verb.
		{`./le heavy-probe haul command echo hi`, "./le heavy-probe haul"},
		{`./le job run label x command echo hi`, "./le job run"},
		{`./le test unit pkg path ./internal/core/hostload/`, "./le test unit pkg"},
		// An area registered heavy as a whole, typed with trailing words.
		{`./le heavy-probe-area statistics x`, "./le heavy-probe-area"},
		{`./le exec go version`, "./le exec"},
		{`./le test integration all`, "./le test integration"},
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+row.command)
		if code != 2 {
			t.Errorf("%q: code=%d, want 2: %s", row.command, code, message)
			continue
		}
		heading := red + bold + handBackHeader + "the `./le` action `" + row.named + "`" + reset
		if !strings.Contains(message, heading+"\n") {
			t.Errorf("%q: the heading does not name `%s`: %s", row.command, row.named, message)
		}
		if !handedBack(message, row.command) {
			t.Errorf("%q: the refusal does not end with the command as typed: %s", row.command, message)
		}
	}
}

// VALIDATES: AC-3. A stated reason to run raw in front of a heavy command,
// alone or behind a launcher, does not lift a subagent's refusal, and the
// refusal's own words name the hand-back and never the escape; the same
// line still passes for the main thread, the caller the escape exists for.
// PREVENTS: a subagent reading the raw-job guard's advice once and running
// every test under a reason.
func TestSubagentAdmitRawReasonDoesNotLiftTheRefusal(t *testing.T) {
	for _, command := range []string{
		`ZE_ADMIT_RAW="one case" go test ./x`,
		`ZE_ADMIT_RAW='bisecting one case' golangci-lint run`,
		`ZE_ADMIT_RAW=bisect timeout 60 go vet ./x`,
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 2 || !handedBack(message, command) {
			t.Errorf("%q: code=%d, want 2 with the hand-back: %s", command, code, message)
		}
		prose, _, _ := strings.Cut(message, "the main thread runs it:")
		for _, banned := range []string{"ZE_ADMIT_RAW", "./le job run", "Use:"} {
			if strings.Contains(prose, banned) {
				t.Errorf("%q: the refusal's own words name %q: %s", command, banned, message)
			}
		}
		if strings.Contains(message, "run raw, outside job admission") {
			t.Errorf("%q: the raw-job refusal printed beside the hand-back: %s", command, message)
		}
		if code, message := runMainBash(t, command); code != 0 {
			t.Errorf("main thread %q: code=%d, want 0, the escape is the scheduler's: %s", command, code, message)
		}
	}
}

// TestSubagentHeavyActionIsReadFromItsDeclaration is the wiring test of the
// declaration: a subagent's le line, read through leroot.
//
// VALIDATES: AC-4. An le line is refused for a subagent when its area
// registered as admitted or heavy, or its action declared Heavy or
// registered as expensive, under every name the program takes; the light
// actions, the listing and the help of the same area pass.
// PREVENTS: a list of heavy areas in the hook, and a subagent refused the
// bookkeeping action of an area that also holds a heavy one (R-10).
func TestSubagentHeavyActionIsReadFromItsDeclaration(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, command := range heavyActionLines {
		code, output, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output: %s", command, code, output, message)
		}
		if !handedBack(message, command) {
			t.Errorf("%q: the refusal is not the hand-back of the command: %s", command, message)
		}
	}
	for _, command := range []string{
		`./le heavy-probe list`,
		`./le heavy-probe`,
		`./le heavy-probe run --help`,
		`./le heavy-probe --help`,
		`./le help heavy-probe`,
		`./le heavy-probe-area --help`,
		`./le heavy-probe-area statistics --help`,
		`./le heavy-probe-unknown run`,
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 0 || strings.Contains(message, handBackHeader) {
			t.Errorf("%q: code=%d, want 0 and no hand-back: %s", command, code, message)
		}
	}
}

// VALIDATES: AC-4 survivors, AC-5 and AC-6. A subagent's readers, a reader
// line that names a heavy command in a pattern, the Go toolchain's reading
// verbs, the linter's subcommands that run no analysis, help pages, spec and
// scratch bookkeeping, the approve actions and a commit whose body names
// `go test` all pass.
// PREVENTS: the refusal over-matching and blocking a command that runs
// nothing (R-1), which would block every subagent in every session at once.
func TestSubagentLightCommandsPass(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, command := range []string{
		`grep -n 'go test ./x' f.go`,
		`grep -c 'x;gofmt -w y | tail -1' f`,
		`for f in a b; do grep -n "x\|go vet ./y" "$f"; done`,
		`git log --oneline -3`,
		identityLineOfAgent + `grep -n 'go build ./cmd/ze' f`,
		identityLineOfAgent + `gopls symbols x.go`,
		identityLineOfAgent + `go version`,
		identityLineOfAgent + `go env GOFLAGS`,
		identityLineOfAgent + `go list ./x`,
		identityLineOfAgent + `go doc fmt`,
		identityLineOfAgent + `go help test`,
		identityLineOfAgent + `golangci-lint version`,
		identityLineOfAgent + `golangci-lint --verbose linters`,
		identityLineOfAgent + `./le help test`,
		identityLineOfAgent + `./le spec state current`,
		identityLineOfAgent + `./le spec current`,
		identityLineOfAgent + `./le session scratch ensure`,
		identityLineOfAgent + `./le test weakened approve unit x.TestY reason "r"`,
		identityLineOfAgent + `./le rfc approve unit x.TestY reason "r"`,
		identityLineOfAgent + `./le commit create subject "s" body "ran go test ./x" file a.go`,
		identityLineOfAgent + `./le ai hooks unit --help`,
		identityLineOfAgent + `./le heavy-probe audit --help`,
	} {
		code, output, message := runSubagentBash(t, command)
		if strings.Contains(message, handBackHeader) {
			t.Errorf("%q: handed back, want the command to pass: %s", command, message)
		}
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 0 and no output: %s", command, code, output, message)
		}
	}
}

// VALIDATES: AC-7. For the main thread the check adds nothing: no heavy line
// is handed back, a raw go test, go vet and lint are still refused with the
// admitted form, and an admitted le action, a heavy probe action and a
// formatter pass as they did.
// PREVENTS: the scheduler refused the work it schedules.
func TestMainThreadHeavyVerdictsAreUnchanged(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	lines := append(append([]string{}, heavyToolLines...), heavyActionLines...)
	for _, command := range lines {
		_, message := runMainBash(t, command)
		for _, added := range []string{handBackHeader, "Owed commands"} {
			if strings.Contains(message, added) {
				t.Errorf("main thread %q: the message holds %q: %s", command, added, message)
			}
		}
	}
	for _, command := range []string{`go test ./x`, `go vet ./x`, `golangci-lint run`, `timeout 60 go test ./x`, `( go test ./x )`} {
		code, message := runMainBash(t, command)
		if code != 2 || !strings.Contains(message, "run raw, outside job admission") || !strings.Contains(message, "./le job run label ") {
			t.Errorf("main thread %q: code=%d, want the raw-job refusal with the admitted form: %s", command, code, message)
		}
	}
	for _, command := range []string{`./le test bgp plugin 42`, `./le heavy-probe run`, `./le heavy-probe-area statistics x`, `gofmt -w x.go`} {
		if code, message := runMainBash(t, command); code != 0 {
			t.Errorf("main thread %q: code=%d, want 0: %s", command, code, message)
		}
	}
}

// VALIDATES: AC-9. A subagent's heavy line receives exactly one refusal, the
// hand-back: the raw-job guard, the piped-command guard and the build guard
// add nothing for it, so the text holds no Use line, no admitted form and no
// build advice. A refusal of another kind, a destructive git verb on the
// same line, still prints beside it.
// PREVENTS: the hand-back printed beside advice for a form the subagent is
// refused too, which reads as two instructions.
func TestSubagentHeavyLineCarriesOneRefusal(t *testing.T) {
	heavyProbe.Do(registerHeavyProbe)
	for _, command := range []string{
		`go test ./x`,
		`go vet ./x`,
		`golangci-lint run ./x`,
		`go build ./cmd/ze`,
		`go build ./...`,
		`go test ./x | tail -5`,
		`golangci-lint run | head -20`,
		`./le test bgp plugin 42 | grep x`,
		`./le heavy-probe audit | tail -5`,
		// A heavy carrier is no expensive command of its pipeline, so the
		// piped-command guard is silent and the hand-back is still the one
		// refusal: the heavy-work check reads the declaration alone.
		`./le heavy-probe haul command echo x | tail -5`,
		`cd internal && go test ./x/`,
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 2 {
			t.Errorf("%q: code=%d, want 2: %s", command, code, message)
			continue
		}
		trimmed := strings.TrimRight(message, "\n")
		if !strings.HasPrefix(trimmed, red+bold+handBackHeader) || !strings.HasSuffix(trimmed, "\n  "+command) {
			t.Errorf("%q: the message is not the hand-back from its first line to its last: %s", command, message)
		}
		if strings.Count(trimmed, "\n") != 4 {
			t.Errorf("%q: the message holds %d lines, want the five of one hand-back: %s", command, strings.Count(trimmed, "\n")+1, message)
		}
		if strings.Count(strings.ToUpper(trimmed), "BLOCKED") != 1 {
			t.Errorf("%q: the message holds more than one refusal: %s", command, message)
		}
		for _, banned := range []string{"./le job run", "Use:", "ZE_ADMIT_RAW", "-o bin/", "outside job admission", "piped"} {
			if strings.Contains(message, banned) {
				t.Errorf("%q: the message holds %q, another guard's advice: %s", command, banned, message)
			}
		}
	}
	command := `git reset --hard; go test ./x`
	code, _, message := runSubagentBash(t, identityLineOfAgent+command)
	if code != 2 || !strings.Contains(message, "git reset") || !handedBack(message, command) {
		t.Errorf("%q: code=%d, want the git refusal beside the hand-back: %s", command, code, message)
	}
}

// TestSubagentIsNeverAdvisedTheAdmittedForm asks whether a guard that still
// prints its own text to a subagent, on a line the heavy-work check did not
// hand back, advises only what a subagent can do. Its method: every shape of
// such a line the three guards leave. For the piped-command guard that is a
// command it classes as expensive and the heavy-work check does not: each
// linter subcommand that runs no analysis (heavytool.LintRunsNoAnalysis). For the build
// guard it is a line whose text names `go build` in an argument of another
// program (goBuildText). The raw-job guard leaves none: every tool it names
// is handed back, which TestRefusalAdviceIsAcceptedByTheHook proves over its
// corpus. Each line is sent as an agent, the one kind of subagent there is (a
// fork's payload names an agent too), and the command the pipe refusal
// advises is sent back by the same caller.
//
// VALIDATES: spec-main-thread-schedules-heavy-work D-1 and AC-9 for a line
// that is refused and not handed back: the text names neither the admitted
// `./le job run` form, a Use line, the stated-reason escape nor an output
// path; the piped-command refusal names the command with no filter behind
// the caller's identity line, and the hook accepts that command from the
// same caller. The main thread's text for the same line is the one its
// builder wrote, admitted form included.
// PREVENTS: a refusal that teaches a subagent a route the same hook refuses
// (plan/journal/guard-message-teaches-the-violation.md): `./le job run ...
// command golangci-lint linters` advised to a caller `job run` is refused to.
func TestSubagentIsNeverAdvisedTheAdmittedForm(t *testing.T) {
	callers := []struct {
		name     string
		identity string
		run      func(*testing.T, string) (int, string, string)
	}{
		{"agent", identityLineOfAgent, runSubagentBash},
	}
	banned := []string{"./le job run", "Use:", "ZE_ADMIT_RAW", "-o bin/", handBackHeader}
	for _, row := range []struct {
		line   string
		filter string
		piped  string
	}{
		{`golangci-lint linters | head -5`, "head", `golangci-lint linters`},
		{`golangci-lint version | tail -1`, "tail", `golangci-lint version`},
		{`golangci-lint help linters | grep gosec`, "grep", `golangci-lint help linters`},
		{`golangci-lint config path | tail -1`, "tail", `golangci-lint config path`},
		{`golangci-lint cache status | head -3`, "head", `golangci-lint cache status`},
		{`golangci-lint completion bash | head -3`, "head", `golangci-lint completion bash`},
	} {
		for _, caller := range callers {
			code, _, message := caller.run(t, caller.identity+row.line)
			if code != 2 {
				t.Errorf("%s %q: code=%d, want the piped-command refusal: %s", caller.name, row.line, code, message)
				continue
			}
			for _, word := range banned {
				if strings.Contains(message, word) {
					t.Errorf("%s %q: the refusal holds %q, a route a subagent is refused: %s", caller.name, row.line, word, message)
				}
			}
			advised := caller.identity + row.piped
			if !strings.Contains(message, "read its whole output:\n     "+advised+"\n") {
				t.Errorf("%s %q: the refusal does not name the command with no filter, %q: %s", caller.name, row.line, advised, message)
			}
			if !strings.Contains(message, "`Owed commands`") {
				t.Errorf("%s %q: the refusal does not name the main thread's run for a long output: %s", caller.name, row.line, message)
			}
			if code, _, message := caller.run(t, advised); code != 0 {
				t.Errorf("%s %q: the advised command %q is refused (code %d): %s", caller.name, row.line, advised, code, message)
			}
		}
		// The main thread reads the text its builder wrote, admitted form
		// included: nothing of the subagent's text reaches it.
		words := strings.Fields(row.piped)
		want := lossyPipeRefusal("", row.filter, words, words).message
		code, message := runMainBash(t, row.line)
		if code != 2 || !strings.Contains(message, want) {
			t.Errorf("main thread %q: code=%d, want the builder's text %q: %s", row.line, code, want, message)
		}
		if !strings.Contains(want, "./le job run label ") || strings.Contains(message, "Owed commands") {
			t.Errorf("main thread %q: the text lost the admitted form or gained the subagent's: %s", row.line, message)
		}
	}

	// The build guard: `go build` named in the argument of another program.
	for _, line := range []string{
		`ssh buildhost 'go build ./cmd/ze'`,
	} {
		for _, caller := range callers {
			code, _, message := caller.run(t, caller.identity+line)
			if code != 2 {
				t.Errorf("%s %q: code=%d, want the build refusal: %s", caller.name, line, code, message)
				continue
			}
			for _, word := range banned {
				if strings.Contains(message, word) {
					t.Errorf("%s %q: the refusal holds %q, a route a subagent is refused: %s", caller.name, line, word, message)
				}
			}
			if !strings.Contains(message, "main thread runs it:\n     "+line) {
				t.Errorf("%s %q: the refusal does not hand the typed line to the main thread: %s", caller.name, line, message)
			}
		}
		code, message := runMainBash(t, line)
		if code != 2 || !strings.Contains(message, "-o bin/") || strings.Contains(message, "Owed commands") {
			t.Errorf("main thread %q: code=%d, want the build advice with its output path, unchanged: %s", line, code, message)
		}
	}
}
