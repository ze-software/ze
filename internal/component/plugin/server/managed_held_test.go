// VALIDATES: the hub keeps one copy per client of the configuration it holds
// (config-held): under the authenticated name only, bounded, with its hash
// recomputed and its text parsed before it is kept, replaced by a newer copy,
// named in the fetch answer (held-version), and removed when hub and client
// agree (a current answer, an ok:true ack). A holding ack with a copy clears
// the refused-configuration row; one with none records it.
// PREVENTS: a stored copy the hub cannot show, a copy written under a name the
// payload chose, and a hold shown twice or not at all.

package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	// heldClientText is the configuration a holding client runs. A
	// whitespace-only text parses under any schema the test binary links.
	heldClientText = "\n\n"
	// heldHubText is what the hub holds for the client.
	heldHubText = "hub copy\n"
)

// heldTestStore is a store the hub keeps its records in.
func heldTestStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	return store
}

// heldTestServer is a managed server over store that serves heldHubText to
// every client. Its answers go to a pipe the test drains.
func heldTestServer(t *testing.T, store storage.Storage) (*ManagedServer, *rpc.MuxConn) {
	t.Helper()
	svc := newManagedConfigService(func(string) ([]byte, error) { return []byte(heldHubText), nil })
	svc.records = newHeldRecords(store)
	s := &ManagedServer{
		svc:   svc,
		ctx:   t.Context(),
		mHeld: metrics.NopRegistry{}.CounterVec("held", "held", []string{"result"}),
	}
	hubEnd, clientEnd := net.Pipe()
	go io.Copy(io.Discard, clientEnd) //nolint:errcheck // drains the hub's answers until the pipe closes
	mc := rpc.NewMuxConn(rpc.NewConn(hubEnd, hubEnd))
	t.Cleanup(func() {
		_ = mc.Close()
		_ = hubEnd.Close()
		_ = clientEnd.Close()
	})
	return s, mc
}

// heldParams is a config-held payload for text, naming version.
func heldParams(t *testing.T, version, text string) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(fleet.ConfigHeld{
		Version: version, Config: base64.StdEncoding.EncodeToString([]byte(text)),
		EditedAt: time.Date(2026, time.November, 19, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	return encoded
}

// sendHeld runs the config-held arm for client edge-01 with text.
func sendHeld(t *testing.T, s *ManagedServer, mc *rpc.MuxConn, text string) {
	t.Helper()
	s.handleConfigHeld("edge-01", mc, &rpc.Request{ID: 1, Method: fleet.VerbConfigHeld,
		Params: heldParams(t, fleet.VersionHash([]byte(text)), text)})
}

// TestHeldArmWritesRecordUnderAuthName proves the copy is kept under the
// authenticated name, with its bytes, hash and both times.
//
// VALIDATES: AC-1 and AC-9.
// PREVENTS: a client writing another client's record.
//
// MUTATION: write under any name but the arm's name argument and this fails.
func TestHeldArmWritesRecordUnderAuthName(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)

	sendHeld(t, s, mc, heldClientText)

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held)
	assert.Equal(t, heldClientText, string(record.Config()))
	assert.Equal(t, fleet.VersionHash([]byte(heldClientText)), record.Version())
	assert.False(t, record.EditedAt().IsZero())
	assert.False(t, record.ReceivedAt().IsZero())
	_, other, err := ReadHeldRecord(store, "edge-02")
	require.NoError(t, err)
	assert.False(t, other)
}

// TestHeldArmRefusesOversize proves a copy one byte over the bound is refused
// and nothing is written, and one at the bound passes the size check.
//
// VALIDATES: AC-8 and the boundary row, on the hub.
// PREVENTS: a client making the hub store more than it can show.
//
// MUTATION: drop the decoded-length check of takeHeld and this fails.
func TestHeldArmRefusesOversize(t *testing.T) {
	over := strings.Repeat(" ", fleet.HeldConfigMax+1)
	_, err := takeHeld(heldParams(t, fleet.VersionHash([]byte(over)), over), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "larger than")

	at := strings.Repeat(" ", fleet.HeldConfigMax)
	_, err = takeHeld(heldParams(t, fleet.VersionHash([]byte(at)), at), time.Now())
	assert.NoError(t, err, "a copy of exactly the bound was refused")

	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, over)
	_, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	assert.False(t, held, "a refused copy was written")
}

