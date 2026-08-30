package terminaldemo

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// VALIDATES: all eight terminal-demo actions keep their names, reason text, and writes metadata, and only the single-demo render takes a value.
// PREVENTS: a port that claims a writing renderer as a read-only check, drops one build action, or lets a demo id sit in an untyped positional slot.
func TestActionsCarryTheEightActionContracts(t *testing.T) {
	want := []struct {
		verb   string
		writes bool
		why    string
	}{
		{"check-all", false, "every demo the manifest declares has its published artifacts"},
		{"validation-check-all", false, "each scenario's output validators pass, so a demo shows the product working"},
		{"release-check-all", false, "the published artifacts carry this release identity, which is what a tag ships"},
		{"image-build", true, "the container every demo is recorded in, tagged as the manifest names it"},
		{"binaries-build-ze", true, "the ze a demo drives, cross-built for the renderer container"},
		{"binaries-build-ze-test", true, "the ze-test a demo drives, which carries ze_test alone and no version"},
		{"render-all", true, "re-record every website demo from its checked-in tape"},
		{"render", true, "re-record ONE website demo from its checked-in tape, for a developer iterating on that demo"},
	}
	got := Actions()
	if got.Area != area {
		t.Fatalf("area = %q, want %q", got.Area, area)
	}
	if len(got.Actions) != len(want) {
		t.Fatalf("actions = %d, want %d", len(got.Actions), len(want))
	}
	for index, expected := range want {
		row := got.Actions[index]
		if row.Verb != expected.verb || row.Writes != expected.writes || row.Why != expected.why {
			t.Errorf("action %d = %#v, want verb=%q writes=%t why=%q", index, row, expected.verb, expected.writes, expected.why)
		}
	}
	// The demo id is typed by a keyword, and no other action consumes a value.
	table := actionTable()
	if !table.TakesArguments("render") {
		t.Error("render declares no keyword grammar, so a demo id would sit in an untyped positional slot")
	}
	for _, verb := range []string{"check-all", "validation-check-all", "release-check-all", "image-build", "binaries-build-ze", "binaries-build-ze-test", "render-all"} {
		if table.TakesArguments(verb) {
			t.Errorf("%s takes arguments, and a sweep of several actions can no longer name it", verb)
		}
	}
}

// VALIDATES: `le terminal-demo render` with no demo id is refused, and never falls through to the whole gallery.
// PREVENTS: a typo re-recording every demo, which is the cost the single-demo action exists to avoid.
func TestRenderWithoutADemoIdIsRefused(t *testing.T) {
	for _, args := range [][]string{{"render"}, {"render", "name", ""}} {
		payload, code := Answer(args)
		if code != 2 {
			t.Errorf("%v answered code %d, want 2", args, code)
		}
		if payload != nil {
			t.Errorf("%v answered the payload %v", args, payload)
		}
	}
}

