// Design: docs/architecture/testing/runner-architecture.md -- nested expectation deadlines.
package webtesting

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var browserExpectationFixture = flag.String("browser-expectation-fixture", "", "owned expectation command fixture")

// TestBrowserExpectationFixture answers one actual agent-browser invocation from
// its test owner's HTTP rendezvous. The wrapper execs this process, so command
// cancellation leaves no shell or detached helper holding its output descriptors.
func TestBrowserExpectationFixture(t *testing.T) {
	if *browserExpectationFixture == "" {
		return
	}
	query := url.Values{}
	for _, arg := range flag.Args() {
		query.Add("arg", arg)
	}
	query.Set("session", os.Getenv("AGENT_BROWSER_SESSION"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, *browserExpectationFixture+"?"+query.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	if err := errors.Join(readErr, response.Body.Close()); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("owned browser command failed: %s", body)
	}
	if _, err := os.Stdout.Write(body); err != nil {
		t.Fatal(err)
	}
	// The parent consumes exact CLI output, not the Go harness's PASS trailer.
	cancel()
	os.Exit(0)
}

// TestWBExpectationNestedAllowance drives RunWBCase through the outer retry and
// an actual executable into both nested positive and fetch retries. The first
// query rendezvous starts the fixture's readiness clock: answers become usable
// one second after the old outer limit, but before the existing inner allowance.
// No global timeout is changed. An earlier absolute case deadline still refuses
// that answer, retains the failed step, and runs separately bounded cleanup.
// MUTATION: bounding checkExpectationRetry below expectDeadline truncates both retries.
func TestWBExpectationNestedAllowance(t *testing.T) {
	for _, test := range []struct {
		name        string
		concurrency int
		delay       time.Duration
		fetch       bool
		earlier     bool
	}{
		{name: "serial-positive", concurrency: 1, delay: 6 * time.Second},
		{name: "concurrent-positive", concurrency: 3, delay: 16 * time.Second},
		{name: "serial-fetch", concurrency: 1, delay: 6 * time.Second, fetch: true},
		{name: "concurrent-fetch", concurrency: 3, delay: 16 * time.Second, fetch: true},
		{name: "serial-earlier-case", concurrency: 1, delay: 6 * time.Second, earlier: true},
		{name: "concurrent-earlier-case", concurrency: 3, delay: 16 * time.Second, earlier: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent := context.Background()
			if test.earlier {
				ctx, cancel := context.WithTimeout(parent, 2*time.Second)
				defer cancel()
				parent = ctx
			}
			ctx, cancel, err := CaseContext(parent, expectDeadline+5*time.Second, test.concurrency)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			deadline, _ := ctx.Deadline()
			allowance := Bound(ctx, expectDeadline)
			if test.delay >= allowance {
				t.Fatalf("fixture delay %s must fit inside nested allowance %s", test.delay, allowance)
			}

			session := "owned-expectation-" + test.name
			fixture := &browserExpectationRendezvous{session: session, delay: test.delay, fetch: test.fetch}
			server := httptest.NewServer(fixture)
			defer server.Close()
			installExpectationAgentBrowser(t, server.URL)
			expect := "expect=html:contains=owned-ready\n"
			if test.fetch {
				expect = "expect=html:not-contains=forbidden\n"
			}
			tc, err := ParseWBFile("action=open:path=/owned\n" + expect + "expect=url:contains=/owned-ready\n")
			if err != nil {
				t.Fatal(err)
			}
			result := RunWBCase(ctx, tc, "http://owned.invalid", session)
			fixture.mu.Lock()
			commands := slices.Clone(fixture.commands)
			queries, first, ready := fixture.queries, fixture.first, fixture.ready
			writeErr := fixture.writeErr
			fixture.mu.Unlock()
			t.Logf("delay=%s nested=%s first-query=%s answer-ready=%s case-deadline=%s queries=%d commands=%v",
				test.delay, allowance, first, first.Add(test.delay), deadline, queries, commands)
			if writeErr != nil {
				t.Errorf("owned fixture response failed: %v", writeErr)
			}
			if queries < 2 {
				t.Errorf("expectation did not retry the real executable: queries=%d", queries)
			}
			if len(commands) == 0 {
				t.Fatal("owned executable received no commands")
			}
			if commands[len(commands)-1] != "close" {
				t.Errorf("case did not finish owned cleanup: %v", commands)
			}
			if count := slices.Index(commands, "close"); count != len(commands)-1 {
				t.Errorf("cleanup was not exactly one final owned command: %v", commands)
			}
			if !result.Passed && strings.Contains(result.Error, "cleanup incomplete") {
				t.Errorf("execution failure prevented separately bounded cleanup: %s", result.Error)
			}
			if test.earlier {
				if result.Passed {
					t.Fatal("nested retry extended the earlier absolute case deadline")
				}
				if !strings.Contains(result.Error, context.DeadlineExceeded.Error()) {
					t.Errorf("case lost its deadline cause: %s", result.Error)
				}
				if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.Errorf("case returned without exhausting its original deadline: %v", ctx.Err())
				}
				if ready {
					t.Error("fixture supplied a delayed answer after the earlier case deadline")
				}
				if slices.Contains(commands, "get url") {
					t.Error("case ran the following expectation after its deadline")
				}
				if len(result.Steps) != 2 {
					t.Fatalf("deadline lost its action and failed expectation: %+v", result.Steps)
				}
				if !result.Steps[0].Passed {
					t.Fatalf("deadline changed the completed action verdict: %+v", result.Steps)
				}
				if result.Steps[1].Passed {
					t.Fatalf("deadline changed the failed expectation verdict: %+v", result.Steps)
				}
				return
			}
			if !result.Passed {
				t.Errorf("expectation refused an answer inside its nested allowance: %s", result.Error)
			}
			if !ready {
				t.Error("case returned before the fixture supplied its delayed answer")
			}
			if err := ctx.Err(); err != nil {
				t.Errorf("nested operation exhausted the case budget: %v", err)
			}
			if len(result.Steps) != 3 {
				t.Fatalf("case did not reach its following expectation: %+v", result.Steps)
			}
			for _, step := range result.Steps {
				if !step.Passed {
					t.Errorf("completed case retained a failed step: %+v", step)
				}
			}
		})
	}
}

