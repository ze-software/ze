//go:build ze_bgp

package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	editorcli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/exabgp/migration"
	"github.com/ze-software/ze/internal/test/exabgpnative"
	"github.com/ze-software/ze/internal/test/labconfig"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/tmpfs"
)

// TestRepositoryConfigsValidate runs `ze config validate`'s own path
// (runValidation) over every Ze config the repository uses or generates.
//
// VALIDATES: every Ze config the repository carries validates with no error,
// and no EBGP direction is left closed by RFC 8212 unless the test carrying it
// is about RFC 8212. The population is each `.conf` outside vendored and
// cached trees, each `.ci` tmpfs `.conf`, each `.ci` stdin block a command
// hands to ze as a daemon config or a `ze config` input, each fragment a test
// or a demo tape loads into a running config (validated merged, in the order
// it is loaded), each `ze config migrate` input (validated as the command's
// output), and the `ze exabgp migrate` output of every ExaBGP config. Every
// config a carrying test declares refused (refused=<code> on its tmpfs or
// stdin block) must be
// refused with that code.
// PREVENTS: a config no test loads going stale when validation tightens, as
// test/perf/configs/ze.conf did when RFC 8212 enforcement landed.
//
// Every exclusion is a sweepRule derived from a declaration, never a list of
// files: TestSweepDerivesEachExclusion proves each one is what turns its
// findings away.
func TestRepositoryConfigsValidate(t *testing.T) {
	report := runSweep(t, ruleNone, nil)
	report.log(t)
	// R-6: a sweep that finds nothing proves nothing.
	if report.zeConfigs == 0 {
		t.Fatal("the sweep found no Ze config")
	}
	if report.migrated == 0 {
		t.Fatal("the sweep migrated no ExaBGP config")
	}
	if len(report.absentRoots) > 0 {
		t.Errorf("%d configs use a root this test binary was built without; run the test under the feature tag set (./le test unit config):\n  %s",
			len(report.absentRoots), strings.Join(report.absentRoots, "\n  "))
	}
	for _, finding := range report.findings {
		t.Error(finding)
	}
}

// sweepRule is one derived exclusion of the sweep. ruleNone disables none.
type sweepRule int

const (
	ruleNone sweepRule = iota
	// ruleHostVerdict: a diagnostic whose code the registry marks a host
	// verdict judges the machine running the sweep, not the config.
	ruleHostVerdict
	// rulePlaceholders: an interop scenario file is rendered through the
	// lab's own placeholder renderer (internal/test/labconfig).
	rulePlaceholders
	// rulePorts: a .ci body has its port placeholders expanded by the
	// runner's own function (runner.ExpandPorts).
	rulePorts
	// ruleTestPlugins: the test-only plugins the functional ze links
	// (internal/test/plugins/all) are registered.
	ruleTestPlugins
	// ruleDeclaredRefusal: a body its test declares refused is asserted
	// refused instead of accepted.
	ruleDeclaredRefusal
	// ruleLoadSequence: a fragment a test or tape loads into a running config
	// is validated merged onto it, in load order, not alone.
	ruleLoadSequence
	// ruleMigrateInput: a body a test feeds to `ze config migrate` is
	// validated as the command's output.
	ruleMigrateInput
	// ruleNativeClassification: a legacy test/exabgp-compat/native fixture is
	// logged with its reason (internal/test/exabgpnative).
	ruleNativeClassification
	// ruleStatementShape: a body is a Ze config only when its first statement
	// has the shape its schema root declares.
	ruleStatementShape
)

// sweepRecordPort stands for the first port the functional runner leases a test.
const sweepRecordPort = 1790

// sweepLabPrefix stands for the IPv6 prefix an interop lab selects; any valid
// prefix renders the link-local placeholders the lab would.
var sweepLabPrefix = netip.MustParsePrefix("fd00:1e:0::/64")

// sweepSkippedDirs are the directories the sweep never enters: version
// control, vendored and cached module trees (a Go toolchain under
// gokrazy/modcache ships resolv.conf fixtures), session scratch, and test/draft,
// which every repo-wide gate skips.
var sweepSkippedDirs = map[string]bool{
	".git": true, "vendor": true, "modcache": true, "node_modules": true, "tmp": true, "draft": true,
}

// refusedExit matches a .ci expectation of a non-zero exit, the marker of a
// test that feeds Ze a config it must refuse.
var refusedExit = regexp.MustCompile(`(?m)(^expect=exit:code=[1-9]|:exit=[1-9])`)

