package transaction

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExecutePluginStepsCommit(t *testing.T) {
	gw := newTestGateway()
	participants := []Participant{
		{Name: "zebra", ConfigRoots: []string{"zebra"}},
		{Name: "alpha", ConfigRoots: []string{"alpha"}},
	}
	coord, err := NewTxCoordinator(gw, participants, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	for _, p := range participants {
		name := p.Name
		gw.SubscribeConfigEvent(EventVerifyFor(name), func(payload []byte) {
			var ev VerifyEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				return
			}
			ack, _ := json.Marshal(VerifyAck{TransactionID: ev.TransactionID, Plugin: name, Status: CodeOK})
			gw.mustEmit(EventVerifyOK, ack)
		})
		gw.SubscribeConfigEvent(EventApplyFor(name), func(payload []byte) {
			var ev ApplyEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				return
			}
			ack, _ := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: name, Status: CodeOK})
			gw.mustEmit(EventApplyOK, ack)
		})
	}

	diffs := map[string][]DiffSection{
		"zebra": {{Root: "zebra", Changed: `{"a":1}`}},
		"alpha": {{Root: "alpha", Changed: `{"b":2}`}},
	}
	res := coord.Execute(context.Background(), diffs)
	if res.State != StateCommitted {
		t.Fatalf("state = %q, want %q (err=%v)", res.State, StateCommitted, res.Err)
	}
	if len(res.PluginSteps) != 2 {
		t.Fatalf("PluginSteps len = %d, want 2", len(res.PluginSteps))
	}
	if res.PluginSteps[0].Plugin != "alpha" || res.PluginSteps[1].Plugin != "zebra" {
		t.Fatalf("PluginSteps order = [%s, %s], want [alpha, zebra]", res.PluginSteps[0].Plugin, res.PluginSteps[1].Plugin)
	}
	for _, step := range res.PluginSteps {
		if step.Status != "committed" {
			t.Errorf("plugin %s status = %q, want committed", step.Plugin, step.Status)
		}
		if step.VerifyMS < 0 || step.ApplyMS < 0 || step.RollbackMS != 0 {
			t.Errorf("plugin %s unexpected timings: verify=%d apply=%d rollback=%d", step.Plugin, step.VerifyMS, step.ApplyMS, step.RollbackMS)
		}
		if step.Error != "" {
			t.Errorf("plugin %s error = %q, want empty", step.Plugin, step.Error)
		}
	}
}

func TestExecutePluginStepsVerifyAbort(t *testing.T) {
	gw := newTestGateway()
	participants := []Participant{
		{Name: "alpha", ConfigRoots: []string{"alpha"}},
		{Name: "beta", ConfigRoots: []string{"beta"}},
	}
	coord, err := NewTxCoordinator(gw, participants, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	longErr := strings.Repeat("x", 700)
	gw.SubscribeConfigEvent(EventVerifyFor("alpha"), func(payload []byte) {
		var ev VerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(VerifyAck{TransactionID: ev.TransactionID, Plugin: "alpha", Status: CodeOK})
		gw.mustEmit(EventVerifyOK, ack)
	})
	gw.SubscribeConfigEvent(EventVerifyFor("beta"), func(payload []byte) {
		var ev VerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(VerifyAck{TransactionID: ev.TransactionID, Plugin: "beta", Status: CodeError, Error: longErr})
		gw.mustEmit(EventVerifyFailed, ack)
	})

	diffs := map[string][]DiffSection{
		"alpha": {{Root: "alpha", Changed: `{"a":1}`}},
		"beta":  {{Root: "beta", Changed: `{"b":2}`}},
	}
	res := coord.Execute(context.Background(), diffs)
	if res.State != StateAborted {
		t.Fatalf("state = %q, want %q", res.State, StateAborted)
	}
	if len(res.PluginSteps) != 2 {
		t.Fatalf("PluginSteps len = %d, want 2", len(res.PluginSteps))
	}
	if res.PluginSteps[0].Plugin != "alpha" || res.PluginSteps[0].Status != "aborted" {
		t.Errorf("alpha step = %+v, want status aborted", res.PluginSteps[0])
	}
	betaStep := res.PluginSteps[1]
	if betaStep.Plugin != "beta" || betaStep.Status != "verify-failed" {
		t.Errorf("beta step = %+v, want status verify-failed", betaStep)
	}
	if len(betaStep.Error) > maxStepErrorBytes {
		t.Errorf("beta error len = %d, want <= %d", len(betaStep.Error), maxStepErrorBytes)
	}
	if !strings.HasSuffix(betaStep.Error, "...") {
		t.Errorf("truncated error should end with ellipsis, got %q", betaStep.Error)
	}
}

func TestExecutePluginStepsApplyRollback(t *testing.T) {
	gw := newTestGateway()
	participants := []Participant{
		{Name: "alpha", ConfigRoots: []string{"alpha"}},
		{Name: "beta", ConfigRoots: []string{"beta"}},
	}
	coord, err := NewTxCoordinator(gw, participants, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}
	coord.SetVerifyDeadline(time.Second)
	coord.SetApplyDeadlineOverride(time.Second)

	for _, name := range []string{"alpha", "beta"} {
		pluginName := name
		gw.SubscribeConfigEvent(EventVerifyFor(pluginName), func(payload []byte) {
			var ev VerifyEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				return
			}
			ack, _ := json.Marshal(VerifyAck{TransactionID: ev.TransactionID, Plugin: pluginName, Status: CodeOK})
			gw.mustEmit(EventVerifyOK, ack)
		})
	}
	gw.SubscribeConfigEvent(EventApplyFor("alpha"), func(payload []byte) {
		var ev ApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: "alpha", Status: CodeOK})
		gw.mustEmit(EventApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventApplyFor("beta"), func(payload []byte) {
		var ev ApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: "beta", Status: CodeError, Error: "apply boom"})
		gw.mustEmit(EventApplyFailed, ack)
	})
	gw.SubscribeConfigEvent(EventRollback, func(payload []byte) {
		var ev RollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		for _, pluginName := range []string{"alpha", "beta"} {
			ack, _ := json.Marshal(RollbackAck{TransactionID: ev.TransactionID, Plugin: pluginName, Code: CodeOK})
			gw.mustEmit(EventRollbackOK, ack)
		}
	})

	diffs := map[string][]DiffSection{
		"alpha": {{Root: "alpha", Changed: `{"a":1}`}},
		"beta":  {{Root: "beta", Changed: `{"b":2}`}},
	}
	res := coord.Execute(context.Background(), diffs)
	if res.State != StateRolledBack {
		t.Fatalf("state = %q, want %q", res.State, StateRolledBack)
	}
	if len(res.PluginSteps) != 2 {
		t.Fatalf("PluginSteps len = %d, want 2", len(res.PluginSteps))
	}
	if res.PluginSteps[0].Plugin != "alpha" || res.PluginSteps[0].Status != "rolled-back" {
		t.Errorf("alpha step = %+v, want status rolled-back", res.PluginSteps[0])
	}
	if res.PluginSteps[1].Plugin != "beta" || res.PluginSteps[1].Status != "apply-failed" || res.PluginSteps[1].Error != "apply boom" {
		t.Errorf("beta step = %+v, want status apply-failed error 'apply boom'", res.PluginSteps[1])
	}
}