// TestHeldArmRefusesHashMismatch proves the claimed hash is never trusted.
//
// VALIDATES: AC-8 and A-1, on the hub.
// PREVENTS: a record whose version is not the hash of its bytes.
//
// MUTATION: drop the VersionHash comparison of takeHeld and this fails.
func TestHeldArmRefusesHashMismatch(t *testing.T) {
	_, err := takeHeld(heldParams(t, fleet.VersionHash([]byte("other")), heldClientText), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash")
}

// TestHeldUploadMalformedRefused proves a text that does not parse is refused.
//
// VALIDATES: AC-8 and A-6.
// PREVENTS: a stored copy whose diff the hub can never show.
//
// MUTATION: drop the checkHeldConfig call of takeHeld and this fails.
func TestHeldUploadMalformedRefused(t *testing.T) {
	text := "device {\n"
	_, err := takeHeld(heldParams(t, fleet.VersionHash([]byte(text)), text), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not parse")
}

// TestHeldUploadRefusalQuotesNoConfiguration proves the refusal of a copy the
// hub cannot parse carries none of the configuration's text: the parser's
// refusal names what it refused, and that refusal reaches the hub's log, the
// client and the hold ack the hub shows in show doctor, while the text
// carries the client's secrets.
//
// VALIDATES: the Security Review "Secret exposure" row (closure review).
// PREVENTS: a secret of a client's configuration printed by show doctor.
//
// MUTATION: wrap checkHeldConfig's error into takeHeld's refusal (%w) and
// this fails.
func TestHeldUploadRefusalQuotesNoConfiguration(t *testing.T) {
	const secret = "s3cr3tmarker"
	text := secret + " " + secret + ";\n"
	_, err := takeHeld(heldParams(t, fleet.VersionHash([]byte(text)), text), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not parse")
	assert.NotContains(t, err.Error(), secret)
}

// TestHeldUploadUncheckedPluginSection proves a section of an external plugin
// the hub holds no schema of is kept, with the plugin noted.
//
// VALIDATES: A-6 and R-5.
// PREVENTS: a device whose configuration names an external plugin being
// unresolvable from the hub.
//
// MUTATION: answer every parse refusal as the error in checkHeldConfig and
// this fails.
func TestHeldUploadUncheckedPluginSection(t *testing.T) {
	text := "plugin {\n\texternal acme {\n\t\trun \"acme-monitor\";\n\t}\n}\nacme {\n\tinterval 7;\n}\n"
	record, err := takeHeld(heldParams(t, fleet.VersionHash([]byte(text)), text), time.Now())
	require.NoError(t, err)
	assert.Equal(t, []string{"acme"}, record.UncheckedPlugins())
}

// TestHeldArmReplacesRecord proves a newer copy replaces the record.
//
// VALIDATES: AC-3 on the hub (the revert decision a record may carry is
// dropped with it: the record is written whole).
// PREVENTS: the hub showing an older local edit.
func TestHeldArmReplacesRecord(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	newer := "\n\n\n"

	sendHeld(t, s, mc, newer)

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held)
	assert.Equal(t, fleet.VersionHash([]byte(newer)), record.Version())
}

// heldRaceStore is a store whose next AcquireLock first runs before: what
// another writer does between an unlocked read and the lock.
type heldRaceStore struct {
	storage.Storage
	before func()
}

func (r *heldRaceStore) AcquireLock(name string) (storage.WriteGuard, error) {
	if before := r.before; before != nil {
		r.before = nil
		before()
	}
	return r.Storage.AcquireLock(name)
}

// TestHeldClearKeepsRecordWrittenAfterRead proves that clear keeps a record
// written between its unlocked read and its lock: the newer copy is one the
// fetch or ack that called clear does not prove stale.
//
// Method: a record of one copy; a store that writes a newer copy just before
// clear takes the lock; then clear.
// VALIDATES: heldRecords.clear's contract (a record written after the read
// is kept), E-11 (replaced by a newer upload).
// PREVENTS: a copy the client sent after the hub answered it current removed
// by that older answer, so the hold it reports is shown nowhere.
//
// MUTATION: remove the record under the lock without reading it again, and
// this fails.
func TestHeldClearKeepsRecordWrittenAfterRead(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	newer := "\n\n\n"
	race := &heldRaceStore{Storage: store, before: func() { sendHeld(t, s, mc, newer) }}

	require.NoError(t, newHeldRecords(race).clear("edge-01"))

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held, "clear removed a record written after its read")
	assert.Equal(t, fleet.VersionHash([]byte(newer)), record.Version())
}

// TestHeldRecordDecodeStrict proves an undecodable record is an error.
//
// VALIDATES: E-11. ReadHeldRecord answers an error, never "no record".
// PREVENTS: the only copy of a device's configuration read as absent.
func TestHeldRecordDecodeStrict(t *testing.T) {
	store := heldTestStore(t)
	require.NoError(t, store.WriteKey(zefs.KeyConfigHeld.Key(ClientConfigKey("edge-01")), []byte(`{"version":"x","unknown":1}`)))

	_, held, err := ReadHeldRecord(store, "edge-01")
	require.Error(t, err)
	assert.False(t, held)
}

// TestFetchAnswersHeldVersion proves the fetch answer names the record.
//
// VALIDATES: E-9 and AC-2, AC-17: held-version is read from the store at each
// fetch, so it survives a hub restart.
// PREVENTS: a device sending its copy at every hub write.
//
// MUTATION: drop HeldVersion from HandleConfigFetch and this fails.
func TestFetchAnswersHeldVersion(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	restarted := newManagedConfigService(func(string) ([]byte, error) { return []byte(heldHubText), nil })
	restarted.records = newHeldRecords(store)

	resp, err := restarted.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: fleet.VersionHash([]byte(heldClientText))})
	require.NoError(t, err)
	assert.Equal(t, fleet.VersionHash([]byte(heldClientText)), resp.HeldVersion)
}

