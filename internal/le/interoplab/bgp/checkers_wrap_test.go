package bgp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestSpecialCheckerWrapsFailureOnce proves a special checker's failure is
// reported once. Method: a checker that already returns a checkerFailure for
// its assertion 4 runs through specialChecker; the message must name the
// scenario and assertion once, carry no outer "assertion 0", and the peer
// logs must be read as often as one checkerFailure reads them. A checker that
// returns a bare error still gets the logs, under assertion 0.
func TestSpecialCheckerWrapsFailureOnce(t *testing.T) {
	const name = "dns-resolver-dhcp-lan"
	cause := errors.New("status NOERROR, want REFUSED")

	single := &recordingLab{logs: "peer clue"}
	_ = checkerFailure(t.Context(), single, name, 4, cause) //nolint:errcheck // counts the log reads of one report

	reported := func(ctx context.Context, check *interoplab.CheckContext) error {
		return checkerFailure(ctx, check.Lab, name, 4, cause)
	}
	lab := &recordingLab{logs: "peer clue"}
	err := specialChecker(name, reported)(t.Context(), &interoplab.CheckContext{Lab: lab})
	if !errors.Is(err, cause) {
		t.Fatalf("wrapped failure %v lost its cause", err)
	}
	if got := strings.Count(err.Error(), "scenario "+name+" assertion"); got != 1 {
		t.Fatalf("scenario named %d times, want once:\n%v", got, err)
	}
	if strings.Contains(err.Error(), "assertion 0") {
		t.Fatalf("failure double-wrapped under assertion 0:\n%v", err)
	}
	if lab.reads != single.reads {
		t.Fatalf("peer logs read %d times, want %d (one report)", lab.reads, single.reads)
	}

	bare := func(context.Context, *interoplab.CheckContext) error { return cause }
	err = specialChecker(name, bare)(t.Context(), &interoplab.CheckContext{Lab: &recordingLab{logs: "peer clue"}})
	if !errors.Is(err, cause) {
		t.Fatalf("bare failure %v lost its cause", err)
	}
	if !strings.HasPrefix(err.Error(), "scenario "+name+" assertion 0: ") || !strings.Contains(err.Error(), "peer clue") {
		t.Fatalf("bare failure %v, want assertion 0 with the peer logs", err)
	}
}
