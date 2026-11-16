// Design: docs/architecture/container/runtime.md -- the stop: the stop signal, the stop timeout, the kill

package container

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// drainInputStop is the drain request of a whole-daemon stop. The handler
// reads none of its fields.
func drainInputStop() rpc.DrainInput { return rpc.DrainInput{} }

// stopFixture is a supervisor over a recording library whose containers obey
// or ignore their stop signal, with one participant, holding the enabled
// containers instances, each started.
func stopFixture(t *testing.T, ignoresStop bool, instances ...instanceConfig) (*supervisor, *fakeLibrary, *eventLog) {
	t.Helper()
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &fakeLibrary{events: events, initProcess: 4242, ignoresStop: ignoresStop}
	participant := &fakeParticipant{name: "network", events: events}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), []Participant{participant})
	if err := sup.install(configOf(instances...)); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	for _, instance := range instances {
		if _, err := sup.start(context.Background(), instance.name); err != nil {
			t.Fatalf("the start of %s failed: %v", instance.name, err)
		}
	}
	return sup, library, events
}

// stopKilled answers the names the stop-killed doctor check reports.
func stopKilled(t *testing.T, sup *supervisor) []string {
	t.Helper()
	findings, err := sup.answerDoctorCheck("container-stop-killed")
	if err != nil {
		t.Fatalf("check container-stop-killed refused: %v", err)
	}
	names := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.Code != codeStopKilled {
			t.Errorf("a stop-killed finding carries code %q, want %q", finding.Code, codeStopKilled)
		}
		names = append(names, strings.Fields(finding.Message)[1])
	}
	return names
}

// TestStopSignalComesFromTheConfigThenTheImage proves the order the stop
// signal is chosen in: the config's stop-signal, else the image's in each form
// an image writes it, else the termination signal; and that an image signal
// Ze cannot read refuses the start with the statement that names one.
func TestStopSignalComesFromTheConfigThenTheImage(t *testing.T) {
	chosen := []struct {
		configured stopSignal
		image      string
		want       syscall.Signal
	}{
		{stopSignalUsr1, "SIGQUIT", syscall.SIGUSR1},
		{stopSignalHup, "", syscall.SIGHUP},
		{stopSignalFromImage, "SIGQUIT", syscall.SIGQUIT},
		{stopSignalFromImage, "quit", syscall.SIGQUIT},
		{stopSignalFromImage, "9", syscall.SIGKILL},
		{stopSignalFromImage, "", syscall.SIGTERM},
	}
	for _, c := range chosen {
		got, err := stopSignalOf("web", c.configured, c.image)
		if err != nil {
			t.Errorf("config %d and image %q refused: %v", c.configured, c.image, err)
			continue
		}
		if got != c.want {
			t.Errorf("config %d and image %q chose signal %d, want %d", c.configured, c.image, got, c.want)
		}
	}
	for _, unreadable := range []string{"SIGRTMIN+3", "0", "65", "SIGNOTHING"} {
		_, err := stopSignalOf("web", stopSignalFromImage, unreadable)
		var refusal *opererr.Error
		if !errors.As(err, &refusal) {
			t.Errorf("image stop signal %q answered %v, want an operator error", unreadable, err)
			continue
		}
		if refusal.Fix() != "set container instance web stop-signal term" {
			t.Errorf("image stop signal %q refused with fix %q, want the stop-signal statement", unreadable, refusal.Fix())
		}
	}
}