// TestFetchCurrentClearsRecord proves a current answer removes the record.
//
// VALIDATES: AC-25 and E-11.
// PREVENTS: a record that outlives the hold it showed.
//
// MUTATION: drop the records.clear call of HandleConfigFetch and this fails.
func TestFetchCurrentClearsRecord(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)

	resp, err := s.svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: fleet.VersionHash([]byte(heldHubText))})
	require.NoError(t, err)
	require.Equal(t, "current", resp.Status)

	_, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	assert.False(t, held)
}

// TestAckOKClearsRecord proves an ok:true ack removes the record.
//
// VALIDATES: E-11.
// PREVENTS: a record of a client that runs what the hub holds.
//
// MUTATION: drop the records.clear call of handleConfigAck and this fails.
func TestAckOKClearsRecord(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)

	s.handleConfigAck("edge-01", fleet.ConfigAck{Version: fleet.VersionHash([]byte(heldHubText)), OK: true}, fleet.VersionHash([]byte(heldHubText)))

	_, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	assert.False(t, held)
}

// TestHoldingAckWithRecordClearsRefusal proves a hold the hub keeps a copy of
// is not shown as a refused configuration.
//
// VALIDATES: E-10 as refined: with a record of the version the client runs,
// no ManagedRejectConfigRejected.
// PREVENTS: one hold shown twice on the hub.
//
// MUTATION: drop the holdShownFromCopy branch and this fails.
func TestHoldingAckWithRecordClearsRefusal(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	s.doctor.recordRejected("edge-01", ManagedRejectConfigRejected, "an earlier refusal")
	sendHeld(t, s, mc, heldClientText)

	s.handleConfigAck("edge-01", fleet.ConfigAck{OK: false, Holding: true, Error: "holding: ..."}, fleet.VersionHash([]byte(heldClientText)))

	assert.Empty(t, s.doctor.snapshot().Rejected)
}

// TestHoldingAckWithOlderRecordRecordsText proves that a holding ack keeps
// the refused row when the record is a copy of another version than the one
// the client named in its last fetch: the client edited again (D2), its D2
// copy was refused or too large, and the hub keeps only the older copy D.
//
// Method: a record of D; the ack's fetched version is D2.
// VALIDATES: spec-fleet-7 AC-4 (a hold whose copy was not sent shows the ack
// text as a refused configuration) with an older record present.
// PREVENTS: a hold the hub has no copy of hidden behind the copy of an older
// version, and a doctor that says the client runs D.
//
// MUTATION: answer held != "" in holdShownFromCopy, without comparing it with
// fetched, and this fails.
func TestHoldingAckWithOlderRecordRecordsText(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	newer := fleet.VersionHash([]byte(heldClientText + "# edited again\n"))

	s.handleConfigAck("edge-01", fleet.ConfigAck{OK: false, Holding: true, Error: "holding: the copy was not sent"}, newer)

	rejected := s.doctor.snapshot().Rejected
	require.Len(t, rejected, 1)
	assert.Equal(t, ManagedRejectConfigRejected, rejected[0].Cause)
	assert.Equal(t, "holding: the copy was not sent", rejected[0].Detail)
}

