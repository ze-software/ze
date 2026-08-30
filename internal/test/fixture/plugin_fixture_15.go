package fixture

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type plugin15Result struct {
	status string
	raw    json.RawMessage
	err    error
}

func (r plugin15Result) text() string {
	if r.err != nil {
		return r.err.Error()
	}
	if len(r.raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(r.raw, &text) == nil {
		return text
	}
	return string(r.raw)
}

func (r plugin15Result) value(dst any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(r.raw, dst); err == nil {
		return nil
	}
	var nested string
	if err := json.Unmarshal(r.raw, &nested); err != nil {
		return err
	}
	return json.Unmarshal([]byte(nested), dst)
}

func plugin15Dispatch(ctx context.Context, p *sdk.Plugin, command string) plugin15Result {
	status, raw, err := p.DispatchCommand(ctx, command)
	if err != nil && status == "" {
		status = statusError
	}
	return plugin15Result{status: status, raw: raw, err: err}
}

func plugin15PollCommand(ctx context.Context, p *sdk.Plugin, command string, attempts int, delay time.Duration, accept func(plugin15Result) bool) plugin15Result {
	var last plugin15Result
	Poll(ctx, attempts, delay, func() bool {
		last = plugin15Dispatch(ctx, p, command)
		return accept(last)
	})
	return last
}

func plugin15Done(r plugin15Result) bool { return r.err == nil && r.status == statusDone }

func plugin15Map(r plugin15Result) (map[string]any, error) {
	value := map[string]any{}
	if err := r.value(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func plugin15Slice(r plugin15Result) ([]any, error) {
	var value []any
	if err := r.value(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func plugin15Number(value any) float64 {
	n, _ := value.(float64)
	return n
}

func plugin15NestedMap(value any, keys ...string) map[string]any {
	current, _ := value.(map[string]any)
	for _, key := range keys {
		if current == nil {
			return nil
		}
		current, _ = current[key].(map[string]any)
	}
	return current
}

func plugin15Observe(name string, registration sdk.Registration, scenario ObserverScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		return Observe(ctx, name, registration, scenario)
	}
}

func plugin15ObserveStarted(name string, registration sdk.Registration, scenario ObserverScenario) Driver {
	return func(ctx context.Context, _ []string) error {
		p, err := newObserver(name)
		if err != nil {
			return err
		}
		defer p.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion
		result := make(chan error, 1)
		started := make(chan struct{})
		p.OnStarted(func(context.Context) error {
			close(started)
			go func() {
				scenarioErr := scenario(ctx, p)
				result <- scenarioErr
				shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				_, _, _ = p.DispatchCommand(shutdownCtx, "request shutdown")
			}()
			return nil
		})
		runErr := p.Run(ctx, registration)
		return awaitObserverResult(started, result, runErr)
	}
}

func init() {
	Register("plugin/rpki-passthrough", plugin15Observe("rpki-passthrough-test", sdk.Registration{}, plugin15RPKIPassthrough))
	Register("plugin/rpki-per-peer-action", plugin15Observe("rpki-per-peer-test", sdk.Registration{}, plugin15RPKIPerPeer))
	Register("plugin/rpki-pipe-summary", plugin15RPKIPipeSummary)
	Register("plugin/rpki-revalidate-late-sync", plugin15Observe("rpki-late-sync-test", sdk.Registration{}, plugin15RPKILateSync))
	Register("plugin/rpki-timeout", plugin15Observe("rpki-timeout-test", sdk.Registration{}, plugin15RPKITimeout))
	Register("plugin/rpki-validate-accept", plugin15Observe("rpki-accept-test", sdk.Registration{}, plugin15RPKIAccept))
	Register("plugin/rpki-validate-batch-mixed", plugin15Observe("rpki-batch-test", sdk.Registration{}, plugin15RPKIBatch))
	Register("plugin/rpki-validate-notfound", plugin15Observe("rpki-notfound-test", sdk.Registration{}, plugin15RPKINotFound))
	Register("plugin/rpki-validate-reject", plugin15Observe("rpki-reject-test", sdk.Registration{}, plugin15RPKIReject))
	Register("plugin/rr-basic", plugin15Observe("rr-test", sdk.Registration{}, plugin15RRBasic))
	Register("plugin/rr-ipv6-config", plugin15Observe("rr-ipv6-test", sdk.Registration{}, plugin15RRIPv6))
	Register("plugin/rr-status-show", plugin15Observe("rr-status-test", sdk.Registration{}, plugin15RRStatus))
	Register("plugin/runtime-memory-show", plugin15Observe("runtime-memory-show-test", sdk.Registration{}, plugin15RuntimeMemory))
	Register("plugin/send-community-suppress", plugin15Observe("test-sendcomm", sdk.Registration{}, plugin15SendCommunity))
	Register("plugin/set-system-file-descriptors", plugin15Observe("set-system-fd-test", sdk.Registration{}, plugin15SetFD))
	Register("plugin/show-bgp-bare-runs-summary", plugin15Observe("bare-show-bgp", sdk.Registration{}, plugin15ShowBGPBare))
	Register("plugin/show-bgp-child-not-swallowed", plugin15Observe("child-not-swallowed", sdk.Registration{}, plugin15ShowBGPChildren))
	Register("plugin/show-bgp-family-arg", plugin15Observe("show-bgp-family-arg", sdk.Registration{}, plugin15ShowBGPFamily))
	Register("plugin/show-bgp-summary-is-gone", plugin15Observe("summary-is-gone", sdk.Registration{}, plugin15ShowBGPSummaryGone))
	Register("plugin/show-rib-best-walk", plugin15Observe("rib-best-walk-test", sdk.Registration{}, plugin15RIBBestWalk))
	Register("plugin/show-rib-under-load", plugin15ObserveStarted("rib-under-load-walk", sdk.Registration{}, plugin15RIBUnderLoad))
	Register("plugin/shutdown-is-prompt", plugin15Observe("shutdown-is-prompt", sdk.Registration{}, plugin15ShutdownPrompt))
	Register("plugin/signal-stop-ssh", plugin15Observe("signal-stop-test", sdk.Registration{}, plugin15SignalStop))
	Register("plugin/smart-show", plugin15Observe("smart-show-done", sdk.Registration{}, plugin15SmartShow))
	Register("plugin/smart-unconfigured-show", plugin15Observe("smart-show-error", sdk.Registration{}, plugin15SmartUnconfigured))
	Register("plugin/stream-answer-renders-table", plugin15StreamAnswerTable)
	Register("plugin/subscriber-enricher-wiring-show", plugin15Observe("subscriber-enricher-wiring-show-test", sdk.Registration{}, plugin15SubscriberEnricher))
	Register("plugin/subscriber-summary-show", plugin15Observe("subscriber-summary-show-test", sdk.Registration{}, plugin15SubscriberSummary))
	Register("plugin/subsystem-list", plugin15Observe("subsystem-list-test", sdk.Registration{Commands: []sdk.CommandDecl{{Name: "show subsystem-list-test alpha"}, {Name: "show subsystem-list-test beta"}}}, plugin15SubsystemList))
	Register("plugin/summary-format", plugin15SummaryFormat)
	Register("plugin/sysctl-describe-show", plugin15Observe("sysctl-describe-show-test", sdk.Registration{}, plugin15SysctlDescribe))
	Register("plugin/sysctl-list", plugin15Observe("sysctl-list-test", sdk.Registration{}, plugin15SysctlList))
}
