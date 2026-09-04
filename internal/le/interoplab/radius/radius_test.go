package radius

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

// bothShape is what an Access-Request would look like if the credential builder
// appended instead of selecting. RFC 2865 Section 4.1: "An Access-Request MUST
// NOT contain both a User-Password and a CHAP-Password." No scenario expects it,
// which is what makes it a red case rather than a shape the checkers accept.
var bothShape = credentialShape{UserPassword: credentialPresent, CHAPPassword: credentialPresent}

// labPolicy models the chain docs/guide/radius.md describes, so a test can
// break one link at a time and watch the checker go red. It is a model of the
// PRODUCT, never of the checker: every knob below names a real defect.
type labPolicy struct {
	// serverPasswords is what the FreeRADIUS user file holds.
	serverPasswords map[string]string
	// serverCanCHAP is false when the server stores a hash, which RFC 2865
	// Section 2.2 makes a rejection for CHAP.
	serverCanCHAP bool
	// shape is the credential ze puts on the wire.
	shape credentialShape
	// profile is the profile ze attaches after an Access-Accept.
	profile string
	// localPasswords is ze's local bcrypt account list.
	localPasswords map[string]string
	// silentUsers are the usernames the lab server answers nothing for.
	silentUsers map[string]bool
	// fallThrough makes ze consult local after an Access-Reject. That is the
	// defect AC-5 exists to catch.
	fallThrough bool
	// recordRequests is false when the server keeps no record, which is the
	// state in which ze's own log would be the only evidence.
	recordRequests bool
	// papProbeAccepted is what radclient hears back from the server.
	papProbeAccepted bool
	// challenges is true when the login runs several rounds and the server
	// records each request that came back carrying its State. A login answered
	// in one request, or one whose State ze failed to echo, is the defect the
	// EAP scenario's third assertion exists to catch.
	challenges bool
}

func conformingPAP() labPolicy {
	return labPolicy{
		serverPasswords:  map[string]string{radiusUser: papPassword},
		serverCanCHAP:    false,
		shape:            papShape,
		profile:          "interop-operator",
		localPasswords:   map[string]string{localUser: localPassword, radiusUser: localPassword},
		silentUsers:      map[string]bool{localUser: true},
		recordRequests:   true,
		papProbeAccepted: true,
	}
}

func conformingCHAP() labPolicy {
	policy := conformingPAP()
	policy.serverPasswords = map[string]string{radiusUser: chapPassword}
	policy.serverCanCHAP = true
	policy.shape = chapShape
	return policy
}

func conformingCHAPHashed() labPolicy {
	policy := conformingCHAP()
	policy.serverCanCHAP = false
	return policy
}

func conformingEAP() labPolicy {
	policy := conformingPAP()
	policy.serverPasswords = map[string]string{radiusUser: eapPassword}
	policy.shape = eapShape
	policy.challenges = true
	return policy
}

// stubLab answers the protocol-neutral CheckerLab from labPolicy, with no
// Docker and no container.
type stubLab struct {
	policy  labPolicy
	records []string
	zeLog   []string
}

func newStubLab(policy labPolicy) *scenarioLab {
	return &scenarioLab{lab: &stubLab{policy: policy}, timeout: 1500 * time.Millisecond}
}

func (s *stubLab) record(verdict, user string) {
	if !s.policy.recordRequests {
		return
	}
	s.records = append(s.records, wantRecord(verdict, user, s.policy.shape).String())
}

// challenge records the intermediate round of a conversation that ran in
// several requests, each carrying back the State the server issued. A
// single-request login records none, which is what makes the EAP scenario's
// State assertion discriminating.
func (s *stubLab) challenge(user string) {
	if !s.policy.challenges {
		return
	}
	s.record(verdictStateEcho, user)
}

