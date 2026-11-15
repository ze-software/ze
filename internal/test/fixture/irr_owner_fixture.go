// Design: docs/architecture/irr-owner.md -- the observers of the irr owner's functional tests
// Related: plugin_fixture_07_irr.go -- the consumer-side IRR observers and their helpers
//
// The irr plugin stores every IRR dataset so a restart enforces the lists it
// enforced before. These observers prove it from outside: `le mock irr
// -query-log irr-queries.log` appends each whois query it receives to a file
// in the test's work directory, and an observer counts the lines.
//
// A restart test runs the SAME daemon block twice, so both runs share one
// store (ci-format.md, "Where the block goes"), with `le test fixture
// daemon/await-exit` between them: a foreground ze does not block the next
// step, and without the barrier the second start is refused the store the
// first still owns. The observer
// tells the runs apart by a marker file it writes at the end of the first:
// the first run waits for the dataset and records the query count and the
// dataset's refreshed-at, the second asserts that both are unchanged.

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	irrQueryLog       = "irr-queries.log"
	irrFirstRunMarker = "irr-first-run.json"
	irrTestASSet      = "AS-TEST"
	// irrTestIPv4 is the IPv4 count `le mock irr` answers for AS-TEST.
	irrTestIPv4 = 3
	// irrQueryV4 is the query a fetch of AS-TEST's IPv4 prefixes sends: one
	// line of it in the log is one fetch of the dataset.
	irrQueryV4 = "!a4" + irrTestASSet
	// irrListedPrefix is a prefix `le mock irr` lists for AS-TEST, the one the
	// BGP restart test's peer announces.
	irrListedPrefix = "10.0.0.0/24"
)

func init() {
	observers := map[string]ObserverScenario{
		"plugin/irr-cold-start-stores":           irrRestartScenario("show bgp irr", true),
		"plugin/irr-restart-keeps-firewall-sets": irrRestartScenario("show firewall irr", false),
		"plugin/irr-shared-dataset":              irrSharedDataset,
		"plugin/irr-update-then-firewall-commit": irrUpdateThenFirewallCommit,
	}
	for name, scenario := range observers {
		n, s := name, scenario
		Register(n, func(ctx context.Context, _ []string) error {
			return Observe(ctx, n, &sdk.Registration{}, s)
		}, StopsDaemon)
	}
	Register("plugin/irr-update-then-firewall-commit/reload", func(ctx context.Context, _ []string) error {
		return reloadFirewallIRR07(ctx, "observer.updated")
	})
}

// irrFirstRun is what the first run of a restart test leaves for the second.
type irrFirstRun struct {
	Queries     int    `json:"queries"`
	RefreshedAt string `json:"refreshed-at"`
}

// irrRestartScenario is the observer of a test that runs one daemon block
// twice. consumerShow is the consumer's own show command, which must name
// AS-TEST in both runs. exactlyOnce makes the first run also assert that the
// cold start fetched the dataset exactly once.
func irrRestartScenario(consumerShow string, exactlyOnce bool) ObserverScenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		first, restart, err := irrReadFirstRun()
		if err != nil {
			return err
		}
		if !restart {
			return irrFirstRunScenario(ctx, p, consumerShow, exactlyOnce)
		}
		return irrSecondRunScenario(ctx, p, consumerShow, first)
	}
}

// irrReadFirstRun reads what the first run of a restart test left for the
// second, and reports whether this run is the restart.
func irrReadFirstRun() (irrFirstRun, bool, error) {
	raw, err := os.ReadFile(irrFirstRunMarker)
	if errors.Is(err, os.ErrNotExist) {
		return irrFirstRun{}, false, nil
	}
	if err != nil {
		return irrFirstRun{}, false, fmt.Errorf("read %s: %w", irrFirstRunMarker, err)
	}
	var first irrFirstRun
	if err := json.Unmarshal(raw, &first); err != nil {
		return irrFirstRun{}, false, fmt.Errorf("decode %s: %w", irrFirstRunMarker, err)
	}
	return first, true, nil
}

// irrBGPEnforcesStored requires `show bgp irr` to report AS-TEST as an
// enforced stored copy: status ok and the IPv4 count the mock answers.
func irrBGPEnforcesStored(ctx context.Context, p *sdk.Plugin) error {
	var entry map[string]any
	result := until07(ctx, p, "show bgp irr", 10, 100*time.Millisecond, func(r commandResult07) bool {
		entry = irrBGPEntry(r)
		return entry != nil && entry["status"] == "ok"
	})
	if result.status != statusDone {
		return fmt.Errorf("show bgp irr: status=%s error=%w", result.status, result.err)
	}
	if entry == nil {
		return fmt.Errorf("show bgp irr names no %s: %s", irrTestASSet, text07(result.data))
	}
	if entry["status"] != "ok" || number07(entry["ipv4-count"]) != irrTestIPv4 {
		return fmt.Errorf("show bgp irr reports %s, want status ok and ipv4-count %d", text07(entry), irrTestIPv4)
	}
	return nil
}