// VALIDATES: the two build actions stage the exact target binaries with their distinct tags and release flags.
// PREVENTS: a ze-test carrying product features, or a demo ze built without ze_distro and release identity.
func TestBuildCommandsAreExact(t *testing.T) {
	root := filepath.Join("checkout", "main")
	toolchain := gotoolchain.Toolchain{
		Root: root, Features: []string{"ze_alpha", "ze_beta"}, ExtraTags: []string{"ze_extra"},
		GoToolchain: "go1.26.6", Version: "26.08.27", BuildDate: "2026-08-27T12:34:56Z",
	}
	ze, zeReport := buildCommand(root, toolchain, "arm64", false)
	wantZe := []string{
		"go", "build", "-tags", "ze_core ze_distro ze_alpha ze_beta ze_extra",
		"-ldflags", "-X main.version=26.08.27 -X main.buildDate=2026-08-27T12:34:56Z",
		"-o", filepath.Join(root, "tmp", "terminal-demos", "bin", "ze"), "./cmd/ze",
	}
	if !reflect.DeepEqual(ze.Args, wantZe) {
		t.Errorf("ze argv = %#v, want %#v", ze.Args, wantZe)
	}
	if zeReport.Action != "terminal-demo binaries-build-ze" {
		t.Errorf("ze action = %q", zeReport.Action)
	}
	assertEnvironmentLast(t, ze.Env, "CGO_ENABLED", "0")
	assertEnvironmentLast(t, ze.Env, "GOTOOLCHAIN", "go1.26.6")
	assertEnvironmentLast(t, ze.Env, "GOOS", "linux")
	assertEnvironmentLast(t, ze.Env, "GOARCH", "arm64")
	helper := ptyBuildCommand(root, ze.Env)
	wantHelper := []string{
		"go", "build", "-o", filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-terminal-pty"),
		"./cmd/ze-terminal-pty",
	}
	if !reflect.DeepEqual(helper.Args, wantHelper) {
		t.Errorf("PTY helper argv = %#v, want %#v", helper.Args, wantHelper)
	}
	runtimeHelper := runtimeBuildCommand(root, ze.Env)
	wantRuntimeHelper := []string{
		"go", "build", "-o", filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-demo"),
		"./demos/terminal/cmd/ze-demo",
	}
	if !reflect.DeepEqual(runtimeHelper.Args, wantRuntimeHelper) {
		t.Errorf("demo runtime argv = %#v, want %#v", runtimeHelper.Args, wantRuntimeHelper)
	}

	zeTest, testReport := buildCommand(root, toolchain, "arm64", true)
	wantTest := []string{
		"go", "build", "-tags", "ze_test",
		"-o", filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-test"), "./cmd/ze",
	}
	if !reflect.DeepEqual(zeTest.Args, wantTest) {
		t.Errorf("ze-test argv = %#v, want %#v", zeTest.Args, wantTest)
	}
	if testReport.Action != "terminal-demo binaries-build-ze-test" {
		t.Errorf("ze-test action = %q", testReport.Action)
	}
	if slices.Contains(zeTest.Args, "-ldflags") {
		t.Error("ze-test unexpectedly carries release ldflags")
	}
}