// runLogin walks the chain: RADIUS first, then local when RADIUS gave no
// answer at all, then authorization over the attached profile.
func (s *stubLab) runLogin(user, password, command string) string {
	authenticated := false
	profile := ""
	switch {
	case s.policy.silentUsers[user]:
		s.record(verdictSilent, user)
		if s.policy.localPasswords[user] == password {
			authenticated = true
			profile = "admin"
			s.zeLog = append(s.zeLog, "INFO SSH auth success username="+user+" remote=127.0.0.1:1 source="+sourceLocal)
		}
	case s.policy.serverPasswords[user] == password && (s.policy.shape.CHAPPassword == credentialAbsent || s.policy.serverCanCHAP):
		s.challenge(user)
		s.record(verdictAccept, user)
		authenticated = true
		profile = s.policy.profile
		s.zeLog = append(s.zeLog, "INFO SSH auth success username="+user+" remote=127.0.0.1:1 source="+sourceRADIUS)
	default:
		s.challenge(user)
		s.record(verdictReject, user)
		if s.policy.fallThrough && s.policy.localPasswords[user] == password {
			authenticated = true
			profile = "admin"
			s.zeLog = append(s.zeLog, "INFO SSH auth success username="+user+" remote=127.0.0.1:1 source="+sourceLocal)
		}
	}
	if !authenticated {
		s.zeLog = append(s.zeLog, "WARN SSH auth failure username="+user+" remote=127.0.0.1:1")
		return "error: cannot connect to daemon: ssh: handshake failed\n" + cliExitMarker + "1\n"
	}
	if profile != "admin" && command == deniedCommand {
		return "error: " + unauthorizedMessage + ": " + command + "\n" + cliExitMarker + "1\n"
	}
	return "ze version fixture\n" + cliExitMarker + "0\n"
}

func (s *stubLab) Exec(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command := strings.Join(argv, " ")
	switch {
	case peer == zePeer && strings.Contains(command, "ze cli --user "):
		user, password, run := parseStubLogin(command)
		return interoplab.CommandResult{Stdout: s.runLogin(user, password, run)}, nil
	case peer == serverPeer && strings.Contains(command, "cat "+serverLogPath):
		if len(s.records) == 0 {
			return interoplab.CommandResult{}, errors.New("cat: no such file")
		}
		return interoplab.CommandResult{Stdout: strings.Join(s.records, "\n") + "\n"}, nil
	case peer == serverPeer && strings.Contains(command, "radclient"):
		if !s.policy.papProbeAccepted {
			return interoplab.CommandResult{Stdout: "Received Access-Reject Id 1\n"}, nil
		}
		return interoplab.CommandResult{Stdout: "Received Access-Accept Id 1\n"}, nil
	}
	return interoplab.CommandResult{}, errors.New("unexpected exec: " + peer + " " + command)
}

func (s *stubLab) Query(ctx context.Context, peer string, argv []string, environ []interoplab.EnvironmentVariable) (string, error) {
	result, err := s.Exec(ctx, peer, argv, environ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", errors.New("stub query returned no output")
	}
	return result.Stdout, nil
}

func (s *stubLab) Logs(_ context.Context, peer string, _ int) (interoplab.LogResult, error) {
	if peer != zePeer {
		return interoplab.LogResult{}, errors.New("unexpected log read: " + peer)
	}
	return interoplab.LogResult{Text: strings.Join(s.zeLog, "\n") + "\n", Available: true}, nil
}

func (s *stubLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	return errors.New("stub lab starts nothing")
}
func (s *stubLab) PeerPID(context.Context, string) (int, error) {
	return 0, errors.New("stub lab has no process")
}
func (s *stubLab) Signal(context.Context, string, string) error {
	return errors.New("stub lab signals nothing")
}
func (s *stubLab) Pause(context.Context, string) error { return errors.New("stub lab pauses nothing") }
func (s *stubLab) Unpause(context.Context, string) error {
	return errors.New("stub lab pauses nothing")
}
func (s *stubLab) Start(context.Context, string) error { return errors.New("stub lab starts nothing") }
func (s *stubLab) Stop(context.Context, string, int) error {
	return errors.New("stub lab stops nothing")
}

