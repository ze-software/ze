// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a holding client runs
// Related: managed_serve.go -- handleRequest, whose config-held and config-ack arms write and clear the record
// Related: managed.go -- HandleConfigFetch, which answers held-version from the record and clears it on current

package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// HeldRecord is the hub's copy of the configuration a managed client holds:
// the bytes the client sent with config-held, their version hash, when that
// version was written on the client, when the hub received it, and the
// external plugins whose sections the hub could not check. A hub keeps at
// most one per client, under KeyConfigHeld of the client's configuration
// (HeldRecordKey), and a newer copy replaces it. Only decodeHeldRecord and
// takeHeld build one, so a HeldRecord always holds bytes whose hash is its
// version. A record carries at most one revert decision, always bound to its
// own version, and a newer copy replaces the record without it.
type HeldRecord struct {
	version    string
	config     []byte
	editedAt   time.Time
	receivedAt time.Time
	unchecked  []string
	// revert is the operator's decision that the client drops this copy's
	// configuration and takes the hub's, nil while none was taken. Only
	// WithRevert and decodeHeldRecord set it.
	revert *RevertDecision
}

// RevertDecision is an operator's decision that a holding client drops the
// configuration it holds and takes the one its hub serves it. It is bound to
// two hashes: the device hash D of the copy it was taken on, and the hash V of
// the configuration the hub served the client when it was taken, the one the
// operator saw. A client that runs another configuration than D, or a hub
// that serves another than V, makes it void (OfferedFor). Only
// HeldRecord.WithRevert and decodeHeldRecord build one.
type RevertDecision struct {
	deviceVersion string
	hubVersion    string
	decidedBy     string
	decidedAt     time.Time
}

// DeviceVersion is D, the device hash of the copy the decision discards.
func (d RevertDecision) DeviceVersion() string { return d.deviceVersion }

// HubVersion is V, the hash of the configuration the hub served the client
// when the decision was taken.
func (d RevertDecision) HubVersion() string { return d.hubVersion }

// DecidedBy is the user who took the decision, empty when the surface named
// none.
func (d RevertDecision) DecidedBy() string { return d.decidedBy }

// DecidedAt is when the decision was taken.
func (d RevertDecision) DecidedAt() time.Time { return d.decidedAt }

// OfferedFor reports whether the decision still holds for a client that
// fetches with version request while the hub serves it served: only for D and
// V. A newer local edit (request is not D) or a newer hub write (served is not
// V) voids it, so a client never discards a change the operator did not see,
// nor takes a configuration the operator did not see.
func (d RevertDecision) OfferedFor(request, served string) bool {
	return request == d.deviceVersion && served == d.hubVersion
}

// Revert answers the revert decision the record carries, and false when it
// carries none.
func (r HeldRecord) Revert() (RevertDecision, bool) {
	if r.revert == nil {
		return RevertDecision{}, false
	}
	return *r.revert, true
}

// WithRevert answers the record carrying the decision that its client drops
// it and takes the configuration whose hash is hubVersion, taken by user at
// decidedAt. The decision is bound to the record's own version, and replaces
// any earlier one.
func (r HeldRecord) WithRevert(hubVersion, user string, decidedAt time.Time) HeldRecord {
	r.revert = &RevertDecision{
		deviceVersion: r.version, hubVersion: hubVersion,
		decidedBy: user, decidedAt: decidedAt,
	}
	return r
}

// Version is the version hash of the held bytes, recomputed by the hub.
func (r HeldRecord) Version() string { return r.version }

// Config answers a copy of the held configuration bytes. They carry the
// client's secrets: a caller MUST NOT print them.
func (r HeldRecord) Config() []byte { return slices.Clone(r.config) }

// EditedAt is when the held version was written on the client. The zero time
// means the client's store records no such time.
func (r HeldRecord) EditedAt() time.Time { return r.editedAt }

// ReceivedAt is when the hub stored the copy.
func (r HeldRecord) ReceivedAt() time.Time { return r.receivedAt }

