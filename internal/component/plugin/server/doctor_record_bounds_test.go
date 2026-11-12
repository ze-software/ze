// Design: docs/architecture/doctor-and-health-checks.md -- the server-side doctor records keep plugin text bounded
// Related: doctor_delivery.go -- recordEnricherFailure, clearEnricherCallFailure, recordHandlerPanic
// Related: doctor_reload_records.go -- capabilityRestoreRecord.failed

package server

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

// assertDoctorCut checks one stored field against what was sent: it is the
// process package's bounded form of sent (whose bounds that package's own
// test pins), shorter than sent, valid UTF-8, a prefix of sent, and marked.
func assertDoctorCut(t *testing.T, field, got, sent, want string) {
	t.Helper()
	assert.Equal(t, want, got, "%s is not the shared bounded form", field)
	assert.Less(t, len(got), len(sent), "%s is kept whole", field)
	assert.True(t, utf8.ValidString(got), "%s was cut inside a rune", field)
	assert.True(t, strings.HasSuffix(got, process.DoctorCutMark), "%s does not say it was cut", field)
	assert.True(t, strings.HasPrefix(sent, strings.TrimSuffix(got, process.DoctorCutMark)), "%s is not a prefix of what was sent", field)
}

// TestServerDoctorRecordsKeepPluginTextBounded records an enricher failure, a
// recovered handler panic and a capability restore failure whose texts are
// far past the doctor bounds, as one plugin answer of up to 16 MiB can be,
// and reads each record back. The texts repeat a two-octet rune, so a cut at
// an odd octet would land inside one.
//
// VALIDATES: each plugin-chosen string is stored cut by the process package's
// helper (process.BoundDoctorName, process.BoundDoctorDetail); a success of
// the same long-named enricher still clears its cut record.
// PREVENTS: up to 16 entries per plugin each keeping a whole plugin answer.
func TestServerDoctorRecordsKeepPluginTextBounded(t *testing.T) {
	long := strings.Repeat("é", 8<<10)

	t.Run("enricher failure", func(t *testing.T) {
		const name = "enrich-long-text"
		t.Cleanup(func() { forgetEnricherFailures(name) })
		command, key := "show "+long, "k"+long
		// Red: the bounds at the top of recordEnricherFailure.
		recordEnricherFailure(name, EnricherFailure{Command: command, Key: key, Error: long})
		got := enricherFailures(name)
		require.Len(t, got, 1)
		assertDoctorCut(t, "command", got[0].Command, command, process.BoundDoctorName(command))
		assertDoctorCut(t, "key", got[0].Key, key, process.BoundDoctorName(key))
		assertDoctorCut(t, "error", got[0].Error, long, process.BoundDoctorDetail(long))

		// Red: the bounds in clearEnricherCallFailure, which must match the
		// cut record with the whole names a success carries.
		clearEnricherCallFailure(name, command, key)
		assert.Empty(t, enricherFailures(name), "a success did not clear the cut record")
	})

	t.Run("handler panic", func(t *testing.T) {
		subscribers := newEngineEventSubscribers()
		// Red: the bound in recordHandlerPanic.
		subscribers.recordHandlerPanic(events.LookupNamespaceID(forkedBGPRouteChange.Namespace()),
			events.LookupEventTypeID(forkedBGPRouteChange.EventType()), long)
		panics := subscribers.handlerPanics()
		require.Len(t, panics, 1)
		assertDoctorCut(t, "panic", panics[0].Panic, long, process.BoundDoctorDetail(long))
	})

	t.Run("capability restore failure", func(t *testing.T) {
		var record capabilityRestoreRecord
		// Red: the bound in capabilityRestoreRecord.failed.
		record.failed(errors.New(long))
		failures := record.read()
		require.Len(t, failures, 1)
		assertDoctorCut(t, "restore error", failures[0].Err, long, process.BoundDoctorDetail(long))
	})
}