// TestWBExpectationSequentialAllowances proves one expectation can spend its
// fetch allowance before starting its separate positive allowance. The owned
// executable first fails transport, then returns nonmatching HTML, then matches.
// Each phase fits its own limit, but their sum exceeds one shared ancestor limit.
// MUTATION: a shared expectDeadline ancestor truncates sequential nested retries.
func TestWBExpectationSequentialAllowances(t *testing.T) {
	for _, test := range []struct {
		name           string
		concurrency    int
		transportDelay time.Duration
		matchDelay     time.Duration
		caseLimit      time.Duration
	}{
		{name: "serial", concurrency: 1, transportDelay: 8 * time.Second, matchDelay: 16 * time.Second},
		{name: "concurrent", concurrency: 3, transportDelay: 23 * time.Second, matchDelay: 46 * time.Second},
		{name: "earlier-case", concurrency: 1, transportDelay: 8 * time.Second, matchDelay: 16 * time.Second, caseLimit: 12 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := "owned-sequential-" + test.name
			fixture := &browserExpectationRendezvous{
				session: session, delay: test.matchDelay, transportDelay: test.transportDelay,
			}
			server := httptest.NewServer(fixture)
			defer server.Close()
			installExpectationAgentBrowser(t, server.URL)
			tc, err := ParseWBFile("action=open:path=/owned\nexpect=html:contains=owned-ready\nexpect=url:contains=/owned-ready\n")
			if err != nil {
				t.Fatal(err)
			}
			parent := context.Background()
			if test.caseLimit > 0 {
				ctx, cancel := context.WithTimeout(parent, test.caseLimit)
				defer cancel()
				parent = ctx
			}
			ctx, cancel, err := CaseContext(parent, expectDeadline+5*time.Second, test.concurrency)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			allowance := Bound(ctx, expectDeadline)
			if test.transportDelay >= allowance {
				t.Fatalf("transport phase %s exceeds its allowance %s", test.transportDelay, allowance)
			}
			if test.matchDelay-test.transportDelay >= allowance {
				t.Fatalf("positive phase %s exceeds its allowance %s", test.matchDelay-test.transportDelay, allowance)
			}
			if test.matchDelay <= allowance {
				t.Fatalf("combined phase %s must exceed a shared ancestor allowance %s", test.matchDelay, allowance)
			}
			deadline, _ := ctx.Deadline()
			result := RunWBCase(ctx, tc, "http://owned.invalid", session)
			fixture.mu.Lock()
			commands := slices.Clone(fixture.commands)
			failures, pending := fixture.transportFailures, fixture.pendingAnswers
			first, ready, writeErr := fixture.first, fixture.ready, fixture.writeErr
			fixture.mu.Unlock()
			t.Logf("phase-allowance=%s first-query=%s transport-ready=%s match-ready=%s case-deadline=%s transport-failures=%d pending-answers=%d",
				allowance, first, first.Add(test.transportDelay), first.Add(test.matchDelay), deadline, failures, pending)
			if writeErr != nil {
				t.Errorf("owned fixture response failed: %v", writeErr)
			}
			if failures < 2 {
				t.Errorf("fetch did not retry executable transport failures: %d", failures)
			}
			if pending < 2 {
				t.Errorf("positive check did not retry usable nonmatching HTML: %d", pending)
			}
			if len(commands) == 0 {
				t.Fatal("owned executable received no commands")
			}
			if commands[len(commands)-1] != "close" {
				t.Errorf("case did not finish owned cleanup: %v", commands)
			}
			if index := slices.Index(commands, "close"); index != len(commands)-1 {
				t.Errorf("cleanup was not exactly one final owned command: %v", commands)
			}
			if strings.Contains(result.Error, "cleanup incomplete") {
				t.Errorf("separate cleanup failed: %s", result.Error)
			}
			if test.caseLimit > 0 {
				if result.Passed {
					t.Fatal("sequential retries extended the earlier absolute case deadline")
				}
				if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.Errorf("original case deadline was not exhausted: %v", ctx.Err())
				}
				if !strings.Contains(result.Error, context.DeadlineExceeded.Error()) {
					t.Errorf("case lost its deadline cause: %s", result.Error)
				}
				if ready {
					t.Error("fixture supplied a match beyond the earlier case deadline")
				}
				if slices.Contains(commands, "get url") {
					t.Error("case ran the following expectation after its deadline")
				}
				if len(result.Steps) != 2 {
					t.Fatalf("deadline lost the completed action or failed expectation: %+v", result.Steps)
				}
				if !result.Steps[0].Passed {
					t.Errorf("deadline changed completed action verdict: %+v", result.Steps)
				}
				if result.Steps[1].Passed {
					t.Errorf("deadline changed failed expectation verdict: %+v", result.Steps)
				}
				return
			}
			if !result.Passed {
				t.Errorf("shared ancestor truncated separately allowed retries: %s", result.Error)
			}
			if !ready {
				t.Error("case returned before the fixture supplied its eventual match")
			}
			if err := ctx.Err(); err != nil {
				t.Errorf("sequential retries exhausted the case budget: %v", err)
			}
			if len(result.Steps) != 3 {
				t.Fatalf("case did not reach its following expectation: %+v", result.Steps)
			}
			for _, step := range result.Steps {
				if !step.Passed {
					t.Errorf("successful case retained a failed step: %+v", step)
				}
			}
		})
	}
}

