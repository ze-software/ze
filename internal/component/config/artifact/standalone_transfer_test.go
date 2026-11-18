// Design: docs/architecture/web-interface.md -- bounded transfers without a web listener.
package artifact_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

// transferServer constructs the production command owner, never an HTTP
// handler, listener or daemon. The actual Server owns final cleanup.
func transferServer(t *testing.T) (*pluginserver.Server, *artifact.Owner) {
	t.Helper()
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	owner := artifact.New()
	owner.FixtureParent(t.TempDir())
	require.NoError(t, configcli.AttachArtifactOwner(server, owner))
	t.Cleanup(server.Stop)
	return server, owner
}

func transferCall(t *testing.T, server *pluginserver.Server, ctx context.Context, user, line string) *plugin.Response {
	t.Helper()
	response, err := server.Dispatcher().Dispatch(&pluginserver.CommandContext{
		Server: server, Username: user, RequestContext: ctx, Sender: plugin.OperatorSender(),
	}, line)
	require.NoError(t, err)
	require.NotNil(t, response)
	return response
}

func transferMap(t *testing.T, server *pluginserver.Server, user, line string) plugin.Map {
	t.Helper()
	response := transferCall(t, server, t.Context(), user, line)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	data, ok := response.Data.(plugin.Map)
	require.True(t, ok)
	return data
}

func transferString(t *testing.T, data plugin.Map, key string) string {
	t.Helper()
	value, ok := data[key].(string)
	require.True(t, ok, key)
	return value
}

func transferUpload(t *testing.T, server *pluginserver.Server, content []byte) string {
	t.Helper()
	begin := transferMap(t, server, "alice", "request config artifact upload begin name original.zefs total "+strconv.Itoa(len(content)))
	token := transferString(t, begin, "transfer")
	for offset := 0; offset < len(content); offset += command.ContentOctetsMax {
		chunk := content[offset:min(offset+command.ContentOctetsMax, len(content))]
		data := transferMap(t, server, "alice", "request config artifact upload append transfer "+token+" offset "+strconv.Itoa(offset)+" content "+base64.StdEncoding.EncodeToString(chunk))
		require.EqualValues(t, offset+len(chunk), data["next"])
	}
	finished := transferMap(t, server, "alice", "request config artifact upload finish transfer "+token)
	return transferString(t, finished, "artifact")
}