// UncheckedPlugins names the external plugins whose sections the hub could
// not parse when it took the copy, because it did not start them. Empty when
// the hub parsed the whole configuration.
func (r HeldRecord) UncheckedPlugins() []string { return slices.Clone(r.unchecked) }

// heldRecordJSON is the stored form of a HeldRecord. Config is base64 in the
// JSON, as encoding/json writes a byte slice.
type heldRecordJSON struct {
	Version    string    `json:"version"`
	Config     []byte    `json:"config"`
	EditedAt   time.Time `json:"edited-at,omitzero"`
	ReceivedAt time.Time `json:"received-at"`
	Unchecked  []string  `json:"unchecked-plugins,omitempty"`
	// Revert is the record's revert decision, absent while none was taken.
	Revert *heldRevertJSON `json:"revert,omitempty"`
}

// heldRevertJSON is the stored form of a RevertDecision.
type heldRevertJSON struct {
	DeviceVersion string    `json:"device-version"`
	HubVersion    string    `json:"hub-version"`
	DecidedBy     string    `json:"decided-by,omitempty"`
	DecidedAt     time.Time `json:"decided-at"`
}

// HeldRecordKey is the store key of the record of client name:
// meta/config/client-<name>.conf/held. It is what an operator removes with
// the hub stopped (ze data rm) when no serving block declares the client.
func HeldRecordKey(name string) string {
	return zefs.KeyConfigHeld.Key(ClientConfigKey(name))
}

// HeldClients answers the names of the clients the hub keeps a record for, in
// the order the store lists their keys, whether or not a serving block still
// declares them. The key form is KeyConfigHeld's, so a key under meta/config/
// of another form is not a record and is skipped.
func HeldClients(store storage.Storage) ([]string, error) {
	prefix, suffix, found := strings.Cut(zefs.KeyConfigHeld.Pattern, "{name}")
	if !found {
		panic("BUG: pluginserver.HeldClients: zefs.KeyConfigHeld has no {name} segment")
	}
	keys, err := store.ListKeys(prefix)
	if err != nil {
		return nil, fmt.Errorf("list the held configurations under %s: %w", prefix, err)
	}
	var names []string
	for _, key := range keys {
		configName, isRecord := strings.CutSuffix(strings.TrimPrefix(key, prefix), suffix)
		if !isRecord {
			continue // Another key of a configuration, such as its active pointer.
		}
		if strings.Contains(configName, "/") {
			continue // A key nested deeper than a configuration name.
		}
		name, isClient := ClientNameFromConfigKey(configName)
		if !isClient {
			continue // A record of a configuration that is not a client's.
		}
		names = append(names, name)
	}
	return names, nil
}

// ReadHeldRecord reads the record the hub keeps for client name. held is
// false only when no record exists; a record that does not decode is an error,
// never "no record", because reading it as none would drop the only copy of
// the client's configuration.
func ReadHeldRecord(store storage.Storage, name string) (record HeldRecord, held bool, err error) {
	return readHeldRecord(store.ReadKey, name)
}

// ReadHeldRecordWithGuard is ReadHeldRecord for a caller that holds guard,
// the writer lock of the client's configuration (ClientConfigKey(name)): the
// record it answers cannot change until the caller releases guard, so a
// decision taken on it and a write under the same guard cannot interleave
// with a newer copy from the client.
func ReadHeldRecordWithGuard(guard storage.WriteGuard, name string) (record HeldRecord, held bool, err error) {
	return readHeldRecord(guard.ReadKey, name)
}

// readHeldRecord is ReadHeldRecord over one raw-key reader: the store's, or a
// held guard's.
func readHeldRecord(readKey func(string) ([]byte, error), name string) (record HeldRecord, held bool, err error) {
	key := HeldRecordKey(name)
	encoded, err := readKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return HeldRecord{}, false, nil
	}
	if err != nil {
		return HeldRecord{}, false, fmt.Errorf("read the held configuration %s: %w", key, err)
	}
	record, err = decodeHeldRecord(key, encoded)
	if err != nil {
		return HeldRecord{}, false, err
	}
	return record, true, nil
}

