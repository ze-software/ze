package process

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestDeliverySlowBoundary pins the slow-batch threshold the doctor reports
// on (AC-19, boundary row "delivery latency threshold 500 ms").
//
// VALIDATES: a batch of exactly 500 ms is not slow and one of 501 ms is, both
// through the predicate and through the record noteBatch keeps.
// PREVENTS: a threshold written >= that reports every batch at the limit.
func TestDeliverySlowBoundary(t *testing.T) {
	if DeliverySlow(500 * time.Millisecond) {
		t.Fatal("a 500 ms batch is at the threshold, not over it")
	}
	if !DeliverySlow(501 * time.Millisecond) {
		t.Fatal("a 501 ms batch is over the threshold")
	}

	p := &Process{config: plugin.PluginConfig{Name: "slow"}}
	p.noteBatch(500*time.Millisecond, 3, nil, true)
	if got := p.DoctorRecord(time.Now()).Delivery.Slow; got != 0 {
		t.Fatalf("a 500 ms batch was recorded slow (%d)", got)
	}
	p.noteBatch(501*time.Millisecond, 3, nil, true)
	window := p.DoctorRecord(time.Now()).Delivery
	if window.Slow != 1 {
		t.Fatalf("a 501 ms batch was not recorded slow: %+v", window)
	}
	if window.Slowest != 501*time.Millisecond {
		t.Fatalf("the slowest batch is not the 501 ms one: %+v", window)
	}
}

// TestDeliveryWindowForgetsOldProblems shows the record is a recent window,
// not a lifetime counter (AC-19).
//
// VALIDATES: a failure counts while it is inside DeliveryWindow and stops
// counting once the window has passed it.
// PREVENTS: one failure at boot reported by every doctor run for the life of
// the plugin.
func TestDeliveryWindowForgetsOldProblems(t *testing.T) {
	p := &Process{config: plugin.PluginConfig{Name: "flaky"}}
	p.noteBatch(time.Millisecond, 1, errors.New("broken pipe"), true)

	now := time.Now()
	window := p.DoctorRecord(now).Delivery
	if window.Failures != 1 {
		t.Fatalf("a recent failure is not in the window: %+v", window)
	}
	if window.LastError != "broken pipe" {
		t.Fatalf("the window does not name the failure: %+v", window)
	}
	if later := p.DoctorRecord(now.Add(DeliveryWindow + time.Second)).Delivery; later.Failures != 0 {
		t.Fatalf("a failure older than the window still counts: %+v", later)
	}

	// A failure a caller waits for is the caller's to report: no record.
	waited := &Process{config: plugin.PluginConfig{Name: "waited"}}
	waited.noteBatch(time.Millisecond, 1, errors.New("refused"), false)
	if got := waited.DoctorRecord(time.Now()).Delivery.Failures; got != 0 {
		t.Fatalf("a failure with a waiting caller was recorded as fire-and-forget (%d)", got)
	}
}

// TestDeliverBatchRecordsFireAndForgetFailure drives the real batch path with
// no connection, so the send fails (AC-18, AC-19).
//
// VALIDATES: deliverBatch records the fire-and-forget failure in the window
// and the first failed application as the projection error the doctor reads.
// PREVENTS: a failure only a Warn line ever saw.
func TestDeliverBatchRecordsFireAndForgetFailure(t *testing.T) {
	p := &Process{config: plugin.PluginConfig{Name: "gone"}}
	p.deliverBatch([]EventDelivery{{Output: "event one"}}, nil, time.Second)

	record := p.DoctorRecord(time.Now())
	if record.Delivery.Failures != 1 {
		t.Fatalf("the failed fire-and-forget batch is not in the window: %+v", record.Delivery)
	}
	if record.ProjectionError == "" {
		t.Fatal("the failed batch did not record the projection error")
	}
}

// TestRelayStoppedRaisesEvent drives a stderr pipe that breaks (AC-20).
//
// VALIDATES: the stopped relay raises doctor-plugin-stderr-relay-stopped on
// the report bus, subject the plugin, with a fix.
// PREVENTS: the stop reaching only a log line, which recent-events never
// reads.
func TestRelayStoppedRaisesEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	p := &Process{config: plugin.PluginConfig{Name: "relay-plugin"}}
	p.relayStderrFrom(&failingStderr{prefix: []byte("partial"), err: errors.New("pipe broke")})

	for _, issue := range report.Errors(0) {
		if issue.Code != diagnostic.CodeDoctorPluginStderrRelayStopped {
			continue
		}
		if issue.Subject != "relay-plugin" {
			t.Fatalf("the event names %q, not the plugin", issue.Subject)
		}
		if issue.Fix == "" {
			t.Fatal("the event carries no fix")
		}
		return
	}
	t.Fatalf("no %s event on the bus: %+v", diagnostic.CodeDoctorPluginStderrRelayStopped, report.Errors(0))
}

// TestDoctorRecordsBoundedAndCopied covers the per-process records the
// registration checks read (AC-13, AC-14, AC-17, AC-22).
//
// VALIDATES: each record is kept, bounded at doctorRecordMax with the drop
// counted, and the snapshot is a copy a later write does not change.
// PREVENTS: an unbounded list a misbehaving plugin can grow, and a doctor row
// that changes under the renderer.
func TestDoctorRecordsBoundedAndCopied(t *testing.T) {
	p := &Process{config: plugin.PluginConfig{Name: "noisy"}}
	for range doctorRecordMax + 4 {
		p.RecordCommandRefusal(CommandRefusal{Command: "show thing", Owner: "other", Reason: "owned"})
		p.RecordSubscriptionDrop(SubscriptionDrop{Cause: SubscriptionDropUnknownNamespace, Namespace: "nope"})
	}
	p.SetPostStartup(PostStartupUndelivered, errors.New("connection closed"))
	p.RecordConfigCommit("tx-7", errors.New("deadline exceeded"))

	record := p.DoctorRecord(time.Now())
	if len(record.CommandRefusals) != doctorRecordMax || record.CommandRefusalsDropped != 4 {
		t.Fatalf("command refusals not bounded: %d kept, %d dropped", len(record.CommandRefusals), record.CommandRefusalsDropped)
	}
	if len(record.SubscriptionDrops) != doctorRecordMax || record.SubscriptionDropsDropped != 4 {
		t.Fatalf("subscription drops not bounded: %d kept, %d dropped", len(record.SubscriptionDrops), record.SubscriptionDropsDropped)
	}
	if record.PostStartup != PostStartupUndelivered || record.PostStartupError != "connection closed" {
		t.Fatalf("post-startup outcome not recorded: %v %q", record.PostStartup, record.PostStartupError)
	}
	if record.ConfigCommit.Transaction != "tx-7" || record.ConfigCommit.Error != "deadline exceeded" {
		t.Fatalf("config commit outcome not recorded: %+v", record.ConfigCommit)
	}

	record.CommandRefusals[0].Owner = "changed"
	if p.DoctorRecord(time.Now()).CommandRefusals[0].Owner != "other" {
		t.Fatal("the record handed out shares its slice with the process")
	}

	p.RecordConfigCommit("tx-8", nil)
	if commit := p.DoctorRecord(time.Now()).ConfigCommit; commit.Error != "" || commit.Transaction != "tx-8" {
		t.Fatalf("a later confirmed commit did not replace the failure: %+v", commit)
	}
}