func transferFixture(t *testing.T, size int64) []byte {
	t.Helper()
	seed := storage.CreateMemory()
	t.Cleanup(func() { require.NoError(t, seed.Close()) })
	require.NoError(t, seed.WriteFile("router.conf", []byte(policyConfig), 0o600))
	if size == 0 {
		path := filepath.Join(t.TempDir(), "small.zefs")
		_, err := storage.Backup(seed, path, false, zefs.Spare(0))
		require.NoError(t, err)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		return content
	}
	// The native exact-fit writer supplies its actual framing overhead. Both
	// candidate files and the seed belong to this test's temporary directory.
	filler := bytes.Repeat([]byte{0, 255, 128, 1}, int(size/4)-1024)
	require.NoError(t, seed.WriteKey("unknown/binary", filler))
	first := filepath.Join(t.TempDir(), "sizing.zefs")
	result, err := storage.Backup(seed, first, false, zefs.Spare(0))
	require.NoError(t, err)
	filler = append(filler, make([]byte, size-result.Bytes)...)
	require.NoError(t, seed.WriteKey("unknown/binary", filler))
	path := filepath.Join(t.TempDir(), "boundary.zefs")
	result, err = storage.Backup(seed, path, false, zefs.Spare(0))
	require.NoError(t, err)
	require.Equal(t, size, result.Bytes, "the accepted boundary must be a complete native artifact")
	require.NoError(t, os.Remove(first))
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

// TestArtifactStandaloneOwnerCannotBindTwoServers proves a refused attachment
// cannot give an unrelated server authority to close the first server's owner.
func TestArtifactStandaloneOwnerCannotBindTwoServers(t *testing.T) {
	first, owner := transferServer(t)
	second, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(second.Stop)
	require.Error(t, configcli.AttachArtifactOwner(second, owner))
	second.Stop()
	begin := transferMap(t, first, "alice", "request config artifact text begin total 0")
	token := transferString(t, begin, "transfer")
	transferMap(t, first, "alice", "request config artifact upload finish transfer "+token)
	transferMap(t, first, "alice", "request config artifact upload abort transfer "+token)
}

// TestArtifactStandaloneFullArtifactBoundary exercises all 64 MiB through the
// registered packet-bounded RPCs, then hashes the complete immutable download.
func TestArtifactStandaloneFullArtifactBoundary(t *testing.T) {
	server, owner := transferServer(t)
	content := transferFixture(t, artifact.ArtifactBytesMax)
	id := transferUpload(t, server, content)
	info := transferMap(t, server, "alice", "show config artifact info artifact "+id)
	require.EqualValues(t, artifact.ArtifactBytesMax, info["bytes"])
	begin := transferMap(t, server, "alice", "request config artifact download begin artifact "+id)
	token := transferString(t, begin, "transfer")
	require.EqualValues(t, artifact.ArtifactBytesMax, begin["total"])
	digest := sha256.New()
	for offset := int64(0); offset < artifact.ArtifactBytesMax; {
		data := transferMap(t, server, "alice", "show config artifact transfer read transfer "+token+" offset "+strconv.FormatInt(offset, 10))
		chunk, ok := data["content"].(command.Blob)
		require.True(t, ok)
		require.NotEmpty(t, chunk)
		require.LessOrEqual(t, len(chunk), command.ContentOctetsMax)
		_, err := digest.Write(chunk)
		require.NoError(t, err)
		offset += int64(len(chunk))
		require.EqualValues(t, offset, data["next"])
		require.Equal(t, offset == artifact.ArtifactBytesMax, data["complete"])
	}
	want := sha256.Sum256(content)
	require.Equal(t, want[:], digest.Sum(nil))
	final := transferMap(t, server, "alice", "show config artifact transfer read transfer "+token+" offset "+strconv.FormatInt(artifact.ArtifactBytesMax, 10))
	require.Empty(t, final["content"])
	require.Equal(t, true, final["complete"])
	transferMap(t, server, "alice", "request config artifact upload abort transfer "+token)
	_, err := owner.Begin(t.Context(), "alice", "too-large.zefs", artifact.ArtifactBytesMax+1, false)
	require.Error(t, err)
	var refusal *artifact.Refusal
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, 413, refusal.Status)
}

// TestArtifactStandaloneTextTransferBoundaries preserves absence versus empty,
// packet offsets, the complete 4 MiB text allowance and cancellation refusal.
func TestArtifactStandaloneTextTransferBoundaries(t *testing.T) {
	server, owner := transferServer(t)
	for _, size := range []int{0, artifact.TextBytesMax} {
		begin := transferMap(t, server, "alice", "request config artifact text begin total "+strconv.Itoa(size))
		token := transferString(t, begin, "transfer")
		empty := transferMap(t, server, "alice", "request config artifact upload append transfer "+token+" offset 0 content \"\"")
		require.EqualValues(t, 0, empty["next"])
		refused := transferCall(t, server, t.Context(), "bob", "request config artifact upload append transfer "+token+" offset 0 content eA==")
		require.Equal(t, plugin.StatusError, refused.Status)
		refused = transferCall(t, server, t.Context(), "alice", "request config artifact upload append transfer "+token+" offset 1 content eA==")
		require.Equal(t, plugin.StatusError, refused.Status)
		text := bytes.Repeat([]byte("x"), size)
		if size > command.ContentOctetsMax {
			_, err := owner.Append(t.Context(), "alice", token, 0, make([]byte, command.ContentOctetsMax+1))
			require.Error(t, err, "a packet cannot consume more than the shared content allowance")
		}
		for offset := 0; offset < size; offset += command.ContentOctetsMax {
			chunk := text[offset:min(offset+command.ContentOctetsMax, size)]
			transferMap(t, server, "alice", "request config artifact upload append transfer "+token+" offset "+strconv.Itoa(offset)+" content "+base64.StdEncoding.EncodeToString(chunk))
		}
		overflow := transferCall(t, server, t.Context(), "alice", "request config artifact upload append transfer "+token+" offset "+strconv.Itoa(size)+" content eA==")
		require.Equal(t, plugin.StatusError, overflow.Status)
		finished := transferMap(t, server, "alice", "request config artifact upload finish transfer "+token)
		require.EqualValues(t, size, finished["next"])
		require.Equal(t, true, finished["complete"])
		retained, err := owner.Text(t.Context(), "alice", token)
		require.NoError(t, err)
		require.Equal(t, text, retained)
		transferMap(t, server, "alice", "request config artifact upload abort transfer "+token)
	}
	before := owner.FixtureState()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	refused := transferCall(t, server, ctx, "alice", "request config artifact text begin total 0")
	require.Equal(t, plugin.StatusError, refused.Status)
	after := owner.FixtureState()
	require.Equal(t, before.Bytes, after.Bytes)
	require.Equal(t, before.Uploads, after.Uploads)
	_, err := owner.Begin(t.Context(), "alice", "text", artifact.TextBytesMax+1, true)
	require.Error(t, err)
}

