// VALIDATES: AC-9 -- every registered le area publishes the action table the
// manifest renders, unless it is named on the migration list, and every
// published action is DISPATCHED through that table.
// PREVENTS: an area that reaches the registry with no published grammar, so
// the only way to learn what it takes is to invoke it. A stale migration row
// that outlives the area's own table. An area that publishes a table and reads
// the line with a parser of its own. That last one is how an option reaches an
// action body as data.
package le

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// TestEveryRegisteredAreaProvidesActions holds le to publishing what it
// declares. An area that registers a table is in the manifest with its
// keywords, and `le <area> <verb> --help` renders that grammar rather than the
// area's node page. An area that forwards every word to another program
// (leroot.RegisterForwarding) registers the pass-through declaration
// (leaction.PassesWords) as its table, because the program owns that grammar.
// No area is exempt: le refuses to start while one holds no table
// (leroot.requireEveryAreaHasATable, D-P3-tablecheck), and this test proves the
// set le registers passes that check.
//
// It runs here rather than in package leroot because the registry is populated
// by the blank imports in register.go. In leroot's own test binary no area is
// registered at all, and the same assertion would pass over an empty set.
func TestEveryRegisteredAreaProvidesActions(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}
	for _, tool := range commandsAtStart {
		if _, declared := leroot.ActionsOf(tool.Name); !declared {
			t.Errorf("area %q publishes no grammar: call leroot.RegisterActions(%q, Actions) in its register.go", tool.Name, tool.Name)
		}
	}
}

// probeWord names no action and no keyword in any le area, so every published
// action must refuse it. It is spelled once here because three probes use it
// and a second spelling would let one of them stop being refused.
const probeWord = "zzprobe"

// refusalCode is what leaction answers for a line its grammar cannot accept.
// It is 2 rather than 1, so a caller tells a mistyped line apart from a gate
// that ran and failed (leaction.Area.refuseVerb).
const refusalCode = 2

// TestEveryPublishedVerbIsDispatchedThroughItsOwnTable holds an area to the
// grammar it publishes.
//
// TestEveryRegisteredAreaProvidesActions above asks only
// whether a table EXISTS. An area can declare one, register it for the manifest,
// and then read the line with a parser of its own. Four did.
//
// `worktree` took `update path -xh` as a path. `doc wiring` answered
// `changed-file -xh dry-run` with success over nothing checked. `functional`
// refused `list` as no such action while it published the verb. `test-chaos`
// published `all` and left it out of every refusal, which no hand probe found.
//
// A published table is a claim about what the area accepts, and this test reads
// the claim back. For each published action it builds a REFERENCE area from the
// published rows alone. It sends both the real handler and the reference a line
// the grammar cannot accept. It then requires the same exit code and the same
// refusal from each. An area that hand-parses answers something else, because
// its parser is not the one it published.
//
// Nothing runs. Every probe is refused before an action body is reached, in the
// reference by construction (its bodies panic) and in a compliant area for the
// same reason. An area where a probe DOES run work is the defect this test
// exists to find, and the divergence it produces is what reports it.
func TestEveryPublishedVerbIsDispatchedThroughItsOwnTable(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}

	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		// A pass-through table publishes no grammar to read back: its program
		// reads and refuses every word, so a probe word would start the program
		// (a mock server, a suite). Its declaration is held to account by
		// TestEveryForwardingAreaDeclaresItsGrammarOrPassThrough instead.
		if list.PassesWords {
			continue
		}
		handler := leroot.LookupCommand(tool.Name)
		if handler == nil {
			t.Errorf("area %q publishes a table and registers no handler", tool.Name)
			continue
		}

		// The area's front door. A word that names no action is refused, so a
		// line the published grammar cannot explain never reaches work. Only
		// the CODE is asserted here. An area CAN map a bare line onto one of
		// its verbs, and the refusal then names the keyword rather than the
		// action.
		// A verbless area's first word is its action's own value when the
		// action takes a positional one (D-P4-areaword), so no word names "no
		// action" there: its rows' probes below refuse the lines instead.
		frontDoor := !list.Verbless() || len(list.Actions[0].Parameters) == 0 || list.Actions[0].Parameters[0].Keyword != ""
		// The probe itself stays behind the flag: there the word would be a
		// valid value and the action would run, and nothing here may run.
		if frontDoor {
			if _, code := probeAnswer(judged(t, tool.Name, handler), []string{probeWord}); code != refusalCode {
				t.Errorf("area %q answered %d for a word naming no action, want %d: a line its table cannot read reached the area",
					tool.Name, code, refusalCode)
			}
		}

		reference := referenceArea(t, list)
		for _, row := range list.Actions {
			for _, probe := range probesFor(row) {
				compareRefusal(t, tool.Name, reference, handler, probe)
			}
		}
	}
}