// rfc8212Expectation matches a `.ci` expectation that names RFC 8212: the
// test asserts what RFC 8212 does to a direction left without policy.
var rfc8212Expectation = regexp.MustCompile(`(?mi)^expect=.*8212`)

// loadCommand matches the editor command that loads a file into the running
// config, in a session-editor script or a terminal demo tape.
var loadCommand = regexp.MustCompile(`load file (?:absolute|relative) (merge|replace) ([^"\s]+)`)

// demoRunStart matches a tape step that starts a demo lab: `ze-demo run <id>
// start` imports <id>/ze.conf as the running config (the lab's base).
var demoRunStart = regexp.MustCompile(`^Type "ze-demo run (\S+) start"`)

// demoSourceRoot is where a terminal demo container mounts the repository
// (demos/terminal/Dockerfile), so a tape's absolute path under it names a
// repository file.
const demoSourceRoot = "/src/"

// sweepCarrier is one config the sweep validates.
type sweepCarrier struct {
	source string
	body   string
	// refused is the diagnostic code the carrying test declares the body
	// refused with, "" when the test says Ze accepts it.
	refused string
	// rfc8212 is true when the test carrying the body is about RFC 8212, so a
	// direction left without policy is the subject under test.
	rfc8212 bool
	// skipped is true when the carrying test expects a non-zero exit and
	// declares nothing: it carries a config Ze must refuse.
	skipped bool
	// migrateInput is true when the carrying test feeds the body to
	// `ze config migrate`.
	migrateInput bool
}

// loadStep is one fragment a sequence loads into the running config.
type loadStep struct {
	source  string
	body    string
	replace bool
	refused string
}

// loadSequence is a base config and the fragments loaded onto it in order.
type loadSequence struct {
	source string
	base   string
	steps  []loadStep
}

// sweepReport is what one sweep found and counted.
type sweepReport struct {
	findings     []string
	absentRoots  []string
	zeConfigs    int
	migrated     int
	refused      int
	skipped      int
	sequences    int
	steps        int
	legacyNative []string
	hostVerdicts map[string]int
}

func (r *sweepReport) find(text string) {
	r.findings = append(r.findings, text)
}

// log states the counts AC-1 asks the sweep to show.
func (r *sweepReport) log(t *testing.T) {
	t.Helper()
	codes := make([]string, 0, len(r.hostVerdicts))
	for code, n := range r.hostVerdicts {
		codes = append(codes, code+"="+strconv.Itoa(n))
	}
	slices.Sort(codes)
	t.Logf("validated %d Ze configs and %d migrated ExaBGP configs; %d declared refused and asserted refused; "+
		"%d skipped as refused by an exit expectation; %d load sequences (%d steps); %d legacy native fixtures skipped; "+
		"host verdicts skipped: [%s]",
		r.zeConfigs, r.migrated, r.refused, r.skipped, r.sequences, r.steps, len(r.legacyNative), strings.Join(codes, " "))
	for _, legacy := range r.legacyNative {
		t.Logf("legacy native fixture, not validated: %s", legacy)
	}
}

// sweepContext carries what one sweep run reads.
type sweepContext struct {
	t      *testing.T
	root   string
	schema *config.Schema
	off    sweepRule
	report *sweepReport
}

func (s *sweepContext) on(rule sweepRule) bool { return s.off != rule }

// sweepKeep narrows a sweep to the files whose path or text holds one of its
// tokens, so a test that sweeps once per rule judges only the files the rule
// can concern. A nil sweepKeep keeps every file.
type sweepKeep []string

// keeps reports whether the file at path, holding text, stays in the sweep.
func (k sweepKeep) keeps(path string, text []byte) bool {
	if k == nil {
		return true
	}
	for _, token := range k {
		if strings.Contains(path, token) || bytes.Contains(text, []byte(token)) {
			return true
		}
	}
	return false
}

