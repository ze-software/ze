// Design: docs/architecture/container/image-store.md -- the binding and the generation record in the state store
// Related: register.go -- the registration that hands the store its engine client
// Related: resolve.go -- the resolve that reads both records
// Related: fetch_steps.go -- the fetch that writes the binding

package imagestore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/zefs"
)

// stateRecordSizeMax bounds a binding or a generation record read back. A
// record Ze wrote is a reference and a digest, a few hundred bytes, so a
// larger one was not written by Ze.
const stateRecordSizeMax = 64 << 10

// engineLink holds the engine client the plugin hands the store: the one
// handle through which the store reads and writes its state keys, the same
// for an internal and a forked plugin. Safe for concurrent use.
type engineLink struct {
	mu     sync.Mutex
	client container.EngineClient
}

// receivedEngine is the link of this process. The plugin hands it the
// client once per run of its engine entry, through the receiver register.go
// installs.
var receivedEngine engineLink

// receive stores client. It MUST only store it: the plugin calls it from its
// engine entry, before the engine has started the plugin.
func (l *engineLink) receive(client container.EngineClient) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.client = client
}

// engine answers the client, or a refusal when the plugin handed none: a
// store whose state cannot be read answers no binding, rather than read the
// missing handle as an empty state.
func (l *engineLink) engine() (container.EngineClient, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.client == nil {
		return nil, opererr.New(
			"the container image store has no connection to the engine's state store",
			"the store reads which image a reference names from the state store, and the container plugin has not handed it the connection yet",
			container.FixShowDoctor,
		)
	}
	return l.client, nil
}

// bindingRecord is the value of a binding key: the whole reference, and the
// digest of the image it is bound to.
type bindingRecord struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

// bindingKey answers the binding key of reference: the SHA-256 of its whole
// form, because a reference holds a slash and a colon and a key parameter is
// one path segment. ParseReference completes a short name, so the short and
// the whole spelling of one image give one key.
func bindingKey(reference registry.Reference) string {
	sum := sha256.Sum256([]byte(reference.String()))
	return zefs.KeyContainerBinding.Key(hex.EncodeToString(sum[:]))
}

// writeBinding binds reference to digest, replacing what the key held: a
// re-fetch of a tag that moved binds it to the digest it resolves to now.
func writeBinding(ctx context.Context, engine container.EngineClient, reference registry.Reference, digest registry.Digest) error {
	content, err := json.Marshal(bindingRecord{Reference: reference.String(), Digest: digest.String()})
	if err != nil {
		return err
	}
	return engine.StatePut(ctx, bindingKey(reference), content)
}

// readBinding answers the digest reference is bound to, and false when no
// fetch bound it. A record that cannot be read, or that names another
// reference, is an error: it is never read as no binding.
func readBinding(ctx context.Context, engine container.EngineClient, reference registry.Reference) (registry.Digest, bool, error) {
	key := bindingKey(reference)
	content, found, err := engine.StateGet(ctx, key)
	if err != nil {
		return registry.Digest{}, false, stateUnreadable(key, err)
	}
	if !found {
		return registry.Digest{}, false, nil
	}
	var record bindingRecord
	if err := decodeStateRecord(content, &record); err != nil {
		return registry.Digest{}, false, stateMalformed(key, err)
	}
	if record.Reference != reference.String() {
		return registry.Digest{}, false, stateMalformed(key, errors.New("the record names "+strconv.Quote(record.Reference)))
	}
	digest, err := registry.ParseDigest(record.Digest)
	if err != nil {
		return registry.Digest{}, false, stateMalformed(key, err)
	}
	return digest, true, nil
}

// generationRecord is the value of a generation key: the reference the
// container's config named at its first start on its current digest, and
// that digest. The previous generation and the events land with the update
// and rollback commands.
type generationRecord struct {
	Reference string `json:"reference"`
	Current   string `json:"current"`
}

// generationKey answers the generation key of instance. The caller MUST have
// checked instance with checkInstance: a key parameter is one path segment.
func generationKey(instance string) string {
	return zefs.KeyContainerGeneration.Key(instance)
}

// recordedGeneration is a generation record read back: the reference it was
// recorded for and the digest it decides.
type recordedGeneration struct {
	reference string
	current   registry.Digest
}

// readGeneration answers the generation record of instance, and false when
// the container never started on a digest of the store.
func readGeneration(ctx context.Context, engine container.EngineClient, instance string) (recordedGeneration, bool, error) {
	key := generationKey(instance)
	content, found, err := engine.StateGet(ctx, key)
	if err != nil {
		return recordedGeneration{}, false, stateUnreadable(key, err)
	}
	if !found {
		return recordedGeneration{}, false, nil
	}
	var record generationRecord
	if err := decodeStateRecord(content, &record); err != nil {
		return recordedGeneration{}, false, stateMalformed(key, err)
	}
	if _, err := registry.ParseReference(record.Reference); err != nil {
		return recordedGeneration{}, false, stateMalformed(key, err)
	}
	current, err := registry.ParseDigest(record.Current)
	if err != nil {
		return recordedGeneration{}, false, stateMalformed(key, err)
	}
	return recordedGeneration{reference: record.Reference, current: current}, true, nil
}

// writeGeneration records digest as the current generation of instance, for
// reference.
func writeGeneration(ctx context.Context, engine container.EngineClient, instance string, reference registry.Reference, digest registry.Digest) error {
	content, err := json.Marshal(generationRecord{Reference: reference.String(), Current: digest.String()})
	if err != nil {
		return err
	}
	key := generationKey(instance)
	if err := engine.StatePut(ctx, key, content); err != nil {
		return opererr.Wrap(err,
			"container "+instance+" was not started: its generation could not be recorded under "+key,
			"the recorded digest decides what the container runs at its next start, so it is recorded before the first",
			container.FixShowDoctor,
		)
	}
	return nil
}

// decodeStateRecord decodes one record of the store's keys, bounded, with no
// field the store does not write.
func decodeStateRecord(content []byte, record any) error {
	if len(content) > stateRecordSizeMax {
		return errors.New("the record is larger than 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	return decoder.Decode(record)
}

// stateUnreadable is the refusal of a state key the engine did not answer.
func stateUnreadable(key string, cause error) error {
	return opererr.Wrap(cause,
		"the container image store could not read "+key,
		"the store reads which image a container runs from the state store, and answers nothing it did not read",
		container.FixShowDoctor,
	)
}

// stateMalformed is the refusal of a state key whose value the store did not
// write.
func stateMalformed(key string, cause error) error {
	return opererr.Wrap(cause,
		"the state key "+key+" holds a record the container image store cannot read",
		"the record decides which image a container runs, so it is never guessed",
		"ze data rm "+key,
	)
}