// TestStopLeavesParseWithTheirDefaults proves the two stop leaves: the stop
// timeout defaults to 10 seconds and accepts 1 to 30, each end included; the
// stop signal is read from its word and is the image's when absent; a value
// outside either is refused with the statement that sets it.
func TestStopLeavesParseWithTheirDefaults(t *testing.T) {
	accepted := []struct {
		section string
		timeout time.Duration
		signal  stopSignal
	}{
		{`{"container":{"instance":{"web":{}}}}`, 10 * time.Second, stopSignalFromImage},
		{`{"container":{"instance":{"web":{"stop-timeout":"1"}}}}`, time.Second, stopSignalFromImage},
		{`{"container":{"instance":{"web":{"stop-timeout":"30","stop-signal":"quit"}}}}`, 30 * time.Second, stopSignalQuit},
	}
	for _, c := range accepted {
		cfg, _, err := parseContainerSections(containerSections(c.section))
		if err != nil {
			t.Errorf("%s refused: %v", c.section, err)
			continue
		}
		instance := cfg.instances[0]
		if instance.stopTimeout != c.timeout {
			t.Errorf("%s gave stop timeout %s, want %s", c.section, instance.stopTimeout, c.timeout)
		}
		if instance.stopSignal != c.signal {
			t.Errorf("%s gave stop signal %d, want %d", c.section, instance.stopSignal, c.signal)
		}
	}
	refused := map[string]string{
		`{"container":{"instance":{"web":{"stop-timeout":"0"}}}}`:   "set container instance web stop-timeout SECONDS",
		`{"container":{"instance":{"web":{"stop-timeout":"31"}}}}`:  "set container instance web stop-timeout SECONDS",
		`{"container":{"instance":{"web":{"stop-signal":"kill"}}}}`: "set container instance web stop-signal term",
	}
	for section, fix := range refused {
		_, _, err := parseContainerSections(containerSections(section))
		var refusal *opererr.Error
		if !errors.As(err, &refusal) {
			t.Errorf("%s answered %v, want an operator error", section, err)
			continue
		}
		if refusal.Fix() != fix {
			t.Errorf("%s refused with fix %q, want %q", section, refusal.Fix(), fix)
		}
	}
}

// TestStopSendsTheSignalThenKillsAfterTheTimeout proves the stop of a
// container that ignores its stop signal: the configured signal is sent, the
// kill comes only after it, the cleanup still runs, and the stop-killed check
// reports the container. A container that obeys gets the signal and no kill,
// and is reported by nothing.
//
// MUTATION: kill before the signal, or skip the kill after the timeout, and
// the stop steps differ; drop the recordProblem of stopTogether's timed-out
// arm and the finding is missing.
func TestStopSendsTheSignalThenKillsAfterTheTimeout(t *testing.T) {
	stubborn := instanceConfig{name: "web", enabled: true, stopTimeout: 20 * time.Millisecond, stopSignal: stopSignalUsr1}
	sup, library, events := stopFixture(t, true, stubborn)

	if err := sup.stop(context.Background(), "web"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}

	if got, want := library.stops.all(), []string{"signal web " + strconv.Itoa(int(syscall.SIGUSR1)), "kill web"}; !slices.Equal(got, want) {
		t.Fatalf("the stop made %v, want %v: the configured signal, then the kill", got, want)
	}
	if !slices.Contains(events.all(), "ended web") {
		t.Errorf("the killed container got no ended call: %v", events.all())
	}
	if got := stopKilled(t, sup); !slices.Equal(got, []string{"web"}) {
		t.Errorf("the stop-killed check reports %v, want [web]", got)
	}

	obedient := instanceConfig{name: "db", enabled: true, stopTimeout: time.Hour}
	sup, library, _ = stopFixture(t, false, obedient)

	if err := sup.stop(context.Background(), "db"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}

	if got, want := library.stops.all(), []string{"signal db 15"}; !slices.Equal(got, want) {
		t.Fatalf("the stop of an obedient container made %v, want %v: the termination signal and no kill", got, want)
	}
	if got := stopKilled(t, sup); len(got) != 0 {
		t.Errorf("the stop-killed check reports %v for a container that obeyed, want none", got)
	}
}

// TestDrainSignalsEveryContainerBeforeAnyKill proves the stop drain's
// arithmetic: every container gets its stop signal before any is killed, so
// the drain waits the longest stop timeout, never their sum; each is killed
// once its own timeout passed; and no container starts after the drain.
//
// MUTATION: stop the containers one after the other (signal, wait, kill per
// container) and the order of the stop steps fails.
func TestDrainSignalsEveryContainerBeforeAnyKill(t *testing.T) {
	sup, library, _ := stopFixture(t, true,
		instanceConfig{name: "a", enabled: true, stopTimeout: 20 * time.Millisecond},
		instanceConfig{name: "b", enabled: true, stopTimeout: 20 * time.Millisecond},
	)

	if err := sup.drain(context.Background(), drainInputStop()); err != nil {
		t.Fatalf("the drain failed: %v", err)
	}

	want := []string{"signal a 15", "signal b 15", "kill a", "kill b"}
	if got := library.stops.all(); !slices.Equal(got, want) {
		t.Fatalf("the drain made %v, want %v: every signal before any kill", got, want)
	}
	if got := stopKilled(t, sup); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("the stop-killed check reports %v, want [a b]", got)
	}
	creates := library.creates()
	sup.converge("a")
	if library.creates() != creates {
		t.Errorf("a worker started a container after the drain")
	}
	if _, err := sup.start(context.Background(), "b"); err == nil {
		t.Errorf("a start after the drain was accepted, want it refused while the daemon stops")
	}
}

