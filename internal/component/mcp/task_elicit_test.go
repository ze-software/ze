package mcp

import (
	"context"
	"testing"
	"time"
)

func TestTaskWorker_ElicitFlipsInputRequired(t *testing.T) {
	reg := newTestTaskRegistry(8, time.Minute)
	defer reg.Close()

	sessReg := newSessionRegistry(time.Minute, 0, 10)
	defer sessReg.Close()
	sess, err := sessReg.CreateWithCapabilities(ProtocolVersion, Identity{Name: "alice"}, true, true, false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sink := &mockSink{}
	release, err := sess.SetActivePostSink(sink)
	if err != nil {
		t.Fatalf("set sink: %v", err)
	}
	defer release()

	taskID, taskCtx, _, err := reg.Create("alice", sess.ID(), 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"confirm": map[string]any{"type": "boolean"},
		},
	}

	elicitDone := make(chan error, 1)

	go func() {
		_, elicitErr := TaskElicit(reg, sess, taskID, taskCtx, "confirm?", schema)
		elicitDone <- elicitErr
	}()

	// Wait for the task to reach input_required AND a pending elicitation to appear.
	waitForState(t, reg, taskID, TaskInputRequired)
	elicitID := waitForPendingElicit(t, sess)
	if elicitID == "" {
		t.Fatal("no pending elicitation found")
	}
	sess.ResolveElicit(elicitID, elicitResponse{
		Action:  "accept",
		Content: map[string]any{"confirm": true},
	})

	if elicitErr := <-elicitDone; elicitErr != nil {
		t.Fatalf("TaskElicit: %v", elicitErr)
	}

	// After elicit completes, task should be back to working.
	info, err := reg.Get("alice", taskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if info.State != TaskWorking {
		t.Errorf("state after elicit = %v, want working", info.State)
	}
}

func TestTaskElicit_DeclineFails(t *testing.T) {
	reg := newTestTaskRegistry(8, time.Minute)
	defer reg.Close()

	sessReg := newSessionRegistry(time.Minute, 0, 10)
	defer sessReg.Close()
	sess, err := sessReg.CreateWithCapabilities(ProtocolVersion, Identity{Name: "alice"}, true, true, false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sink := &mockSink{}
	release, err := sess.SetActivePostSink(sink)
	if err != nil {
		t.Fatalf("set sink: %v", err)
	}
	defer release()

	taskID, taskCtx, _, err := reg.Create("alice", sess.ID(), 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}

	elicitDone := make(chan error, 1)
	go func() {
		_, elicitErr := TaskElicit(reg, sess, taskID, taskCtx, "name?", schema)
		elicitDone <- elicitErr
	}()

	waitForState(t, reg, taskID, TaskInputRequired)
	elicitID := waitForPendingElicit(t, sess)

	sess.ResolveElicit(elicitID, elicitResponse{Action: "decline"})

	elicitErr := <-elicitDone
	if elicitErr == nil {
		t.Fatal("expected error on decline")
	}

	// After decline, TaskElicit's defer transitions back to working.
	info, _ := reg.Get("alice", taskID)
	if info.State != TaskWorking {
		t.Errorf("state = %v, want working (defer transitions back)", info.State)
	}
}

// waitForState is defined in tasks_test.go; available because same package.
// mockSink is defined in elicit_test.go or reply_sink_test.go.

func TestTaskElicit_CtxCancelUnblocks(t *testing.T) {
	reg := newTestTaskRegistry(8, time.Minute)
	defer reg.Close()

	sessReg := newSessionRegistry(time.Minute, 0, 10)
	defer sessReg.Close()
	sess, err := sessReg.CreateWithCapabilities(ProtocolVersion, Identity{Name: "alice"}, true, true, false)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sink := &mockSink{}
	release, err := sess.SetActivePostSink(sink)
	if err != nil {
		t.Fatalf("set sink: %v", err)
	}
	defer release()

	taskID, _, _, err := reg.Create("alice", sess.ID(), 0)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": "string"},
		},
	}

	elicitDone := make(chan error, 1)
	go func() {
		_, elicitErr := TaskElicit(reg, sess, taskID, ctx, "question", schema)
		elicitDone <- elicitErr
	}()

	waitForState(t, reg, taskID, TaskInputRequired)
	cancel()

	elicitErr := <-elicitDone
	if elicitErr == nil {
		t.Fatal("expected ctx cancel error")
	}
}

func waitForPendingElicit(t *testing.T, sess *session) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sess.mu.Lock()
		for id := range sess.correlations {
			sess.mu.Unlock()
			return id
		}
		sess.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no pending elicitation appeared within deadline")
	return ""
}
