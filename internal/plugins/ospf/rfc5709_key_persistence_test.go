// Design: docs/architecture/ospf/ospf-12-auth.md -- key chains and the OSPFv2 key store.
// Related: auth_keystore.go -- configure and resolveChainKeys, which load the keys on start.
// Related: config.go -- parseKeyChain, which reads the key chain out of the config section.
//
// VALIDATES: RFC 5709 Section 3.2, "Key storage SHOULD persist across a system restart, warm
// or cold, to avoid operational issues". Ze keeps the keys in the committed configuration.
// The test writes the configuration into an on-disk config store the way a commit does
// (config.LoadConfig, the set serializer, storage.WriteCandidateVersion, then
// storage.PromoteCandidate), closes the store, opens it again from its directory as a
// restarted daemon does, reads the active config with storage.ReadActiveConfig (the read
// cmd/ze/hub/config_source.go makes at start), loads it through config.LoadConfig, and
// hands the engine its `ospf` section as the daemon does.
// PREVENTS: a restart that loses or alters the keys, so the interface either comes up
// unauthenticated or signs with a key its neighbors no longer accept.
package ospf

import (
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

const (
	// persistedKeySecret is the key the operator committed and the neighbor holds.
	persistedKeySecret = "s3cr3t-persisted"
	// persistedConfigFile is the name the committed config is stored under.
	persistedConfigFile = "ze.conf"
)

// authConfigText is an operator config whose area key chain holds one HMAC-SHA-256 key.
func authConfigText(keySecret string) string {
	return `ospf {
	router-id 1.1.1.1
	key-chains kc1 {
		key 1 {
			algorithm hmac-sha-256
			secret ` + keySecret + `
		}
	}
	areas {
		area 0.0.0.0 {
			authentication {
				key-chain kc1
			}
		}
	}
	interfaces {
		interface eth0 {
			area 0.0.0.0
			authentication {
				mode inherit
			}
		}
	}
}
`
}

// commitToStore persists text in a fresh config store under dir and closes the store. It
// loads the text with config.LoadConfig, stages what the set serializer emits as the
// candidate, and promotes it to the active config, as a commit does.
func commitToStore(t *testing.T, dir, text string) {
	t.Helper()
	loaded, err := config.LoadConfig(text, "", nil)
	if err != nil {
		t.Fatalf("load the operator config: %v", err)
	}
	schema, err := config.YANGSchemaWithPlugins(plugin.CollectPluginYANG(nil))
	if err != nil {
		t.Fatalf("build the config schema: %v", err)
	}
	committed := []byte(config.FormatSchemaStamp() + config.SerializeSet(loaded.Tree, schema))
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create the config store: %v", err)
	}
	if _, err := storage.WriteCandidateVersion(store, persistedConfigFile, committed, time.Now()); err != nil {
		t.Fatalf("stage the committed config: %v", err)
	}
	if err := storage.PromoteCandidate(store, persistedConfigFile); err != nil {
		t.Fatalf("promote the committed config: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close the config store: %v", err)
	}
}

// restartFromStore starts an engine the way a restarted daemon does: it opens the config
// store in dir, reads the active config with storage.ReadActiveConfig, loads it with config.LoadConfig, and applies
// the `ospf` section with setConfig. The caller MUST call shutdown on the engine once.
func restartFromStore(t *testing.T, dir string) *engine {
	t.Helper()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("open the config store after the restart: %v", err)
	}
	stored, err := storage.ReadActiveConfig(store, persistedConfigFile)
	if err != nil {
		t.Fatalf("read the committed config after the restart: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close the config store: %v", err)
	}
	loaded, err := config.LoadConfig(string(stored), "", nil)
	if err != nil {
		t.Fatalf("load the committed config after the restart: %v", err)
	}
	ospfTree := loaded.Tree.GetContainer(Namespace)
	if ospfTree == nil {
		t.Fatal("the committed config holds no ospf section after the restart")
	}
	data, err := json.Marshal(map[string]any{Namespace: ospfTree.ToMap()})
	if err != nil {
		t.Fatalf("marshal the ospf section: %v", err)
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: Namespace, Data: string(data)}}, nil)
	if err != nil {
		t.Fatalf("parse the ospf section after the restart: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validate the ospf section after the restart: %v", err)
	}
	eng := newEngine(transport.New(&fakeBackend{}))
	eng.setConfig(cfg)
	return eng
}

