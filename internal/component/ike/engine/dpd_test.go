package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestDPDSendReceive(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 30,
		Timeout:  90,
		Action:   ipsec.DPDActionRestart,
	})
	if dpd == nil {
		t.Fatal("newDPDState returned nil")
	}

	now := time.Now()
	if dpd.shouldSend(now) {
		t.Error("should not send immediately after creation")
	}

	dpd.lastSent = now.Add(-31 * time.Second)
	if !dpd.shouldSend(now) {
		t.Error("should send after interval elapsed")
	}

	sa := testSA()
	sa.NextMsgID = 1
	sendDPD(sa, nil, dpd, slogutil.DiscardLogger())

	if !dpd.awaitReply {
		t.Error("awaitReply should be true after send")
	}
	if dpd.shouldSend(now) {
		t.Error("should not send while awaiting reply")
	}

	handleDPDResponse(dpd, slogutil.DiscardLogger(), "test-peer")
	if dpd.awaitReply {
		t.Error("awaitReply should be false after response")
	}
}

func TestDPDTimeout(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 10,
		Timeout:  30,
		Action:   ipsec.DPDActionClear,
	})

	now := time.Now()
	dpd.sentAt = now.Add(-31 * time.Second)
	dpd.awaitReply = true

	if !dpd.timedOut(now) {
		t.Error("should be timed out after timeout period")
	}

	if dpd.action != ipsec.DPDActionClear {
		t.Errorf("action = %v, want clear", dpd.action)
	}
}

func TestDPDDisabled(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{Interval: 0})
	if dpd != nil {
		t.Error("DPD with interval 0 should return nil")
	}
}

func TestDPDNotTimedOutBeforeTimeout(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 10,
		Timeout:  60,
		Action:   ipsec.DPDActionRestart,
	})

	now := time.Now()
	dpd.sentAt = now.Add(-30 * time.Second)
	dpd.awaitReply = true

	if dpd.timedOut(now) {
		t.Error("should not be timed out before timeout period")
	}
}

func TestDPDNextDeadline(t *testing.T) {
	dpd := newDPDState(ipsec.DPDConfig{
		Interval: 30,
		Timeout:  90,
		Action:   ipsec.DPDActionRestart,
	})

	now := time.Now()
	dpd.lastSent = now

	deadline := dpd.nextDeadline()
	expected := now.Add(30 * time.Second)
	if deadline.Sub(expected) > time.Millisecond {
		t.Errorf("nextDeadline = %v, want ~%v", deadline, expected)
	}

	dpd.awaitReply = true
	dpd.sentAt = now
	deadline = dpd.nextDeadline()
	expected = now.Add(90 * time.Second)
	if deadline.Sub(expected) > time.Millisecond {
		t.Errorf("nextDeadline (await) = %v, want ~%v", deadline, expected)
	}
}