// decodeHeldRecord decodes the record stored under key, strictly: an unknown
// field, a missing receive time, or bytes whose hash is not the recorded
// version is an error.
func decodeHeldRecord(key string, encoded []byte) (HeldRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var stored heldRecordJSON
	if err := decoder.Decode(&stored); err != nil {
		return HeldRecord{}, fmt.Errorf("decode the held configuration %s: %w", key, err)
	}
	if stored.ReceivedAt.IsZero() {
		return HeldRecord{}, fmt.Errorf("the held configuration %s has no receive time", key)
	}
	if fleet.VersionHash(stored.Config) != stored.Version {
		return HeldRecord{}, fmt.Errorf("the held configuration %s names version %s, which is not the hash of its bytes", key, stored.Version)
	}
	record := HeldRecord{
		version: stored.Version, config: stored.Config, editedAt: stored.EditedAt,
		receivedAt: stored.ReceivedAt, unchecked: stored.Unchecked,
	}
	if stored.Revert == nil {
		return record, nil
	}
	decision := stored.Revert
	if decision.DeviceVersion != stored.Version {
		return HeldRecord{}, fmt.Errorf("the held configuration %s is version %s and carries a revert decision of version %s",
			key, stored.Version, decision.DeviceVersion)
	}
	if decision.HubVersion == "" || decision.DecidedAt.IsZero() {
		return HeldRecord{}, fmt.Errorf("the revert decision of the held configuration %s names no hub version or no time", key)
	}
	return record.WithRevert(decision.HubVersion, decision.DecidedBy, decision.DecidedAt), nil
}

// encodeHeldRecord is the stored form of record, the record of client name.
func encodeHeldRecord(name string, record HeldRecord) ([]byte, error) {
	stored := heldRecordJSON{
		Version: record.version, Config: record.config, EditedAt: record.editedAt,
		ReceivedAt: record.receivedAt, Unchecked: record.unchecked,
	}
	if decision, decided := record.Revert(); decided {
		stored.Revert = &heldRevertJSON{
			DeviceVersion: decision.deviceVersion, HubVersion: decision.hubVersion,
			DecidedBy: decision.decidedBy, DecidedAt: decision.decidedAt,
		}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("encode the held configuration of %s: %w", name, err)
	}
	return encoded, nil
}

// WriteHeldRecordWithGuard stores record as the record of client name under
// guard, the writer lock of the client's configuration
// (ClientConfigKey(name)), which the caller MUST hold. It replaces the record
// whole; the caller read the one it replaces under the same guard
// (ReadHeldRecordWithGuard), so no newer copy from the client is lost.
func WriteHeldRecordWithGuard(guard storage.WriteGuard, name string, record HeldRecord) error {
	encoded, err := encodeHeldRecord(name, record)
	if err != nil {
		return err
	}
	if err := guard.WriteKey(HeldRecordKey(name), encoded); err != nil {
		return fmt.Errorf("write the held configuration %s: %w", HeldRecordKey(name), err)
	}
	return nil
}

// heldRecords is the managed server's access to the held records in the hub's
// store. Every write and removal holds the writer lock of the client's
// configuration (ClientConfigKey), the lock every writer of that
// configuration takes. A nil *heldRecords is a server built with no store: it
// keeps no record, so it refuses every copy (handleConfigHeld) and names no
// held version. Safe for concurrent use: the store's lock orders the writers.
type heldRecords struct {
	store storage.Storage
}

// newHeldRecords answers the records of store, or nil for a nil store.
func newHeldRecords(store storage.Storage) *heldRecords {
	if store == nil {
		return nil
	}
	return &heldRecords{store: store}
}

// version answers the version of the record of client name, and "" when the
// hub keeps none.
func (h *heldRecords) version(name string) (string, error) {
	if h == nil {
		return "", nil
	}
	record, held, err := ReadHeldRecord(h.store, name)
	if err != nil {
		return "", err
	}
	if !held {
		return "", nil
	}
	return record.version, nil
}

