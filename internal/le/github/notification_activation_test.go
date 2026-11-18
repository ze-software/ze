// Design: docs/contributing/github-integration.md -- explicit private approval activates one registered delivery attempt.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

type notificationActivationFixture struct {
	config, credential, state string
	data                      map[string]any
}

func notificationActivationTestFixture(t *testing.T) notificationActivationFixture {
	t.Helper()
	private := t.TempDir()
	if err := os.Chmod(private, 0o700); err != nil {
		t.Fatal(err)
	}
	fixture := notificationActivationFixture{config: filepath.Join(private, "config.json"),
		credential: filepath.Join(private, "webhook"), state: filepath.Join(private, "receipts")}
	if err := os.Mkdir(fixture.state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.credential, []byte("https://discord.com/api/webhooks/456/fixture-private-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	preview, _ := notificationTestPreview(t, notificationTestReport(101, true))
	template, err := candidateNotificationTemplate()
	if err != nil {
		t.Fatal(err)
	}
	fixture.data = map[string]any{"version": 1, "destination": "maintainers", "channel-id": "123", "webhook-id": "456",
		"failure-template": template.failure, "recovery-template": template.recovery,
		"approval-digest": preview.ApprovalDigest, "credential-file": fixture.credential, "state-directory": fixture.state}
	fixture.write(t)
	return fixture
}

func (fixture notificationActivationFixture) write(t *testing.T) {
	t.Helper()
	body, err := json.Marshal(fixture.data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.config, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func notificationActivationTestArea(read notificationRunReader, transport http.RoundTripper) leaction.Area {
	return leaction.New(area, notificationActivationActions(read, transport)...)
}

func notificationActivationTestRead(_ context.Context, id uint64) (Report, error) {
	return notificationTestReport(id, true), nil
}

func notificationActivationTestReady(t *testing.T, fixture notificationActivationFixture) {
	t.Helper()
	preview, approval := notificationTestPreview(t, notificationTestReport(101, true))
	receipt, err := newNotificationReceipt(preview, approval)
	if err != nil {
		t.Fatal(err)
	}
	store, err := newFileNotificationStore(fixture.state)
	if err != nil {
		t.Fatal(err)
	}
	initializeErr := store.initialize(t.Context(), receipt)
	closeErr := store.Close()
	if initializeErr != nil {
		t.Fatal(initializeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}

// TestNotificationActivationRegisteredDeliveryPersistsAcrossReopen drives the
// registered parser, private files, real locked ledger and bounded sender with
// a recording transport. A second invocation must observe the persisted send.
func TestNotificationActivationRegisteredDeliveryPersistsAcrossReopen(t *testing.T) {
	fixture := notificationActivationTestFixture(t)
	posts := 0
	requests := 0
	transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Host != "discord.com" {
			t.Fatal("activation escaped the fixed destination")
		}
		if request.Method == http.MethodGet {
			return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"123"}`), nil
		}
		if request.Method != http.MethodPost {
			t.Fatal("activation attempted an unexpected HTTP method")
		}
		posts++
		body, err := os.ReadFile(filepath.Join(fixture.state, notificationStateFile))
		if err != nil {
			t.Fatal(err)
		}
		var receipt notificationReceipt
		if err := json.Unmarshal(body, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Mode != notificationReceiptInFlight {
			t.Error("registered POST preceded durable intent")
		}
		return notificationHTTPResponse(t, 200, `{"id":"900","channel_id":"123"}`), nil
	})
	action := notificationActivationTestArea(notificationActivationTestRead, transport)
	if _, code := action.Answer([]string{"receipt", "run", "101", "config", fixture.config}); code != 0 {
		t.Fatalf("explicit initialization code = %d", code)
	}
	if requests != 0 {
		t.Fatal("initialization reached the credential transport")
	}
	if _, code := action.Answer([]string{"receipt", "run", "101", "config", fixture.config}); code == 0 {
		t.Fatal("registered initialization replaced existing state")
	}
	payload, code := action.Answer([]string{"delivery", "run", "101", "config", fixture.config})
	if code != 0 {
		t.Fatalf("registered delivery code = %d", code)
	}
	report, ok := payload.(notificationDeliveryReport)
	if !ok {
		t.Fatalf("delivery payload = %T", payload)
	}
	if report.State != "delivered" {
		t.Fatalf("delivery state = %s", report.State)
	}
	if report.MessageID != "900" {
		t.Error("registered delivery lost confirmation identity")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "fixture-private-token") {
		t.Error("credential reached the action payload")
	}
	// Each action opens and closes its store; no shared in-memory state can
	// satisfy this repeat, including an independently constructed action.
	reopened := notificationActivationTestArea(notificationActivationTestRead, transport)
	payload, code = reopened.Answer([]string{"delivery", "run", "101", "config", fixture.config})
	if code != 0 {
		t.Fatalf("reopened duplicate code = %d", code)
	}
	report, ok = payload.(notificationDeliveryReport)
	if !ok {
		t.Fatalf("reopened payload = %T", payload)
	}
	if report.State != "quiet" {
		t.Errorf("reopened duplicate state = %s", report.State)
	}
	if posts != 1 {
		t.Errorf("registered delivery posted %d messages", posts)
	}
	body, err := os.ReadFile(filepath.Join(fixture.state, notificationStateFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "fixture-private-token") {
		t.Error("credential reached the receipt")
	}
}

// TestNotificationActivationRefusesUnapprovedConfiguration proves neither
// initialization nor delivery can turn unapproved file data into approval.
func TestNotificationActivationRefusesUnapprovedConfiguration(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing approval", func(data map[string]any) { delete(data, "approval-digest") }},
		{"wrong approval", func(data map[string]any) { data["approval-digest"] = strings.Repeat("a", 64) }},
		{"changed channel", func(data map[string]any) { data["channel-id"] = "124" }},
		{"changed webhook", func(data map[string]any) { data["webhook-id"] = "457" }},
		{"changed failure", func(data map[string]any) { data["failure-template"] = "Changed {run_url}" }},
		{"changed recovery", func(data map[string]any) { data["recovery-template"] = "Changed recovery {run_url}" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			fixture := notificationActivationTestFixture(t)
			change.mutate(fixture.data)
			fixture.write(t)
			requests := 0
			reads := 0
			action := notificationActivationTestArea(func(ctx context.Context, id uint64) (Report, error) {
				reads++
				return notificationActivationTestRead(ctx, id)
			}, notificationTransportFixture(func(*http.Request) (*http.Response, error) {
				requests++
				return nil, errors.New("unexpected credential request")
			}))
			for _, verb := range []string{"receipt", "delivery"} {
				if _, code := action.Answer([]string{verb, "run", "101", "config", fixture.config}); code == 0 {
					t.Errorf("%s accepted unapproved configuration", verb)
				}
			}
			if requests != 0 {
				t.Error("unapproved configuration reached credential transport")
			}
			if reads != 0 {
				t.Error("unapproved configuration reached API evidence")
			}
			if _, err := os.Stat(filepath.Join(fixture.state, notificationStateFile)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("unapproved configuration created state: %v", err)
			}
		})
	}
}

// TestNotificationActivationRefusesMissingCorruptAndUnresolvedState requires
// explicit initialization and preserves an ambiguous attempt after reopening.
func TestNotificationActivationRefusesMissingCorruptAndUnresolvedState(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationActivationTestFixture(t)
			posts := 0
			transport := notificationTransportFixture(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					return notificationHTTPResponse(t, 200, `{"id":"456","channel_id":"123"}`), nil
				}
				posts++
				return nil, errors.New("fixture lost acknowledgment")
			})
			action := notificationActivationTestArea(notificationActivationTestRead, transport)
			if mode != "missing" {
				if _, code := action.Answer([]string{"receipt", "run", "101", "config", fixture.config}); code != 0 {
					t.Fatalf("initialization code = %d", code)
				}
			}
			if mode == "corrupt" {
				if err := os.WriteFile(filepath.Join(fixture.state, notificationStateFile), []byte("{broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, code := action.Answer([]string{"delivery", "run", "101", "config", fixture.config}); code == 0 {
				t.Error("unusable delivery state succeeded")
			}
			reopened := notificationActivationTestArea(notificationActivationTestRead, transport)
			if _, code := reopened.Answer([]string{"delivery", "run", "102", "config", fixture.config}); code == 0 {
				t.Error("reopened unusable state authorized another event")
			}
			wantPosts := 0
			if mode == "ambiguous" {
				wantPosts = 1
			}
			if posts != wantPosts {
				t.Errorf("POST count = %d, want %d", posts, wantPosts)
			}
		})
	}
}

// TestNotificationActivationPrivateFileBoundary rejects inaccessible authority,
// public files, aliases and unrecognized fields before any credential request.
func TestNotificationActivationPrivateFileBoundary(t *testing.T) {
	for _, mode := range []string{"missing config", "public config", "public parent", "symlink config", "public credential", "symlink credential", "oversized config", "unknown field"} {
		t.Run(mode, func(t *testing.T) {
			fixture := notificationActivationTestFixture(t)
			notificationActivationTestReady(t, fixture)
			switch mode {
			case "missing config":
				fixture.config = filepath.Join(filepath.Dir(fixture.config), "missing-private-config")
			case "public config":
				if err := os.Chmod(fixture.config, 0o644); err != nil {
					t.Fatal(err)
				}
			case "public parent":
				if err := os.Chmod(filepath.Dir(fixture.config), 0o755); err != nil {
					t.Fatal(err)
				}
			case "symlink config":
				alias := filepath.Join(filepath.Dir(fixture.config), "config-alias")
				if err := os.Symlink(fixture.config, alias); err != nil {
					t.Fatal(err)
				}
				fixture.config = alias
			case "public credential":
				if err := os.Chmod(fixture.credential, 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink credential":
				alias := filepath.Join(filepath.Dir(fixture.config), "credential-alias")
				if err := os.Symlink(fixture.credential, alias); err != nil {
					t.Fatal(err)
				}
				fixture.data["credential-file"] = alias
				fixture.write(t)
			case "oversized config":
				if err := os.WriteFile(fixture.config, []byte(strings.Repeat(" ", 65537)), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unknown field":
				fixture.data["approved"] = true
				fixture.write(t)
			}
			requests := 0
			action := notificationActivationTestArea(notificationActivationTestRead, notificationTransportFixture(func(*http.Request) (*http.Response, error) {
				requests++
				return nil, errors.New("unexpected credential request")
			}))
			if _, code := action.Answer([]string{"delivery", "run", "101", "config", fixture.config}); code == 0 {
				t.Error("private file boundary accepted unsafe authority")
			}
			if requests != 0 {
				t.Error("unsafe authority reached credential transport")
			}
		})
	}
}

// TestNotificationActivationProductionRegistration requires both explicit
// actions to be discoverable through the same action table as preview/report.
func TestNotificationActivationProductionRegistration(t *testing.T) {
	for _, verb := range []string{"receipt", "delivery"} {
		found := false
		for _, action := range Actions().Actions {
			if action.Verb == verb {
				found = true
			}
		}
		if !found {
			t.Errorf("production GitHub grammar does not register %s", verb)
		}
	}
}