func assertEnvironmentLast(t *testing.T, environ []string, key, want string) {
	t.Helper()
	prefix := key + "="
	for _, entry := range slices.Backward(environ) {
		if got, ok := strings.CutPrefix(entry, prefix); ok {
			if got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Errorf("%s is absent from environment", key)
}

type demoFixture struct {
	root            string
	artifacts       string
	commands        []Command
	failValidation  int
	wrongTranscript bool
	failFFmpeg      int
	// missingImage is the exit code `docker image inspect` answers with, so a
	// case can model a host where the renderer image was never built.
	missingImage int
	// failImageBuild is the exit code `docker build` answers with.
	failImageBuild int
}

func newDemoFixture(t *testing.T) *demoFixture {
	t.Helper()
	base := t.TempDir()
	fixture := &demoFixture{root: filepath.Join(base, "main"), artifacts: filepath.Join(base, "published")}
	fixture.writeTree(t)
	return fixture
}

func (f *demoFixture) writeTree(t *testing.T) {
	t.Helper()
	mustMkdir(t, filepath.Join(f.root, "docs", "guide"))
	mustWrite(t, filepath.Join(f.root, "docs", "guide", "gallery.md"), "gallery\n")
	mustWrite(t, filepath.Join(f.root, "docs", "guide", "demo.md"), "demo\n")
	demoRoot := filepath.Join(f.root, "demos", "terminal")
	for _, name := range []string{"common.tape", "Dockerfile"} {
		mustWrite(t, filepath.Join(demoRoot, name), name+"\n")
	}
	for _, relative := range recorderBinaries {
		mustWrite(t, filepath.Join(f.root, filepath.FromSlash(relative)), relative+"\n")
	}
	// The package half of the recorder sources is walked, so the fixture needs
	// the directory rather than a copy of a list.
	mustMkdir(t, filepath.Join(f.root, filepath.FromSlash(recorderPackageDir)))
	for _, name := range []string{"manifest.go", "actions.go", "cards.json"} {
		mustWrite(t, filepath.Join(f.root, filepath.FromSlash(recorderPackageDir), name), name+"\n")
	}
	mustWrite(t, filepath.Join(demoRoot, "term", "demo.tape"), "Source common.tape\nSleep 5s\nType show term\n")
	mustWrite(t, filepath.Join(demoRoot, "term", "transcript.txt"), "Terminal session\n\n$ show term\n")
	mustWrite(t, filepath.Join(demoRoot, "browser", "run.cjs"), "browser driver\n")
	mustWrite(t, filepath.Join(demoRoot, "browser", "transcript.txt"), "Browser session\n\n$ browser\n")
	manifest := Manifest{
		Schema:      manifestSchema,
		Renderer:    Renderer{Name: "fixture", Version: "3", Image: "fixture-renderer:3", Platform: "linux/amd64"},
		GalleryPage: "guide/gallery.md",
		Demos: []Demo{
			{ID: "term", Title: "Terminal", Description: "Terminal demo", Page: "guide/demo.md", Anchor: "terminal", Platform: "portable", Kind: "terminal", Engine: "Ze recorder", Source: "term/demo.tape", Validate: "term"},
			{ID: "browser", Title: "Browser", Description: "Browser demo", Page: "guide/demo.md", Anchor: "browser", Platform: "portable", Kind: "browser", Engine: "Playwright", Source: "browser/run.cjs", Validate: "browser", Duration: "10 seconds", Privileged: new(true)},
		},
	}
	writeJSON(t, filepath.Join(demoRoot, "manifest.json"), manifest)
	mustWrite(t, filepath.Join(f.root, "tmp", "terminal-demos", "bin", "ze"), "fixture-binary\n")
}

func (f *demoFixture) engine(output *bytes.Buffer) *Engine {
	return New(Options{
		Root:         f.root,
		ArtifactRoot: f.artifacts,
		Executor:     f.execute,
		Lookup:       func(string) (string, error) { return "fixture-ffmpeg", nil },
		Output:       output,
		LockWait:     30 * time.Millisecond,
		LockPoll:     time.Millisecond,
	})
}

func (f *demoFixture) execute(command Command) int {
	f.commands = append(f.commands, command)
	if len(command.Args) == 0 {
		return 127
	}
	if command.Args[0] == "ffmpeg" {
		if f.failFFmpeg != 0 {
			return f.failFFmpeg
		}
		target := command.Args[len(command.Args)-1]
		if err := os.WriteFile(target, []byte("media-output\n"), 0o644); err != nil {
			return 1
		}
		return 0
	}
	// `docker image inspect <image>` is how the pipeline asks whether the renderer
	// image is built. The fixture image is, unless a case says otherwise.
	if len(command.Args) >= 3 && command.Args[1] == "image" && command.Args[2] == "inspect" {
		return f.missingImage
	}
	// `docker build` writes no artifact this fixture publishes; the test that
	// covers it reads the argv.
	if len(command.Args) >= 2 && command.Args[1] == "build" {
		return f.failImageBuild
	}
	entry := command.Args[len(command.Args)-1]
	if len(command.Args) >= 2 && command.Args[len(command.Args)-2] == "validate" {
		if f.failValidation != 0 {
			return f.failValidation
		}
		return 0
	}
	if strings.HasSuffix(entry, "term.tape") {
		burst := "$ show term\r\n"
		if f.wrongTranscript {
			burst = "$ another command\r\n"
		}
		cast := "{\"version\": 2, \"width\": 80, \"height\": 24}\n[0.1, \"o\", " + strconv.Quote(burst) + "]\n"
		if err := os.WriteFile(filepath.Join(f.artifacts, "term.cast"), []byte(cast), 0o644); err != nil {
			return 1
		}
		return 0
	}
	if err := os.WriteFile(filepath.Join(f.artifacts, "browser.webm"), []byte("raw-video\n"), 0o644); err != nil {
		return 1
	}
	if err := os.WriteFile(filepath.Join(f.artifacts, "browser.png"), []byte("raw-poster\n"), 0o644); err != nil {
		return 1
	}
	return 0
}

// VALIDATES: validation and render use the exact Docker contract, including platform, privilege, mounts, and renderer environment.
// PREVENTS: a renderer with network access, a missing lock marker, or release metadata absent from the container.
func TestRenderAllRunsValidationAndPublishesExactArtifacts(t *testing.T) {
	fixture := newDemoFixture(t)
	mustWrite(t, filepath.Join(fixture.artifacts, "term.webm"), "superseded\n")
	mustWrite(t, filepath.Join(fixture.artifacts, "term.png"), "superseded\n")
	mustWrite(t, filepath.Join(fixture.artifacts, "browser.cast"), "superseded\n")
	var output bytes.Buffer
	engine := fixture.engine(&output)
	report, err := engine.RenderAll("26.08.27")
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	if report.Mode != "render" || !reflect.DeepEqual(report.Demos, []string{"term", "browser"}) {
		t.Errorf("report = %#v", report)
	}
	// The first command asks whether the renderer image is built. The six that
	// follow are the two validations, the two recordings and the two ffmpeg passes.
	if len(fixture.commands) != 7 {
		t.Fatalf("commands = %d, want 7: %#v", len(fixture.commands), fixture.commands)
	}
	assertSubsequence(t, fixture.commands[0].Args, []string{"docker", "image", "inspect", "fixture-renderer:3"})
	validation := fixture.commands[1].Args
	assertSubsequence(t, validation, []string{"docker", "run", "--rm", "--platform", "linux/amd64", "--network", "none"})
	assertSubsequence(t, validation, []string{"--env", "ZE_DEMO_LOCK_HELD=1"})
	assertSubsequence(t, validation, []string{"fixture-renderer:3", "/src/tmp/terminal-demos/bin/ze-demo", "validate", "term"})
	if containsPrefix(validation, "ZE_DEMO_RELEASE=") {
		t.Errorf("validation received render release env: %v", validation)
	}
	render := fixture.commands[3].Args
	assertSubsequence(t, render, []string{"--env", "ZE_DEMO_RELEASE=26.08.27", "--env", "ZE_DEMO_SPEEDUP=5", "fixture-renderer:3"})
	browserRender := fixture.commands[4].Args
	assertSubsequence(t, browserRender, []string{"docker", "run", "--rm", "--platform", "linux/amd64", "--privileged", "--network", "none"})
	for _, name := range []string{"term.cast", "term.txt", "browser.webm", "browser.png", "browser.txt", "manifest.json"} {
		info, statErr := os.Stat(filepath.Join(fixture.artifacts, name))
		if statErr != nil || info.Size() == 0 {
			t.Errorf("published %s: info=%v err=%v", name, info, statErr)
		}
	}
	for _, name := range []string{"term.webm", "term.png", "browser.cast"} {
		if _, statErr := os.Stat(filepath.Join(fixture.artifacts, name)); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("superseded %s remains: %v", name, statErr)
		}
	}
	cast := mustRead(t, filepath.Join(fixture.artifacts, "term.cast"))
	if !strings.Contains(cast, `[0.5, "o", "$ show term\r\n"]`) {
		t.Errorf("expanded cast = %q", cast)
	}
	if !strings.Contains(output.String(), "Ze demo artifacts verified: term, browser") {
		t.Errorf("output = %q", output.String())
	}
	before := mustRead(t, filepath.Join(fixture.artifacts, "manifest.json"))
	if _, err := engine.checkAll("26.08.27"); err != nil {
		t.Fatalf("release check after render: %v", err)
	}
	after := mustRead(t, filepath.Join(fixture.artifacts, "manifest.json"))
	if before != after {
		t.Error("a check rewrote the artifact manifest")
	}
}

// VALIDATES: a single-demo render validates and records that demo alone, and publishes beside the demos it did not record.
// PREVENTS: a developer iterating on one tape paying for every demo in the gallery, and a partial render dropping the others from the artifact manifest.
func TestRenderOneRecordsTheNamedDemoAndKeepsTheRest(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	fixture.commands = nil

	report, err := engine.RenderOne("term", "26.08.28")
	if err != nil {
		t.Fatalf("RenderOne: %v", err)
	}
	if report.Mode != rendererRenderMode || !reflect.DeepEqual(report.Demos, []string{"term"}) {
		t.Errorf("report = %#v, want mode %q over [term] alone", report, rendererRenderMode)
	}
	// One image check, one validation and one recording. RenderAll ran seven
	// commands for the two demos, and every one of the four this run did not
	// repeat is the cost the action exists to remove.
	if len(fixture.commands) != 3 {
		t.Fatalf("commands = %d, want 3: %#v", len(fixture.commands), fixture.commands)
	}
	for _, command := range fixture.commands {
		for _, argument := range command.Args {
			if strings.Contains(argument, "browser") {
				t.Errorf("a single-demo render of term ran %v", command.Args)
			}
		}
	}

	var published artifactManifest
	if err := readJSON(fixture.artifacts, &published); err != nil {
		t.Fatalf("read published manifest: %v", err)
	}
	if published.Demos["term"].Release != "26.08.28" {
		t.Errorf("term was published for release %q", published.Demos["term"].Release)
	}
	// The demo this run did not record keeps its entry and its release. A
	// partial render that rewrote the manifest from its own selection would
	// unpublish the rest of the gallery.
	if published.Demos["browser"].Release != "26.08.27" {
		t.Errorf("browser was published for release %q, want the release of the run that recorded it",
			published.Demos["browser"].Release)
	}
	for _, name := range []string{"browser.webm", "browser.png", "browser.txt"} {
		if _, statErr := os.Stat(filepath.Join(fixture.artifacts, name)); statErr != nil {
			t.Errorf("a single-demo render removed %s: %v", name, statErr)
		}
	}
}

// VALIDATES: an id the manifest does not declare is refused by name, before any container runs.
// PREVENTS: a typo reported an hour later as a missing artifact, or as a silent no-op render.
func TestRenderOneRefusesAnIdTheManifestDoesNotDeclare(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})

	_, err := engine.RenderOne("termm", "26.08.27")
	if err == nil {
		t.Fatal("an unknown demo id was accepted")
	}
	if !strings.Contains(err.Error(), `"termm"`) || !strings.Contains(err.Error(), "term, browser") {
		t.Errorf("the refusal is %q, want it to quote the id and name the declared ids", err)
	}
	if len(fixture.commands) != 0 {
		t.Errorf("an unknown demo id ran %#v", fixture.commands)
	}
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func assertSubsequence(t *testing.T, values, wanted []string) {
	t.Helper()
	at := 0
	for _, value := range values {
		if at < len(wanted) && value == wanted[at] {
			at++
		}
	}
	if at != len(wanted) {
		t.Errorf("%v does not contain ordered subsequence %v", values, wanted)
	}
}