// runSweep walks the repository and validates every carrier of the files
// keep keeps, with the rule off disabled (ruleNone disables none).
func runSweep(t *testing.T, off sweepRule, keep sweepKeep) *sweepReport {
	t.Helper()
	diagnostic.ResetForTest()
	diagnostic.RegisterBuiltinCodes()
	if off == ruleTestPlugins {
		withoutTestPlugins(t)
	}
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	s := &sweepContext{
		t: t, root: filepath.Join("..", "..", "..", ".."), schema: schema, off: off,
		report: &sweepReport{hostVerdicts: map[string]int{}},
	}

	var confs, cis, tapes []string
	err = filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if sweepSkippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".conf", ".ci", ".tape":
		default:
			return nil
		}
		if keep != nil {
			text, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if !keep.keeps(path, text) {
				return nil
			}
		}
		switch filepath.Ext(path) {
		case ".conf":
			confs = append(confs, path)
		case ".ci":
			cis = append(cis, path)
		case ".tape":
			tapes = append(tapes, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", s.root, err)
	}

	loaded := map[string]bool{}
	var sequences []loadSequence
	for _, tape := range tapes {
		sequences = append(sequences, s.tapeSequences(tape, loaded)...)
	}
	var carriers []sweepCarrier
	for _, path := range confs {
		if loaded[filepath.Clean(path)] && s.on(ruleLoadSequence) {
			continue
		}
		if c, ok := s.confCarrier(path); ok {
			carriers = append(carriers, c)
		}
	}
	for _, path := range cis {
		ciCarriers, ciSequences := s.ciCarriers(path)
		carriers = append(carriers, ciCarriers...)
		sequences = append(sequences, ciSequences...)
	}
	if !s.on(ruleLoadSequence) {
		sequences = nil
	}

	for i := range carriers {
		s.judgeCarrier(&carriers[i])
	}
	for i := range sequences {
		s.judgeSequence(&sequences[i])
	}
	return s.report
}

// confCarrier reads one `.conf` file: a legacy native fixture is logged with
// its reason, and an interop scenario file is rendered as its lab renders it.
func (s *sweepContext) confCarrier(path string) (sweepCarrier, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.t.Fatalf("read %s: %v", path, err)
	}
	if s.on(ruleNativeClassification) {
		relative, relErr := filepath.Rel(s.root, path)
		if relErr == nil && filepath.ToSlash(filepath.Dir(relative)) == exabgpnative.Dir {
			class := exabgpnative.Classify(filepath.Base(path))
			switch class.Kind {
			case exabgpnative.KindLegacy:
				s.report.legacyNative = append(s.report.legacyNative, path+": "+class.Reason)
				return sweepCarrier{}, false
			case exabgpnative.KindUnclassified:
				s.report.find(path + ": unclassified " + exabgpnative.Dir + " fixture; classify it in internal/test/exabgpnative")
				return sweepCarrier{}, false
			case exabgpnative.KindCurated, exabgpnative.KindPluginOnly:
			}
		}
	}
	body := string(data)
	if s.on(rulePlaceholders) {
		rendered, renderErr := s.renderScenario(path, body)
		if renderErr != nil {
			s.report.find(path + ": " + renderErr.Error())
			return sweepCarrier{}, false
		}
		body = rendered
	}
	return sweepCarrier{source: path, body: body, rfc8212: directoryIsAboutRFC8212(path)}, true
}

// renderScenario fills the placeholders an interop lab fills before a run.
func (s *sweepContext) renderScenario(path, body string) (string, error) {
	rendered, _, err := labconfig.RenderLinkLocals(body, sweepLabPrefix)
	if err != nil {
		return "", err
	}
	return labconfig.RenderPKIPlaceholders(rendered, labconfig.FindPKIDir(s.root, filepath.Dir(path)))
}

// ciCarriers reads the configs one `.ci` carries, through the runner's own
// parse, so a block is a config exactly when the runner hands it to ze as one.
func (s *sweepContext) ciCarriers(path string) ([]sweepCarrier, []loadSequence) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	parsed, err := tmpfs.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, nil //nolint:nilerr // A .ci the tmpfs reader refuses carries no body; its own suite fails it.
	}
	expand := func(body string) string {
		if s.on(rulePorts) {
			return runner.ExpandPorts(body, sweepRecordPort)
		}
		return body
	}
	skipped := refusedExit.MatchString(text)
	rfc8212 := rfc8212Expectation.MatchString(text)

	use := ciCommandUse(path, parsed)
	files := map[string]*tmpfs.File{}
	for _, f := range parsed.Files {
		files[f.Path] = f
	}

	var sequences []loadSequence
	consumed := map[string]bool{}
	if seq, ok := ciLoadSequence(path, parsed, files, use, expand); ok && s.on(ruleLoadSequence) {
		sequences = append(sequences, seq)
		for _, step := range seq.steps {
			consumed[strings.TrimPrefix(step.source, path+":")] = true
		}
	}
	var carriers []sweepCarrier
	for _, f := range parsed.Files {
		if !strings.HasSuffix(f.Path, ".conf") || consumed[f.Path] {
			continue
		}
		c := sweepCarrier{source: path + ":" + f.Path, body: expand(string(f.Content)), rfc8212: rfc8212, skipped: skipped}
		if s.on(ruleDeclaredRefusal) {
			c.refused = f.Refused
		}
		if s.on(ruleMigrateInput) {
			c.migrateInput = use.migrateInputs[f.Path]
		}
		carriers = append(carriers, c)
	}
	names := make([]string, 0, len(use.configBlocks))
	for name := range use.configBlocks {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		body, ok := parsed.StdinBlocks[name]
		if !ok {
			continue
		}
		c := sweepCarrier{source: path + ":stdin=" + name, body: expand(string(body)), rfc8212: rfc8212, skipped: skipped}
		if s.on(ruleDeclaredRefusal) {
			c.refused = parsed.StdinRefused[name]
		}
		carriers = append(carriers, c)
	}
	return carriers, sequences
}

