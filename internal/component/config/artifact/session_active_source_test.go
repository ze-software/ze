// Design: docs/architecture/web-interface.md -- artifact sessions use committed version authority.
package artifact_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestArtifactSessionReadsTheActiveVersion reaches the real explicit session
// commands on pointer-only and stale-mirror artifacts. An unresolved pointer
// refuses admission without editing its otherwise readable fallback mirror.
func TestArtifactSessionReadsTheActiveVersion(t *testing.T) {
	for _, state := range []string{"pointer-only", "stale-mirror", "unresolved"} {
		t.Run(state, func(t *testing.T) {
			fixture := newPolicyHTTP(t, nil, false)
			id := artifactOperationUpload(t, fixture)
			require.NoError(t, fixture.owner.ChangeStore(t.Context(), "alice", id, func(target storage.Storage) error {
				if _, err := storage.PublishArtifactConfig(target, "router.conf", []byte(policyConfig)); err != nil {
					return err
				}
				if state == "pointer-only" {
					return target.Remove("router.conf")
				}
				stale := strings.ReplaceAll(policyConfig, "1.2.3.4", "192.0.2.199")
				if err := target.WriteFile("router.conf", []byte(stale), 0o600); err != nil {
					return err
				}
				if state == "unresolved" {
					return target.WriteKey(zefs.KeyConfigActive.Key("router.conf"), []byte("20000101-000000.000\n"))
				}
				return nil
			}))
			before, err := fixture.owner.Inspect(t.Context(), "alice", id)
			require.NoError(t, err)
			selection := " artifact " + id + " name router.conf"
			response := artifactOperationCall(t, fixture, "request config artifact session resume"+selection)
			if state == "unresolved" {
				require.Equal(t, plugin.StatusError, response.Status)
				after, err := fixture.owner.Inspect(t.Context(), "alice", id)
				require.NoError(t, err)
				require.Equal(t, before.Digest, after.Digest)
				return
			}
			require.Equal(t, plugin.StatusDone, response.Status, response.Error)
			text := artifactOperationText(t, fixture, "set bgp session asn local 65001")
			artifactOperationData(t, fixture, "request config artifact session set"+selection+" text "+text)
			result := artifactOperationData(t, fixture, "request config artifact session commit"+selection)
			require.Equal(t, true, result["committed"])
			current, err := fixture.owner.Config(t.Context(), "alice", id, "router.conf")
			require.NoError(t, err)
			require.Contains(t, string(current), "1.2.3.4")
			require.Contains(t, string(current), "65001")
			require.NotContains(t, string(current), "192.0.2.199")
		})
	}
}
