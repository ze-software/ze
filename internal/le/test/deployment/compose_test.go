package testdeployment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// fixtureComposeProof is a proof over a recording Docker client, with its
// project files set as Run sets them.
func fixtureComposeProof(t *testing.T, recorder *argvRecorder, image string, breakVolume bool) *composeProof {
	t.Helper()
	proof, err := newComposeProof("/checkout", interoplab.NewFixtureDocker(recorder), image, breakVolume, nil)
	if err != nil {
		t.Fatalf("newComposeProof: %v", err)
	}
	proof.project.Files = []string{"/checkout/docker/compose.yaml", "/checkout/tmp/evidence/compose-x/override.yaml"}
	return proof
}

// VALIDATES: the override changes only what a shared daemon forces: the image
// reference, the published ports (one loopback SSH port the daemon picks), and
// owner labels; the counterfactual alone drops the volume mount.
// PREVENTS: a proof that publishes the fixed 179 and 2222 another session holds,
// or a counterfactual that silently still mounts the store.
func TestComposeOverrideReplacesOnlyImagePortsAndLabels(t *testing.T) {
	proof := fixtureComposeProof(t, &argvRecorder{}, "", false)
	text, err := proof.override()
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	for _, want := range []string{
		`image: "ze-compose-proof:` + proof.project.Name + `"`,
		"pull_policy: never",
		"ports: !override\n      - \"127.0.0.1::2222/tcp\"\n",
		`"ze.lab.kind": "container"`,
		`"ze.lab.kind": "network"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("override lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "179/tcp") || strings.Contains(text, "179:179") {
		t.Errorf("override publishes port 179:\n%s", text)
	}
	if strings.Contains(text, "\n    volumes:") {
		t.Errorf("override touches volumes outside the counterfactual:\n%s", text)
	}

	broken := fixtureComposeProof(t, &argvRecorder{}, "", true)
	text, err = broken.override()
	if err != nil {
		t.Fatalf("counterfactual override: %v", err)
	}
	if !strings.Contains(text, "    volumes: !reset []\n") {
		t.Errorf("counterfactual override keeps the volume mount:\n%s", text)
	}
}

// VALIDATES: a supplied image runs under its own reference and is never built.
func TestComposeOverrideRunsTheSuppliedImage(t *testing.T) {
	const image = "sha256:89bb701df87656a0c9901e3377b4d19b08fa3c5c46d21f180ebce60a47d23391"
	proof := fixtureComposeProof(t, &argvRecorder{}, image, false)
	text, err := proof.override()
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if !strings.Contains(text, `image: "`+image+`"`) {
		t.Errorf("override does not run %s:\n%s", image, text)
	}
	if proof.builtImage() != "" {
		t.Errorf("builtImage = %q for a supplied image, want none", proof.builtImage())
	}
	recorder := &argvRecorder{}
	proof = fixtureComposeProof(t, recorder, image, false)
	if _, err := proof.build(t.Context(), &ComposeReport{}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(recorder.commands) != 0 {
		t.Errorf("a supplied image ran docker: %v", recorder.commands)
	}
}

// VALIDATES: teardown removes the project's containers, network and volume by
// project name, then the image a tree build tagged, and lists what the project
// label still names. A supplied image is never removed.
// PREVENTS: a teardown that prunes, removes another run's objects, or deletes
// the preview image it was handed.
func TestComposeTeardownRemovesOnlyWhatTheRunCreated(t *testing.T) {
	for _, supplied := range []string{"", "ze-preview-deployment:preview-x"} {
		recorder := &argvRecorder{}
		proof := fixtureComposeProof(t, recorder, supplied, false)
		report := ComposeReport{}
		proof.teardown(t.Context(), &report)
		if len(report.LeftBehind) != 0 {
			t.Errorf("image %q: left behind %v with a clean daemon", supplied, report.LeftBehind)
		}
		joined := make([]string, 0, len(recorder.commands))
		for _, argv := range recorder.commands {
			joined = append(joined, strings.Join(argv, " "))
		}
		prefix := "docker compose --project-name " + proof.project.Name +
			" --file /checkout/docker/compose.yaml --file /checkout/tmp/evidence/compose-x/override.yaml"
		if joined[0] != prefix+" down --remove-orphans --volumes" {
			t.Errorf("first teardown command = %q", joined[0])
		}
		removesImage := slices.ContainsFunc(joined, func(line string) bool { return strings.HasPrefix(line, "docker image rm") })
		if supplied == "" && !slices.Contains(joined, "docker image rm ze-compose-proof:"+proof.project.Name) {
			t.Errorf("a tree build's image is not removed: %v", joined)
		}
		if supplied != "" && removesImage {
			t.Errorf("the supplied image is removed: %v", joined)
		}
		for _, object := range []string{"docker ps --all", "docker volume ls", "docker network ls"} {
			want := object + " --quiet --filter label=com.docker.compose.project=" + proof.project.Name
			if !slices.Contains(joined, want) {
				t.Errorf("teardown does not list %q: %v", want, joined)
			}
		}
		for _, line := range joined {
			if strings.Contains(line, "prune") || strings.Contains(line, "--rmi") {
				t.Errorf("teardown ran %q", line)
			}
		}
	}
}

// VALIDATES: an object the project label still names after teardown is named
// with the command that removes it, and fails a run that otherwise passed.
func TestComposeTeardownNamesWhatIsLeftBehind(t *testing.T) {
	recorder := &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
		if len(argv) > 2 && argv[1] == "volume" && argv[2] == "ls" {
			return interoplab.ProcessResult{Stdout: "ze-compose-x_ze-data\n"}, nil
		}
		return interoplab.ProcessResult{}, nil
	}}
	proof := fixtureComposeProof(t, recorder, "img", false)
	report := ComposeReport{}
	proof.teardown(t.Context(), &report)
	want := "volume ze-compose-x_ze-data. Fix: `docker volume rm ze-compose-x_ze-data`"
	if !slices.Contains(report.LeftBehind, want) {
		t.Errorf("left behind = %v, want %q", report.LeftBehind, want)
	}
}

// VALIDATES: a supplied image the daemon does not hold is refused before any
// project object exists, with the command that loads a preview export.
func TestComposeRefusesAnAbsentSuppliedImage(t *testing.T) {
	const image = "sha256:absent"
	recorder := &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
		return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error: No such image: " + image}, nil
	}}
	proof := fixtureComposeProof(t, recorder, image, false)
	err := proof.requireSuppliedImage(t.Context())
	if err == nil {
		t.Fatal("an absent image was accepted")
	}
	for _, want := range []string{image, "Fix: load the preview export", "docker load --input", "docker tag sha256:<id> ze-local:preview", "image ze-local:preview"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}
}

// VALIDATES: the one-off runs are the guide's `run --rm` with no terminal, and
// ze init reads its answers from standard input.
func TestComposeInitRunsTheGuideCommand(t *testing.T) {
	recorder := &argvRecorder{}
	proof := fixtureComposeProof(t, recorder, "img", false)
	if _, err := proof.initStore(t.Context(), &ComposeReport{}); err != nil {
		t.Fatalf("initStore: %v", err)
	}
	got := strings.Join(recorder.commands[0], " ")
	if !strings.HasSuffix(got, " run --rm --no-TTY ze init") {
		t.Errorf("init argv = %q", got)
	}
}

// VALIDATES: the project names are run-unique and valid Compose names, and the
// operator password is never the same twice.
func TestComposeProjectsAreRunUnique(t *testing.T) {
	first := fixtureComposeProof(t, &argvRecorder{}, "", false)
	second := fixtureComposeProof(t, &argvRecorder{}, "", false)
	if first.project.Name == second.project.Name {
		t.Errorf("two runs share project %s", first.project.Name)
	}
	if first.password == second.password {
		t.Error("two runs share a password")
	}
	if err := interoplab.NewFixtureDocker(&argvRecorder{}).ComposeDown(t.Context(), first.project, true); err != nil {
		t.Errorf("project name %s refused: %v", first.project.Name, err)
	}
	bad := interoplab.ComposeProject{Name: "Ze Compose", Files: []string{"f"}}
	err := interoplab.NewFixtureDocker(&argvRecorder{}).ComposeDown(t.Context(), bad, true)
	if err == nil || !strings.Contains(err.Error(), "use lowercase letters") {
		t.Errorf("an invalid project name answered %v", err)
	}
}

// VALIDATES: the running-configuration oracle reads a leaf, an absent leaf
// and refuses a shape it cannot read rather than answering "absent".
func TestComposeLeafAt(t *testing.T) {
	cases := []struct {
		document string
		value    string
		present  bool
		fails    bool
	}{
		{`{"environment":{"log":{"level":"warn"}}}`, "warn", true, false},
		{`{"environment":{"ssh":{"enabled":true}}}`, "", false, false},
		{`{"environment":"flat"}`, "", false, true},
		{`{"environment":{"log":{"level":3}}}`, "", true, true},
		{`not json`, "", false, true},
	}
	for _, tc := range cases {
		value, present, err := leafAt([]byte(tc.document), composeEditPath)
		if (err != nil) != tc.fails || value != tc.value || present != tc.present {
			t.Errorf("leafAt(%s) = %q, %v, %v", tc.document, value, present, err)
		}
	}
}

// VALIDATES: the verdict line a pass prints is the one AC-5 quotes, and a
// counterfactual failure says it was one.
func TestComposeVerdict(t *testing.T) {
	if got := (ComposeReport{Proven: true}).Verdict(); got != composePassLine {
		t.Errorf("pass verdict = %q", got)
	}
	got := ComposeReport{BreakVolume: true, Reason: "import: no store"}.Verdict()
	if got != "DEPLOYMENT-COMPOSE: FAIL (counterfactual: no volume mounted) import: no store" {
		t.Errorf("fail verdict = %q", got)
	}
}

type stubComposeRun struct {
	report ComposeReport
	err    error
}

func (s stubComposeRun) Run(context.Context) (ComposeReport, error) { return s.report, s.err }

// VALIDATES: only a proven run answers 0.
func TestComposeExitCode(t *testing.T) {
	if _, code := runCompose(t.Context(), stubComposeRun{report: ComposeReport{Proven: true}}); code != 0 {
		t.Errorf("proven run answered %d", code)
	}
	if _, code := runCompose(t.Context(), stubComposeRun{report: ComposeReport{Reason: "x"}}); code != 1 {
		t.Errorf("disproved run answered %d", code)
	}
	if _, code := runCompose(t.Context(), stubComposeRun{err: errors.New("no docker")}); code != 1 {
		t.Errorf("unstarted run answered %d", code)
	}
}

// VALIDATES: AC-5's static half. docker/compose.yaml mounts its named volume at
// the ZE_CONFIG_DIR every step uses, runs bare `start`, publishes the ports the
// Docker guide documents, adds no capability, and lifts the memlock limit; the
// guide's plain-Docker form states the same settings.
// PREVENTS: init and the daemon choosing different store paths (spec R-2), or
// the guide documenting a port, mount or privilege the Compose file lacks.
func TestComposeFileMatchesTheDockerGuide(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "docker", "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	var file struct {
		Services map[string]struct {
			Hostname    string            `yaml:"hostname"`
			Command     []string          `yaml:"command"`
			Environment map[string]string `yaml:"environment"`
			Ports       []string          `yaml:"ports"`
			Volumes     []string          `yaml:"volumes"`
			CapAdd      []string          `yaml:"cap_add"`
			Privileged  bool              `yaml:"privileged"`
			NetworkMode string            `yaml:"network_mode"`
			Ulimits     map[string]struct {
				Soft int `yaml:"soft"`
				Hard int `yaml:"hard"`
			} `yaml:"ulimits"`
		} `yaml:"services"`
		Volumes map[string]any `yaml:"volumes"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse compose.yaml: %v", err)
	}
	service, ok := file.Services[composeService]
	if !ok {
		t.Fatalf("compose.yaml declares no %s service", composeService)
	}
	store := service.Environment["ZE_CONFIG_DIR"]
	if store != "/etc/ze" {
		t.Errorf("ZE_CONFIG_DIR = %q, want /etc/ze", store)
	}
	if !slices.Contains(service.Volumes, "ze-data:"+store) {
		t.Errorf("volumes %v do not mount ze-data at ZE_CONFIG_DIR %s", service.Volumes, store)
	}
	if _, declared := file.Volumes["ze-data"]; !declared {
		t.Error("ze-data is not a top-level named volume")
	}
	if !slices.Equal(service.Command, []string{"start"}) {
		t.Errorf("command = %v, want bare start", service.Command)
	}
	if service.Hostname != "ze" {
		t.Errorf("hostname = %q, want ze (the instance name init keeps)", service.Hostname)
	}
	if !slices.Equal(service.Ports, []string{"179:179", "127.0.0.1:2222:2222"}) {
		t.Errorf("ports = %v, want BGP 179 and loopback SSH 2222", service.Ports)
	}
	if len(service.CapAdd) != 0 || service.Privileged || service.NetworkMode != "" {
		t.Errorf("BGP-only role takes extra privilege: cap_add %v, privileged %v, network_mode %q",
			service.CapAdd, service.Privileged, service.NetworkMode)
	}
	if memlock := service.Ulimits["memlock"]; memlock.Soft != -1 || memlock.Hard != -1 {
		t.Errorf("memlock = %+v, want -1/-1", memlock)
	}

	guide, err := os.ReadFile(filepath.Join(root, "docs", "guide", "docker.md"))
	if err != nil {
		t.Fatalf("read the Docker guide: %v", err)
	}
	for _, want := range []string{
		"-e ZE_CONFIG_DIR=/etc/ze -v ze-data:/etc/ze",
		"--ulimit memlock=-1",
		"-p 179:179 -p 127.0.0.1:2222:2222 ze:local start",
		"Compose publishes SSH on the host's loopback port `2222`, and BGP on port `179`.",
	} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("the Docker guide no longer states %q", want)
		}
	}
}
