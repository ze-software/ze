// VALIDATES: the log command RPC registrations and the arg-parsing error branches
// of handleLogRecent (missing values, unknown option, non-positive count) and
// handleLogSet (usage on too few args), plus the happy path of handleLogRecent.
// PREVENTS: a malformed log-recent/log-set invocation being accepted, or the RPC
// set losing a wire method.

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func TestRPCsRegistered(t *testing.T) {
	rpcs := RPCs()
	want := map[string]bool{"ze-log:bgp-log-levels": false, "ze-log:bgp-log-set": false, "ze-log:bgp-log-recent": false}
	for _, r := range rpcs {
		if _, ok := want[r.WireMethod]; ok {
			want[r.WireMethod] = true
		}
	}
	for method, seen := range want {
		if !seen {
			t.Errorf("RPCs() missing wire method %q", method)
		}
	}
}

func TestHandleLogRecentErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"level missing value", []string{"level"}},
		{"component missing value", []string{"component"}},
		{"count missing value", []string{"count"}},
		{"count non-numeric", []string{"count", "abc"}},
		{"count zero", []string{"count", "0"}},
		{"unknown option", []string{"bogus"}},
	} {
		resp, err := handleLogRecent(nil, commandtest.Args(tc.args...))
		if err != nil {
			t.Errorf("%s: unexpected transport error %v", tc.name, err)
			continue
		}
		if resp.Status != plugin.StatusError {
			t.Errorf("%s: status = %v, want StatusError", tc.name, resp.Status)
		}
	}
}