// irrBGPEntry is the `show bgp irr` entry of AS-TEST, nil when there is none.
func irrBGPEntry(result commandResult07) map[string]any {
	if result.status != statusDone {
		return nil
	}
	entries, _ := object07(result.data)["entries"].([]any)
	for _, raw := range entries {
		if entry := object07(raw); entry["as-set"] == irrTestASSet {
			return entry
		}
	}
	return nil
}

// irrListedUpdateAccepted waits for the peer's UPDATE announcing the listed
// prefix to reach the observer. The reactor delivers a received UPDATE only
// after the import chain accepted it (reactor_notify.go), so its arrival is
// the filter's accept.
func irrListedUpdateAccepted(ctx context.Context, updates <-chan string) error {
	wait := WaitBudget(50, 10*time.Second)
	select {
	case update := <-updates:
		if !strings.Contains(update, irrListedPrefix) {
			return fmt.Errorf("the accepted UPDATE does not announce %s: update=%s", irrListedPrefix, update)
		}
		return nil
	case <-time.After(wait):
		return fmt.Errorf("no UPDATE announcing %s reached the observer within %s: the IRR filter rejected it, and holds it", irrListedPrefix, wait)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func irrFirstRunScenario(ctx context.Context, p *sdk.Plugin, consumerShow string, exactlyOnce bool) error {
	row, err := irrAwaitStored(ctx, p, irrTestASSet)
	if err != nil {
		return err
	}
	if err := irrConsumerNames(ctx, p, consumerShow, irrTestASSet); err != nil {
		return err
	}
	queries, err := irrCountQueries("")
	if err != nil {
		return err
	}
	if exactlyOnce {
		fetches, err := irrCountQueries(irrQueryV4)
		if err != nil {
			return err
		}
		if fetches != 1 {
			return fmt.Errorf("cold start fetched %s %d times, want exactly once", irrTestASSet, fetches)
		}
	}
	refreshedAt, _ := row["refreshed-at"].(string)
	if refreshedAt == "" {
		return fmt.Errorf("stored %s has no refreshed-at: %s", irrTestASSet, text07(row))
	}
	encoded, err := json.Marshal(irrFirstRun{Queries: queries, RefreshedAt: refreshedAt})
	if err != nil {
		return err
	}
	if err := os.WriteFile(irrFirstRunMarker, encoded, 0o600); err != nil {
		return err
	}
	_ = textbuf.New().Str("OK: first run stored ").Str(irrTestASSet).Str(" after ").Int(int64(queries)).Str(" queries\n").StdErr()
	return nil
}

func irrSecondRunScenario(ctx context.Context, p *sdk.Plugin, consumerShow string, first irrFirstRun) error {
	row, err := irrStoredRow(ctx, p, irrTestASSet)
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	if row["refreshed-at"] != first.RefreshedAt {
		return fmt.Errorf("restart changed %s: refreshed-at %v, before the restart %s", irrTestASSet, row["refreshed-at"], first.RefreshedAt)
	}
	if err := irrConsumerNames(ctx, p, consumerShow, irrTestASSet); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	// The consumers read at OnStarted, which is before this observer runs;
	// two seconds more is the window in which a start-time fetch would land.
	if err := irrSettle(ctx, 2*time.Second); err != nil {
		return err
	}
	queries, err := irrCountQueries("")
	if err != nil {
		return err
	}
	if queries != first.Queries {
		return fmt.Errorf("restart sent %d IRR queries, want none", queries-first.Queries)
	}
	_ = textbuf.New().Str("OK: restart enforced the stored ").Str(irrTestASSet).Str(" with no IRR query\n").StdErr()
	return nil
}

// irrSharedDataset proves BGP and the firewall naming one AS-SET cost one
// fetch and one stored dataset that both consumers enforce.
func irrSharedDataset(ctx context.Context, p *sdk.Plugin) error {
	row, err := irrAwaitStored(ctx, p, irrTestASSet)
	if err != nil {
		return err
	}
	consumers := text07(row["consumers"])
	for _, consumer := range []string{"bgp-filter-irr", "firewall-irr"} {
		if !strings.Contains(consumers, `"`+consumer+`"`) {
			return fmt.Errorf("stored %s names consumers %s, missing %s", irrTestASSet, consumers, consumer)
		}
	}
	for _, show := range []string{"show bgp irr", "show firewall irr"} {
		if err := irrConsumerNames(ctx, p, show, irrTestASSet); err != nil {
			return err
		}
	}
	if err := irrSettle(ctx, 2*time.Second); err != nil {
		return err
	}
	fetches, err := irrCountQueries(irrQueryV4)
	if err != nil {
		return err
	}
	if fetches != 1 {
		return fmt.Errorf("%s was fetched %d times for two consumers, want once", irrTestASSet, fetches)
	}
	_ = textbuf.New().Str("OK: one fetch of ").Str(irrTestASSet).Str(" serves both consumers\n").StdErr()
	return nil
}

// irrUpdateThenFirewallCommit fetches AS-TEST with the owner's command, then
// lets the reload fixture commit a firewall term naming it, and requires the
// term's table to be programmed.
func irrUpdateThenFirewallCommit(ctx context.Context, p *sdk.Plugin) error {
	result := done07(ctx, p, "update irr as-set "+irrTestASSet)
	if result.status != statusDone {
		return fmt.Errorf("update irr as-set %s: status=%s error=%w", irrTestASSet, result.status, result.err)
	}
	if data := object07(result.data); number07(data["ipv4-count"]) != irrTestIPv4 {
		return fmt.Errorf("update irr as-set %s answered %s, want ipv4-count %d", irrTestASSet, text07(data), irrTestIPv4)
	}
	if err := os.WriteFile("observer.updated", []byte("ok"), 0o600); err != nil {
		return err
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool { _, err := os.Stat("reload.done"); return err == nil }) {
		return errors.New("reload trigger never created reload.done marker")
	}
	wan := until07(ctx, p, "show firewall ruleset wan", 50, 100*time.Millisecond, func(r commandResult07) bool { return r.status == statusDone })
	if wan.status != statusDone {
		return fmt.Errorf("the commit naming %s after update irr did not program wan: status=%s error=%w", irrTestASSet, wan.status, wan.err)
	}
	_ = textbuf.New().Str("OK: update irr then the firewall commit programmed wan\n").StdErr()
	return nil
}

// irrAwaitStored polls `show irr` until asSet is stored with the mock's IPv4
// count, for up to ten seconds.
func irrAwaitStored(ctx context.Context, p *sdk.Plugin, asSet string) (map[string]any, error) {
	var row map[string]any
	var last error
	Poll(ctx, 50, 200*time.Millisecond, func() bool {
		row, last = irrStoredRow(ctx, p, asSet)
		return last == nil && number07(row["ipv4-count"]) == irrTestIPv4
	})
	if last != nil {
		return nil, last
	}
	if number07(row["ipv4-count"]) != irrTestIPv4 {
		return nil, fmt.Errorf("stored %s holds %s, want ipv4-count %d", asSet, text07(row), irrTestIPv4)
	}
	return row, nil
}

// irrStoredRow answers the `show irr` row of asSet, or why there is none.
func irrStoredRow(ctx context.Context, p *sdk.Plugin, asSet string) (map[string]any, error) {
	result := command07(ctx, p, "show irr")
	if result.status != statusDone {
		return nil, fmt.Errorf("show irr: status=%s error=%w", result.status, result.err)
	}
	rows, _ := object07(result.data)["datasets"].([]any)
	for _, entry := range rows {
		row := object07(entry)
		if row["as-set"] == asSet {
			return row, nil
		}
	}
	return nil, fmt.Errorf("show irr names no stored %s: %s", asSet, text07(result.data))
}

// irrConsumerNames requires the consumer's own show command to answer and to
// name asSet, so the consumer enforces the stored list.
func irrConsumerNames(ctx context.Context, p *sdk.Plugin, show, asSet string) error {
	result := until07(ctx, p, show, 10, 100*time.Millisecond, func(r commandResult07) bool {
		return r.status == statusDone && strings.Contains(text07(r.data), `"`+asSet+`"`)
	})
	if result.status != statusDone {
		return fmt.Errorf("%s: status=%s error=%w", show, result.status, result.err)
	}
	if !strings.Contains(text07(result.data), `"`+asSet+`"`) {
		return fmt.Errorf("%s does not name %s: %s", show, asSet, text07(result.data))
	}
	return nil
}

// irrCountQueries counts the lines of the mock's query log that start with
// prefix, or every line for an empty prefix. A missing log counts zero: the
// mock creates it when it starts, so no file means no query was logged.
func irrCountQueries(prefix string) (int, error) {
	raw, err := os.ReadFile(irrQueryLog)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", irrQueryLog, err)
	}
	count := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		if line != "" && strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count, nil
}

// irrSettle waits d, or until ctx ends.
func irrSettle(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