// ciUse is what a `.ci`'s commands do with its blocks and files.
type ciUse struct {
	// configBlocks names each stdin block a command hands to ze as a daemon
	// config or as the input of `ze config <verb> -`.
	configBlocks map[string]bool
	// baseFile is the tmpfs file a ze daemon starts on, "" when none.
	baseFile string
	// baseBlock is the stdin block a ze daemon starts on, "" when none.
	baseBlock string
	// migrateInputs names each tmpfs file `ze config migrate` reads.
	migrateInputs map[string]bool
}

// ciCommandUse reads the `.ci`'s commands through the runner's own parse and
// its own rule for which operand a ze invocation reads as its config.
func ciCommandUse(path string, parsed *tmpfs.Tmpfs) ciUse {
	use := ciUse{configBlocks: map[string]bool{}, migrateInputs: map[string]bool{}}
	// A stdin=config block is a config by the name the .ci format gives it,
	// whichever route hands it to ze: a wrapped argv (le test fail-syscall
	// ... -- ze -), a pipeline inside ze cli -c, a leading global flag, or the
	// parse suite's command-less validate (runLegacyTest).
	if _, ok := parsed.StdinBlocks["config"]; ok {
		use.configBlocks["config"] = true
	}
	rec, err := runner.ParseCIFile(path)
	if err != nil || rec == nil {
		return use // A .ci the runner refuses fails its own suite.
	}
	for _, cmd := range rec.RunCommands {
		words, splitErr := runner.SplitCommand(runner.ExpandPorts(cmd.Exec, sweepRecordPort))
		if splitErr != nil || len(words) == 0 || words[0] != "ze" {
			continue
		}
		args := words[1:]
		if index := runner.DaemonConfigArg(args); index >= 0 {
			noteDaemonConfig(&use, args[index], cmd.Stdin)
			continue
		}
		if len(args) < 2 || args[0] != "config" {
			continue
		}
		if args[1] == "migrate" {
			for _, arg := range args[2:] {
				if strings.HasSuffix(arg, ".conf") {
					use.migrateInputs[arg] = true
				}
			}
			continue
		}
		if cmd.Stdin != "" && slices.Contains(args, "-") {
			use.configBlocks[cmd.Stdin] = true
		}
	}
	return use
}

// noteDaemonConfig records the config operand of a ze daemon invocation: a
// stdin block when the operand is "-", else a file. The first daemon a test
// starts gives the running config its editor scripts load into.
func noteDaemonConfig(use *ciUse, operand, stdin string) {
	first := use.baseFile == "" && use.baseBlock == ""
	if operand != "-" {
		if first {
			use.baseFile = operand
		}
		return
	}
	if stdin == "" {
		return
	}
	use.configBlocks[stdin] = true
	if first {
		use.baseBlock = stdin
	}
}

// ciLoadSequence finds the fragments a `.ci`'s editor scripts load into the
// daemon's running config, in the order the scripts send them.
func ciLoadSequence(path string, parsed *tmpfs.Tmpfs, files map[string]*tmpfs.File, use ciUse, expand func(string) string) (loadSequence, bool) {
	seq := loadSequence{source: path}
	switch {
	case use.baseFile != "":
		if f, ok := files[use.baseFile]; ok {
			seq.base = expand(string(f.Content))
		}
	case use.baseBlock != "":
		seq.base = expand(string(parsed.StdinBlocks[use.baseBlock]))
	}
	for _, f := range parsed.Files {
		if strings.HasSuffix(f.Path, ".conf") {
			continue
		}
		for line := range strings.SplitSeq(string(f.Content), "\n") {
			match := loadCommand.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			fragment, ok := files[match[2]]
			if !ok {
				continue
			}
			seq.steps = append(seq.steps, loadStep{
				source: path + ":" + fragment.Path, body: expand(string(fragment.Content)),
				replace: match[1] == "replace", refused: fragment.Refused,
			})
		}
	}
	return seq, len(seq.steps) > 0
}