// originalKeyNeighbor returns an engine whose eth0 holds the committed key in plaintext.
func originalKeyNeighbor(t *testing.T) *engine {
	t.Helper()
	neighbor := newEngine(transport.New(&fakeBackend{}))
	t.Cleanup(neighbor.shutdown)
	neighbor.auth.configure(lifetimeAuthCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: persistedKeySecret}))
	return neighbor
}

// TestRFC5709KeysSurviveRestart checks that the committed key is the key used after a restart.
// Goal: a router restarted from its config store signs with the key it was committed with,
// so a neighbor holding that key keeps accepting its packets. Method: commit the key chain
// into an on-disk store, close it, restart an engine from the reopened store, sign a Hello
// on eth0, and verify it on a neighbor configured with the plaintext key.
func TestRFC5709KeysSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	commitToStore(t, dir, authConfigText(persistedKeySecret))
	after := restartFromStore(t, dir)
	t.Cleanup(after.shutdown)
	neighbor := originalKeyNeighbor(t)

	// RFC requirement: RFC5709-3.2-6 positive -- after the key chain is committed to an
	// on-disk config store and the engine restarts from the reopened store, eth0 signs a
	// Hello with the cryptographic AuType and Key ID 1, and a neighbor holding the committed
	// plaintext key accepts it.
	signed := after.signPacket("eth0", unsignedHello())
	if signed == nil {
		t.Fatal("the restarted engine refused to sign")
	}
	if got := packet.AuType(binary.BigEndian.Uint16(signed[14:16])); got != packet.AuTypeCryptographic {
		t.Fatalf("the restarted engine sent AuType %d, want cryptographic: its key did not survive the restart", got)
	}
	if signed[18] != 1 {
		t.Fatalf("the restarted engine signed with Key ID %d, want 1", signed[18])
	}
	if reason, ok := neighbor.auth.verify("eth0", ridOf("1.1.1.1"), [4]byte{}, signed); !ok {
		t.Fatalf("the neighbor refused the restarted engine's Hello (%s): the key changed across the restart", reason)
	}
}

// TestRFC5709AlteredStoredKeyRefused is the control for TestRFC5709KeysSurviveRestart: the
// neighbor's acceptance depends on the stored key itself. Goal: when the store holds a different key than the one the neighbor
// was given, the restarted engine's Hellos are refused, so the positive case cannot pass
// with any key that merely signs. Method: commit a key chain whose secret differs, restart
// from the reopened store, sign a Hello, and hand it to a neighbor holding the original key.
func TestRFC5709AlteredStoredKeyRefused(t *testing.T) {
	dir := t.TempDir()
	commitToStore(t, dir, authConfigText("s3cr3t-altered"))
	after := restartFromStore(t, dir)
	t.Cleanup(after.shutdown)
	neighbor := originalKeyNeighbor(t)

	// Control for TestRFC5709KeysSurviveRestart, untagged: it proves the neighbor's acceptance
	// there depends on which key signed, not that a restart can violate persistence. The
	// engine restarted from a store holding a different key signs a Hello on eth0 that the
	// neighbor refuses with reason digest-mismatch.
	signed := after.signPacket("eth0", unsignedHello())
	if signed == nil {
		t.Fatal("the restarted engine refused to sign")
	}
	if got := packet.AuType(binary.BigEndian.Uint16(signed[14:16])); got != packet.AuTypeCryptographic {
		t.Fatalf("the restarted engine sent AuType %d, want cryptographic: the stored key chain was lost", got)
	}
	reason, accepted := neighbor.auth.verify("eth0", ridOf("1.1.1.1"), [4]byte{}, signed)
	if accepted {
		t.Fatal("the neighbor accepted a Hello signed with a key it was never given")
	}
	if reason != "digest-mismatch" {
		t.Fatalf("refusal reason %q, want digest-mismatch", reason)
	}
}