// probesFor builds the lines one published action must refuse. Each shape is a
// refusal the published grammar itself produces (leaction.parseArguments), so
// an area that reads the line with a parser of its own answers one of them in
// its own words.
//
// The shapes are: an undeclared keyword, two of them, an option standing where
// each declared keyword's value goes, a keyword with nothing behind it, and a
// keyword given twice that never declared Repeat. A value never begins with a
// dash (ai/rules/cli.md), so the option shape is refused wherever the keyword
// stands on the line.
//
// A Repeat keyword is left out of the last shape because a second occurrence is
// what it declares it takes: probing it would RUN the action, and every probe
// here is refused before an action body.
//
// An argv keyword (leaction.KindArgv) keeps only the missing-value shape. Every
// word after it is another program's, so a dash-leading word or the keyword
// spelled again is the child's argument, which the table hands on rather than
// refuses: `le job run label x command go test -v` runs `go test -v`.
func probesFor(row leaction.Row) [][]string {
	// A forwarding verb publishes that its words are another program's
	// command line, so the table refuses none of them and no probe applies.
	if row.Forwards {
		return nil
	}
	// A positional value takes the first word after the verb, so a word naming
	// no keyword is a stray only after it: `commit push <words> <probe>`. A
	// dash-leading word in its slot is an option where a value is expected.
	// A verbless action (D-P4-areaword) is typed with no verb word, so every
	// probe line starts at its first keyword.
	head := func(words ...string) []string { return append(strings.Fields(row.Verb), words...) }
	lead := head()
	positional := len(row.Parameters) != 0 && row.Parameters[0].Keyword == ""
	// A positional argv is the whole line (`commit push <words>`), so every
	// word is data and the table refuses none: any probe would RUN the action,
	// and `commit push -probe` would prepare a real push on this checkout. The
	// empty line is the body's refusal, which a reference cannot answer, so the
	// area's own test asserts it (commit TestCommitRefusesWhatItsTableRefuses).
	if positional && row.Parameters[0].Kind == leaction.KindArgv {
		return nil
	}
	if positional {
		lead = append(lead, probeWord+"-value")
	}
	probes := [][]string{
		append(append([]string{}, lead...), probeWord),
		append(append([]string{}, lead...), probeWord, probeWord+"-second"),
	}
	for _, parameter := range row.Parameters {
		if parameter.Keyword == "" {
			probes = append(probes, head("-"+probeWord))
			continue
		}
		if parameter.Kind == leaction.KindArgv {
			probes = append(probes, head(parameter.Keyword))
			continue
		}
		if parameter.Value != "" {
			probes = append(probes,
				head(parameter.Keyword, "-"+probeWord),
				head(parameter.Keyword))
		}
		if parameter.Repeat {
			continue
		}
		twice := head(parameter.Keyword, parameter.Keyword)
		if parameter.Value != "" {
			twice = head(parameter.Keyword, probeWord, parameter.Keyword, probeWord)
		}
		probes = append(probes, twice)
	}
	return probes
}

// compareRefusal requires the area to answer one probe exactly as its own
// published grammar answers it.
//
// Answer and Sweep are both accepted. An area that runs several actions from
// one command line refuses an unknown name through Sweep. An area that runs one
// refuses it through Answer. Both are the table reading the line, and a third
// answer is a parser the area did not publish.
func compareRefusal(t *testing.T, area string, reference leaction.Area, handler command.LocalDataHandler, probe []string) {
	t.Helper()

	gotText, gotCode := probeAnswer(judged(t, area, handler), probe)
	answerText, answerCode := probeAnswer(reference.Answer, probe)
	sweepText, sweepCode := probeAnswer(func(args []string) (any, int) {
		return reference.Sweep(args, leaction.RunEveryAction)
	}, probe)

	if gotCode == answerCode && gotText == answerText {
		return
	}
	if gotCode == sweepCode && gotText == sweepText {
		return
	}
	t.Errorf("area %q does not dispatch %v through its own table:\n got   %d %q\n table %d %q\n sweep %d %q",
		area, probe, gotCode, gotText, answerCode, answerText, sweepCode, sweepText)
}

// judged answers the registered handler of an area as a function of the
// typed words. The registered handler takes judged arguments, and an le tool
// declares no definitions, so the words are judged against none, as leroot.Run
// judges them.
func judged(t *testing.T, area string, handler command.LocalDataHandler) func(args []string) (any, int) {
	t.Helper()
	return func(args []string) (any, int) {
		validated, err := command.ValidateArgs(args, nil, nil)
		if err != nil {
			t.Fatalf("area %q: judging %v against no definitions: %v", area, args, err)
		}
		return handler(validated)
	}
}

