package transaction

import (
	"strings"
	"testing"
)

// VALIDATES: All topic strings match expected hierarchy.
// PREVENTS: Typos in topic constants causing silent routing failures.
func TestConfigTopicConstants(t *testing.T) {
	// All topics must start with the config prefix.
	allTopics := []string{
		TopicVerifyPrefix,
		TopicApplyPrefix,
		TopicVerifyAbort,
		TopicRollback,
		TopicCommitted,
		TopicApplied,
		TopicAckVerifyOK,
		TopicAckVerifyFailed,
		TopicAckApplyOK,
		TopicAckApplyFailed,
		TopicAckRollbackOK,
	}

	for _, topic := range allTopics {
		if topic == "" {
			t.Errorf("topic constant is empty")
		}
		if !strings.HasPrefix(topic, TopicPrefix) {
			t.Errorf("topic %q does not start with prefix %q", topic, TopicPrefix)
		}
	}

	// Verify exact values for per-plugin topics (engine -> plugin).
	if TopicVerifyPrefix != "config/verify/" {
		t.Errorf("TopicVerifyPrefix = %q, want %q", TopicVerifyPrefix, "config/verify/")
	}
	if TopicApplyPrefix != "config/apply/" {
		t.Errorf("TopicApplyPrefix = %q, want %q", TopicApplyPrefix, "config/apply/")
	}

	// Verify exact values for broadcast topics (engine -> all).
	if TopicVerifyAbort != "config/verify/abort" {
		t.Errorf("TopicVerifyAbort = %q, want %q", TopicVerifyAbort, "config/verify/abort")
	}
	if TopicRollback != "config/rollback" {
		t.Errorf("TopicRollback = %q, want %q", TopicRollback, "config/rollback")
	}
	if TopicCommitted != "config/committed" {
		t.Errorf("TopicCommitted = %q, want %q", TopicCommitted, "config/committed")
	}
	if TopicApplied != "config/applied" {
		t.Errorf("TopicApplied = %q, want %q", TopicApplied, "config/applied")
	}

	// Verify exact values for ack topics (plugin -> engine).
	if TopicAckVerifyOK != "config/ack/verify/ok" {
		t.Errorf("TopicAckVerifyOK = %q, want %q", TopicAckVerifyOK, "config/ack/verify/ok")
	}
	if TopicAckVerifyFailed != "config/ack/verify/failed" {
		t.Errorf("TopicAckVerifyFailed = %q, want %q", TopicAckVerifyFailed, "config/ack/verify/failed")
	}
	if TopicAckApplyOK != "config/ack/apply/ok" {
		t.Errorf("TopicAckApplyOK = %q, want %q", TopicAckApplyOK, "config/ack/apply/ok")
	}
	if TopicAckApplyFailed != "config/ack/apply/failed" {
		t.Errorf("TopicAckApplyFailed = %q, want %q", TopicAckApplyFailed, "config/ack/apply/failed")
	}
	if TopicAckRollbackOK != "config/ack/rollback/ok" {
		t.Errorf("TopicAckRollbackOK = %q, want %q", TopicAckRollbackOK, "config/ack/rollback/ok")
	}

	// Verify helper functions.
	if got := TopicVerifyFor("bgp"); got != "config/verify/bgp" {
		t.Errorf("TopicVerifyFor(bgp) = %q, want %q", got, "config/verify/bgp")
	}
	if got := TopicApplyFor("interface"); got != "config/apply/interface" {
		t.Errorf("TopicApplyFor(interface) = %q, want %q", got, "config/apply/interface")
	}

	// Verify failure codes.
	codes := []string{CodeOK, CodeTimeout, CodeTransient, CodeError, CodeBroken}
	for _, code := range codes {
		if code == "" {
			t.Errorf("failure code constant is empty")
		}
	}
	if CodeOK != "ok" {
		t.Errorf("CodeOK = %q, want %q", CodeOK, "ok")
	}
	if CodeBroken != "broken" {
		t.Errorf("CodeBroken = %q, want %q", CodeBroken, "broken")
	}
}
