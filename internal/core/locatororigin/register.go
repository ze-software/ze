// Design: docs/architecture/isis/isis-4-component-config.md -- locator-originator EventBus registration
// Related: snapshot.go -- Namespace, EventSnapshot, EventSnapshotRequest

package locatororigin

import "github.com/ze-software/ze/internal/core/events"

func init() {
	if err := events.RegisterMessage(Namespace, EventSnapshot); err != nil {
		panic("BUG: locatororigin: register snapshot message: " + err.Error())
	}
	if err := events.RegisterMessage(Namespace, EventSnapshotRequest); err != nil {
		panic("BUG: locatororigin: register snapshot-request message: " + err.Error())
	}
}