// tapeSequences reads a top-level demo tape (one that writes an Output) with
// every tape it sources, in order, and answers the load sequences it runs:
// the lab a `ze-demo run <id> start` step starts gives the base, and each
// `load file` step loads a repository fragment onto the running config.
// Every fragment a tape loads is recorded in loaded.
func (s *sweepContext) tapeSequences(tape string, loaded map[string]bool) []loadSequence {
	data, err := os.ReadFile(tape)
	if err != nil {
		s.t.Fatalf("read %s: %v", tape, err)
	}
	if !bytes.HasPrefix(data, []byte("Output ")) && !bytes.Contains(data, []byte("\nOutput ")) {
		return nil
	}
	var lines []string
	s.expandTape(tape, filepath.Dir(filepath.Dir(tape)), 0, &lines)

	var sequences []loadSequence
	current := -1
	unreadLab := "" // a lab started with no readable ze.conf, "" when none
	for _, line := range lines {
		if match := demoRunStart.FindStringSubmatch(line); match != nil {
			base, readErr := os.ReadFile(filepath.Join(s.root, "demos", "terminal", match[1], "ze.conf"))
			if readErr != nil {
				// A lab may start from a config its scenario builds; only a
				// load onto it needs the base, so the load reports it. The
				// loads after it never go onto the previous lab's config.
				unreadLab = match[1]
				current = -1
				continue
			}
			unreadLab = ""
			sequences = append(sequences, loadSequence{source: tape + " (" + match[1] + ")", base: string(base)})
			current = len(sequences) - 1
			continue
		}
		match := loadCommand.FindStringSubmatch(line)
		if match == nil || !strings.HasPrefix(match[2], demoSourceRoot) {
			continue
		}
		fragmentPath := filepath.Join(s.root, filepath.FromSlash(strings.TrimPrefix(match[2], demoSourceRoot)))
		loaded[filepath.Clean(fragmentPath)] = true
		if current < 0 {
			if unreadLab != "" {
				s.report.find(tape + ": loads " + match[2] + " onto lab " + unreadLab + ", whose ze.conf the sweep cannot read")
				continue
			}
			s.report.find(tape + ": loads " + match[2] + " before any `ze-demo run <lab> start` gives it a running config")
			continue
		}
		body, readErr := os.ReadFile(fragmentPath)
		if readErr != nil {
			s.report.find(tape + ": loads " + match[2] + ", which does not exist")
			continue
		}
		sequences[current].steps = append(sequences[current].steps,
			loadStep{source: fragmentPath, body: string(body), replace: match[1] == "replace"})
	}
	return sequences
}

// expandTape appends tape's lines to lines with each `Source <file>` replaced
// by the sourced tape's lines. A sourced path is relative to runDir, the
// directory the top-level tape's recorder runs in. Recursion is bounded by
// depth: a source chain deeper than eight tapes is a loop.
func (s *sweepContext) expandTape(tape, runDir string, depth int, lines *[]string) {
	if depth > 8 {
		s.report.find(tape + ": Source chain deeper than eight tapes")
		return
	}
	data, err := os.ReadFile(tape)
	if err != nil {
		s.report.find(tape + ": " + err.Error())
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		sourced, ok := strings.CutPrefix(trimmed, "Source ")
		if !ok {
			*lines = append(*lines, trimmed)
			continue
		}
		s.expandTape(filepath.Join(runDir, sourced), runDir, depth+1, lines)
	}
}

