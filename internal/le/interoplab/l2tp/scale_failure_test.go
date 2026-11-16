// Design: docs/labs/l2tp-interop.md -- preserve the cause and bounded peer files.
// Related: scale_failure.go -- reads actual process and PPP logs before cleanup.
package l2tp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type scaleFailureLab struct {
	recordingLab
	readError    error
	oversize     bool
	deadline     time.Time
	contextError error
}

func (l *scaleFailureLab) Exec(ctx context.Context, peer string, arguments []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), arguments...)})
	if err := ctx.Err(); err != nil {
		l.contextError = err
	}
	deadline, exists := ctx.Deadline()
	if !exists {
		l.contextError = errors.New("diagnostic read has no deadline")
	}
	if time.Until(deadline) > 15*time.Second {
		l.contextError = errors.New("diagnostic deadline exceeds its single 15-second bound")
	}
	if l.deadline.IsZero() {
		l.deadline = deadline
	} else if !deadline.Equal(l.deadline) {
		l.contextError = errors.New("diagnostic reads renew their deadline")
	}
	output := peer + " read " + strings.Join(arguments, " ") + "\n"
	if l.oversize {
		output += strings.Repeat("X", 128*1024)
	}
	if peer == "kl2tpd-1" && slices.Contains(arguments, "/run/kl2tpd.log") {
		return interoplab.CommandResult{Stdout: output, Stderr: "partial peer-log error\n"}, l.readError
	}
	return interoplab.CommandResult{Stdout: output}, nil
}

// TestScalePeerLogLaunch proves the detached peer owns a real log from exec,
// without backgrounding kl2tpd or changing its per-tunnel launch order.
func TestScalePeerLogLaunch(t *testing.T) {
	lab := &recordingLab{}
	if err := startScalePeer(t.Context(), lab, 2); err != nil {
		t.Fatal(err)
	}
	if len(lab.calls) != 1 {
		t.Fatalf("launch calls=%d, want one", len(lab.calls))
	}
	call := lab.calls[0]
	want := []string{"sh", "-c", `exec kl2tpd -config "$1" >"$2" 2>&1`, "scale-peer", scaleConfigPath, "/run/kl2tpd.log"}
	if call.operation != "exec-detached" || call.peer != "kl2tpd-2" || !slices.Equal(call.arguments, want) {
		t.Fatalf("detached log launch=%+v, want exact peer/config/exec redirection", call)
	}
}

// TestScaleFailureDiagnosticsEvidence proves a canceled assertion still gets
// one bounded read of every peer's daemon, PPP and PID files plus process state.
func TestScaleFailureDiagnosticsEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	primary := errors.New("original subscriber assertion")
	lab := &scaleFailureLab{}
	err := scaleFailureDiagnostics(ctx, lab, primary)
	if !errors.Is(err, primary) {
		t.Fatalf("primary assertion identity lost: %v", err)
	}
	if lab.contextError != nil {
		t.Fatal(lab.contextError)
	}
	if len(lab.calls) != 12 {
		t.Fatalf("diagnostic reads=%d, want four per LAC", len(lab.calls))
	}
	for lac := 1; lac <= 3; lac++ {
		peer := "kl2tpd-" + strconv.Itoa(lac)
		ppp := []string{"tail", "-c", "4096"}
		pids := []string{"head", "-c", "256"}
		for number := 1; number <= 16; number++ {
			user := fmt.Sprintf("scale-%d-%02d", lac, number)
			ppp = append(ppp, "/run/"+user+".log")
			pids = append(pids, "/var/run/pppd/ppp-"+user+".pid")
		}
		for _, arguments := range [][]string{
			{"tail", "-c", "16384", "/run/kl2tpd.log"},
			ppp,
			pids,
			{"sh", "-c", "ps | head -c 16384"},
		} {
			if !slices.ContainsFunc(lab.calls, func(call recordedCall) bool {
				return call.peer == peer && slices.Equal(call.arguments, arguments)
			}) {
				t.Fatalf("missing bounded %s read %v", peer, arguments)
			}
		}
		if !strings.Contains(err.Error(), peer+" read ") {
			t.Fatalf("%s evidence missing from failure", peer)
		}
	}
}

// TestScaleFailureDiagnosticsReadFailure proves partial output and an unavailable
// peer remain evidence, while the original assertion and later peer reads survive.
func TestScaleFailureDiagnosticsReadFailure(t *testing.T) {
	primary := errors.New("original route assertion")
	readError := errors.New("selected diagnostic read failed")
	lab := &scaleFailureLab{readError: readError}
	err := scaleFailureDiagnostics(t.Context(), lab, primary)
	if !errors.Is(err, primary) {
		t.Fatalf("primary assertion lost: %v", err)
	}
	if !errors.Is(err, readError) {
		t.Fatalf("diagnostic error lost: %v", err)
	}
	for _, text := range []string{"partial peer-log error", "kl2tpd-1 read", "kl2tpd-3 read"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("failure lost %q", text)
		}
	}
	if len(lab.calls) != 12 {
		t.Fatal("diagnostic read error prevented later peer evidence")
	}
}

// TestScaleFailureDiagnosticsOutputBound proves an unexpectedly large command
// result cannot make the returned failure unbounded, and truncation is explicit.
func TestScaleFailureDiagnosticsOutputBound(t *testing.T) {
	lab := &scaleFailureLab{oversize: true}
	err := scaleFailureDiagnostics(t.Context(), lab, errors.New("original assertion"))
	if len(err.Error()) > 12*(80*1024+1024) {
		t.Fatalf("diagnostic failure is unbounded: %d bytes", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatal("oversized evidence was dropped without a truncation marker")
	}
}

// TestScaleFailureDiagnosticsBoundary proves the scenario's own early failure
// collects peer evidence before returning to the Suite's cleanup lifecycle.
func TestScaleFailureDiagnosticsBoundary(t *testing.T) {
	lab := &scaleFailureLab{}
	err := checkScaleSessions(t.Context(), lab, 0)
	if err == nil {
		t.Fatal("invalid timeout passed")
	}
	for _, text := range []string{"assertion 1", "scale proof timeout must be positive", "kl2tpd-3 read"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("scenario failure lost %q: %v", text, err)
		}
	}
	if len(lab.calls) != 12 {
		t.Fatal("scenario returned before its peer-file diagnostics")
	}
}