// browserExpectationRendezvous owns the fixture state; mu protects observations
// shared by HTTP handlers and the case owner. first is unset until the executable
// asks for HTML, so navigation and process startup do not consume the delay.
type browserExpectationRendezvous struct {
	mu                sync.Mutex
	session           string
	delay             time.Duration
	transportDelay    time.Duration
	transportFailures int
	pendingAnswers    int
	fetch             bool
	commands          []string
	queries           int
	first             time.Time
	ready             bool
	writeErr          error
}

func (f *browserExpectationRendezvous) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Query().Get("session") != f.session {
		http.Error(w, "foreign browser session", http.StatusBadRequest)
		return
	}
	args := r.URL.Query()["arg"]
	if len(args) > 0 && args[0] == "--ignore-https-errors" {
		args = args[1:]
	}
	command := strings.Join(args, " ")
	f.commands = append(f.commands, command)
	body := ""
	switch {
	case command == "open about:blank", command == "open http://owned.invalid/owned", command == "close":
	case command == "eval "+inflightIdleExpr:
		body = "true\n"
	case command == "get url":
		body = "http://owned.invalid/owned-ready\n"
	case command == "get html body":
		f.queries++
		if f.first.IsZero() {
			f.first = time.Now()
		}
		if time.Now().Before(f.first.Add(f.transportDelay)) {
			f.transportFailures++
			http.Error(w, "owned HTML transport not ready", http.StatusServiceUnavailable)
			return
		}
		if time.Now().Before(f.first.Add(f.delay)) {
			if f.fetch {
				http.Error(w, "owned HTML query not ready", http.StatusServiceUnavailable)
				return
			}
			body = "<p>owned-pending</p>\n"
			f.pendingAnswers++
		} else {
			f.ready = true
			body = "<p>owned-ready</p>\n"
		}
	default:
		http.Error(w, "unexpected owned command: "+command, http.StatusBadRequest)
		return
	}
	if _, err := io.WriteString(w, body); err != nil {
		f.writeErr = errors.Join(f.writeErr, err)
	}
}

func installExpectationAgentBrowser(t *testing.T, endpoint string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Browser.Open registers the render observer before it navigates: it opens
	// the owned blank page, then evaluates and registers inflightInitJS through
	// the native SDK socket. The shared fake observer answers that socket and
	// refuses an application open that precedes registration, as it does for
	// the other runner fixtures. Its bootstrap helper exits before the exec, so
	// the exec'd fixture still owns the command's output descriptors.
	bootstrap := installFakeObserver(t, filepath.Join(dir, "observer.log"))
	script := "#!/bin/sh\nexport GORACE=atexit_sleep_ms=0\n" + bootstrap + "exec " + strconv.Quote(binary) +
		" -test.run=^TestBrowserExpectationFixture$ -browser-expectation-fixture=" + strconv.Quote(endpoint) + " -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, agentBrowserBin), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