// parseStubLogin reads the user, the password and the command back out of the
// shell the checker built, so the stub answers the real script rather than a
// second copy of it.
func parseStubLogin(script string) (user, password, command string) {
	// The command is read from the ze cli invocation onward, not from the whole
	// script. The checker hands Exec an argv of {"sh", "-c", script}, so the
	// FIRST " -c " in the joined string is the shell's own and the field after
	// it is the script rather than the command. Reading that one silently
	// yielded an empty command on every call, which made the denied-command
	// branch of runLogin unreachable and its polarity untestable.
	cli := strings.Index(script, "ze cli --user ")
	if cli < 0 {
		return "", "", ""
	}
	// The user comes from --user and the password from ZE_SSH_PASSWORD. Naming
	// the results is not enough to keep them in that order: returning them
	// swapped compiles, and every login then arrives at runLogin as an unknown
	// user with the username as its password, which the chain refuses.
	// TestParseStubLoginReadsEachFieldInOrder pins the positions.
	return stubField(script, "--user "), stubField(script, "ZE_SSH_PASSWORD="), stubField(script[cli:], " -c ")
}

func stubField(script, token string) string {
	_, rest, found := strings.Cut(script, token)
	if !found {
		return ""
	}
	if !strings.HasPrefix(rest, "'") {
		return ""
	}
	end := strings.Index(rest[1:], "'")
	if end < 0 {
		return ""
	}
	return rest[1 : 1+end]
}

func TestPAPCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() labPolicy
		wantErr string
	}{
		{
			name:   "a conforming chain passes",
			policy: conformingPAP,
		},
		{
			name: "a chain that falls through to local after Access-Reject fails",
			policy: func() labPolicy {
				policy := conformingPAP()
				policy.fallThrough = true
				return policy
			},
			wantErr: "the chain fell through",
		},
		{
			name: "an Access-Accept that attaches admin instead of the named profile fails",
			policy: func() labPolicy {
				policy := conformingPAP()
				policy.profile = "admin"
				return policy
			},
			wantErr: "was allowed for radiusop",
		},
		{
			name: "a login with no server record fails, whatever ze logged",
			policy: func() labPolicy {
				policy := conformingPAP()
				policy.recordRequests = false
				return policy
			},
			wantErr: "never measured peer state",
		},
		{
			name: "an Access-Request carrying both credentials fails",
			policy: func() labPolicy {
				policy := conformingPAP()
				policy.shape = bothShape
				return policy
			},
			wantErr: "FreeRADIUS record",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkPAP(context.Background(), newStubLab(testCase.policy()))
			assertCheckerVerdict(t, err, testCase.wantErr)
		})
	}
}

func TestCHAPCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() labPolicy
		wantErr string
	}{
		{
			name:   "a server that reproduces the digest passes",
			policy: conformingCHAP,
		},
		{
			name: "a server that cannot verify the digest fails",
			policy: func() labPolicy {
				policy := conformingCHAP()
				policy.serverCanCHAP = false
				return policy
			},
			wantErr: "could not run",
		},
		{
			name: "a request carrying a User-Password beside the CHAP-Password fails",
			policy: func() labPolicy {
				policy := conformingCHAP()
				policy.shape = bothShape
				return policy
			},
			wantErr: "FreeRADIUS record",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkCHAP(context.Background(), newStubLab(testCase.policy()))
			assertCheckerVerdict(t, err, testCase.wantErr)
		})
	}
}

func TestCHAPHashedCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() labPolicy
		wantErr string
	}{
		{
			name:   "a server holding a hash rejects, which is what the scenario asserts",
			policy: conformingCHAPHashed,
		},
		{
			name: "a server that accepts CHAP against a hash fails, because RFC 2865 Section 2.2 forbids it",
			policy: func() labPolicy {
				policy := conformingCHAPHashed()
				policy.serverCanCHAP = true
				return policy
			},
			wantErr: "RFC 2865 Section 2.2 requires an Access-Reject",
		},
		{
			name: "a user file entry the PAP probe refuses fails before the rejection is read",
			policy: func() labPolicy {
				policy := conformingCHAPHashed()
				policy.papProbeAccepted = false
				return policy
			},
			wantErr: "its user entry is wrong",
		},
		{
			name: "a rejection ze answered from a local account fails",
			policy: func() labPolicy {
				policy := conformingCHAPHashed()
				policy.fallThrough = true
				policy.localPasswords[radiusUser] = chapPassword
				return policy
			},
			wantErr: "while the server rejected the request",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkCHAPHashed(context.Background(), newStubLab(testCase.policy()))
			assertCheckerVerdict(t, err, testCase.wantErr)
		})
	}
}

func TestEAPCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() labPolicy
		wantErr string
	}{
		{
			name:   "a server that runs the whole conversation passes",
			policy: conformingEAP,
		},
		{
			name: "a login answered in one request fails, because no EAP conversation ran",
			policy: func() labPolicy {
				policy := conformingEAP()
				policy.challenges = false
				return policy
			},
			wantErr: "verdict=state-echo",
		},
		{
			name: "a request carrying a password credential beside the EAP-Message fails",
			policy: func() labPolicy {
				policy := conformingEAP()
				policy.shape = papShape
				return policy
			},
			wantErr: "FreeRADIUS record",
		},
		{
			name: "a chain that falls through to local after Access-Reject fails",
			policy: func() labPolicy {
				policy := conformingEAP()
				policy.fallThrough = true
				return policy
			},
			wantErr: "the chain fell through",
		},
		{
			name: "a server that keeps no record fails, because ze's own log cannot prove the request arrived",
			policy: func() labPolicy {
				policy := conformingEAP()
				policy.recordRequests = false
				return policy
			},
			wantErr: "FreeRADIUS record",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkEAP(context.Background(), newStubLab(testCase.policy()))
			assertCheckerVerdict(t, err, testCase.wantErr)
		})
	}
}

func assertCheckerVerdict(t *testing.T, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("the conforming chain failed: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("the broken chain passed; the checker should have named %q", wantErr)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("the checker failed for the wrong reason: want %q, got %v", wantErr, err)
	}
}

func TestParseLoginOutput(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		wantCode int
		wantBody string
		wantErr  string
	}{
		{name: "a successful run", output: "ze version\nZE-CLI-EXIT=0\n", wantCode: 0, wantBody: "ze version"},
		{name: "a refused command", output: "error: denied\nZE-CLI-EXIT=1\n", wantCode: 1, wantBody: "error: denied"},
		{name: "no marker is never a zero exit", output: "ze version\n", wantErr: "produced no ZE-CLI-EXIT= marker"},
		{name: "a store that could not be seeded", output: "ZE-CLI-INIT-FAILED\nboom\n", wantErr: "could not be seeded"},
		{name: "a marker that is not a number", output: "ZE-CLI-EXIT=x\n", wantErr: "is not a number"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := parseLoginOutput(testCase.output)
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("want error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ExitCode != testCase.wantCode || result.Output != testCase.wantBody {
				t.Fatalf("want exit %d body %q, got exit %d body %q",
					testCase.wantCode, testCase.wantBody, result.ExitCode, result.Output)
			}
		})
	}
}

func TestParseServerRecord(t *testing.T) {
	complete := "verdict=accept user=radiusop user-password=present chap-password=absent eap-message=absent nas-identifier=ze-interop-nas"
	record, ok := parseServerRecord(complete)
	if !ok {
		t.Fatalf("a complete line was refused: %s", complete)
	}
	want := wantRecord(verdictAccept, radiusUser, papShape)
	if !want.matches(record) {
		t.Fatalf("a complete line did not match its own expectation: %+v", record)
	}

	for _, line := range []string{
		"",
		"verdict=accept user=radiusop",
		"verdict=accept user=radiusop user-password=present chap-password=absent",
		"user=radiusop user-password=present chap-password=absent eap-message=absent nas-identifier=ze-interop-nas",
		// A line written by the linelog format this lab carried before EAP: the
		// eap-message field is absent, so it says nothing about whether an
		// EAP-Message arrived and is not a verdict this checker can read.
		"verdict=accept user=radiusop user-password=present chap-password=absent nas-identifier=ze-interop-nas",
	} {
		if _, ok := parseServerRecord(line); ok {
			t.Fatalf("an incomplete line was read as a verdict: %q", line)
		}
	}

	// One field wrong is a mismatch, not a near miss.
	other := wantRecord(verdictAccept, radiusUser, chapShape)
	if other.matches(record) {
		t.Fatal("a record with the other credential shape matched")
	}
}