// judgeCarrier classifies one body and validates it.
func (s *sweepContext) judgeCarrier(c *sweepCarrier) {
	root, isZe := s.zeRoot(c.body)
	if !isZe {
		if requirements, gated := registry.GatedConfigRootRequirements(root); gated && s.schema.Get(root) == nil {
			s.report.absentRoots = append(s.report.absentRoots, c.source+" (root "+root+" needs "+joinRequirements(requirements)+")")
			return
		}
		if body, ok := migrateExaBGP(c.body); ok {
			s.report.migrated++
			s.checkBody(sweepCarrier{source: "migrate " + c.source, body: body, rfc8212: c.rfc8212})
		}
		return
	}
	s.report.zeConfigs++
	if c.refused != "" {
		s.report.refused++
		s.checkRefused(c.source, c.body, c.refused)
		return
	}
	if c.skipped {
		s.report.skipped++
		return
	}
	if c.migrateInput {
		output, err := s.migrateConfig(c.body)
		if err != nil {
			s.report.find(c.source + ": ze config migrate: " + err.Error())
			return
		}
		s.checkBody(sweepCarrier{source: "config migrate " + c.source, body: output, rfc8212: c.rfc8212})
		return
	}
	s.checkBody(*c)
}

// judgeSequence validates the running config after each step of a sequence.
// The steps load through the editor's own LoadMerge and LoadReplace, so the
// running config after a step is the one an operator's `load file` leaves.
func (s *sweepContext) judgeSequence(seq *loadSequence) {
	s.report.sequences++
	for i := range seq.steps {
		step := &seq.steps[i]
		s.report.steps++
		source := step.source + " (loaded by " + seq.source + ")"
		running := s.runningAfter(seq, i)
		if step.refused != "" && s.on(ruleDeclaredRefusal) {
			s.report.refused++
			body := step.body
			if loaded, loadErr := s.load(running, step); loadErr == nil {
				body = loaded
			}
			s.checkRefused(source, body, step.refused)
			continue
		}
		loaded, err := s.load(running, step)
		if err != nil {
			s.report.find(source + ": " + err.Error())
			continue // Reported; later steps load onto the config before it.
		}
		s.checkBody(sweepCarrier{source: source, body: loaded})
	}
}

// runningAfter answers the running config once the steps before index have
// loaded: a declared refused step, or one the editor refuses (reported at its
// own index), leaves it unchanged, as the editor does.
func (s *sweepContext) runningAfter(seq *loadSequence, index int) string {
	running := seq.base
	for i := range index {
		step := &seq.steps[i]
		if step.refused != "" && s.on(ruleDeclaredRefusal) {
			continue
		}
		if loaded, err := s.load(running, step); err == nil {
			running = loaded
		}
	}
	return running
}

// load answers the config an editor holding running holds after it loads
// the step's fragment, merging or replacing as the step says.
func (s *sweepContext) load(running string, step *loadStep) (string, error) {
	editor, err := editorcli.NewEditorFromContent([]byte(running), "-")
	if err != nil {
		return "", err
	}
	fragment, err := parseAnyFormat(s.schema, step.body)
	if err != nil {
		return "", errors.New("config-parse: " + err.Error())
	}
	if step.replace {
		err = editor.LoadReplace(nil, fragment)
	} else {
		err = editor.LoadMerge(nil, fragment)
	}
	if err != nil {
		return "", err
	}
	return editor.WorkingContent(), nil
}

// parseAnyFormat parses a body in whichever format it is written, as
// `ze config migrate` does.
func parseAnyFormat(schema *config.Schema, body string) (*config.Tree, error) {
	format := config.DetectFormat(body)
	if format == config.FormatSet || format == config.FormatSetMeta {
		return config.NewSetParser(schema).Parse(body)
	}
	return config.NewParser(schema).Parse(body)
}

// migrateConfig answers `ze config migrate`'s output for body.
func (s *sweepContext) migrateConfig(body string) (string, error) {
	input := filepath.Join(s.t.TempDir(), "input.conf")
	if err := os.WriteFile(input, []byte(body), 0o600); err != nil {
		return "", err
	}
	output, _, _, err := configMigrateWithWarnings(input, "", formHierarchical)
	return output, err
}

// checkBody validates one body and records each error and each RFC 8212
// absence the carrying test is not about.
func (s *sweepContext) checkBody(c sweepCarrier) {
	result := runValidation(c.body, c.source)
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		if s.on(ruleHostVerdict) && isHostVerdict(d.Code) {
			s.report.hostVerdicts[d.Code]++
			continue
		}
		switch {
		case d.Severity == diagnostic.SeverityError:
			s.report.find(c.source + ": " + d.Code + ": " + lineMessage(d))
		case d.Code == rfc8212WarningCode && !c.rfc8212:
			s.report.find(c.source + ": " + d.Code + ": " + d.Message)
		}
	}
}