// TestForcedDrainKillsAtOnce proves a second signal during the drain: when
// the drain's context ends, a container whose stop timeout has not passed is
// killed at once, and it is not reported as one that ignored its signal.
func TestForcedDrainKillsAtOnce(t *testing.T) {
	sup, library, _ := stopFixture(t, true, instanceConfig{name: "web", enabled: true, stopTimeout: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := sup.drain(ctx, drainInputStop()); err != nil {
		t.Fatalf("the drain failed: %v", err)
	}

	if got, want := library.stops.all(), []string{"signal web 15", "kill web"}; !slices.Equal(got, want) {
		t.Fatalf("the forced drain made %v, want %v", got, want)
	}
	if got := stopKilled(t, sup); len(got) != 0 {
		t.Errorf("the stop-killed check reports %v after a forced drain, want none", got)
	}
}

// TestKillRunningKillsWhatStillRuns proves the plugin's exit: a container
// still running when the engine connection closed, because the daemon's stop
// skipped the drain, is killed at once with no stop signal, and nothing
// starts after it.
func TestKillRunningKillsWhatStillRuns(t *testing.T) {
	sup, library, events := stopFixture(t, true, instanceConfig{name: "web", enabled: true, stopTimeout: time.Hour})

	sup.killRunning()

	if got, want := library.stops.all(), []string{"kill web"}; !slices.Equal(got, want) {
		t.Fatalf("the exit made %v, want %v", got, want)
	}
	if sup.isRunning("web") {
		t.Errorf("the container still runs after the exit")
	}
	if !slices.Contains(events.all(), "ended web") {
		t.Errorf("the killed container got no ended call: %v", events.all())
	}
	if !sup.isStopping() {
		t.Errorf("the supervisor accepts starts after the exit")
	}
}

// TestSweepEndsEveryLeftoverBeforeAnyStart proves the sweep of a previous
// run: each leftover gets the ended call, whether or not the config holds it;
// one the kernel refused to remove whole is reported by the cleanup check,
// with the reboot as its fix, and one removed whole is not; and the report of
// a leftover ends once its container runs again.
//
// MUTATION: drop the ended calls from supervisor.sweep, or keep the leftover
// of a container that runs again, and an assertion fails.
func TestSweepEndsEveryLeftoverBeforeAnyStart(t *testing.T) {
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &fakeLibrary{events: events, initProcess: 4242, leftovers: []leftover{
		{name: "gone"},
		{name: "web", err: errors.New("remove control group /sys/fs/cgroup/ze-container/web: device or resource busy")},
	}}
	participant := &fakeParticipant{name: "network", events: events}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), []Participant{participant})

	sup.sweep()

	if got, want := events.all(), []string{"sweep", "ended gone", "ended web"}; !slices.Equal(got, want) {
		t.Fatalf("the sweep made %v, want %v", got, want)
	}
	findings, err := sup.answerDoctorCheck("container-cleanup-incomplete")
	if err != nil {
		t.Fatalf("check container-cleanup-incomplete refused: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("the cleanup check reports %v, want the one leftover the kernel kept", findings)
	}
	if !strings.HasPrefix(findings[0].Message, "container web ") {
		t.Errorf("the finding %q does not name the leftover web", findings[0].Message)
	}
	if findings[0].Fix != leftoverFix {
		t.Errorf("the finding has fix %q, want %q", findings[0].Fix, leftoverFix)
	}

	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true, stopTimeout: time.Second})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if _, err := sup.start(context.Background(), "web"); err != nil {
		t.Fatalf("the start failed: %v", err)
	}
	findings, err = sup.answerDoctorCheck("container-cleanup-incomplete")
	if err != nil {
		t.Fatalf("check container-cleanup-incomplete refused: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("the cleanup check still reports %v once the container runs again", findings)
	}
	t.Cleanup(sup.killRunning)
}