// VALIDATES: all three renderer modes validate the same source manifest before effects.
// PREVENTS: validation or render accepting a manifest that the artifact check refuses.
func TestEveryModeFailsClosedOnTheManifestContract(t *testing.T) {
	for _, mode := range []struct {
		name string
		run  func(*Engine) error
	}{
		{"check", func(engine *Engine) error { _, err := engine.checkAll(""); return err }},
		{"release-check", func(engine *Engine) error { _, err := engine.checkAll("26.08.27"); return err }},
		{"validation", func(engine *Engine) error { _, err := engine.validationCheckAll(); return err }},
		{"render", func(engine *Engine) error { _, err := engine.RenderAll("26.08.27"); return err }},
	} {
		t.Run(mode.name, func(t *testing.T) {
			fixture := newDemoFixture(t)
			path := filepath.Join(fixture.root, "demos", "terminal", "manifest.json")
			var manifest Manifest
			readJSONForTest(t, path, &manifest)
			manifest.Schema = 99
			writeJSON(t, path, manifest)
			err := mode.run(fixture.engine(&bytes.Buffer{}))
			if err == nil || err.Error() != "manifest.json: unsupported schema" {
				t.Fatalf("error = %v", err)
			}
			if len(fixture.commands) != 0 {
				t.Errorf("invalid manifest started commands: %#v", fixture.commands)
			}
		})
	}
}