// checkRefused asserts a body its test declares refused is refused with the
// declared code.
func (s *sweepContext) checkRefused(source, body, code string) {
	result := runValidation(body, source)
	var codes []string
	for i := range result.Diagnostics {
		d := &result.Diagnostics[i]
		if d.Severity != diagnostic.SeverityError {
			continue
		}
		if d.Code == code {
			return
		}
		codes = append(codes, d.Code)
	}
	if len(codes) == 0 {
		s.report.find(source + ": declared refused=" + code + ", and ze config validate accepts it")
		return
	}
	s.report.find(source + ": declared refused=" + code + ", and ze config validate refuses it with " + strings.Join(codes, ", "))
}

// isHostVerdict answers whether the registry marks code a verdict about the
// host running the command rather than about the config.
func isHostVerdict(code string) bool {
	meta := diagnostic.Lookup(code)
	return meta != nil && meta.HostVerdict
}

// zeRoot answers the first top-level keyword of a body and whether the body
// is a Ze config: the keyword is a schema root and, unless the shape rule is
// off, the first statement has the shape that root declares.
func (s *sweepContext) zeRoot(body string) (string, bool) {
	root, words, opener := firstStatement(body)
	node := s.schema.Get(root)
	if node == nil {
		return root, false
	}
	if !s.on(ruleStatementShape) || words == 0 {
		return root, true
	}
	return root, statementFitsRoot(node.Kind(), words, opener)
}

// statementFitsRoot answers whether a first statement of words words ended by
// opener ('{', ';', or 0 at the end of the body) is a statement of a schema
// root of this kind: a container opens a block on its own name, a list on its
// name and one key. Other kinds take inline values, so any statement fits.
func statementFitsRoot(kind config.NodeKind, words int, opener byte) bool {
	switch kind {
	case config.NodeContainer:
		return words == 1 && opener == '{'
	case config.NodeList:
		return words == 2 && opener == '{'
	case config.NodeLeaf, config.NodeFreeform, config.NodeFlex, config.NodeInlineList:
		return true
	}
	return true
}

// firstStatement answers the first top-level keyword of a body, the number of
// words its first statement holds before '{' or ';' (0 for the set form), and
// the byte that ends it (0 when the body ends first).
func firstStatement(body string) (string, int, byte) {
	var words []string
	for raw := range strings.SplitSeq(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if len(words) == 0 {
			if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "set" {
				return strings.Trim(fields[1], "{;"), 0, 0
			}
		}
		end := strings.IndexAny(line, "{;")
		if end < 0 {
			words = append(words, strings.Fields(line)...)
			continue
		}
		words = append(words, strings.Fields(line[:end])...)
		if len(words) == 0 {
			return "", 0, line[end]
		}
		return words[0], len(words), line[end]
	}
	if len(words) == 0 {
		return "", 0, 0
	}
	return words[0], len(words), 0
}

// lineMessage prefixes a diagnostic's message with its line when it has one.
func lineMessage(d *diagnostic.Diagnostic) string {
	if d.Line > 0 {
		return "line " + strconv.Itoa(d.Line) + ": " + d.Message
	}
	return d.Message
}

// migrateExaBGP answers `ze exabgp migrate`'s output for an ExaBGP config, by
// the three calls the command makes, and false for text that is not one.
func migrateExaBGP(body string) (string, bool) {
	tree, err := migration.ParseExaBGPConfig(body)
	if err != nil {
		return "", false
	}
	result, err := migration.MigrateFromExaBGP(tree)
	if err != nil {
		return "", false
	}
	return migration.SerializeTree(result.Tree), true
}

// joinRequirements renders gated root requirements as "ze_a+ze_b or ze_c".
func joinRequirements(requirements [][]string) string {
	alternatives := make([]string, 0, len(requirements))
	for _, tags := range requirements {
		alternatives = append(alternatives, strings.Join(tags, "+"))
	}
	return strings.Join(alternatives, " or ")
}

// directoryIsAboutRFC8212 reports whether the scenario holding a `.conf` is
// about RFC 8212 by its own name, as bgp-rfc8212-default-reject-frr is.
func directoryIsAboutRFC8212(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(filepath.Dir(path))), "8212")
}