// offer answers what a config-fetch answer of client name says of the hub's
// copy, for a fetch that sent version request while the hub serves the
// client the configuration whose hash is served: held is the version of the
// record ("" when the hub keeps none), and discard is the version the record's
// revert decision offers the client to discard, "" unless the decision still
// holds for request and served (RevertDecision.OfferedFor).
func (h *heldRecords) offer(name, request, served string) (held, discard string, err error) {
	if h == nil {
		return "", "", nil
	}
	record, found, err := ReadHeldRecord(h.store, name)
	if err != nil || !found {
		return "", "", err
	}
	if decision, decided := record.Revert(); decided && decision.OfferedFor(request, served) {
		discard = decision.deviceVersion
	}
	return record.version, discard, nil
}

// write stores record as the record of client name, replacing any earlier
// one with whatever it held: a newer copy from the client drops the revert
// decision of the copy it replaces, because that decision was taken on
// another version.
func (h *heldRecords) write(name string, record HeldRecord) (err error) {
	guard, err := h.store.AcquireLock(ClientConfigKey(name))
	if err != nil {
		return fmt.Errorf("lock %s: %w", ClientConfigKey(name), err)
	}
	defer func() { err = errors.Join(err, guard.Release()) }()
	return WriteHeldRecordWithGuard(guard, name, record)
}

// clear removes the record of client name that a fetch answered current or
// an ok:true ack proved stale. No record is not an error.
func (h *heldRecords) clear(name string) (err error) {
	if h == nil {
		return nil
	}
	// Every in-sync client's fetch reaches here, so the lock is taken only
	// when a record exists. One written after this read, by another writer
	// that took the lock first, is a copy or a decision this fetch or ack
	// does not prove stale, so it is read again under the lock and kept
	// unless it is still the record read here.
	key := HeldRecordKey(name)
	seen, readErr := h.store.ReadKey(key)
	if errors.Is(readErr, fs.ErrNotExist) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("read the held configuration %s: %w", key, readErr)
	}
	guard, err := h.store.AcquireLock(ClientConfigKey(name))
	if err != nil {
		return fmt.Errorf("lock %s: %w", ClientConfigKey(name), err)
	}
	defer func() { err = errors.Join(err, guard.Release()) }()
	current, readErr := guard.ReadKey(key)
	if errors.Is(readErr, fs.ErrNotExist) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("read the held configuration %s: %w", key, readErr)
	}
	if !bytes.Equal(current, seen) {
		return nil // Written after the read: kept.
	}
	removeErr := guard.RemoveKey(key)
	if errors.Is(removeErr, fs.ErrNotExist) {
		return nil
	}
	if removeErr != nil {
		return fmt.Errorf("remove the held configuration %s: %w", key, removeErr)
	}
	return nil
}

