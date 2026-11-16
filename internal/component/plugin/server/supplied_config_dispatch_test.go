// Design: docs/architecture/api/commands.md -- supplied input through the shared dispatcher.

package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/redact"
)

func suppliedInspectionDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	d, _ := localBridgeDispatcher(t)
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "reader",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "blocked",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("reader", []string{"reader"})
	store.AssignProfiles("blocked", []string{"blocked"})
	d.SetAuthorizer(authz.StoreAuthorizer{Store: store})
	return d
}

func suppliedInspectionLine(t *testing.T, operation, content string) (line, encoded string) {
	t.Helper()
	if operation == "diff" {
		data, err := json.Marshal(map[string]string{"before": content, "after": content})
		if err != nil {
			t.Fatal(err)
		}
		content = string(data)
	}
	encoded = base64.StdEncoding.EncodeToString([]byte(content))
	return "show config supplied " + operation + " content " + encoded, encoded
}

// TestSuppliedConfigDispatcherPreservesReaderPolicyAndAccounting proves the
// actual model/local-data bridge admits a read-only caller, refuses a denied
// caller and records the original identity with neither source in accounting.
// This is shared-dispatch evidence; authenticated transport acceptance is
// separate from this test.
func TestSuppliedConfigDispatcherPreservesReaderPolicyAndAccounting(t *testing.T) {
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			d := suppliedInspectionDispatcher(t)
			path := "show config supplied " + operation
			registered := d.Lookup(path)
			if registered == nil {
				t.Fatalf("%s is not registered through its real YANG/local-data owner", path)
			}
			if !registered.ReadOnly || registered.Owner == "" {
				t.Fatalf("%s lost its read-only classification or owner", path)
			}
			line, encoded := suppliedInspectionLine(t, operation, "environment { log { level debug; } }\n")
			masked := path + " content " + redact.Placeholder
			for _, user := range []string{"reader", "blocked"} {
				t.Run(user, func(t *testing.T) {
					acct := &fakeAccountant{}
					d.SetAccountingHook(acct)
					cc := &CommandContext{
						Username: user, RemoteAddr: "192.0.2.51:2200",
						RequestContext: context.Background(), Sender: plugin.OperatorSender(),
					}
					resp, err := d.Dispatch(cc, line)
					if user == "reader" {
						if err != nil || resp == nil || resp.Status == plugin.StatusError || resp.Data == nil {
							t.Fatalf("read-only caller did not get the inspection answer: response %+v, error %v", resp, err)
						}
						encodedAnswer, marshalErr := json.Marshal(resp.Data)
						if marshalErr != nil {
							t.Fatalf("inspection answer is not valid response data: %v", marshalErr)
						}
						var answer map[string]json.RawMessage
						if decodeErr := json.Unmarshal(encodedAnswer, &answer); decodeErr != nil || len(answer) == 0 {
							t.Fatalf("inspection answer is not a nonempty document: %s, %v", encodedAnswer, decodeErr)
						}
					} else if !errors.Is(err, ErrUnauthorized) || resp == nil || resp.Status != plugin.StatusError {
						t.Fatalf("denied caller was not refused: response %+v, error %v", resp, err)
					}
					assertRecords(t, "START", acct.starts, masked, encoded)
					assertRecords(t, "STOP", acct.stops, masked, encoded)
					if len(acct.startUsers) != 1 || acct.startUsers[0] != user ||
						len(acct.stopUsers) != 1 || acct.stopUsers[0] != user ||
						len(acct.startRemotes) != 1 || acct.startRemotes[0] != cc.RemoteAddr ||
						len(acct.stopRemotes) != 1 || acct.stopRemotes[0] != cc.RemoteAddr {
						t.Errorf("accounting lost the original caller: %+v", acct)
					}
				})
			}
		})
	}
}

// TestSuppliedConfigDispatcherNeverAppliesEnvironment proves read-only source
// inspection does not publish an environment block to the running daemon.
func TestSuppliedConfigDispatcherNeverAppliesEnvironment(t *testing.T) {
	const key = "ze.log"
	if value := env.Get(key); value != "" {
		t.Fatalf("positive control requires the unset daemon log level; found %q", value)
	}
	t.Cleanup(func() {
		if err := os.Unsetenv(key); err != nil {
			t.Errorf("unset %s: %v", key, err)
		}
		env.ResetCache()
	})
	d := suppliedInspectionDispatcher(t)
	for _, operation := range []string{"dump", "validation", "fmt", "graph", "diff"} {
		t.Run(operation, func(t *testing.T) {
			line, _ := suppliedInspectionLine(t, operation, "environment { log { level debug; } }\n")
			resp, err := d.Dispatch(&CommandContext{
				Username: "reader", RequestContext: context.Background(),
				Sender: plugin.OperatorSender(),
			}, line)
			if err != nil || resp == nil || resp.Status == plugin.StatusError {
				t.Fatalf("inspection refused before testing environment isolation: %+v, %v", resp, err)
			}
			if value := env.Get(key); value != "" {
				t.Errorf("inspection applied supplied environment to the daemon: %s=%q", key, value)
			}
		})
	}
}

// TestSuppliedConfigDispatcherPassesCancellation proves the request context
// reaches the owner handler through localDataHandler, rather than beginning a
// fresh background operation after the original caller has disconnected.
func TestSuppliedConfigDispatcherPassesCancellation(t *testing.T) {
	d := suppliedInspectionDispatcher(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	line, _ := suppliedInspectionLine(t, "dump", "environment { log { level debug; } }\n")
	resp, err := d.Dispatch(&CommandContext{
		Username: "reader", RequestContext: ctx, Sender: plugin.OperatorSender(),
	}, line)
	if err == nil || resp == nil || resp.Status != plugin.StatusError || resp.Data != nil ||
		!strings.Contains(resp.Error, context.Canceled.Error()) {
		t.Errorf("canceled dispatcher caller received an answer: %+v, %v", resp, err)
	}
}