// TestArtifactStandaloneBinarySourceBoundary covers arbitrary bytes through
// the same bounded upload authority without mistaking them for config or ZeFS.
func TestArtifactStandaloneBinarySourceBoundary(t *testing.T) {
	_, owner := transferServer(t)
	begin, err := owner.BeginBytes(t.Context(), "alice", "private.bin", artifact.ArtifactBytesMax)
	require.NoError(t, err)
	want := sha256.New()
	chunk := bytes.Repeat([]byte{0, 255, 128, 1}, command.ContentOctetsMax/4)
	for offset := int64(0); offset < artifact.ArtifactBytesMax; {
		part := chunk[:min(int64(len(chunk)), artifact.ArtifactBytesMax-offset)]
		next, err := owner.Append(t.Context(), "alice", begin.ID, offset, part)
		require.NoError(t, err)
		offset += int64(len(part))
		require.Equal(t, offset, next.Next)
		_, err = want.Write(part)
		require.NoError(t, err)
	}
	_, finished, err := owner.Finish(t.Context(), "alice", begin.ID)
	require.NoError(t, err)
	require.True(t, finished.Complete)
	content, err := owner.Bytes(t.Context(), "alice", begin.ID)
	require.NoError(t, err)
	require.EqualValues(t, artifact.ArtifactBytesMax, len(content))
	digest := sha256.Sum256(content)
	require.Equal(t, want.Sum(nil), digest[:])
	_, err = owner.Text(t.Context(), "alice", begin.ID)
	require.Error(t, err, "binary allowance cannot enlarge the text allowance")
	_, err = owner.Bytes(t.Context(), "bob", begin.ID)
	require.Error(t, err)
	require.NoError(t, owner.Abort(t.Context(), "alice", begin.ID))
	_, err = owner.BeginBytes(t.Context(), "alice", "too-large.bin", artifact.ArtifactBytesMax+1)
	require.Error(t, err)
}

// TestArtifactStandaloneIncompleteAndCanceledTransfers preserves the last
// acknowledged offset and never exposes incomplete or canceled bytes as ready.
func TestArtifactStandaloneIncompleteAndCanceledTransfers(t *testing.T) {
	server, owner := transferServer(t)
	begin := transferMap(t, server, "alice", "request config artifact text begin total 2")
	token := transferString(t, begin, "transfer")
	response := transferCall(t, server, t.Context(), "alice", "request config artifact upload finish transfer "+token)
	require.Equal(t, plugin.StatusError, response.Status)
	_, err := owner.Text(t.Context(), "alice", token)
	require.Error(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = owner.Append(ctx, "alice", token, 0, []byte("bad"))
	require.Error(t, err)
	accepted := transferMap(t, server, "alice", "request config artifact upload append transfer "+token+" offset 0 content eA==")
	require.EqualValues(t, 1, accepted["next"])
	response = transferCall(t, server, t.Context(), "alice", "request config artifact upload finish transfer "+token)
	require.Equal(t, plugin.StatusError, response.Status)
	transferMap(t, server, "alice", "request config artifact upload append transfer "+token+" offset 1 content eQ==")
	transferMap(t, server, "alice", "request config artifact upload finish transfer "+token)
	content, err := owner.Text(t.Context(), "alice", token)
	require.NoError(t, err)
	require.Equal(t, "xy", string(content))
	transferMap(t, server, "alice", "request config artifact upload abort transfer "+token)
}

// TestArtifactStandaloneSnapshotsRemainImmutable covers both disk downloads
// and committed memory views while a native publication changes their source.
func TestArtifactStandaloneSnapshotsRemainImmutable(t *testing.T) {
	server, owner := transferServer(t)
	id := transferUpload(t, server, transferFixture(t, 0))
	disk, err := owner.Snapshot(t.Context(), "alice", id, "")
	require.NoError(t, err)
	view, err := owner.Snapshot(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	_, original, err := owner.Read(t.Context(), "alice", disk.ID, 0)
	require.NoError(t, err)
	changed := bytes.ReplaceAll([]byte(policyConfig), []byte("1.2.3.4"), []byte("192.0.2.20"))
	require.NoError(t, owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
		_, err := storage.PublishArtifactConfig(target, "router.conf", changed)
		return err
	}))
	_, retained, err := owner.Read(t.Context(), "alice", disk.ID, 0)
	require.NoError(t, err)
	require.Equal(t, original, retained)
	_, retained, err = owner.Read(t.Context(), "alice", view.ID, 0)
	require.NoError(t, err)
	require.Equal(t, policyConfig, string(retained))
	current, err := owner.Config(t.Context(), "alice", id, "router.conf")
	require.NoError(t, err)
	require.Equal(t, changed, current)
	for _, token := range []string{disk.ID, view.ID} {
		require.NoError(t, owner.Abort(t.Context(), "alice", token))
	}
}