// handleConfigHeld is the config-held arm of handleRequest: it stores the copy
// client name sent of the configuration it holds. The name is the
// authenticated one: the payload names no client. A copy the hub refuses is
// answered with fleet.HeldRefusedCode and writes nothing, so a record already
// there is unchanged; a hub with no store, or a store that cannot take the
// record, answers an error with no code, so the client sends no copy again on
// this connection.
func (s *ManagedServer) handleConfigHeld(name string, mc *rpc.MuxConn, req *rpc.Request) {
	if s.svc.records == nil {
		s.mHeld.With("refused").Inc()
		_ = mc.SendError(s.ctx, req.ID, "this hub keeps no copy of a held configuration")
		return
	}
	record, refusal := takeHeld(req.Params, time.Now())
	if refusal != nil {
		s.mHeld.With("refused").Inc()
		managedLogger().Warn("managed client sent a copy of its held configuration that the hub refused",
			"code", CodeDoctorManagedClientRejected, "name", name, "error", refusal)
		_ = mc.SendCodedError(s.ctx, req.ID, fleet.HeldRefusedCode, refusal.Error())
		return
	}
	if err := s.svc.records.write(name, record); err != nil {
		s.mHeld.With("refused").Inc()
		managedLogger().Warn("the hub could not store the copy of a managed client's held configuration",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", name, "error", err)
		_ = mc.SendError(s.ctx, req.ID, "the hub could not store the copy: "+err.Error())
		return
	}
	s.mHeld.With("stored").Inc()
	managedLogger().Info("managed client holds, and the hub stored a copy of its configuration",
		"name", name, "version", record.version, "size", len(record.config), "unchecked-plugins", record.unchecked)
	_ = mc.SendOK(s.ctx, req.ID)
}

// takeHeld decodes and checks a config-held payload, received at receivedAt.
// The bound is checked on the encoded length before anything is decoded, then
// on the decoded bytes, whatever the client claims. The hash is recomputed and
// never trusted, and the text is parsed before it is kept (checkHeldConfig).
func takeHeld(params json.RawMessage, receivedAt time.Time) (HeldRecord, error) {
	var held fleet.ConfigHeld
	if err := json.Unmarshal(params, &held); err != nil {
		return HeldRecord{}, fmt.Errorf("the copy does not decode: %w", err)
	}
	if len(held.Config) > base64.StdEncoding.EncodedLen(fleet.HeldConfigMax) {
		return HeldRecord{}, heldOversize(base64.StdEncoding.DecodedLen(len(held.Config)))
	}
	data, err := base64.StdEncoding.DecodeString(held.Config)
	if err != nil {
		return HeldRecord{}, fmt.Errorf("the copy's configuration is not base64: %w", err)
	}
	if len(data) > fleet.HeldConfigMax {
		return HeldRecord{}, heldOversize(len(data))
	}
	if len(data) == 0 {
		return HeldRecord{}, errors.New("the copy carries no configuration")
	}
	version := fleet.VersionHash(data)
	if version != held.Version {
		return HeldRecord{}, fmt.Errorf("the copy names version %s, and its bytes hash to %s", held.Version, version)
	}
	unchecked, err := checkHeldConfig(string(data))
	if err != nil {
		// The parser's refusal is dropped, never wrapped: it can quote the
		// configuration (a refused leaf value and its fix line), which carries
		// the client's secrets, and this refusal reaches the hub's log, the
		// client, and the hold ack the hub then shows in show doctor. The show
		// drops it for the same reason (divergenceParseRefusal).
		return HeldRecord{}, errHeldUnparsed
	}
	return HeldRecord{
		version: version, config: data, editedAt: held.EditedAt,
		receivedAt: receivedAt, unchecked: unchecked,
	}, nil
}

// errHeldUnparsed is the refusal of a copy whose text the hub's parser
// refuses. It carries no part of the parser's refusal (takeHeld).
var errHeldUnparsed = errors.New("the copy does not parse as a configuration on this hub; " +
	"the parser's refusal is not repeated because it can quote the configuration, which carries secrets")

// heldOversize is the refusal of a copy larger than fleet.HeldConfigMax.
func heldOversize(size int) error {
	return errors.New("the copy is " + strconv.Itoa(size) + " bytes, larger than the " +
		strconv.Itoa(fleet.HeldConfigMax) + " bytes a hub keeps")
}

// checkHeldConfig parses a held configuration with the hub's schema, so the
// hub keeps only a configuration it can show. It answers the external plugins
// whose sections the hub could not check: a section of an external plugin the
// hub did not start is refused by the parse as a name the schema does not
// define, and that refusal is accepted when the configuration declares such a
// plugin (config.DeclaredSchemas.WithoutSchema). Every other refusal is
// answered as the error. The parse stops at its first refusal, so a
// configuration accepted on an unchecked section is not checked past it.
func checkHeldConfig(text string) ([]string, error) {
	schema, declared, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: text})
	if err != nil {
		return nil, err
	}
	_, parseErr := config.NewParser(schema).Parse(text)
	if parseErr == nil {
		return nil, nil
	}
	if !config.RefusesUnknownNode(parseErr) {
		return nil, parseErr
	}
	unchecked := declared.WithoutSchema()
	if len(unchecked) == 0 {
		return nil, parseErr
	}
	return unchecked, nil
}
