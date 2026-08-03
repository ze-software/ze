// Design: ai/rules/plugins.md -- ze_gnmi reload seam validation
//
//go:build ze_gnmi

package hub

import (
	"testing"
	"time"

	zegnmi "github.com/ze-software/ze/internal/component/gnmi"
)

func TestGNMIReloadNotify(t *testing.T) {
	if gnmiReloadNotify == nil {
		t.Fatal("ze_gnmi build: gNMI reload hook not installed")
	}

	old := activeGNMINotifier
	defer func() { activeGNMINotifier = old }()

	notifier := zegnmi.NewChangeNotifier()
	ch := notifier.Subscribe()
	if ch == nil {
		t.Fatal("expected gNMI subscription channel")
	}
	defer notifier.Unsubscribe(ch)
	activeGNMINotifier = notifier

	gnmiReloadNotify()

	select {
	case got := <-ch:
		updates := got.GetUpdate()
		if len(updates) != 1 || updates[0].GetVal().GetStringVal() != "config-reload" {
			t.Fatalf("reload notification = %#v, want config-reload update", got)
		}
	case <-time.After(time.Second):
		t.Fatal("gNMI reload hook did not notify subscribers")
	}
}