func TestTxCoordinatorPluginStepsOperationPathApplyFailure(t *testing.T) {
	gw := newTestGateway()
	participants := []Participant{
		{Name: "alpha", ConfigRoots: []string{"alpha"}},
		{Name: "beta", ConfigRoots: []string{"beta"}},
	}
	coord, err := NewTxCoordinator(gw, participants, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}
	coord.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{
			{ID: "op-alpha-1", Owner: "alpha", Root: "alpha", Type: "alpha-add", Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Address: "10.0.0.1"}},
			{ID: "op-beta-1", Owner: "beta", Root: "beta", Type: "beta-add", Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Address: "10.0.0.2"}},
		}, nil
	})

	for _, p := range participants {
		pluginName := p.Name
		gw.SubscribeConfigEvent(EventVerifyFor(pluginName), func(payload []byte) {
			var ev VerifyEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				return
			}
			ack, _ := json.Marshal(VerifyAck{TransactionID: ev.TransactionID, Plugin: pluginName, Status: CodeOK})
			gw.mustEmit(EventVerifyOK, ack)
		})
	}
	gw.SubscribeConfigEvent(EventOperationVerifyFor("alpha"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, OperationID: ev.Operation.ID, Plugin: "alpha", Status: CodeOK})
		gw.mustEmit(EventOperationVerifyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationVerifyFor("beta"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, OperationID: ev.Operation.ID, Plugin: "beta", Status: CodeOK})
		gw.mustEmit(EventOperationVerifyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("alpha"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, OperationID: ev.Operation.ID, Plugin: "alpha", Status: CodeOK})
		gw.mustEmit(EventOperationApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("beta"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, OperationID: ev.Operation.ID, Plugin: "beta", Status: CodeError, Error: "beta op apply boom"})
		gw.mustEmit(EventOperationApplyFailed, ack)
	})
	gw.SubscribeConfigEvent(EventOperationRollbackFor("alpha"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		for _, op := range ev.Operations {
			ack, _ := json.Marshal(ConfigOperationRollbackAck{TransactionID: ev.TransactionID, OperationID: op.ID, Plugin: "alpha", Status: CodeOK})
			gw.mustEmit(EventOperationRollbackOK, ack)
		}
	})

	diffs := map[string][]DiffSection{
		"alpha": {{Root: "alpha", Changed: `{"a":1}`}},
		"beta":  {{Root: "beta", Changed: `{"b":2}`}},
	}
	res := coord.Execute(context.Background(), diffs)
	if res.State != StateRolledBack {
		t.Fatalf("state = %q, want %q", res.State, StateRolledBack)
	}
	if len(res.PluginSteps) != 2 {
		t.Fatalf("PluginSteps len = %d, want 2", len(res.PluginSteps))
	}
	if res.PluginSteps[0].Plugin != "alpha" || res.PluginSteps[0].Status != "rolled-back" {
		t.Errorf("alpha step = %+v, want status rolled-back", res.PluginSteps[0])
	}
	if res.PluginSteps[1].Plugin != "beta" || res.PluginSteps[1].Status != "apply-failed" || !strings.Contains(res.PluginSteps[1].Error, "beta op apply boom") {
		t.Errorf("beta step = %+v, want status apply-failed containing 'beta op apply boom'", res.PluginSteps[1])
	}
}