// VALIDATES: release and source digests are checked before a published artifact is accepted.
// PREVENTS: a tag shipping an older recording, or changed tape inputs retaining a fresh verdict.
func TestCheckRejectsReleaseAndSourceDrift(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatalf("render fixture: %v", err)
	}
	if _, err := engine.checkAll("26.08.28"); err == nil || !strings.Contains(err.Error(), "rendered for '26.08.27', expected '26.08.28'") {
		t.Fatalf("release drift error = %v", err)
	}
	mustWrite(t, filepath.Join(fixture.root, "demos", "terminal", "term", "demo.tape"), "Source common.tape\nSleep 6s\nType show term\n")
	if _, err := engine.checkAll(""); err == nil || err.Error() != "term: source changed since the last render" {
		t.Fatalf("source drift error = %v", err)
	}
}

// VALIDATES: a held state lock refuses a second owner without reading or writing artifacts.
// PREVENTS: two render or check processes racing on the artifact manifest and temporary tapes.
func TestCheckRefusesAContendedLock(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	mustMkdir(t, filepath.Dir(engine.lockPath))
	handle, err := os.OpenFile(engine.lockPath, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close() //nolint:errcheck // Test cleanup.
	if err := syscall.Flock(int(handle.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(handle.Fd()), syscall.LOCK_UN) //nolint:errcheck // Test cleanup.
	started := time.Now()
	_, err = engine.checkAll("")
	if err == nil || !strings.Contains(err.Error(), "another demo run held") {
		t.Fatalf("lock error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Errorf("short fixture lock wait took %s", time.Since(started))
	}
}

// VALIDATES: validation stops at the first failing scenario and retains that process code in the failure.
// PREVENTS: later validators hiding the first failure or running after the result is already red.
func TestValidationStopsAtTheFirstFailure(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.failValidation = 23
	_, code, err := executeRenderer(fixture.engine(&bytes.Buffer{}), "validate", "")
	var failure commandFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %T %v, want commandFailure", err, err)
	}
	if failure.code != 23 {
		t.Errorf("failure code = %d, want 23", failure.code)
	}
	if code != 1 {
		t.Errorf("action code = %d, want renderer code 1", code)
	}
	// The image check, then the validation that failed. Nothing after it ran.
	if len(fixture.commands) != 2 {
		t.Errorf("commands after first failure = %d, want 2", len(fixture.commands))
	}
}

// VALIDATES: image-build tags the image the manifest names and reads the
// Dockerfile from the demo tree.
// PREVENTS: building under any other tag, which leaves the recorder pulling an
// image nobody publishes while a correctly built one sits beside it.
func TestBuildImageTagsTheImageTheManifestNames(t *testing.T) {
	fixture := newDemoFixture(t)
	var output bytes.Buffer
	report, err := fixture.engine(&output).BuildImage()
	if err != nil {
		t.Fatalf("BuildImage: %v", err)
	}
	if report.Mode != rendererImageBuildMode {
		t.Errorf("report = %#v, want mode %q", report, rendererImageBuildMode)
	}
	if len(fixture.commands) != 1 {
		t.Fatalf("commands = %d, want the build alone: %#v", len(fixture.commands), fixture.commands)
	}
	assertSubsequence(t, fixture.commands[0].Args, []string{
		"docker", "build",
		"-f", filepath.Join(fixture.root, "demos", "terminal", "Dockerfile"),
		"-t", "fixture-renderer:3",
		filepath.Join(fixture.root, "demos", "terminal"),
	})
}

// VALIDATES: a failed image build is reported with the argv that failed, and its
// report still names the action that ran.
// PREVENTS: a build failure reaching the operator as a bare exit code, with no
// way to see which tag or Dockerfile Docker refused.
func TestBuildImageReportsTheArgvThatFailed(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.failImageBuild = 7
	var output bytes.Buffer
	report, err := fixture.engine(&output).BuildImage()
	if err == nil {
		t.Fatal("a failed docker build was reported as success")
	}
	if report.Mode != rendererImageBuildMode {
		t.Errorf("report = %#v, want mode %q", report, rendererImageBuildMode)
	}
	if !strings.Contains(err.Error(), "fixture-renderer:3") || !strings.Contains(err.Error(), "code 7") {
		t.Errorf("error = %v, want the exit code and the tag it refused", err)
	}
}

// VALIDATES: a render on a host where the renderer image is not built is refused
// by name, before any validator runs, and the refusal names the action that builds it.
// PREVENTS: Docker answering a missing local image with a registry pull, which fails
// as "pull access denied" after every validator has already run.
func TestRenderRefusesAnUnbuiltRendererImage(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.missingImage = 1
	var output bytes.Buffer
	_, err := fixture.engine(&output).RenderAll("26.08.30")
	if err == nil {
		t.Fatal("a render with no renderer image was accepted")
	}
	if !strings.Contains(err.Error(), "le terminal-demo image-build") {
		t.Errorf("error = %v, want the action that builds the image", err)
	}
	if len(fixture.commands) != 1 {
		t.Errorf("commands = %d, want the image check alone: %#v", len(fixture.commands), fixture.commands)
	}
}

// VALIDATES: transcript refusal removes the bad cast from the publish tree and leaves the manifest unwritten.
// PREVENTS: a failed demo recording remaining publishable after its transcript no longer matches.
func TestTranscriptRefusalMovesTheRecordingOutOfPublishedArtifacts(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.wrongTranscript = true
	_, err := fixture.engine(&bytes.Buffer{}).RenderAll("26.08.27")
	if err == nil || !strings.Contains(err.Error(), "the recording does not show") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.artifacts, "term.cast")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("bad cast remains published: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, "tmp", "terminal-demos", "rejected", "term.cast")); statErr != nil {
		t.Errorf("rejected cast is absent: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.artifacts, "manifest.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("failed render wrote a manifest: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, "tmp", "terminal-demos", "render-tapes", "term.tape")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("accelerated tape remains: %v", statErr)
	}
}

// VALIDATES: a failed media transform restores the capture the renderer wrote.
// PREVENTS: ffmpeg failure deleting both the source capture and its destination.
func TestMediaFailureRestoresTheOriginalCapture(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.failFFmpeg = 9
	_, err := fixture.engine(&bytes.Buffer{}).RenderAll("26.08.27")
	if err == nil {
		t.Fatal("RenderAll succeeded despite ffmpeg failure")
	}
	if got := mustRead(t, filepath.Join(fixture.artifacts, "browser.webm")); got != "raw-video\n" {
		t.Errorf("restored video = %q", got)
	}
	if _, statErr := os.Stat(filepath.Join(fixture.artifacts, "browser.fast.webm")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("compressed temporary remains: %v", statErr)
	}
}

// VALIDATES: artifact bytes and metadata are both part of the check verdict.
// PREVENTS: a present but modified published file passing because only its path was checked.
func TestCheckRejectsArtifactDigestDrift(t *testing.T) {
	fixture := newDemoFixture(t)
	engine := fixture.engine(&bytes.Buffer{})
	if _, err := engine.RenderAll("26.08.27"); err != nil {
		t.Fatalf("render fixture: %v", err)
	}
	mustWrite(t, filepath.Join(fixture.artifacts, "term.cast"), "modified\n")
	if _, err := engine.checkAll(""); err == nil || err.Error() != "term: cast digest mismatch" {
		t.Fatalf("asset drift error = %v", err)
	}
}

// VALIDATES: demo kind and duration rules fail closed at the source manifest.
// PREVENTS: a terminal demo publishing browser assets, or a browser demo with no duration contract.
func TestManifestRejectsUnsupportedKindsAndDurationDrift(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Manifest)
		want   string
	}{
		{
			name:   "unknown kind",
			change: func(manifest *Manifest) { manifest.Demos[0].Kind = "unknown" },
			want:   "manifest.json: term.kind is unsupported",
		},
		{
			name:   "cast duration duplicated",
			change: func(manifest *Manifest) { manifest.Demos[0].Duration = "5 seconds" },
			want:   "manifest.json: term.duration is read from the cast, so the manifest must not state it",
		},
		{
			name:   "browser duration absent",
			change: func(manifest *Manifest) { manifest.Demos[1].Duration = "" },
			want:   "manifest.json: browser.duration is required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDemoFixture(t)
			path := filepath.Join(fixture.root, "demos", "terminal", "manifest.json")
			var manifest Manifest
			readJSONForTest(t, path, &manifest)
			test.change(&manifest)
			writeJSON(t, path, manifest)
			if _, err := fixture.engine(&bytes.Buffer{}).checkAll(""); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

// VALIDATES: transcript verification resolves the cursor, erases, scroll region, wrapping, and repaint history used by real demo casts.
// PREVENTS: searching the raw byte stream, which contains ghost completions and overwritten commands a reader never saw together.
func TestScreenModelReconstructsTheRenderedTerminal(t *testing.T) {
	t.Run("inline completion", func(t *testing.T) {
		screen := paintScreen("\x1b[36;1Hze> ")
		typed := "monitor"
		for index, character := range typed {
			var burst bytes.Buffer
			ghost := typed[index+1:]
			burst.WriteString("\x1b[36;")
			burst.WriteString(strconv.Itoa(5 + index))
			burst.WriteByte('H')
			burst.WriteRune(character)
			burst.WriteString(ghost)
			burst.WriteString("\x1b[K")
			screen.settle(burst.String())
		}
		if !strings.Contains(screen.text(), "ze> monitor") {
			t.Errorf("screen = %q", screen.text())
		}
	})
	t.Run("scroll region", func(t *testing.T) {
		screen := paintScreen(
			"\x1b[10;1Hanswer\x1b[34;1Hstatus\x1b[36;1Hze# commit",
			"\x1b[6;32r\x1b[32;1H\x1b[7S\x1b[1;36r",
		)
		rows := strings.Split(screen.text(), "\n")
		if !slices.Contains(rows, "status") || !slices.Contains(rows, "ze# commit") || slices.Contains(rows, "answer") {
			t.Errorf("rows = %#v", rows)
		}
	})
	t.Run("delete and reverse index", func(t *testing.T) {
		deleted := paintScreen("\x1b[1;1Hone\r\ntwo\r\nthree", "\x1b[2;1H\x1b[M")
		if got := strings.Split(deleted.text(), "\n"); !reflect.DeepEqual(got, []string{"one", "three"}) {
			t.Errorf("deleted rows = %#v", got)
		}
		reversed := paintScreen("\x1b[1;1Hone\r\ntwo", "\x1bM\rONE")
		if got := strings.Split(reversed.text(), "\n"); !reflect.DeepEqual(got, []string{"ONE", "two"}) {
			t.Errorf("reverse-index rows = %#v", got)
		}
	})
	t.Run("trailing blank and repaint history", func(t *testing.T) {
		prompt := paintScreen("\x1b[1;1H$ ")
		if !strings.Contains(prompt.text(), "$ ") {
			t.Errorf("prompt screen = %q", prompt.text())
		}
		repaint := paintScreen("\x1b[36;1Hze# commit", "\x1b[36;1Hze# ......")
		if !slices.Contains(repaint.painted(), "ze# commit") {
			t.Errorf("painted history = %#v", repaint.painted())
		}
	})
	t.Run("wrap and scrollback", func(t *testing.T) {
		wrapped := paintScreen("\x1b[1;1H" + strings.Repeat("x", 200))
		rows := strings.Split(wrapped.text(), "\n")
		if len(rows) != 2 || len(rows[0]) != 137 || len(rows[1]) != 63 {
			t.Errorf("wrapped row lengths = %d/%d across %d rows", len(rows[0]), len(rows[len(rows)-1]), len(rows))
		}
		bursts := []string{"\x1b[1;1Hfirst"}
		for row := 2; row < 40; row++ {
			bursts = append(bursts, "\x1b["+strconv.Itoa(row)+";1Hrow"+strconv.Itoa(row))
		}
		scrolled := paintScreen(bursts...)
		if !slices.Contains(scrolled.painted(), "first") || strings.Contains(scrolled.text(), "first") {
			t.Errorf("painted=%#v screen=%q", scrolled.painted(), scrolled.text())
		}
	})
}

func paintScreen(bursts ...string) *terminalScreen {
	screen := newTerminalScreen(36, 137)
	for _, burst := range bursts {
		screen.settle(burst)
	}
	return screen
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSONForTest(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: a new file in the recorder package changes what a recording is a
// function of.
// PREVENTS: the defect this replaced. The sources were hand-listed and the list
// had gone stale in the direction that costs most -- actions.go and register.go
// were absent, so a change to the command surface a demo records moved no
// digest and invalidated no recording. A recorded demo went on claiming to show
// the behavior of code that had moved under it.
func TestRecorderSourcesFollowThePackage(t *testing.T) {
	fixture := newDemoFixture(t)
	fixture.writeTree(t)

	before, err := nativeRecorderSources(fixture.root)
	if err != nil {
		t.Fatalf("read recorder sources: %v", err)
	}

	added := filepath.Join(fixture.root, filepath.FromSlash(recorderPackageDir), "register.go")
	mustWrite(t, added, "package terminaldemo\n")
	ignored := filepath.Join(fixture.root, filepath.FromSlash(recorderPackageDir), "manifest_test.go")
	mustWrite(t, ignored, "package terminaldemo\n")

	after, err := nativeRecorderSources(fixture.root)
	if err != nil {
		t.Fatalf("read recorder sources: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("sources went from %d to %d; want exactly the one new non-test file", len(before), len(after))
	}
	if !slices.Contains(after, recorderPackageDir+"/register.go") {
		t.Errorf("a new package file is not a recorder source: %v", after)
	}
	if slices.Contains(after, recorderPackageDir+"/manifest_test.go") {
		t.Error("a test file became a recorder source, so every test edit would invalidate every recording")
	}
	if !slices.IsSorted(after) {
		t.Error("recorder sources are unsorted, so the digest depends on directory order")
	}
}
