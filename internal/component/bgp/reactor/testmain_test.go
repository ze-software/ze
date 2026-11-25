// Design: docs/architecture/core-design.md -- reactor test bootstrap
package reactor

import (
	"os"
	"testing"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
)

func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	if err := events.RegisterNamespace(
		bgpevents.Namespace,
		bgpevents.EventUpdate,
		bgpevents.EventOpen,
		bgpevents.EventNotification,
		bgpevents.EventKeepalive,
		bgpevents.EventRefresh,
		bgpevents.EventState,
		bgpevents.EventNegotiated,
		bgpevents.EventConnected,
		bgpevents.EventFSM,
		bgpevents.EventEOR,
		bgpevents.EventSignal,
		bgpevents.SendRaw,
	); err != nil {
		panic("BUG: reactor testmain register bgp events: " + err.Error())
	}
	os.Exit(m.Run())
}
