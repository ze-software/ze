package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// localBridgeDispatcher answers a dispatcher loaded the way the server loads
// one (server.go), from the YANG model this test binary links.
func localBridgeDispatcher(t *testing.T) (*Dispatcher, map[string][]string) {
	t.Helper()
	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("YANG loader: %v", err)
	}
	wireToPaths := yang.WireMethodToPaths(loader)
	d := NewDispatcher()
	if err := loadBuiltinsWithAliases(d, AllBuiltinRPCs(), wireToPaths, yang.PathToDescription(loader),
		yang.PathToHelp(loader), yang.PathToArgDefs(loader), yang.BuildCommandTree(loader)); err != nil {
		t.Fatalf("loadBuiltinsWithAliases: %v", err)
	}
	return d, wireToPaths
}

// TestDeclarationsConfigRefusedReadOnly proves the daemon form of
// `show plugin declarations config <file>` is authorized as more than a read,
// because answering starts each external plugin's `run` line.
//
// VALIDATES: AC-17, R-3. A caller holding a read-only profile (run section
// allows, edit section denies by default, as the built-in read-only profile
// does) is refused with ErrUnauthorized and the `run` line never starts: the
// marker file it would create stays absent. An admin caller gets the answer
// the local route gives for the same file, and the `run` line did start.
// PREVENTS: the verb `show` making the command read-only on the daemon, so a
// read-only operator could start processes under the daemon's identity.
func TestDeclarationsConfigRefusedReadOnly(t *testing.T) {
	const path = "show plugin declarations config"
	d, _ := localBridgeDispatcher(t)
	cmd := d.Lookup(path)
	if cmd == nil {
		t.Fatalf("%q is not served by the dispatcher: the local-data bridge did not load it", path)
	}
	if cmd.ReadOnly {
		t.Fatalf("%q is ReadOnly on the daemon, want not read-only (registry.Meta.NotReadOnly)", path)
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	configFile := filepath.Join(dir, "probe.conf")
	config := "plugin {\n\texternal probe {\n\t\trun \"touch " + marker + "\";\n\t}\n}\n"
	if err := os.WriteFile(configFile, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "read-only",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "admin",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("viewer", []string{"read-only"})
	store.AssignProfiles("operator", []string{"admin"})
	d.SetAuthorizer(authz.StoreAuthorizer{Store: store})

	input := path + " " + configFile
	_, err := d.Dispatch(&CommandContext{Username: "viewer"}, input)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("read-only caller: got %v, want ErrUnauthorized", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("read-only caller: the plugin run line started although authorization refused the command")
	}

	resp, err := d.Dispatch(&CommandContext{Username: "operator"}, input)
	if err != nil {
		t.Fatalf("admin caller: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("admin caller: status %q error %q, want done", resp.Status, resp.Error)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("admin caller: the plugin run line did not start: %v", statErr)
	}
	remote, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatal(err)
	}

	handler, args := command.LookupLocalData(strings.Fields(input))
	validated, err := command.ValidateModelArgs(path, args, nil)
	if err != nil {
		t.Fatalf("local route arguments: %v", err)
	}
	answer := handler(validated)
	local, err := json.Marshal(answer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(remote) != string(local) {
		t.Errorf("daemon answer differs from the local answer\ndaemon: %s\nlocal:  %s", remote, local)
	}
}

// TestLocalAnswerResponseMapsEveryField proves each LocalAnswer field lands
// where the local route puts it: the payload as Data, a non-zero code as a
// failed status, the Diagnostic as the error text.
//
// VALIDATES: AC-7 mapping half. A refusal arrives as the error with its final
// newline dropped; a payload with a non-zero code keeps its data beside the
// error; a failure with no Diagnostic still names the path and the code.
// PREVENTS: a refusal becoming an empty failure on the daemon route.
func TestLocalAnswerResponseMapsEveryField(t *testing.T) {
	// The local route's "error: " is left to the surface, which adds its own:
	// SSH then prints "error: no such key", the local route's exact bytes,
	// where carrying it printed "error: error: no such key".
	resp, err := localAnswerResponse("show x", command.LocalRefusal(2, "error: no such key\n"))
	if err == nil || resp.Status != plugin.StatusError || resp.Error != "no such key" {
		t.Fatalf("refusal: status %q error %q err %v", resp.Status, resp.Error, err)
	}
	if resp.Data != nil {
		t.Errorf("refusal carries data %v, want none", resp.Data)
	}

	resp, err = localAnswerResponse("show x", command.LocalAnswer{Payload: map[string]any{"k": 1}, Code: 1, Diagnostic: "error: rejected\n"})
	if err == nil || resp.Status != plugin.StatusError {
		t.Fatalf("failed answer: status %q err %v", resp.Status, err)
	}
	if encoded, _ := json.Marshal(resp.Data); string(encoded) != `{"k":1}` {
		t.Errorf("failed answer data %s, want {\"k\":1}", encoded)
	}

	resp, err = localAnswerResponse("show x", command.LocalAnswer{Code: 3})
	if err == nil || resp.Error != "show x failed with exit code 3 and gave no reason" {
		t.Errorf("silent failure: error %q err %v", resp.Error, err)
	}

	resp, err = localAnswerResponse("show x", command.LocalAnswer{Payload: []string{"a"}})
	if err != nil || resp.Status != plugin.StatusDone {
		t.Fatalf("answer: status %q err %v", resp.Status, err)
	}
	if encoded, _ := json.Marshal(resp.Data); string(encoded) != `["a"]` {
		t.Errorf("answer data %s, want [\"a\"]", encoded)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("answer with no diagnostic warns %q, want nothing", resp.Warnings)
	}

	// A diagnostic beside a zero code is a warning about an answer still
	// given (`show config dump` parse warnings): every line reaches the
	// caller, and the payload stays the data.
	resp, err = localAnswerResponse("show x", command.LocalAnswer{Payload: []string{"a"}, Diagnostic: "warning: first\nwarning: second\n"})
	if err != nil || resp.Status != plugin.StatusWarning || resp.Error != "" {
		t.Fatalf("warned answer: status %q error %q err %v", resp.Status, resp.Error, err)
	}
	if got := resp.WarningText(); got != "warning: first\nwarning: second" {
		t.Errorf("warned answer warnings %q, want both lines as the local route prints them", got)
	}
	if encoded, _ := json.Marshal(resp.Data); string(encoded) != `["a"]` {
		t.Errorf("warned answer data %s, want [\"a\"]", encoded)
	}
}
