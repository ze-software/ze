// Design: docs/architecture/container/runtime.md -- the container listing

package container

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	containeryang "github.com/ze-software/ze/internal/plugins/container/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The whole reference the image leaf "redis" completes to, which every row
// of these tests shows.
const listedImage = "docker.io/library/redis:latest"

// TestContainerListIsDeclaredOnEverySurface proves the listing is reachable
// on each surface a command needs: the YANG command node binds its wire
// method, the proxy forwards that method to the plugin command, the Stage 1
// declaration names the command with its rows and their column order, and
// handleCommand answers it rather than refusing it as unknown.
//
// Method: read the embedded command module, the proxy registration, the
// declared list and the answer of handleCommand for a supervisor with no
// container.
//
// MUTATION: drop the cmdShowList branch from handleCommand and the command is
// refused as unknown; drop listCommandDecl from commandDecls and the
// declaration is missing.
func TestContainerListIsDeclaredOnEverySurface(t *testing.T) {
	if !strings.Contains(containeryang.ZeContainerCmdYANG, `ze:command "`+wireShowList+`";`) {
		t.Errorf("the command module binds no node to %s", wireShowList)
	}
	proxy := listCommandRPC()
	if proxy.WireMethod != "ze-container:show-list" {
		t.Errorf("the proxy registers wire method %q, want ze-container:show-list", proxy.WireMethod)
	}
	if proxy.PluginCommand != "show container list" {
		t.Errorf("the proxy forwards to %q, want show container list", proxy.PluginCommand)
	}
	if proxy.Handler == nil {
		t.Error("the proxy has no handler")
	}

	index := slices.IndexFunc(commandDecls(), func(decl sdk.CommandDecl) bool { return decl.Name == cmdShowList })
	if index < 0 {
		t.Fatalf("commandDecls declares no %s", cmdShowList)
	}
	decl := commandDecls()[index]
	if decl.Shape != shapeTable {
		t.Errorf("the listing declares shape %q, want rows (%q)", decl.Shape, shapeTable)
	}
	wantColumns := []string{"name", "state", "reason", "image", "digest", "since"}
	if !slices.Equal(decl.Columns, wantColumns) {
		t.Errorf("the listing declares columns %v, want %v", decl.Columns, wantColumns)
	}

	sup := newSupervisor(slogutil.DiscardLogger(), &fakeLibrary{events: &eventLog{}}, storeFactoryOf(&fakeStore{}), nil)
	status, answer, err := handleCommand(context.Background(), sup, cmdShowList, nil)
	if err != nil {
		t.Fatalf("the listing answered %v, want rows", err)
	}
	if status != rpc.StatusDone {
		t.Errorf("the listing answered status %q, want %q", status, rpc.StatusDone)
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("the answer does not encode: %v", err)
	}
	if string(encoded) != `{"containers":[]}` {
		t.Errorf("the listing of no container answered %s, want {\"containers\":[]}", encoded)
	}
}