// probeAnswer runs one handler over one probe and answers what it wrote to
// stderr with the code it returned. The refusals every le area writes go to
// stderr, so the stream is the only place the answer to a refused line is
// readable.
func probeAnswer(handler func(args []string) (any, int), probe []string) (string, int) {
	read, write, err := os.Pipe()
	if err != nil {
		panic("BUG: le: the probe cannot open a pipe: " + err.Error())
	}
	saved := os.Stderr
	os.Stderr = write

	// The reader runs while the handler writes: a refusal longer than the pipe
	// buffer would otherwise block the handler forever.
	captured := make(chan string, 1)
	go func() {
		var buffer strings.Builder
		io.Copy(&buffer, read) //nolint:errcheck // the pipe closes when the handler is done
		captured <- buffer.String()
	}()

	_, code := handler(probe)

	os.Stderr = saved
	write.Close() //nolint:errcheck // the write end is this function's own
	text := <-captured
	read.Close() //nolint:errcheck // the read end is this function's own
	return text, code
}

// referenceArea builds an area from the rows one area PUBLISHED, and from
// nothing else. Its bodies panic, so a probe that reaches one says the
// reference itself is wrong rather than the area under test.
func referenceArea(t *testing.T, list leaction.List) leaction.Area {
	t.Helper()

	rows := make([]leaction.Action, 0, len(list.Actions))
	for _, row := range list.Actions {
		action := leaction.Action{
			Verb: row.Verb, Does: row.Does, Why: row.Why, Writes: row.Writes, Alone: row.Alone,
			Parameters: row.Parameters,
		}
		switch {
		case row.Forwards:
			action.AnswerWords = func([]string) (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		case len(row.Parameters) == 0:
			action.Answer = func() (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		default:
			action.AnswerArgs = func(leaction.Arguments) (any, int) {
				panic("BUG: le: a probe reached a reference action body")
			}
		}
		rows = append(rows, action)
	}
	return leaction.New(list.Area, rows...)
}

// VALIDATES: AC-1 over every registered action -- each published parameter
// declares a kind and a help line, and an enum lists its values.
// PREVENTS: a keyword the manifest, help, completion or `le help ai` renders
// with no type and no explanation, which sends the reader to the source.
func TestEveryParameterHasAKindAndHelp(t *testing.T) {
	if len(commandsAtStart) == 0 {
		t.Fatal("le registered no local-data command")
	}
	checked := 0
	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		for _, row := range list.Actions {
			for _, parameter := range row.Parameters {
				checked++
				line := "le " + tool.Name + " " + row.Verb + " " + parameter.Keyword
				if parameter.Kind == leaction.KindUnspecified {
					t.Errorf("%s: no kind", line)
				}
				if strings.TrimSpace(parameter.Help) == "" {
					t.Errorf("%s: no help", line)
				}
				if parameter.Kind == leaction.KindEnum && len(parameter.Values) == 0 {
					t.Errorf("%s: an enum with no values", line)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no registered action publishes a parameter, so the check judged nothing")
	}
}

// VALIDATES: R-10 over the real registry -- every action the derived tree
// holds, typed with every keyword its node offers and a value of the
// keyword's kind, passes that action's own parser.
// PREVENTS: completion offering a keyword of a registered area that dispatch
// refuses. internal/le/le/root/tree_test.go proves the walk over a probe; only
// this package links every area.
func TestDerivedTreeLinesAreAcceptedForEveryRegisteredArea(t *testing.T) {
	root := leroot.Tree()
	checked := 0
	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared {
			continue
		}
		// A hidden area is absent from the tree on purpose: completion never
		// offers it (leroot.RegisterHidden).
		if leroot.Hidden(tool.Name) {
			continue
		}
		node := root
		for word := range strings.FieldsSeq(tool.Name) {
			node = node.Children[word]
			if node == nil {
				t.Fatalf("the derived tree has no node for %q", tool.Name)
			}
		}
		for _, row := range list.Actions {
			action := node.Children[row.Verb]
			// A verbless action is the area's own node (D-P4-areaword).
			if list.Verbless() {
				action = node
			}
			if action == nil {
				t.Errorf("le %s %s: absent from the derived tree", tool.Name, row.Verb)
				continue
			}
			if row.Forwards {
				continue
			}
			offered := make(map[string]command.ArgKind, len(action.ArgDefs))
			for _, def := range action.ArgDefs {
				offered[def.Name()] = def.Kind()
			}
			var line []string
			keywords := 0
			for _, parameter := range row.Parameters {
				if parameter.Keyword == "" {
					line = append([]string{sampleOf(parameter)}, line...)
					continue
				}
				if _, held := offered[parameter.Keyword]; !held {
					t.Errorf("le %s %s: the tree does not offer keyword %q", tool.Name, row.Verb, parameter.Keyword)
				}
				keywords++
				line = append(line, parameter.Keyword)
				if parameter.Kind == leaction.KindSwitch {
					continue
				}
				line = append(line, sampleOf(parameter))
				if parameter.Pair != "" {
					line = append(line, sampleOf(parameter))
				}
			}
			if len(offered) != keywords {
				t.Errorf("le %s %s: the tree offers a keyword the action does not declare", tool.Name, row.Verb)
			}
			checked++
			if err := list.Accepts(row.Verb, line); err != nil {
				t.Errorf("le %s %s %s: refused: %v", tool.Name, row.Verb, strings.Join(line, " "), err)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no registered action was checked")
	}
}

// sampleOf answers a value the parameter's kind accepts.
func sampleOf(parameter leaction.Parameter) string {
	value := "x"
	if len(parameter.Values) > 0 {
		value = parameter.Values[0]
	}
	for _, candidate := range []string{value, "1", "1s", "HEAD", "plan/x.md"} {
		if parameter.Kind.Check(candidate, parameter.Values) == nil {
			value = candidate
			break
		}
	}
	return value
}

// VALIDATES: AC-15, AC-19 -- every registered area is either tabled or declares
// that its words pass to another program. A pass-through area's table holds no
// action and names the program (leaction.PassesWords), and `le | json`
// publishes `passes-words` and `passes-to` for exactly those areas, while every
// other area publishes its actions.
// PREVENTS: a forwarding area whose manifest, help and completion say nothing
// about where its words go, so the only way to learn its grammar is to run it.
// Also an area that claims to pass its words on while it publishes actions of
// its own, which would make the dispatcher hand le's help word to a program.
func TestEveryForwardingAreaDeclaresItsGrammarOrPassThrough(t *testing.T) {
	passing := make(map[string]string, 64)
	for _, tool := range commandsAtStart {
		list, declared := leroot.ActionsOf(tool.Name)
		if !declared || !list.PassesWords {
			continue
		}
		if !leroot.Forwards(tool.Name) {
			t.Errorf("area %q declares a pass-through table but leroot.Forwards answers false", tool.Name)
		}
		if list.PassesTo == "" {
			t.Errorf("area %q passes its words on without naming the program", tool.Name)
		}
		if len(list.Actions) != 0 {
			t.Errorf("area %q passes its words on and also declares %d action(s)", tool.Name, len(list.Actions))
		}
		passing[tool.Name] = list.PassesTo
	}
	if len(passing) == 0 {
		t.Fatal("no registered area passes its words to a program, so the registry was not populated")
	}

	var manifest struct {
		Areas []struct {
			Name        string            `json:"name"`
			Actions     []json.RawMessage `json:"actions"`
			PassesWords bool              `json:"passes-words"`
			PassesTo    string            `json:"passes-to"`
		} `json:"areas"`
	}
	out, code := dispatchToStdout(t, "|", "json")
	if code != 0 {
		t.Fatalf("`le | json` answered %d: %s", code, out)
	}
	if err := json.Unmarshal([]byte(out), &manifest); err != nil {
		t.Fatalf("`le | json` is not one JSON document: %v", err)
	}

	for _, area := range manifest.Areas {
		program, passes := passing[area.Name]
		if !passes {
			if area.PassesWords || area.PassesTo != "" {
				t.Errorf("manifest area %q publishes passes-words %v to %q, but its table declares actions", area.Name, area.PassesWords, area.PassesTo)
			}
			if len(area.Actions) == 0 {
				t.Errorf("manifest area %q publishes neither actions nor passes-words", area.Name)
			}
			continue
		}
		delete(passing, area.Name)
		if !area.PassesWords || area.PassesTo != program {
			t.Errorf("manifest area %q publishes passes-words %v to %q, want true to %q", area.Name, area.PassesWords, area.PassesTo, program)
		}
		if len(area.Actions) != 0 {
			t.Errorf("manifest area %q passes its words on and publishes %d action(s)", area.Name, len(area.Actions))
		}
	}
	for name := range passing {
		t.Errorf("the manifest omits pass-through area %q", name)
	}
}

// dispatchToStdout runs one le line through the dispatcher the binary uses and
// answers what it printed on stdout and its exit code. Stdout goes to a file,
// not a pipe, so a page larger than the pipe buffer cannot block the write.
func dispatchToStdout(t *testing.T, args ...string) (string, int) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create the stdout file: %v", err)
	}
	defer file.Close() //nolint:errcheck // a test's temporary file

	saved := os.Stdout
	os.Stdout = file
	code := leroot.Dispatch("le", args)
	os.Stdout = saved

	out, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read the stdout file: %v", err)
	}
	return string(out), code
}