func TestLogFieldReadsWholeKeysOnly(t *testing.T) {
	line := "INFO SSH auth success username=radiusop remote=127.0.0.1:51408 source=radius profiles=[interop-operator]"
	if got := logField(line, "username"); got != "radiusop" {
		t.Fatalf("username: got %q", got)
	}
	if got := logField(line, "source"); got != "radius" {
		t.Fatalf("source: got %q", got)
	}
	// "name" is a suffix of "username" and must not match it.
	if got := logField(line, "name"); got != "" {
		t.Fatalf("a suffix key matched a longer key: got %q", got)
	}
	if got := logField(line, "absent"); got != "" {
		t.Fatalf("an absent key answered %q", got)
	}
}

func TestAuthSuccessSourcesAndFailures(t *testing.T) {
	text := strings.Join([]string{
		"INFO SSH auth success username=localop remote=1 source=local",
		"INFO SSH auth success username=radiusop remote=1 source=radius",
		"WARN SSH auth failure username=radiusop remote=1",
		"INFO SSH auth success username=radiusop2 remote=1 source=local",
	}, "\n")
	sources := authSuccessSources(text, radiusUser)
	if len(sources) != 1 || sources[0] != sourceRADIUS {
		t.Fatalf("want one radius source for %s, got %v", radiusUser, sources)
	}
	if got := authSuccessSources(text, localUser); len(got) != 1 || got[0] != sourceLocal {
		t.Fatalf("want one local source for %s, got %v", localUser, got)
	}
	if got := authFailures(text, radiusUser); got != 1 {
		t.Fatalf("want one recorded failure for %s, got %d", radiusUser, got)
	}
	if got := authFailures(text, localUser); got != 0 {
		t.Fatalf("a failure was attributed to %s: %d", localUser, got)
	}
}

func TestLoginScriptQuotesItsArguments(t *testing.T) {
	script := loginScript("radiusop", "pass'word", "show bgp")
	for _, want := range []string{"--user 'radiusop'", "ZE_SSH_PASSWORD='pass'\\''word'", "-c 'show bgp'", cliExitMarker} {
		if !strings.Contains(script, want) {
			t.Fatalf("the login script does not carry %q:\n%s", want, script)
		}
	}
}

// TestScenarioPopulationMatchesRegistry pins the fixture directories against
// the checker registry in both directions. Discover already refuses a directory
// with no checker; this also refuses a checker with no directory, which would
// otherwise be a scenario nobody runs.
func TestScenarioPopulationMatchesRegistry(t *testing.T) {
	root := repositoryRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, labDirectory, "scenarios"))
	if err != nil {
		t.Fatalf("read scenario directory: %v", err)
	}
	onDisk := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			onDisk = append(onDisk, entry.Name())
		}
	}
	sort.Strings(onDisk)
	registered := ScenarioNames()
	if strings.Join(onDisk, ",") != strings.Join(registered, ",") {
		t.Fatalf("scenario directories %v do not match the checker registry %v", onDisk, registered)
	}
}

// TestScenarioNamesAreNamedNotNumbered holds the rule in
// ai/rules/interop-and-goal-validation.md: a scenario directory is NAMED, and a
// numeric prefix goes stale in two ways a name cannot.
func TestScenarioNamesAreNamedNotNumbered(t *testing.T) {
	for _, name := range ScenarioNames() {
		if name == "" || (name[0] >= '0' && name[0] <= '9') {
			t.Fatalf("scenario %q carries a numeric prefix", name)
		}
	}
}