// TestHoldingAckWithoutRecordRecordsText proves a hold the hub keeps no copy
// of stays visible as a refused configuration.
//
// VALIDATES: R-10.
// PREVENTS: a holding client whose copy was too large invisible on the hub.
//
// MUTATION: clear the refusal on every holding ack and this fails.
func TestHoldingAckWithoutRecordRecordsText(t *testing.T) {
	store := heldTestStore(t)
	s, _ := heldTestServer(t, store)

	s.handleConfigAck("edge-01", fleet.ConfigAck{OK: false, Holding: true, Error: "holding: no copy was sent"}, fleet.VersionHash([]byte(heldClientText)))

	rejected := s.doctor.snapshot().Rejected
	require.Len(t, rejected, 1)
	assert.Equal(t, ManagedRejectConfigRejected, rejected[0].Cause)
	assert.Equal(t, "holding: no copy was sent", rejected[0].Detail)
}

// decideRevert stores on the record of client name the decision that it
// takes the configuration whose hash is hubVersion, as the revert command
// stores it: read and written under the writer lock of the client's
// configuration.
func decideRevert(t *testing.T, store storage.Storage, name, hubVersion string) {
	t.Helper()
	guard, err := store.AcquireLock(ClientConfigKey(name))
	require.NoError(t, err)
	defer func() { require.NoError(t, guard.Release()) }()
	record, held, err := ReadHeldRecordWithGuard(guard, name)
	require.NoError(t, err)
	require.True(t, held)
	require.NoError(t, WriteHeldRecordWithGuard(guard, name,
		record.WithRevert(hubVersion, "operator", time.Date(2026, time.November, 19, 11, 0, 0, 0, time.UTC))))
}

// fetchDiscard answers the discard-version of a fetch of edge-01 from a hub
// that serves hubText, with the client sending version.
func fetchDiscard(t *testing.T, store storage.Storage, hubText, version string) string {
	t.Helper()
	svc := newManagedConfigService(func(string) ([]byte, error) { return []byte(hubText), nil })
	svc.records = newHeldRecords(store)
	resp, err := svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: version})
	require.NoError(t, err)
	return resp.DiscardVersion
}

// TestFetchOffersDiscardOnlyForDAndV proves the fetch answer carries the
// revert decision only while the client fetches with the copy's version D
// and the hub serves the configuration V the decision was taken on.
//
// VALIDATES: spec-fleet-7 AC-13 (the fetch answer carries the discard field
// D), AC-15 and AC-16 (a newer local edit or a newer hub write voids it), E-4.
// PREVENTS: a revert that discards a local edit the operator never saw, or
// that makes the client take a hub configuration the operator never saw.
// MUTATION: offer the decision without RevertDecision.OfferedFor (the D2 and
// V2 rows fail), or drop DiscardVersion from HandleConfigFetch's answer (the
// first row fails).
func TestFetchOffersDiscardOnlyForDAndV(t *testing.T) {
	device := fleet.VersionHash([]byte(heldClientText))
	cases := []struct {
		name    string
		hubText string
		version string
		want    string
	}{
		{"the client runs D and the hub serves V", heldHubText, device, device},
		{"the client runs a newer edit D2", heldHubText, fleet.VersionHash([]byte("a newer local edit\n")), ""},
		{"the hub serves a newer configuration V2", "a newer hub write\n", device, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := heldTestStore(t)
			s, mc := heldTestServer(t, store)
			sendHeld(t, s, mc, heldClientText)
			decideRevert(t, store, "edge-01", fleet.VersionHash([]byte(heldHubText)))

			assert.Equal(t, tc.want, fetchDiscard(t, store, tc.hubText, tc.version))
		})
	}
}

// TestFetchOffersNoDiscardWithoutDecision proves a record with no decision
// offers nothing.
//
// VALIDATES: E-4 (no decision, no revert).
// PREVENTS: a holding client told to discard a change nobody decided on.
// MUTATION: answer the record's version as the discard in heldRecords.offer
// whether or not it carries a decision.
func TestFetchOffersNoDiscardWithoutDecision(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)

	assert.Empty(t, fetchDiscard(t, store, heldHubText, fleet.VersionHash([]byte(heldClientText))))
}

// TestHeldArmReplacesAndDropsDecision proves a newer copy from the client
// replaces the record without the decision taken on the older one.
//
// VALIDATES: spec-fleet-7 AC-3 and AC-15 (the record is replaced and the
// decision dropped).
// PREVENTS: a decision on D outliving D, so the show reports a pending revert
// of a configuration the client no longer runs.
// MUTATION: carry the earlier record's decision into the record the
// config-held arm writes.
func TestHeldArmReplacesAndDropsDecision(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	decideRevert(t, store, "edge-01", fleet.VersionHash([]byte(heldHubText)))

	sendHeld(t, s, mc, "\n\n\n")

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held)
	_, decided := record.Revert()
	assert.False(t, decided)
}