// TestSweepDerivesEachExclusion turns each sweep rule off in turn.
//
// VALIDATES: every exclusion the sweep makes is the rule's doing: with the rule
// off, findings return that name what the rule covers (an interop scenario's
// placeholder, a $PORT, a test-only plugin, a declared refusal, a loaded
// fragment, a migrate input, a legacy native fixture, an FRR vtysh.conf).
// PREVENTS: a rule that turns nothing away, or findings that vanish for a
// reason no rule states.
func TestSweepDerivesEachExclusion(t *testing.T) {
	cases := []struct {
		name   string
		rule   sweepRule
		keep   sweepKeep
		marker string
	}{
		{"placeholders", rulePlaceholders, sweepKeep{"test/interop"}, "test/interop"},
		{"ports", rulePorts, sweepKeep{"$PORT"}, "$PORT"},
		{"test plugins", ruleTestPlugins, sweepKeep{"fakeredist", "fakedrain"}, "fakeredist"},
		{"declared refusal", ruleDeclaredRefusal, sweepKeep{"refused="}, "test/reload/reload-bad-config.ci"},
		{"load sequence", ruleLoadSequence, sweepKeep{"demos/terminal"}, "demos/terminal/topics/"},
		{"migrate input", ruleMigrateInput, sweepKeep{"config migrate"}, "test/parse/policy-root-migrate.ci"},
		{"native classification", ruleNativeClassification, sweepKeep{exabgpnative.Dir}, exabgpnative.Dir},
		{"statement shape", ruleStatementShape, sweepKeep{"vtysh"}, "vtysh.conf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireRuleTurnsFindingsAway(t, tc.rule, tc.keep, tc.marker)
		})
	}
	t.Run("host verdict", func(t *testing.T) {
		// On the host the sweep runs on, the configs carry no node of another
		// OS, so the schema is built as an OS that prunes the Linux-only nodes
		// (ze:os "linux"): their findings are host verdicts with the rule on,
		// and return with it off.
		restore := config.SetSchemaOSForTest("plan9")
		defer restore()
		requireRuleTurnsFindingsAway(t, ruleHostVerdict, sweepKeep{"xfrm", "pppoe-client", "mirror"}, diagnostic.CodeConfigNodeOtherOS)
	})
}

// requireRuleTurnsFindingsAway sweeps the files keep keeps with every rule on,
// then with rule off, and fails unless findings naming marker return.
func requireRuleTurnsFindingsAway(t *testing.T, rule sweepRule, keep sweepKeep, marker string) {
	t.Helper()
	baseline := map[string]bool{}
	for _, finding := range runSweep(t, ruleNone, keep).findings {
		baseline[finding] = true
	}
	var returned []string
	for _, finding := range runSweep(t, rule, keep).findings {
		if !baseline[finding] && strings.Contains(finding, marker) {
			returned = append(returned, finding)
		}
	}
	if len(returned) == 0 {
		t.Fatalf("with the rule off, no finding naming %q returned: the rule turns nothing away", marker)
	}
	t.Logf("with the rule off, %d findings return, first: %s", len(returned), returned[0])
}

// TestTapeSequencesStartFreshPerLab reads a demo tape that starts two labs,
// the second without a ze.conf.
//
// VALIDATES: a `ze-demo run <lab> start` whose lab has no readable ze.conf is
// a finding, and the loads after it attach to no earlier lab's sequence.
// PREVENTS: a fragment loaded after a lab the sweep could not read being
// validated merged onto the previous lab's running config.
func TestTapeSequencesStartFreshPerLab(t *testing.T) {
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("demos/terminal/good/ze.conf", "bgp {\n}\n")
	write("demos/terminal/topics/a.conf", "bgp {\n}\n")
	write("demos/terminal/topics/b.conf", "bgp {\n}\n")
	write("demos/terminal/show/demo.tape", "Output x.webm\n"+
		"Type \"ze-demo run good start\"\n"+
		"Type \"load file absolute merge /src/demos/terminal/topics/a.conf\"\n"+
		"Type \"ze-demo run missing start\"\n"+
		"Type \"load file absolute merge /src/demos/terminal/topics/b.conf\"\n")

	s := &sweepContext{t: t, root: root, report: &sweepReport{}}
	sequences := s.tapeSequences(filepath.Join(root, "demos/terminal/show/demo.tape"), map[string]bool{})
	if len(sequences) != 1 {
		t.Fatalf("got %d sequences, want 1 (the good lab)", len(sequences))
	}
	if len(sequences[0].steps) != 1 || !strings.HasSuffix(sequences[0].steps[0].source, "a.conf") {
		t.Errorf("good lab's steps = %+v, want a.conf only", sequences[0].steps)
	}
	var named bool
	for _, finding := range s.report.findings {
		if strings.Contains(finding, "missing") {
			named = true
		}
	}
	if !named {
		t.Errorf("no finding names the unreadable lab: %v", s.report.findings)
	}
}