// TestSuiteNeedsNoKernelModule is AC-8. The L2TP suite refuses to run without
// l2tp_ppp or pppol2tp, and hosting these scenarios there would have inherited
// that gate. Admin login is a UDP socket and ze's own listeners, so no peer
// here mounts the module tree, asks for a capability, or runs privileged.
func TestSuiteNeedsNoKernelModule(t *testing.T) {
	root := repositoryRoot(t)
	suite, err := suiteFor(root, interoplab.Environment{Suffix: "test", SessionTimeout: time.Second}, interoplab.NewDocker())
	if err != nil {
		t.Fatalf("build the suite: %v", err)
	}
	if len(suite.Scenarios) == 0 {
		t.Fatal("the suite declares no scenario")
	}
	for _, plan := range suite.Scenarios {
		for _, peer := range plan.Peers {
			for _, mount := range peer.Mounts {
				if strings.HasPrefix(mount.Source, "/lib/modules") || strings.HasPrefix(mount.Target, "/lib/modules") {
					t.Fatalf("%s in %s mounts the kernel module tree", peer.Name, plan.Source.Name)
				}
			}
			if len(peer.Capabilities) != 0 {
				t.Fatalf("%s in %s asks for capabilities %v", peer.Name, plan.Source.Name, peer.Capabilities)
			}
			for _, argument := range peer.Arguments {
				if argument == "--privileged" {
					t.Fatalf("%s in %s runs privileged", peer.Name, plan.Source.Name)
				}
			}
			if peer.Ready == nil {
				t.Fatalf("%s in %s has no readiness probe", peer.Name, plan.Source.Name)
			}
		}
	}
}

// TestServerImageIsPinned holds R-3: a moving tag changes what these scenarios
// mean with no ze change.
func TestServerImageIsPinned(t *testing.T) {
	name, version, found := strings.Cut(serverImage, ":")
	if !found || version == "" || version == "latest" {
		t.Fatalf("the FreeRADIUS image %q is not pinned to an exact version", serverImage)
	}
	if !strings.Contains(name, "freeradius") {
		t.Fatalf("the peer image %q is not FreeRADIUS; a peer built from ze's own code proves nothing", serverImage)
	}
}

// TestServerReadsTheLabConfiguration proves each mount this suite declares
// names a file that exists, so a renamed fixture fails here rather than as an
// unexplained FreeRADIUS start failure inside a container.
func TestServerReadsTheLabConfiguration(t *testing.T) {
	root := repositoryRoot(t)
	suite, err := suiteFor(root, interoplab.Environment{Suffix: "test", SessionTimeout: time.Second}, interoplab.NewDocker())
	if err != nil {
		t.Fatalf("build the suite: %v", err)
	}
	for _, plan := range suite.Scenarios {
		for _, peer := range plan.Peers {
			for _, mount := range peer.Mounts {
				if _, err := os.Stat(mount.Source); err != nil {
					t.Fatalf("%s in %s mounts a missing file: %v", peer.Name, plan.Source.Name, err)
				}
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Skipf("repository root not resolvable: %v", err)
	}
	return root
}

// TestParseStubLoginReadsEachFieldInOrder pins the three fields to their
// positions. The stub reads one shell script and answers three strings of the
// same type, so a swapped pair compiles and reads correctly as a set: the
// username arrives as the password, the chain refuses every login, and every
// polarity case fails at its first assertion with one identical message.
func TestParseStubLoginReadsEachFieldInOrder(t *testing.T) {
	script := strings.Join([]string{"sh", "-c", loginScript(radiusUser, papPassword, deniedCommand)}, " ")
	user, password, command := parseStubLogin(script)
	if user != radiusUser {
		t.Fatalf("user: want %q, got %q", radiusUser, user)
	}
	if password != papPassword {
		t.Fatalf("password: want %q, got %q", papPassword, password)
	}
	if command != deniedCommand {
		t.Fatalf("command: want %q, got %q", deniedCommand, command)
	}
	if _, _, got := parseStubLogin("sh -c echo hello"); got != "" {
		t.Fatalf("a script with no ze cli invocation answered command %q", got)
	}
}
