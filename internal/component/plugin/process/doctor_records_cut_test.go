package process

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBoundDoctorTextCutsAtRuneBoundary proves the cut never splits a rune
// and marks itself: a text within the bound is kept whole, and a longer one
// whose cut point falls inside a two-octet rune is cut before that rune, with
// DoctorCutMark after it.
func TestBoundDoctorTextCutsAtRuneBoundary(t *testing.T) {
	if got := boundDoctorText("show thing", DoctorNameMax); got != "show thing" {
		t.Errorf("a text within the bound = %q, want it whole", got)
	}
	ascii := strings.Repeat("a", 10)
	text := ascii + strings.Repeat("é", 100)
	// The cut point is octet 11, the second octet of the first é.
	got := boundDoctorText(text, 10+1+len(DoctorCutMark))
	if got != ascii+DoctorCutMark {
		t.Errorf("cut inside a rune = %q, want %q", got, ascii+DoctorCutMark)
	}
}

// TestDoctorRecordsCutPluginChosenStrings proves every string a doctor record
// keeps that a plugin chose is bounded at the recorder: an over-long
// namespace, command, refusal reason, subscription detail, post-startup
// error, config-commit error, projection error and delivery error are each
// stored cut, valid UTF-8, within their bound, marked as cut, and a prefix of
// what was sent.
//
// PREVENTS: a plugin keeping about 256 MiB per record kind alive with 16
// requests of 16 MiB each, and a doctor row printing a cut name as the whole.
func TestDoctorRecordsCutPluginChosenStrings(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "huge"})
	namespace := "x" + strings.Repeat("ns-é", 100_000)
	command := strings.Repeat("show é ", 50_000)
	reason := strings.Repeat("refused é ", 50_000)

	proc.RecordSubscriptionDrop(SubscriptionDrop{Cause: SubscriptionDropUnknownNamespace, Namespace: namespace, Detail: reason})
	proc.RecordCommandRefusal(CommandRefusal{Command: command, Reason: reason})
	proc.SetPostStartup(PostStartupUndelivered, errors.New(reason))
	proc.RecordConfigCommit("tx-1", errors.New(reason))
	proc.recordProjectionErr(errors.New(reason))
	proc.noteBatch(time.Millisecond, 1, errors.New(reason), true)

	record := proc.DoctorRecord(time.Now())
	if len(record.SubscriptionDrops) != 1 || len(record.CommandRefusals) != 1 {
		t.Fatalf("records kept: %d drops, %d refusals, want one each", len(record.SubscriptionDrops), len(record.CommandRefusals))
	}
	kept := []struct {
		field, got, sent string
		octetsMax        int
	}{
		{"namespace", record.SubscriptionDrops[0].Namespace, namespace, DoctorNameMax},
		{"subscription detail", record.SubscriptionDrops[0].Detail, reason, doctorTextMax},
		{"command", record.CommandRefusals[0].Command, command, DoctorNameMax},
		{"refusal reason", record.CommandRefusals[0].Reason, reason, doctorTextMax},
		{"post-startup error", record.PostStartupError, reason, doctorTextMax},
		{"config commit error", record.ConfigCommit.Error, reason, doctorTextMax},
		{"projection error", record.ProjectionError, reason, doctorTextMax},
		{"delivery error", record.Delivery.LastError, reason, doctorTextMax},
	}
	for _, k := range kept {
		if len(k.got) > k.octetsMax {
			t.Errorf("%s kept %d octets, over its bound %d", k.field, len(k.got), k.octetsMax)
		}
		if !utf8.ValidString(k.got) {
			t.Errorf("%s kept is not valid UTF-8", k.field)
		}
		if !strings.HasSuffix(k.got, DoctorCutMark) {
			t.Errorf("%s kept %q carries no cut mark", k.field, k.got)
			continue
		}
		if !strings.HasPrefix(k.sent, strings.TrimSuffix(k.got, DoctorCutMark)) {
			t.Errorf("%s kept is not a prefix of what the plugin sent", k.field)
		}
	}
	if record.ConfigCommit.Transaction != "tx-1" {
		t.Errorf("config commit transaction = %q, want tx-1 whole", record.ConfigCommit.Transaction)
	}
}

// TestSlowWaitedBatchKeepsNoError proves the delivery row's last error is
// always one of the failures it counts: a batch that was slow and whose
// error went to a waiting caller counts as slow, not failed, and keeps no
// error.
//
// PREVENTS: "last error: X" beside a failure count that leaves X out.
func TestSlowWaitedBatchKeepsNoError(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "slow"})
	proc.noteBatch(DeliverySlowThreshold+time.Millisecond, 1, errors.New("the waiting caller got this"), false)
	window := proc.DoctorRecord(time.Now()).Delivery
	if window.Slow != 1 || window.Failures != 0 {
		t.Fatalf("window = %+v, want one slow batch and no failure", window)
	}
	if window.LastError != "" {
		t.Errorf("last error %q names a failure the row's count leaves out", window.LastError)
	}
}