// TestArtifactStandaloneServerAndAccountIsolation reaches the same native
// registrations on two servers, without constructing a web listener.
func TestArtifactStandaloneServerAndAccountIsolation(t *testing.T) {
	first, _ := transferServer(t)
	second, _ := transferServer(t)
	id := transferUpload(t, first, transferFixture(t, 0))
	for _, attempt := range []struct {
		server *pluginserver.Server
		user   string
	}{{first, "bob"}, {second, "alice"}} {
		response := transferCall(t, attempt.server, t.Context(), attempt.user, "show config artifact info artifact "+id)
		require.Equal(t, plugin.StatusError, response.Status)
	}
	transferMap(t, first, "alice", "show config artifact info artifact "+id)
	first.Stop()
	begin := transferMap(t, second, "alice", "request config artifact text begin total 0")
	transferMap(t, second, "alice", "request config artifact upload abort transfer "+transferString(t, begin, "transfer"))
}

type heldArtifactWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	bytes   int
}

func (writer *heldArtifactWriter) Write(content []byte) (int, error) {
	writer.once.Do(func() { close(writer.entered); <-writer.release })
	writer.bytes += len(content)
	return len(content), nil
}

// TestArtifactStandaloneStopJoinsAcceptedExport holds the actual export under
// the shared owner, proving Server.Stop cannot close private storage early.
func TestArtifactStandaloneStopJoinsAcceptedExport(t *testing.T) {
	server, owner := transferServer(t)
	content := transferFixture(t, 0)
	id := transferUpload(t, server, content)
	writer := &heldArtifactWriter{entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(writer.release) }) }
	t.Cleanup(unblock)
	exported := make(chan error, 1)
	go func() { exported <- owner.ExportTo(t.Context(), "alice", id, writer) }()
	select {
	case <-writer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("export did not acquire its private artifact")
	}
	stopped := make(chan struct{})
	go func() { server.Stop(); close(stopped) }()
	require.Eventually(t, func() bool { return server.CheckAdmission() != nil }, 3*time.Second, time.Millisecond,
		"the real stop must enter before checking whether it joins its held owner")
	select {
	case <-stopped:
		t.Fatal("server stopped while accepted artifact export was held")
	case <-time.After(200 * time.Millisecond):
		// The accepted disk export cannot release its owner until unblock.
	}
	unblock()
	require.NoError(t, <-exported)
	require.Equal(t, len(content), writer.bytes)
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("server failed to join the released export")
	}
	require.Nil(t, server.StopFailure())
	_, err := owner.List(t.Context(), "alice")
	require.Error(t, err)
}

// TestArtifactStandaloneStopRetainsCleanupFailure proves private owner cleanup
// failure reaches Server.StopFailure without needing a web service.
func TestArtifactStandaloneStopRetainsCleanupFailure(t *testing.T) {
	server, owner := transferServer(t)
	transferUpload(t, server, transferFixture(t, 0))
	root := owner.FixtureState().Root
	failure := errors.New("standalone cleanup sentinel")
	owner.FixtureRemoval(func(string) error { return failure })
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	server.Stop()
	require.ErrorIs(t, server.StopFailure(), failure)
	require.ErrorIs(t, owner.Close(), failure)
}

var _ io.Writer = (*heldArtifactWriter)(nil)