// TestContainerListShowsEachState proves each state renders as the operator
// surface's table T-1 states it: a running container with the digest of the
// image it runs and when it started; a waiting one with the reference the
// store waits for and each of the store's four reasons; a failed one with
// the step that failed, on one line; a disabled one with nothing more; and an
// enabled one no start ended for as stopped, saying so. Every row carries the
// whole reference its image leaf completes to.
//
// Method: a supervisor over a store whose answer the test sets before each
// start, and a library that creates nothing; the failed record is set as a
// failed start sets it.
//
// MUTATION: key the waiting reason on the configured leaf rather than the
// store's answer, and the reason loses the reference the store completed and
// the fetch's error; drop the digest from the running record and the running
// row carries none.
func TestContainerListShowsEachState(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	library := &fakeLibrary{events: &eventLog{}}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), nil)
	t.Cleanup(sup.killRunning)
	names := []string{"broken", "fetching", "fresh", "lost", "never", "refused", "web"}
	instances := make([]instanceConfig, 0, len(names)+1)
	for _, name := range names {
		instances = append(instances, instanceConfig{name: name, image: "redis", enabled: true})
	}
	instances = append(instances, instanceConfig{name: "off", image: "redis"})
	if err := sup.install(configOf(instances...)); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	waits := map[string]Absent{
		"never":    {Reference: listedImage, Reason: AbsentNeverFetched},
		"fetching": {Reference: listedImage, Reason: AbsentFetchRunning},
		"refused":  {Reference: listedImage, Reason: AbsentFetchFailed, Detail: "the registry answered 404\nfor the manifest"},
		"lost":     {Reference: listedImage, Reason: AbsentImageLost, Detail: "digest sha256:01 is gone; ze create container image reference redis"},
	}
	for name, absent := range waits {
		store.setAnswer(absent)
		state, err := sup.start(ctx, name)
		if err != nil {
			t.Fatalf("the start of %s answered %v, want the waiting-image state", name, err)
		}
		if state != StateWaitingImage {
			t.Fatalf("the start of %s answered %s, want %s", name, state, StateWaitingImage)
		}
	}
	store.setAnswer(readyImage())
	before := time.Now()
	if _, err := sup.start(ctx, "web"); err != nil {
		t.Fatalf("the start of web answered %v, want running", err)
	}
	after := time.Now()
	sup.setFailed("broken", opererr.New(
		"the entrypoint /bin/probe does not exist in the image",
		"the image holds no file at that path\nso nothing can run",
		"ze show container list",
	))

	rows, err := sup.containerRows()
	if err != nil {
		t.Fatalf("the listing answered %v, want rows", err)
	}

	want := []ContainerRow{
		{Name: "broken", State: "failed", Image: listedImage,
			Reason: "the entrypoint /bin/probe does not exist in the image; the image holds no file at that path so nothing can run"},
		{Name: "fetching", State: "waiting-image", Image: listedImage,
			Reason: "the fetch of its image " + listedImage + " is running"},
		{Name: "fresh", State: "stopped", Image: listedImage, Reason: "no start of it has ended yet"},
		{Name: "lost", State: "waiting-image", Image: listedImage,
			Reason: "the image store no longer holds its image " + listedImage + " whole: digest sha256:01 is gone; ze create container image reference redis"},
		{Name: "never", State: "waiting-image", Image: listedImage,
			Reason: "its image " + listedImage + " was never fetched"},
		{Name: "off", State: "disabled", Image: listedImage},
		{Name: "refused", State: "waiting-image", Image: listedImage,
			Reason: "the last fetch of its image " + listedImage + " failed: the registry answered 404 for the manifest"},
		{Name: "web", State: "running", Image: listedImage, Digest: readyImage().Digest},
	}
	if len(rows) != len(want) {
		t.Fatalf("the listing holds %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	running := rows[len(rows)-1]
	if running.Since.Before(before) {
		t.Errorf("the running row says it started at %v, before its start began at %v", running.Since, before)
	}
	if running.Since.After(after) {
		t.Errorf("the running row says it started at %v, after its start ended at %v", running.Since, after)
	}
	rows[len(rows)-1].Since = time.Time{}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d is\n%+v\nwant\n%+v", i, rows[i], want[i])
		}
	}
}

// TestContainerRowKeys proves a row's JSON keys are the kebab names of the
// operator surface's command table, and that a state with no digest or start
// time carries neither key rather than an empty value.
//
// Method: encode one running row and one disabled row.
func TestContainerRowKeys(t *testing.T) {
	since := time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)
	running, err := json.Marshal(ContainerRow{Name: "web", State: "running", Image: listedImage, Digest: "sha256:01", Since: since})
	if err != nil {
		t.Fatalf("the running row does not encode: %v", err)
	}
	wantRunning := `{"name":"web","state":"running","image":"` + listedImage + `","digest":"sha256:01","since":"2026-11-17T09:30:00Z"}`
	if string(running) != wantRunning {
		t.Errorf("the running row encodes as %s, want %s", running, wantRunning)
	}
	disabled, err := json.Marshal(ContainerRow{Name: "off", State: "disabled", Image: listedImage})
	if err != nil {
		t.Fatalf("the disabled row does not encode: %v", err)
	}
	wantDisabled := `{"name":"off","state":"disabled","image":"` + listedImage + `"}`
	if string(disabled) != wantDisabled {
		t.Errorf("the disabled row encodes as %s, want %s", disabled, wantDisabled)
	}
}

// TestContainerListDoesNotWaitForALifecycle proves the listing answers while
// a container's start or stop holds its lifecycle lock: a fetch or a stop
// takes that lock for as long as it runs, and an operator asking what runs
// must not wait behind it.
//
// Method: hold the lifecycle lock of a configured container and ask for the
// listing from another goroutine, bounded at five seconds.
//
// MUTATION: take the lifecycle lock of each container in containerRows and
// the listing waits until the bound fails the test.
func TestContainerListDoesNotWaitForALifecycle(t *testing.T) {
	sup := newSupervisor(slogutil.DiscardLogger(), &fakeLibrary{events: &eventLog{}}, storeFactoryOf(&fakeStore{}), nil)
	if err := sup.install(configOf(instanceConfig{name: "web", image: "redis", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	lifecycle := sup.lifecycle("web")
	lifecycle.Lock()
	defer lifecycle.Unlock()

	answered := make(chan []ContainerRow, 1)
	failed := make(chan error, 1)
	go func() {
		rows, err := sup.containerRows()
		if err != nil {
			failed <- err
			return
		}
		answered <- rows
	}()

	select {
	case rows := <-answered:
		if len(rows) != 1 {
			t.Errorf("the listing holds %d rows, want 1", len(rows))
		}
	case err := <-failed:
		t.Fatalf("the listing answered %v, want rows", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the listing waited for a container's lifecycle lock")
	}
}