func TestHandleLogRecentHappyPath(t *testing.T) {
	resp, err := handleLogRecent(nil, commandtest.Args("count", "5"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Errorf("status = %v, want StatusDone", resp.Status)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("resp.Data is %T, want plugin.Map", resp.Data)
	}
	if _, ok := data["entries"]; !ok {
		t.Error("response missing entries key")
	}
}

func TestHandleLogSetUsage(t *testing.T) {
	resp, err := handleLogSet(nil, commandtest.Args("onlyone"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Errorf("status = %v, want StatusError for too few args", resp.Status)
	}
}

// registeredHandler answers the handler RPCs registers for a wire method, so a
// test drives the command through its registration rather than the function.
func registeredHandler(t *testing.T, wireMethod string) pluginserver.Handler {
	t.Helper()
	for _, r := range RPCs() {
		if r.WireMethod == wireMethod {
			return r.Handler
		}
	}
	t.Fatalf("RPCs() registers no handler for %q", wireMethod)
	return nil
}

// TestLogSetDisabledSilencesSubsystem drives `request log level <subsystem>
// disabled` through the registered log-set handler and checks that the
// subsystem then writes nothing, that `show log levels` reports it as
// disabled, and that a later level enables it again.
//
// VALIDATES: the enumeration value `disabled` the command offers is accepted
// and silences the subsystem.
// PREVENTS: the schema offering a level word the handler refuses.
func TestLogSetDisabledSilencesSubsystem(t *testing.T) {
	const subsystem = "test.log.set.disabled"
	logger := slogutil.Logger(subsystem)
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("precondition: a fresh logger starts at warn")
	}

	set := registeredHandler(t, "ze-log:bgp-log-set")
	resp, err := set(nil, commandtest.Args(subsystem, "disabled"))
	if err != nil {
		t.Fatalf("log-set transport error: %v", err)
	}
	if resp.Status != plugin.StatusError && logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("request log level disabled answered done and left an ERROR record enabled")
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("request log level %s disabled: status %v, error %q, want StatusDone", subsystem, resp.Status, resp.Error)
	}

	levels := registeredHandler(t, "ze-log:bgp-log-levels")
	resp, err = levels(nil, commandtest.Args())
	if err != nil {
		t.Fatalf("log-levels transport error: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("log-levels Data is %T, want plugin.Map", resp.Data)
	}
	reported, ok := data["levels"].(map[string]string)
	if !ok {
		t.Fatalf("log-levels levels is %T, want map[string]string", data["levels"])
	}
	if reported[subsystem] != "disabled" {
		t.Fatalf("show log levels reports %q for %s, want disabled", reported[subsystem], subsystem)
	}

	resp, err = set(nil, commandtest.Args(subsystem, "info"))
	if err != nil {
		t.Fatalf("log-set transport error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("request log level %s info after disabled: status %v, error %q", subsystem, resp.Status, resp.Error)
	}
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("request log level info after disabled left the logger silent")
	}
}

// recentMessages answers the messages `show log recent` returns for args,
// through the registered log-recent handler.
func recentMessages(t *testing.T, args []string) []string {
	t.Helper()
	recent := registeredHandler(t, "ze-log:bgp-log-recent")
	resp, err := recent(nil, commandtest.Args(args...))
	if err != nil {
		t.Fatalf("log-recent %v transport error: %v", args, err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("log-recent %v: status %v, error %q, want StatusDone", args, resp.Status, resp.Error)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("log-recent Data is %T, want plugin.Map", resp.Data)
	}
	entries, ok := data["entries"].([]map[string]any)
	if !ok {
		t.Fatalf("log-recent entries is %T, want []map[string]any", data["entries"])
	}
	messages := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry["component"] != "test.log.recent.level" {
			continue
		}
		message, ok := entry["message"].(string)
		if !ok {
			t.Fatalf("log-recent message is %T, want string", entry["message"])
		}
		level, ok := entry["level"].(string)
		if !ok {
			t.Fatalf("log-recent level is %T, want string", entry["level"])
		}
		messages = append(messages, message+" at "+level)
	}
	return messages
}

// TestLogRecentLevelFilterMatchesTypedWord writes one record at info and one
// at error, then drives `show log recent level <word>` through the registered
// log-recent handler with the words the enumeration offers. Each filter
// answers the entry written at that level and no other, the level field of
// the answer carries the spelling `show log levels` reports, and a word that
// names no level is refused.
//
// VALIDATES: the filter word an operator types selects the entries the ring
// stored, whatever case slog spells the level in.
// PREVENTS: every level filter answering no entry because the ring stored
// slog's upper-case name and the enumeration offers the lower-case one.
func TestLogRecentLevelFilterMatchesTypedWord(t *testing.T) {
	const subsystem = "test.log.recent.level"
	logger := slogutil.Logger(subsystem)
	if err := slogutil.SetLevel(subsystem, "info"); err != nil {
		t.Fatalf("SetLevel(info): %v", err)
	}
	logger.Info("recent filter probe")
	logger.Error("recent filter failure")

	got := recentMessages(t, []string{"level", "info"})
	if len(got) != 1 || got[0] != "recent filter probe at info" {
		t.Fatalf("show log recent level info answered %q, want the one info entry", got)
	}

	got = recentMessages(t, []string{"level", "err"})
	if len(got) != 1 || got[0] != "recent filter failure at error" {
		t.Fatalf("show log recent level err answered %q, want the one error entry", got)
	}

	got = recentMessages(t, []string{"level", "warn", "component", subsystem})
	if len(got) != 0 {
		t.Fatalf("show log recent level warn answered %q, want no entry", got)
	}

	recent := registeredHandler(t, "ze-log:bgp-log-recent")
	resp, err := recent(nil, commandtest.Args("level", "verbose"))
	if err != nil {
		t.Fatalf("log-recent transport error: %v", err)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("show log recent level verbose: status %v, want StatusError for a word that names no level", resp.Status)
	}
}

// VALIDATES: a structured diagnostic reaches the registered query and both
// output formats in one entry, without changing its message or level.
// PREVENTS: a useful error/peer attribute being visible only in daemon stderr.
func TestLogRecentStructuredDiagnostic(t *testing.T) {
	const message = "peer authenticated and refused the Child SA, keeping the IKE SA"
	logger := slogutil.Logger(t.Name()).With("peer", "responder")
	record := slog.NewRecord(time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC), slog.LevelWarn, message, 0)
	record.AddAttrs(slog.String("notify", "NO_PROPOSAL_CHOSEN"), slog.Any("error", errors.New("no shared ESP proposal")))
	if err := logger.Handler().Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	recent := registeredHandler(t, "ze-log:bgp-log-recent")
	response, err := recent(nil, commandtest.Args("component", t.Name(), "level", "warn", "count", "1"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != plugin.StatusDone {
		t.Fatalf("log query: %s", response.Error)
	}
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Entries []struct {
			Timestamp  string            `json:"timestamp"`
			Level      string            `json:"level"`
			Component  string            `json:"component"`
			Message    string            `json:"message"`
			Attributes map[string]string `json:"attributes"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("entries: %s", encoded)
	}
	entry := result.Entries[0]
	if entry.Timestamp != "2026-10-26T10:00:00Z" || entry.Level != "warn" ||
		entry.Component != t.Name() || entry.Message != message {
		t.Fatalf("metadata changed: %+v", entry)
	}
	for key, want := range map[string]string{
		"peer": "responder", "notify": "NO_PROPOSAL_CHOSEN", "error": "no shared ESP proposal",
	} {
		if entry.Attributes[key] != want {
			t.Errorf("attributes[%q] = %q, want %q", key, entry.Attributes[key], want)
		}
	}
	_, pipes := command.ParsePipe("show log recent | yaml")
	visible, diagnostic := command.ApplyPipes(string(encoded), pipes, nil, nil)
	if diagnostic != "" {
		t.Fatal(diagnostic)
	}
	for _, want := range []string{"notify: NO_PROPOSAL_CHOSEN", "peer: responder", "error: no shared ESP proposal", message} {
		if !strings.Contains(visible, want) {
			t.Errorf("visible query lacks %q: %s", want, visible)
		}
	}
}

type redactedLogSecret string

func (redactedLogSecret) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}

type redactedStringLogSecret struct {
	Secret string
}

func (redactedStringLogSecret) String() string {
	return "[REDACTED]"
}

// VALIDATES: the query preserves slog group scope and snapshots mutable
// persistent/record values, while retaining LogValuer redaction.
// PREVENTS: sibling logger contamination, leaked raw secrets, or a later caller
// mutation changing a previously recorded diagnostic.
func TestLogRecentOwnsGroupedAttributes(t *testing.T) {
	persistent := map[string]string{"router": "edge-original"}
	base := slogutil.Logger(t.Name()).With("inventory", persistent)
	persistent["router"] = "changed-after-With"
	group := []slog.Attr{slog.String("peer", "responder")}
	left := slog.New(base.Handler().WithGroup("session").WithAttrs(group))
	group[0] = slog.String("peer", "changed-attribute-slice")
	right := base.WithGroup("other").With("peer", "alternate")
	details := map[string]any{"ciphers": []string{"aes256"}, "counter": uint64(9007199254740993)}
	left.Warn("grouped",
		slog.Attr{},
		slog.Group("", slog.String("notify", "NO_PROPOSAL_CHOSEN")),
		slog.Group("empty"),
		slog.Any("details", details),
		slog.Uint64("counter", 9007199254740993),
		slog.Any("stringer", redactedStringLogSecret{Secret: "must-not-be-exposed"}),
		slog.Any("secret", redactedLogSecret("must-not-be-exposed")))
	details["ciphers"].([]string)[0] = "changed-after-Log"
	details["counter"] = uint64(0)
	base.WithGroup("").Warn("ungrouped", "peer", "root")
	right.Warn("sibling", "notify", "OTHER_REASON")

	recent := registeredHandler(t, "ze-log:bgp-log-recent")
	// Repeat after mutating the first query's decoded data: it must not own
	// any part of the retained ring entry.
	for range 2 {
		response, err := recent(nil, commandtest.Args("component", t.Name(), "count", "3"))
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != plugin.StatusDone {
			t.Fatalf("log query: %s", response.Error)
		}
		payload := response.Data.(plugin.Map)
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "must-not-be-exposed") || strings.Contains(string(encoded), "changed-") {
			t.Fatalf("query exposed raw or subsequently mutated data: %s", encoded)
		}
		var result struct {
			Entries []struct {
				Message    string         `json:"message"`
				Attributes map[string]any `json:"attributes"`
			} `json:"entries"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(encoded)))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if len(result.Entries) != 3 {
			t.Fatalf("entries: %s", encoded)
		}
		sibling, root, grouped := result.Entries[0], result.Entries[1], result.Entries[2]
		if sibling.Message != "sibling" || root.Message != "ungrouped" || grouped.Message != "grouped" {
			t.Fatalf("query lost newest-first ordering: %s", encoded)
		}
		if sibling.Attributes["session"] != nil || root.Attributes["session"] != nil || root.Attributes["other"] != nil {
			t.Fatalf("group escaped its logger: %s", encoded)
		}
		other := sibling.Attributes["other"].(map[string]any)
		if other["peer"] != "alternate" || root.Attributes["peer"] != "root" {
			t.Fatalf("sibling fields changed: %s", encoded)
		}
		attrs := grouped.Attributes["session"].(map[string]any)
		if attrs["peer"] != "responder" || attrs["notify"] != "NO_PROPOSAL_CHOSEN" ||
			attrs["secret"] != "[REDACTED]" || attrs["stringer"] != "[REDACTED]" ||
			attrs["empty"] != nil || attrs[""] != nil {
			t.Fatalf("grouped fields changed: %s", encoded)
		}
		if grouped.Attributes["inventory"] != "map[router:edge-original]" ||
			attrs["details"] != "map[ciphers:[aes256] counter:9007199254740993]" ||
			attrs["counter"] != json.Number("9007199254740993") {
			t.Fatalf("captured values changed: %s", encoded)
		}
		entries := payload["entries"].([]map[string]any)
		entries[2]["attributes"].(map[string]any)["session"].(map[string]any)["peer"] = "changed-query-result"
	}
}
