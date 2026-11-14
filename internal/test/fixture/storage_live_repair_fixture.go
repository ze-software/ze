// Design: docs/architecture/storage-backends.md -- a live salvage walk under the configuration guard
// Related: storage_live_fixture.go -- storageLiveClient, storageLiveRequest, storageLiveRefused
// Related: register_storage_live.go -- registers storage/data-repair-live and storage/data-check-live

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
)

// storageLiveRepair proves `request data repair path <absolute-output>` on a
// running daemon: the daemon writes a new store at the path that `ze data
// --path <output> check` accepts and that holds the runtime key the .ci
// seeded, and answers the path and the counts; a relative path, a ".."
// element, a symlink, an existing output, a name the store owns and a path
// inside the live tree are each refused naming the rule; the offline `ze data
// repair --output` still works beside the daemon; the daemon stays up.
func storageLiveRepair(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	output := filepath.Join(cwd, "live-salvage")
	answer, err := storageLiveRequest(ctx, env, "request data repair path "+output)
	if err != nil {
		return err
	}
	for _, want := range []string{output, "recovered", "skipped"} {
		if !strings.Contains(answer, want) {
			return fmt.Errorf("repair answer lacks %q:\n%s", want, answer)
		}
	}
	checked, err := storageRequireCommand(ctx, "", "data", "--path", output, "check")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(checked), "ok: ") {
		return fmt.Errorf("the salvage does not check clean:\n%s", checked)
	}
	held, err := storageRequireCommand(ctx, "", "data", "--path", output, "cat", storageLiveStaleKey)
	if err != nil {
		return err
	}
	if !strings.Contains(string(held), "stale runtime state") {
		return fmt.Errorf("the salvage lost %s:\n%s", storageLiveStaleKey, held)
	}

	link := filepath.Join(cwd, "live-salvage-link")
	if err := os.Symlink(output, link); err != nil {
		return err
	}
	refusals := []struct{ command, want string }{
		{"request data repair path live-salvage-relative", "the path must be absolute"},
		{"request data repair path " + cwd + "/../live-salvage", `must not contain ".."`},
		{"request data repair path " + link, "the path is a symlink"},
		{"request data repair path " + output, "already exists"},
		{"request data repair path " + filepath.Join(cwd, "salvage.lock"), "a lock sidecar"},
		{"request data repair path " + filepath.Join(cwd, storageTreeName, "inside"), "inside the live tree"},
	}
	for _, refusal := range refusals {
		if err := storageLiveRefused(ctx, env, refusal.command, refusal.want); err != nil {
			return err
		}
	}

	offline, err := storageRequireCommand(ctx, "", "data", "repair", "--output", filepath.Join(cwd, "offline-salvage"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(offline), "0 skipped") {
		return fmt.Errorf("the offline repair beside the daemon did not salvage the store:\n%s", offline)
	}
	if _, err := storageLiveRequest(ctx, env, "show version"); err != nil {
		return err
	}
	return storageLiveOK("data-repair-live")
}

// storageLiveVerdict is the `show data check | json` document.
type storageLiveVerdict struct {
	Path           string `json:"path"`
	Entries        *int   `json:"entries"`
	Corrupt        *int   `json:"corrupt-entries"`
	HistoryChecked *bool  `json:"history-checked"`
	Findings       []any  `json:"findings"`
}

// storageLiveCleanVerdict decodes a `show data check | json` answer and
// requires the verdict of a clean store over the tree at wantPath.
func storageLiveCleanVerdict(route, encoded, wantPath string) error {
	var verdict storageLiveVerdict
	if err := json.Unmarshal([]byte(strings.TrimSpace(encoded)), &verdict); err != nil {
		return fmt.Errorf("%s: show data check | json is not the verdict document: %w:\n%s", route, err, encoded)
	}
	if verdict.Path != wantPath {
		return fmt.Errorf("%s: show data check checked %q, want %q:\n%s", route, verdict.Path, wantPath, encoded)
	}
	if verdict.Entries == nil || *verdict.Entries == 0 {
		return fmt.Errorf("%s: show data check counted no entry:\n%s", route, encoded)
	}
	if verdict.Corrupt == nil || *verdict.Corrupt != 0 {
		return fmt.Errorf("%s: show data check did not answer a clean store:\n%s", route, encoded)
	}
	if verdict.HistoryChecked == nil || !*verdict.HistoryChecked {
		return fmt.Errorf("%s: show data check did not check history:\n%s", route, encoded)
	}
	if len(verdict.Findings) != 0 {
		return fmt.Errorf("%s: show data check answered findings for a clean store:\n%s", route, encoded)
	}
	return nil
}

// storageLiveCheck proves `show data check` on both routes.
//
// The daemon route is asked over SSH exec, which the daemon's dispatcher
// answers (the local-data bridge runs the handler on command.RouteRemote): its
// verdict MUST name the daemon's own tree, <cwd>/database, which only the
// daemon branch of dataCheck answers (storage.HoldWrites hands it the live
// tree). `ze cli -c` is NOT that route: it answers a local-data command in the
// caller's own process (command.ServeLocal in cli/client main.go), over the
// store the client's ZE_CONFIG_DIR names, so that step proves the local route
// and its verdict names client-db/database. The local `ze data check` beside
// the daemon keeps its line.
func storageLiveCheck(ctx context.Context, args []string) error {
	env, err := storageLiveClient(ctx, args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	remote, err := sshclient.ExecCommand(sshclient.Credentials{
		Host: addrLoopback, Port: args[0], Username: storageSeedUser, Auth: "testpass",
	}, "show data check | json")
	if err != nil {
		return fmt.Errorf("daemon route: show data check over SSH exec: %w:\n%s", err, remote)
	}
	if err := storageLiveCleanVerdict("daemon route", remote, filepath.Join(cwd, storageTreeName)); err != nil {
		return err
	}

	plain, err := storageLiveRequest(ctx, env, "show data check")
	if err != nil {
		return err
	}
	for _, want := range []string{"entries", "corrupt-entries", "history-checked"} {
		if !strings.Contains(plain, want) {
			return fmt.Errorf("local route: show data check lacks %q:\n%s", want, plain)
		}
	}
	encoded, err := storageLiveRequest(ctx, env, "show data check | json")
	if err != nil {
		return err
	}
	if err := storageLiveCleanVerdict("local route", encoded, filepath.Join(cwd, "client-db", storageTreeName)); err != nil {
		return err
	}
	local, err := storageRequireCommand(ctx, "", "data", "check")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(local), "ok: ") {
		return fmt.Errorf("ze data check beside the daemon did not answer its line:\n%s", local)
	}
	return storageLiveOK("data-check-live")
}
