package iface

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// VALIDATES: each advertising unit's RDNSS selection is published for show
// system dns and ze doctor, named "<interface> unit <unit>", once per
// change, and a unit that stops advertising leaves the published set.
// Method: reconcile over a recording bus and the typed publication handle.
func TestRDNSSSelectionPublished(t *testing.T) {
	bus := newRecordingEventBus()
	var published []*dnssource.Publication
	dnssource.PublicationEvent.Subscribe(bus, func(p *dnssource.Publication) { published = append(published, p) })
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(nil) })

	newRAFactoryRecorder(t)
	active := make(map[raUnitKey]raEntry)
	log := slog.New(slog.DiscardHandler)
	installLearned(t, raLeaseRecord(time.Now().Add(time.Hour), "2001:db8::53"))

	cfg := raTestConfig(t, `{"enabled": "true"}`)
	reconcileRA(cfg, active, log)
	require.Len(t, published, 1)
	require.Equal(t, raConsumer, published[0].Consumer)
	require.Len(t, published[0].Selections, 1)
	got := published[0].Selections[0]
	assert.Equal(t, raRDNSSSourcePath, got.Path)
	assert.Equal(t, "eth0 unit 0", got.Instance)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:db8::53")}, got.Servers)
	assert.True(t, got.Default)

	reconcileRA(cfg, active, log)
	assert.Len(t, published, 1, "an unchanged selection is published again")

	reconcileRA(raTestConfig(t, `{"enabled": "false"}`), active, log)
	require.Len(t, published, 2)
	assert.Empty(t, published[1].Selections, "a unit that stopped advertising is still published")
}