// TestHeldRecordKeepsDecision proves the decision is stored with the record
// and read back whole.
//
// VALIDATES: E-11 (the record holds the pending revert decision D/V), AC-17
// (it survives a hub restart: it is in the store).
// PREVENTS: a decision lost between the command and the fetch.
// MUTATION: drop Revert from encodeHeldRecord.
func TestHeldRecordKeepsDecision(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	sendHeld(t, s, mc, heldClientText)
	decideRevert(t, store, "edge-01", fleet.VersionHash([]byte(heldHubText)))

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held)
	decision, decided := record.Revert()
	require.True(t, decided)
	assert.Equal(t, fleet.VersionHash([]byte(heldClientText)), decision.DeviceVersion())
	assert.Equal(t, fleet.VersionHash([]byte(heldHubText)), decision.HubVersion())
	assert.Equal(t, "operator", decision.DecidedBy())
	assert.Equal(t, time.Date(2026, time.November, 19, 11, 0, 0, 0, time.UTC), decision.DecidedAt().UTC())
}

// TestHeldRecordRefusesForeignDecision proves a stored decision bound to
// another version than the record's is an error, never a decision.
//
// VALIDATES: E-11 (strict decode), R-2.
// PREVENTS: a record edited by hand offering a revert of a version it does
// not hold.
// MUTATION: drop the DeviceVersion != Version check of decodeHeldRecord.
func TestHeldRecordRefusesForeignDecision(t *testing.T) {
	store := heldTestStore(t)
	version := fleet.VersionHash([]byte(heldClientText))
	encoded := `{"version":"` + version + `","config":"` + base64.StdEncoding.EncodeToString([]byte(heldClientText)) +
		`","received-at":"2026-11-19T10:00:00Z","revert":{"device-version":"0123456789abcdef","hub-version":"` +
		fleet.VersionHash([]byte(heldHubText)) + `","decided-at":"2026-11-19T11:00:00Z"}}`
	require.NoError(t, store.WriteKey(HeldRecordKey("edge-01"), []byte(encoded)))

	_, _, err := ReadHeldRecord(store, "edge-01")
	assert.Error(t, err)
}

// TestHoldingAckAfterFetchOfNewerEditClearsRefusal proves the sequence a
// device sends when a local commit (D2) landed during the fetch of a revert
// decided on D leaves no refused row: the device fetches again naming D2,
// sends its copy of D2 and acks holding, on one connection through the
// request arms, so the fetch arm sets the version the ack is judged against.
//
// Method: a record of D and an earlier refusal; then fetch(D2), held(D2) and
// ack(holding) through handleRequest.
// VALIDATES: review round 2 finding 1 (the hub side of the fetch made again
// in managed applyReverted): the copy of D2 is the hold the hub shows.
// PREVENTS: a hub that lists the copy and reports the client rejected.
//
// MUTATION: drop the session.fetched assignment of the config-fetch arm, and
// the ack is judged against no fetch and recorded as a refusal.
func TestHoldingAckAfterFetchOfNewerEditClearsRefusal(t *testing.T) {
	store := heldTestStore(t)
	s, mc := heldTestServer(t, store)
	s.mFetch = metrics.NopRegistry{}.CounterVec("fetch", "fetch", []string{"result"})
	sendHeld(t, s, mc, heldClientText)
	s.doctor.recordRejected("edge-01", ManagedRejectConfigRejected, "an earlier refusal")
	newer := heldClientText + "\n"
	newerVersion := fleet.VersionHash([]byte(newer))
	fetch, err := json.Marshal(fleet.ConfigFetchRequest{Version: newerVersion})
	require.NoError(t, err)
	ack, err := json.Marshal(fleet.ConfigAck{Version: fleet.VersionHash([]byte(heldHubText)), Holding: true, Error: "holding: ..."})
	require.NoError(t, err)
	var session managedSession

	s.handleRequest("edge-01", mc, &rpc.Request{ID: 1, Method: fleet.VerbConfigFetch, Params: fetch}, &session)
	s.handleRequest("edge-01", mc, &rpc.Request{ID: 2, Method: fleet.VerbConfigHeld, Params: heldParams(t, newerVersion, newer)}, &session)
	s.handleRequest("edge-01", mc, &rpc.Request{ID: 3, Method: fleet.VerbConfigAck, Params: ack}, &session)

	record, held, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.True(t, held)
	require.Equal(t, newerVersion, record.Version())
	assert.Empty(t, s.doctor.snapshot().Rejected)
}
