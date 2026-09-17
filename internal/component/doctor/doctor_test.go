// Design: docs/features/ai-first.md — doctor command tests

package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	plugindoctor "github.com/ze-software/ze/internal/component/plugin/doctor"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func TestMain(m *testing.M) {
	diagnostic.RegisterBuiltinCodes()
	env.MustRegister(env.EnvEntry{Key: "ze.config.dir", Type: "string", Description: "config dir"})
	os.Exit(m.Run())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

const minimalConfig = `# empty config
`

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "ze.conf")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	return f
}

func TestDoctorHelp(t *testing.T) {
	code := Run([]string{"--help"})
	assert.Equal(t, 0, code)
}

func TestDoctorMissingConfig(t *testing.T) {
	out := captureStdout(t, func() {
		code := Run([]string{"--json", "/nonexistent/ze.conf"})
		assert.Equal(t, 1, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.False(t, result.Ready)
	assert.Equal(t, diagnostic.SchemaVersion, result.SchemaVersion)

	found := false
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-config-missing" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected doctor-config-missing diagnostic")
}

func TestDoctorValidConfigJSON(t *testing.T) {
	cfgPath := writeTestConfig(t, minimalConfig)
	out := captureStdout(t, func() {
		code := Run([]string{"--json", cfgPath})
		assert.Equal(t, 0, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.True(t, result.Ready)
	assert.Equal(t, diagnostic.SchemaVersion, result.SchemaVersion)

	for i := range result.Diagnostics {
		assert.NotEqual(t, diagnostic.SeverityError, result.Diagnostics[i].Severity,
			"unexpected error: %s", result.Diagnostics[i].Message)
	}
}

func TestDoctorValidConfigText(t *testing.T) {
	cfgPath := writeTestConfig(t, minimalConfig)
	out := captureStdout(t, func() {
		code := Run([]string{cfgPath})
		assert.Equal(t, 0, code)
	})
	assert.True(t, strings.Contains(out, "all checks passed") || strings.Contains(out, "ready (0 errors"), "unexpected doctor output: %s", out)
}

func TestDoctorInvalidConfig(t *testing.T) {
	cfgPath := writeTestConfig(t, "this is not valid config {{{")
	out := captureStdout(t, func() {
		code := Run([]string{"--json", cfgPath})
		assert.Equal(t, 1, code)
	})

	var result diagnostic.DoctorResult
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.False(t, result.Ready)

	found := false
	for i := range result.Diagnostics {
		if result.Diagnostics[i].Code == "doctor-config-parse" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected doctor-config-parse diagnostic")
}

func TestDoctorExtraArg(t *testing.T) {
	code := Run([]string{"file1.conf", "file2.conf"})
	assert.Equal(t, 1, code)
}

func TestCheckPlugins_InternalSkipped(t *testing.T) {
	// A plugin declared with the `internal` keyword is already in-process:
	// no external binary is required.
	plugins := []zeplugin.PluginConfig{
		{Name: "rib", Internal: true, Run: "bgp-rib"},
	}
	diags := plugindoctor.CheckPluginBinaries(plugins)
	assert.Empty(t, diags)
}

func TestCheckPlugins_MissingBinary(t *testing.T) {
	plugins := []zeplugin.PluginConfig{
		{Name: "custom", Internal: false, Run: "/nonexistent/binary"},
	}
	diags := plugindoctor.CheckPluginBinaries(plugins)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-plugin-missing", diags[0].Code)
}

func TestRunChecksExecutesRegisteredPluginCheck(t *testing.T) {
	// VALIDATES: AC-1 runChecks executes a registered post-config plugin check through the production runner.
	// PREVENTS: plugin check migration that only registers metadata but is never reached from ze doctor.
	cfgPath := writeTestConfig(t, `plugin {
	internal rib {
		use bgp-rib
	}
}
`)

	const fixtureName = "doctor-runner-fixture"
	called := false
	require.NoError(t, diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         fixtureName,
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        700,
		Component:    "plugin",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-plugin-missing"},
		Check: func(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			tree, isTree := ctx.Tree.(*config.Tree)
			if !isTree || tree == nil || ctx.Store == nil || ctx.Platform == nil || ctx.ConfigDir == "" {
				return nil
			}
			if len(ctx.Plugins) != 1 || ctx.Plugins[0].Name != "rib" {
				return nil
			}
			called = true
			return []diagnostic.Diagnostic{{
				Code:     "doctor-plugin-missing",
				Severity: diagnostic.SeverityError,
				Message:  "registered plugin check executed",
			}}
		},
	}))
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(fixtureName) })

	diags := runChecks(cfgPath)
	assert.True(t, called, "registered plugin check did not receive parsed plugin context")
	assertDiagCode(t, diags, "doctor-plugin-missing")
}

// The systemd unit tests that sat here moved with the check to
// internal/plugins/systemd/doctor_test.go, one for one.

func assertDiagCode(t *testing.T, diags []diagnostic.Diagnostic, code string) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			return
		}
	}
	t.Fatalf("missing diagnostic %s in %#v", code, diags)
}

func TestCheckListeners_FreePort(t *testing.T) {
	tree := config.NewTree()
	diags := checkListeners(tree)
	assert.Empty(t, diags, "empty tree should produce no listener diagnostics")
}

func TestCheckListeners_PortInUse(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	web := env.GetOrCreateContainer("web")
	web.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	web.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-listen-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "web")
}

func TestCheckListeners_SSH(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "0")
	ssh.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	assert.Empty(t, diags, "port 0 should bind successfully")
}

func TestCheckListeners_API(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	api := env.GetOrCreateContainer("api-server")
	rest := api.GetOrCreateContainer("rest")
	rest.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", port)
	rest.AddListEntry("server", "s1", srv)

	diags := checkListeners(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-listen-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "api-server-rest")
}

func TestCheckListeners_BGP(t *testing.T) {
	// VALIDATES: AC-5 BGP configured with a local address reports doctor-bgp-listen when the port is unavailable.
	// PREVENTS: BGP TCP bind conflicts being hidden until reactor startup.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	conn := peer.GetOrCreateContainer("connection")
	local := conn.GetOrCreateContainer("local")
	local.Set("ip", "127.0.0.1")
	local.Set("port", port)
	remote := conn.GetOrCreateContainer("remote")
	remote.Set("ip", "192.0.2.1")
	session := peer.GetOrCreateContainer("session")
	asn := session.GetOrCreateContainer("asn")
	asn.Set("remote", "65001")
	bgp.AddListEntry("peer", "p1", peer)

	diags := checkListeners(tree)
	requireDiag(t, diags, "doctor-bgp-listen", diagnostic.SeverityWarning)
}

func TestCheckListeners_ServicePorts(t *testing.T) {
	// VALIDATES: AC-8/10/12/13/14 service listener failures use service-specific doctor codes.
	// PREVENTS: New UDP/TCP runtime dependencies falling back to an unhelpful generic code.
	oldProbe := listenerProbe
	listenerProbe = func(l serviceListener) error {
		if l.code == "doctor-bfd-port" || l.code == "doctor-ipsec-listen" || l.code == "doctor-tftp-listen" || l.code == "doctor-image-listen" || l.code == "doctor-ntp-listen" {
			return errors.New("bind failed")
		}
		return nil
	}
	t.Cleanup(func() { listenerProbe = oldProbe })

	tree := config.NewTree()
	tree.GetOrCreateContainer("bfd")
	// One site-to-site peer, not an empty block. An empty `vpn { ipsec { } }`
	// installs no Security Association, so ze binds neither IKE port for it and
	// doctor probes neither (AC-11, owner decision 6).
	ipsecPeer := config.NewTree()
	ipsecPeer.Set("remote-address", "203.0.113.7")
	tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec").
		GetOrCreateContainer("site-to-site").AddListEntry("peer", "branch", ipsecPeer)
	service := tree.GetOrCreateContainer("service")
	tftp := service.GetOrCreateContainer("tftp-server")
	tftp.Set("enabled", "true")
	image := service.GetOrCreateContainer("image-server")
	image.Set("enabled", "true")
	env := tree.GetOrCreateContainer("environment")
	ntp := env.GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")

	diags := checkListeners(tree)
	requireDiag(t, diags, "doctor-bfd-port", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-ipsec-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-tftp-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-image-listen", diagnostic.SeverityWarning)
	requireDiag(t, diags, "doctor-ntp-listen", diagnostic.SeverityWarning)
}

type stubStorage struct {
	storage.Storage
	data map[string][]byte
	// unreadable names files that exist in storage and fail to read, the state
	// a corrupt or unreadable file reaches. Exists says yes, ReadFile says no.
	unreadable map[string]error
}

func (s *stubStorage) ReadFile(name string) ([]byte, error) {
	if err, ok := s.unreadable[name]; ok {
		return nil, err
	}
	if d, ok := s.data[name]; ok {
		return d, nil
	}
	return s.Storage.ReadFile(name)
}

func (s *stubStorage) Exists(name string) bool {
	if _, ok := s.data[name]; ok {
		return true
	}
	if _, ok := s.unreadable[name]; ok {
		return true
	}
	return s.Storage.Exists(name)
}

func TestResolveDefaultConfig_NoInstanceFile(t *testing.T) {
	store := doctorTestStore(t)
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "ze.conf", name, "filesystem storage with no instance file should return ze.conf")
}

func TestResolveDefaultConfig_InvalidRegex(t *testing.T) {
	store := &stubStorage{
		Storage: doctorTestStore(t),
		data:    map[string][]byte{zefs.KeyInstanceName.Pattern: []byte("../etc")},
	}
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "ze.conf", name, "instance name failing regex should return ze.conf")
}

func TestResolveDefaultConfig_ValidName(t *testing.T) {
	store := &stubStorage{
		Storage: doctorTestStore(t),
		data:    map[string][]byte{zefs.KeyInstanceName.Pattern: []byte("myrouter")},
	}
	name := resolve.DefaultConfig(store)
	assert.Equal(t, "myrouter.conf", name)
}

func TestCollectSchemaListeners_SSHDefault(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")

	listeners := collectSchemaListeners(tree)
	found := false
	for _, l := range listeners {
		if l.service == "ssh" {
			found = true
			assert.Equal(t, "127.0.0.1", l.host)
			assert.Equal(t, "2222", l.port)
			break
		}
	}
	assert.True(t, found, "expected ssh listener from fallback collection")
}

func TestCollectSchemaListeners_SSHExplicit(t *testing.T) {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ssh := env.GetOrCreateContainer("ssh")
	ssh.Set("enabled", "true")
	srv := config.NewTree()
	srv.Set("ip", "10.0.0.1")
	srv.Set("port", "2223")
	ssh.AddListEntry("server", "s1", srv)

	listeners := collectSchemaListeners(tree)
	found := false
	for _, l := range listeners {
		if strings.Contains(l.service, "ssh") && l.port == "2223" {
			found = true
			assert.Equal(t, "10.0.0.1", l.host)
			break
		}
	}
	assert.True(t, found, "expected explicit ssh listener from fallback collection")
}

func TestResolveStorageWithDiag_ReadOnly(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Create(dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, owner.Close()) })
	store, diags := resolveStorageWithDiag(filepath.Join(dir, "ze.conf"))
	require.NotNil(t, store)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	assert.Empty(t, diags)
	assert.ErrorIs(t, store.WriteKey("meta/test", []byte("refused")), storage.ErrReadOnly)
}

func requireDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) {
	t.Helper()
	for i := range diags {
		if diags[i].Code == code {
			assert.Equal(t, severity, diags[i].Severity, "severity for %s", code)
			return
		}
	}
	require.Failf(t, "missing diagnostic", "expected %s in %+v", code, diags)
}

func assertNoDiagCode(t *testing.T, diags []diagnostic.Diagnostic, code string) {
	t.Helper()
	for i := range diags {
		assert.NotEqual(t, code, diags[i].Code, "unexpected diagnostic: %+v", diags[i])
	}
}

func testPlatform(platformType host.PlatformType) *host.PlatformInfo {
	return &host.PlatformInfo{Type: platformType}
}

func ntpTree(enabled bool) *config.Tree {
	tree := config.NewTree()
	envTree := tree.GetOrCreateContainer("environment")
	ntp := envTree.GetOrCreateContainer("ntp")
	if enabled {
		ntp.Set("enabled", "true")
	}
	return tree
}

func ntpPersistTree(path string) *config.Tree {
	tree := ntpTree(true)
	ntp := getContainerPath(tree, "environment", "ntp")
	ntp.Set("persist-path", path)
	return tree
}

func withWritableProbe(t *testing.T, fn func(string) error) {
	t.Helper()
	oldProbeWritable := probeWritable
	probeWritable = fn
	t.Cleanup(func() { probeWritable = oldProbeWritable })
}

func TestResolveDoctorPlatformReturnsPlatformInfo(t *testing.T) {
	// VALIDATES: resolveDoctorPlatform returns a usable PlatformInfo and no
	// diagnostic when detection succeeds.
	// PREVENTS: runChecks discarding platform context before the phase
	// dispatch filters on it.
	require.NoError(t, env.Set(doctorPlatformEnv, "systemd"))
	t.Cleanup(func() { _ = env.Set(doctorPlatformEnv, "") })

	platform, diags := resolveDoctorPlatform()

	require.NotNil(t, platform)
	assert.Equal(t, host.PlatformSystemd, platform.Type)
	assert.Empty(t, diags)
}

func TestResolveDoctorPlatformReportsDetectionFailure(t *testing.T) {
	// VALIDATES: a platform that cannot be resolved is a nil platform beside
	// doctor-platform-detect, so the runner still runs the wildcard checks and
	// the operator learns why the platform-gated ones did not run.
	// PREVENTS: a detection failure read as "no platform to judge" in silence.
	require.NoError(t, env.Set(doctorPlatformEnv, "no-such-platform"))
	t.Cleanup(func() { _ = env.Set(doctorPlatformEnv, "") })

	platform, diags := resolveDoctorPlatform()

	assert.Nil(t, platform)
	requireDiag(t, diags, diagnostic.CodeDoctorPlatformDetect, diagnostic.SeverityWarning)
}

func TestCheckPlatformJudgesTheResolvedPlatform(t *testing.T) {
	// VALIDATES: the registered platform check emits doctor-platform-unknown
	// for an unidentified platform, doctor-platform-perm as an error for a
	// gokrazy whose /perm is not writable, doctor-platform-container-ro for a
	// container with a read-only root, and nothing for a systemd host or for
	// the nil platform the runner already reported.
	// PREVENTS: the split from the platform resolver dropping one of the three
	// judgements the runner used to make in the same call.
	cases := []struct {
		name     string
		platform *host.PlatformInfo
		code     string
		severity diagnostic.Severity
	}{
		{"unknown", &host.PlatformInfo{Type: host.PlatformUnknown}, diagnostic.CodeDoctorPlatformUnknown, diagnostic.SeverityWarning},
		{"gokrazy perm", &host.PlatformInfo{Type: host.PlatformGokrazy}, diagnostic.CodeDoctorPlatformPerm, diagnostic.SeverityError},
		{"container ro", &host.PlatformInfo{Type: host.PlatformContainer, ReadOnlyRoot: true}, diagnostic.CodeDoctorPlatformContainerRO, diagnostic.SeverityWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := checkPlatform(diagnostic.DoctorCheckContext{Platform: tc.platform})
			require.Len(t, diags, 1)
			requireDiag(t, diags, tc.code, tc.severity)
		})
	}

	assert.Empty(t, checkPlatform(diagnostic.DoctorCheckContext{Platform: testPlatform(host.PlatformSystemd)}))
	assert.Empty(t, checkPlatform(diagnostic.DoctorCheckContext{Platform: &host.PlatformInfo{Type: host.PlatformGokrazy, PersistentStorageWritable: true}}))
	assert.Empty(t, checkPlatform(diagnostic.DoctorCheckContext{Platform: testPlatform(host.PlatformContainer)}))
	assert.Empty(t, checkPlatform(diagnostic.DoctorCheckContext{}))
}

func TestCheckPersistPathMismatchSystemd(t *testing.T) {
	// VALIDATES: AC-8 /perm/ze/timefile on systemd emits doctor-config-platform-mismatch.
	// PREVENTS: gokrazy persistence defaults being silently used on standard Linux.
	withWritableProbe(t, func(string) error { return nil })

	diags := checkWritableDestinations(ntpPersistTree("/perm/ze/timefile"), testPlatform(host.PlatformSystemd))

	requireDiag(t, diags, "doctor-config-platform-mismatch", diagnostic.SeverityWarning)
}

func TestCheckPersistPathMatchGokrazy(t *testing.T) {
	// VALIDATES: AC-9 /perm/ze/timefile on gokrazy emits no mismatch diagnostic.
	// PREVENTS: appliance defaults being reported as wrong on appliances.
	withWritableProbe(t, func(string) error { return nil })

	diags := checkWritableDestinations(ntpPersistTree("/perm/ze/timefile"), testPlatform(host.PlatformGokrazy))

	assertNoDiagCode(t, diags, "doctor-config-platform-mismatch")
}

func TestCheckCoherenceNilPlatform(t *testing.T) {
	// VALIDATES: AC-15 nil platform preserves current behavior without new coherence diagnostics.
	// PREVENTS: platform detection failures from crashing or inventing platform-specific warnings.
	withWritableProbe(t, func(string) error { return nil })

	var diags []diagnostic.Diagnostic
	diags = append(diags, checkWritableDestinations(ntpPersistTree("/perm/ze/timefile"), nil)...)
	diags = append(diags, checkMachineID(nil, nil)...)
	diags = append(diags, checkRandomSeed(nil)...)

	assertNoDiagCode(t, diags, "doctor-config-platform-mismatch")
	assertNoDiagCode(t, diags, "doctor-machine-id-missing")
	assertNoDiagCode(t, diags, "doctor-random-seed")
}

// --- Disk space tests ---

func TestCheckDiskSpace_ReturnsNilOnWorkingFilesystem(t *testing.T) {
	diags := checkDiskSpace()
	assert.Empty(t, diags)
}

// --- DNS resolver tests ---

// --- Store integrity code registration test ---

func TestDoctorStoreIntegrityCodeRegistered(t *testing.T) {
	meta := diagnostic.Lookup("doctor-store-integrity")
	require.NotNil(t, meta, "doctor-store-integrity code must be registered")
	assert.NotEmpty(t, meta.Title)
}

// pinConfigDir points ze.config.dir at dir for the duration of the test.
func pinConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", orig) })
	require.NoError(t, env.Set("ze.config.dir", dir))
}

// VALIDATES: checkStoreIntegrity reads the store at ze.config.dir and reports
// corruption found there.
// PREVENTS: the silent skip where checkStoreIntegrity resolved the store from the
// binary location, os.Stat missed the operator's real store, and it returned nil --
// so ze doctor reported a healthy store it had never opened. Reachable in production
// via `ze install systemd --config <dir>`, which pins ZE_CONFIG_DIR in the generated
// unit (internal/plugins/systemd/unit.go) while the binary sits in a standard prefix.
func TestCheckStoreIntegrity_HonorsConfigDirEnv(t *testing.T) {
	dir := t.TempDir()
	// A damaged frame proves the configured tree was visited.
	store, err := storage.Create(dir)
	require.NoError(t, err)
	require.NoError(t, store.WriteKey("meta/test", []byte("good")))
	require.NoError(t, store.Close())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "database", "meta", "test"), []byte("not a valid frame"), 0o600))
	pinConfigDir(t, dir)

	diags := checkStoreIntegrity("")

	require.Len(t, diags, 1, "corrupt store at ze.config.dir must produce a diagnostic, not a silent skip")
	assert.Equal(t, "doctor-store-integrity", diags[0].Code)
	assert.Contains(t, diags[0].Message, "meta/test")
	assert.Equal(t, filepath.Join(dir, "database", "meta", "test"), diags[0].Path)
}

// VALIDATES: checkStoreIntegrity stays silent when the pinned dir holds no store.
// PREVENTS: ze doctor reporting a spurious integrity error on a host that has not
// run ze init yet -- absence is not corruption.
func TestCheckStoreIntegrity_NoStoreIsSilent(t *testing.T) {
	pinConfigDir(t, t.TempDir())

	assert.Empty(t, checkStoreIntegrity(""), "absence is reported by the open diagnostic, not corruption")
}

// notADirConfigDir pins ze.config.dir BELOW a regular file and returns that path.
//
// Both syscalls under test then fail ENOTDIR. That is a genuine "cannot check",
// and it is not os.ErrNotExist: Go maps ErrNotExist to ENOENT alone
// (syscall.Errno.Is). The absence branch therefore cannot swallow it.
//
// The path must be a CHILD of the file, never the file itself. statfs(2)
// resolves the filesystem that CONTAINS its argument, so it succeeds on a
// regular file. The first version of this helper pinned the file, and
// checkDiskSpace measured the real tmpfs and reported healthy. Only a
// non-directory component mid-path fails for os.Stat and syscall.Statfs alike.
//
// A chmod-0 parent was the other option. It stays readable for root, so the
// assertions would go vacuous exactly where they matter most.
func notADirConfigDir(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(file, []byte("regular file"), 0o600))
	p := filepath.Join(file, "config")
	pinConfigDir(t, p)
	return p
}

// VALIDATES: a stat error that is NOT "file absent" produces a diagnostic naming
// the path, rather than the healthy verdict an empty result means.
// PREVENTS: the fail-open branch where ANY os.Stat error returned nil, so a store
// ze doctor could not read (unreadable config dir, ENOTDIR, EACCES, I/O error) was
// reported as healthy. Absence is not corruption, which the test above pins, but
// "I could not look" is not health either: a guard that cannot deny must say so
// (ai/rules/evidence.md).
func TestCheckStoreIntegrity_UnreadableStoreIsReported(t *testing.T) {
	notADir := notADirConfigDir(t)

	diags := checkStoreIntegrity("")

	require.Len(t, diags, 1, "an unreadable store must be reported, not treated as healthy")
	assert.Equal(t, "doctor-store-integrity", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, notADir, "the message must name the path that could not be read")
}

func doctorTestStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

// A missing or unsafe store must have a named diagnosis without creating one.
func TestStorageOpenDiagnostics(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ze.conf")
	store, diags := resolveStorageWithDiag(configPath)
	require.Nil(t, store)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-storage-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "ze init")
	_, err := os.Stat(filepath.Join(dir, "database"))
	require.True(t, os.IsNotExist(err))

	owner, err := storage.Create(dir)
	require.NoError(t, err)
	require.NoError(t, owner.Close())
	require.NoError(t, os.Chmod(filepath.Join(dir, "database"), 0o755))
	store, diags = resolveStorageWithDiag(configPath)
	require.Nil(t, store)
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorStorePermissions, diags[0].Code)
	assert.Contains(t, diags[0].Message, filepath.Join(dir, "database"))
	assert.Contains(t, diags[0].Message, "755")
}

// VALIDATES: checkDiskSpace stays silent when the config dir does not exist.
// PREVENTS: ze doctor reporting spurious disk errors on a host that has not run
// ze init -- the same "absence is not a fault" rule checkStoreIntegrity follows.
func TestCheckDiskSpace_MissingDirIsSilent(t *testing.T) {
	pinConfigDir(t, filepath.Join(t.TempDir(), "does-not-exist"))

	assert.Empty(t, checkDiskSpace(), "absent config dir must report nothing")
}

// VALIDATES: a Statfs error that is NOT "file absent" produces a diagnostic.
// PREVENTS: the sibling fail-open of checkStoreIntegrity's -- every syscall.Statfs
// failure returned nil, so ze doctor reported free space it never measured
// (ai/rules/evidence.md, ai/rules/architecture.md sibling audit).
func TestCheckDiskSpace_UnreadableDirIsReported(t *testing.T) {
	notADir := notADirConfigDir(t)

	diags := checkDiskSpace()

	require.Len(t, diags, 1, "an unreadable config dir must be reported, not treated as healthy")
	assert.Equal(t, "doctor-disk-space", diags[0].Code)
	assert.Contains(t, diags[0].Message, notADir, "the message must name the path that could not be measured")
}

func TestDoctorCoverageCodesRegistered(t *testing.T) {
	// VALIDATES: AC-17 every new doctor coverage diagnostic code is registered for ze explain.
	// PREVENTS: ze doctor emitting codes that ze explain cannot describe.
	for _, code := range []string{
		"doctor-l2tp-module",
		"doctor-pppoe-module",
		"doctor-firewall-nftables",
		"doctor-dhcp-iface",
		"doctor-bgp-listen",
		"doctor-tacacs-unreachable",
		"doctor-radius-unreachable",
		"doctor-radius-admin-unreachable",
		"doctor-bfd-port",
		"doctor-pki-cert",
		"doctor-ipsec-listen",
		"doctor-telemetry-procfs",
		"doctor-tftp-listen",
		"doctor-image-listen",
		"doctor-ntp-listen",
		"doctor-sysctl-procfs",
		"doctor-conntrack-procfs",
		"doctor-policyroute-netlink",
	} {
		meta := diagnostic.Lookup(code)
		require.NotNil(t, meta, "%s code must be registered", code)
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	}
}

func TestDoctorImprovementsCodesRegistered(t *testing.T) {
	// VALIDATES: AC-13 every new doctor-improvements diagnostic code is registered.
	for _, code := range []string{
		"doctor-bgp-md5",
		"doctor-ntp-server-unreachable",
		"doctor-clock-no-sync",
		"doctor-rpki-unreachable",
		"doctor-bmp-unreachable",
		"doctor-write-destination",
		"doctor-config-platform-mismatch",
		"doctor-machine-id-missing",
	} {
		meta := diagnostic.Lookup(code)
		require.NotNil(t, meta, "%s code must be registered", code)
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	}
}

func TestDoctorWritableDestinations(t *testing.T) {
	// VALIDATES: AC-11 writable file destinations.
	origProbeWritable := probeWritable
	defer func() { probeWritable = origProbeWritable }()
	probeWritable = func(string) error { return errors.New("no such directory") }

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ntp := env.GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")
	ntp.Set("persist-path", "/perm/ze/timefile")
	bfd := tree.GetOrCreateContainer("bfd")
	bfd.Set("persist-dir", "/perm/bfd")

	diags := checkWritableDestinations(tree, nil)
	var codes []string
	for i := range diags {
		codes = append(codes, diags[i].Code)
	}
	assert.Contains(t, codes, "doctor-write-destination")
	assert.GreaterOrEqual(t, len(diags), 2, "expected at least NTP persist + BFD persist diagnostics")
}

func TestDoctorWritableDestinations_DNSResolvConf(t *testing.T) {
	origProbeWritable := probeWritable
	defer func() { probeWritable = origProbeWritable }()
	probeWritable = func(string) error { return errors.New("permission denied") }

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	dns := system.GetOrCreateContainer("dns")
	dns.Set("resolv-conf-path", "/nonexistent/resolv.conf")

	diags := checkWritableDestinations(tree, nil)
	found := false
	for i := range diags {
		if diags[i].Code == "doctor-write-destination" && strings.Contains(diags[i].Message, "DNS resolv-conf-path") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected DNS resolv-conf-path diagnostic")
}

func TestDoctorWritableDestinations_ArchiveFile(t *testing.T) {
	origProbeWritable := probeWritable
	defer func() { probeWritable = origProbeWritable }()
	probeWritable = func(string) error { return errors.New("no such directory") }

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	arch := config.NewTree()
	arch.Set("location", "file:///nonexistent/backup")
	system.AddListEntry("archive", "local-backup", arch)

	diags := checkWritableDestinations(tree, nil)
	found := false
	for i := range diags {
		if diags[i].Code == "doctor-write-destination" && strings.Contains(diags[i].Message, "archive") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected archive file location diagnostic")
}

func TestDoctorWritableDestinations_SelfUpdate(t *testing.T) {
	origProbeWritable := probeWritable
	defer func() { probeWritable = origProbeWritable }()
	probeWritable = func(string) error { return errors.New("read-only filesystem") }

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	uc := system.GetOrCreateContainer("update-check")
	uc.Set("url", "https://update.example.com/version.json")
	uc.Set("auto-apply", "true")

	diags := checkWritableDestinations(tree, nil)
	found := false
	for i := range diags {
		if diags[i].Code == "doctor-write-destination" && strings.Contains(diags[i].Message, "self-update") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected self-update auto-apply writable diagnostic")
}

func TestDoctorGokrazySkipsWritable(t *testing.T) {
	// VALIDATES: AC-9 ze doctor on gokrazy with auto-apply skips writable-binary warning.
	// PREVENTS: gokrazy appliances warning about Ze binary replacement when gokrazy owns image updates.
	withWritableProbe(t, func(string) error { return errors.New("read-only filesystem") })

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	uc := system.GetOrCreateContainer("update-check")
	uc.Set("url", "https://update.example.com/version.json")
	uc.Set("auto-apply", "true")

	diags := checkWritableDestinations(tree, testPlatform(host.PlatformGokrazy))
	for _, diag := range diags {
		if diag.Code == "doctor-write-destination" && strings.Contains(diag.Message, "self-update") {
			t.Fatalf("unexpected self-update writable diagnostic on gokrazy: %+v", diag)
		}
	}
}

func TestDoctorLinuxWritableUnchanged(t *testing.T) {
	// VALIDATES: AC-11 ze doctor on plain Linux with auto-apply keeps the existing writable-binary check.
	// PREVENTS: the gokrazy skip from disabling the normal Linux self-update readiness warning.
	withWritableProbe(t, func(string) error { return errors.New("read-only filesystem") })

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	uc := system.GetOrCreateContainer("update-check")
	uc.Set("url", "https://update.example.com/version.json")
	uc.Set("auto-apply", "true")

	diags := checkWritableDestinations(tree, testPlatform(host.PlatformPlainLinux))
	found := false
	for _, diag := range diags {
		if diag.Code == "doctor-write-destination" && strings.Contains(diag.Message, "self-update") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected self-update writable diagnostic on plain Linux")
}

func TestDoctorImprovementsCodesRegistered_Extended(t *testing.T) {
	for _, code := range []string{
		"doctor-bgp-md5",
		"doctor-ntp-server-unreachable",
		"doctor-clock-no-sync",
		"doctor-rpki-unreachable",
		"doctor-bmp-unreachable",
		"doctor-write-destination",
		"doctor-config-platform-mismatch",
		"doctor-machine-id-missing",
		"doctor-ntp-clock-privilege",
		"doctor-vpp-dpdk",
		"doctor-update-check-unreachable",
		"doctor-archive-unreachable",
		"doctor-random-seed",
	} {
		meta := diagnostic.Lookup(code)
		require.NotNil(t, meta, "%s code must be registered", code)
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
	}
}

// doctorDependencyCovered maps a runtime dependency ze doctor checks to the
// diagnostic code that reports it. TestDoctorProbesEveryCoveredListener reads the
// schema-listener rows and proves each one names a probe that really runs, so a
// row here cannot outlive the registration that produces its endpoint.
var doctorDependencyCovered = map[string]string{
	"listener/web":             "doctor-listen-unavailable",
	"listener/gnmi":            "doctor-listen-unavailable",
	"listener/looking-glass":   "doctor-listen-unavailable",
	"listener/api-server-rest": "doctor-listen-unavailable",
	"listener/api-server-grpc": "doctor-listen-unavailable",
	"listener/ssh":             "doctor-listen-unavailable",
	"listener/bgp":             "doctor-bgp-listen",
	"listener/bfd":             "doctor-bfd-port",
	"listener/ipsec":           "doctor-ipsec-listen",
	"listener/tftp":            "doctor-tftp-listen",
	"listener/image-server":    "doctor-image-listen",
	"listener/ntp":             "doctor-ntp-listen",
	// The Prometheus exporter endpoint under the name the schema-FALLBACK path
	// gives it (extractTelemetryListeners, reached only when the YANG schema
	// carries no ze:listener service). The schema path probes the same endpoint
	// as listener/prometheus, so the two rows are one dependency seen from the
	// two collection paths, not two listeners.
	"listener/telemetry":    "doctor-listen-unavailable",
	"external/tacacs":       "doctor-tacacs-unreachable",
	"external/radius":       "doctor-radius-unreachable",
	"external/radius-admin": "doctor-radius-admin-unreachable",
	"external/rpki":         "doctor-rpki-unreachable",
	"external/bmp":          "doctor-bmp-unreachable",
	"external/ntp-server":   "doctor-ntp-server-unreachable",
	"external/update-check": "doctor-update-check-unreachable",
	"external/archive-http": "doctor-archive-unreachable",
	"external/dns":          "doctor-dns-resolver",
	"writable/ntp-persist":  "doctor-write-destination",
	"writable/bfd-persist":  "doctor-write-destination",
	"writable/dns-resolv":   "doctor-write-destination",
	"writable/archive-file": "doctor-write-destination",
	"writable/self-update":  "doctor-write-destination",
	"module/l2tp":           "doctor-l2tp-module",
	"module/pppoe":          "doctor-pppoe-module",
	"module/ipsec":          "doctor-module-missing",
	"procfs/mpls":           "doctor-mpls-unavailable",
	"netlink/xfrm":          "doctor-ipsec-xfrm-unavailable",
	"module/nftables":       "doctor-firewall-nftables",
	"module/vfio":           "doctor-vpp-dpdk",
	"socket/vpp":            "doctor-vpp-unreachable",
	"binary/plugin":         "doctor-plugin-missing",
	// The shell is the second binary an external plugin start needs: the run
	// string names the first, and the shell is what the run string is given to.
	"binary/plugin-shell":     "doctor-plugin-shell-missing",
	"binary/vpp":              "doctor-vpp-version",
	"cert/tls":                "doctor-tls-missing",
	"cert/pki":                "doctor-pki-cert",
	"cert/ssh":                "doctor-ssh-hostkey-missing",
	"privilege/ntp":           "doctor-ntp-clock-privilege",
	"sysfs/dpdk":              "doctor-vpp-dpdk",
	"procfs/telemetry":        "doctor-telemetry-procfs",
	"procfs/sysctl":           "doctor-sysctl-procfs",
	"procfs/conntrack":        "doctor-conntrack-procfs",
	"netlink/policyroute":     "doctor-policyroute-netlink",
	"config/bgp-md5":          "doctor-bgp-md5",
	"coherence/clock-sync":    "doctor-clock-no-sync",
	"coherence/machine-id":    "doctor-machine-id-missing",
	"coherence/platform-path": "doctor-config-platform-mismatch",
	"coherence/random-seed":   "doctor-random-seed",
	"config/references":       "doctor-config-reference",
	"config/semantic":         "config-mcp-invalid",

	// prometheus is the telemetry exporter's own ze:listener service, and it
	// carries a registered default (internal/component/config/listener_defaults.go),
	// so the schema path probes it as well as the Go extractor that produces
	// listener/telemetry.
	"listener/prometheus": "doctor-listen-unavailable",

	// extractMCPBlock (internal/component/config/loader_extract.go) applies the
	// YANG refine default of 8080 to every server entry that names no port, and
	// synthesizes 127.0.0.1:8080 for an empty list, so every enabled mcp block
	// yields an endpoint to probe.
	"listener/mcp": "doctor-listen-unavailable",

	// l2tp starts one listener per server entry and applies its own
	// DefaultListenPort to an entry that omits the port
	// (internal/component/l2tp/config.go, ParseParameters), which
	// RegisterListenerEntryDefault mirrors, so every config that starts an L2TP
	// listener yields an endpoint to probe.
	"listener/l2tp": "doctor-listen-unavailable",

	// geodns binds 127.0.0.1:5300 AND ::1:5300 for an empty listener list
	// (parseListeners, internal/plugins/geodns/config.go), so its plugin
	// registers both as listener defaults and checkListeners probes each. Its own
	// check, checkGeoDNSListenCapability, adds the privileged-port case that the
	// bind probe cannot distinguish from a busy port; it is silent at or above
	// 1024, which is why the row below and not that check is the coverage.
	"listener/service-geodns-listener": "doctor-listen-unavailable",

	// as112 registers its own listener defaults in
	// internal/plugins/as112/register.go, so CollectListenersWithDefaults
	// yields an endpoint for both families and checkListeners probes them.
	"listener/service-as112-ipv4-anycast-listener": "doctor-listen-unavailable",
	"listener/service-as112-ipv6-anycast-listener": "doctor-listen-unavailable",
}

// doctorDependencyExcluded names a ze:listener service ze doctor emits NO
// diagnostic for when the operator relies on the service's default endpoint. The
// reason is read off the service's OWN config extraction, because that is what
// decides whether a default endpoint exists at all: a service that binds nothing
// on an empty list, or that hands the kernel an ephemeral port, has no endpoint
// for doctor to probe and is not a coverage gap.
//
// It is not "the doctor cannot probe this service": every one of these is probed
// from a config that names both ip and port (config.CollectListeners).
var doctorDependencyExcluded = map[string]string{
	"listener/wireguard": "buildWireguardConfig (internal/plugins/iface/netlink/wireguard_linux.go) sends no ListenPort unless the interface names one, so the kernel picks the port and there is no endpoint to probe",
	"listener/plugin-hub": "extractHubServerConfig (internal/component/config/loader_extract.go) reads ip and port verbatim and fills in no default, " +
		"so an entry without a port carries none, and an empty server list starts no hub server",
	"listener/bmp": "(*BMPPlugin).startReceiver (internal/component/bgp/plugins/bmp/bmp.go) joins ip and port verbatim, so an entry without a port binds an ephemeral one, " +
		"and it iterates cfg.Servers so an empty list starts nothing at all",
}

// listenerProbeNetworks records the transports each schema-declared listener
// service BINDS, read off the code that binds them. It is deliberately
// independent of config.RegisterListenerProtocols: an expectation derived from
// the thing under test asserts nothing, and this class of defect is exactly a
// registration that says TCP for a service that binds UDP.
//
// Every service the probe assertion exercises needs a row, so adding a
// ze:listener service means reading its binder rather than accepting whatever
// the list shape implies.
var listenerProbeNetworks = map[string][]string{
	// Go http.Server / grpc.Serve over a net.Listener: TCP.
	"web":             {"tcp"},
	"gnmi":            {"tcp"},
	"mcp":             {"tcp"}, // startMCPServer (cmd/ze/hub/service_mcp.go) calls lc.Listen with "tcp"
	"looking-glass":   {"tcp"},
	"api-server-rest": {"tcp"},
	"api-server-grpc": {"tcp"},
	"prometheus":      {"tcp"},
	"ssh":             {"tcp"},
	// (*UDPListener).Start (internal/component/l2tp/listener.go) binds with
	// ListenPacket and asserts *net.UDPConn. UDP only.
	"l2tp": {"udp"},
	// dnsserver.Manager.bind (internal/core/dnsserver/manager.go) takes a udp
	// PacketConn AND a tcp Listener for every endpoint, and fails the endpoint if
	// either fails. Both must be probed.
	"service-geodns-listener":             {"udp", "tcp"},
	"service-as112-ipv4-anycast-listener": {"udp", "tcp"},
	"service-as112-ipv6-anycast-listener": {"udp", "tcp"},
}

// goExtractorListeners names the listener/ inventory rows that come from a
// hand-written extractor in checks_listener.go rather than from a ze:listener in
// the schema. No registry produces them, so this is the one place they are
// listed, and it is what lets the schema own every other row: a listener/ row in
// neither this set nor the schema is a row nothing exercises.
//
// TestGoExtractorListenersStillProduceProbes binds these seven names to the
// extractors, so the set cannot outlive them. Without that, deleting
// extractBFDListeners would leave listener/bfd in the inventory and in this map,
// skipped by both loops and claiming coverage that no longer exists.
var goExtractorListeners = map[string]bool{
	"listener/bgp":          true, // extractBGPListeners
	"listener/bfd":          true, // extractBFDListeners
	"listener/ipsec":        true, // extractIPsecListeners
	"listener/tftp":         true, // extractTFTPListeners
	"listener/image-server": true, // extractImageListeners
	"listener/ntp":          true, // extractNTPListeners
	"listener/telemetry":    true, // extractTelemetryListeners
}

// goExtractorListenerTree turns on every service the Go extractors in
// checks_listener.go read, in one config. They live under separate containers, so
// one tree drives all of them and no service masks another.
func goExtractorListenerTree() *config.Tree {
	tree := config.NewTree()

	bgpC := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	local := config.NewTree()
	local.Set("ip", "127.0.0.1")
	peer.GetOrCreateContainer("connection").SetContainer("local", local)
	bgpC.AddListEntry("peer", "p1", peer)

	tree.GetOrCreateContainer("bfd").Set("enabled", "true")

	// extractIPsecListeners asks kernelcap.IPsecInUse, which reads a TUNNEL and
	// not an `enabled` leaf (owner decision 6, 2026-08-14: one predicate, three
	// readers). An empty `vpn { ipsec { } }` describes no tunnel, so ze binds
	// neither UDP port and doctor probes neither. One site-to-site peer is the
	// smallest config that makes the daemon bind 500 and 4500.
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	ipsecPeer := config.NewTree()
	ipsecPeer.Set("remote-address", "198.51.100.1")
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", "s1", ipsecPeer)

	svc := tree.GetOrCreateContainer("service")
	svc.GetOrCreateContainer("tftp-server").Set("enabled", "true")
	svc.GetOrCreateContainer("image-server").Set("enabled", "true")
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("ntp").Set("enabled", "true")

	// Read by extractTelemetryListeners and extractSSHListeners, which run only
	// on the schema-FALLBACK path (collectHardcodedListeners). The same
	// containers feed the schema path under different service names, so one tree
	// drives both.
	tree.GetOrCreateContainer("telemetry").GetOrCreateContainer("prometheus").Set("enabled", "true")
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("ssh").Set("enabled", "true")

	return tree
}

// TestGoExtractorListenersStillProduceProbes binds every goExtractorListeners
// name to the extractor that produces it.
//
// The schema owns every other listener row, so those cannot go stale. These seven
// are hand-written on both sides -- an extractor in checks_listener.go and a name
// here -- and nothing connected them: deleting extractBFDListeners left
// listener/bfd sitting in the inventory and in the skip set, claiming a probe
// that no longer ran, and both inventory loops stepped over it.
//
// VALIDATES: each name in goExtractorListeners is produced by a live extractor.
// PREVENTS: an inventory row outliving the code that produces its endpoint, in
// the one corner the schema cannot police.
func TestGoExtractorListenersStillProduceProbes(t *testing.T) {
	tree := goExtractorListenerTree()
	services := map[string]bool{}
	// Both paths, because they do not produce the same set.
	// collectHardcodedListeners is the schema-FALLBACK, and it owns
	// extractTelemetryListeners and extractSSHListeners; the schema path reaches
	// the same two containers under the service names the YANG derives
	// ("prometheus", "ssh"). A row bound to one path is not bound by the other.
	all := append(collectAllListeners(tree), collectHardcodedListeners(tree)...)
	for _, l := range all {
		// Endpoint names carry a list key for some services ("bgp" does not, but
		// a schema listener would); take the leading word.
		services[strings.Fields(l.service)[0]] = true
	}
	for dep := range goExtractorListeners {
		name := strings.TrimPrefix(dep, "listener/")
		assert.Truef(t, services[name],
			"inventory row %q names no listener any extractor in checks_listener.go produces; either its extractor is gone or the config that drives it changed", dep)
	}
}

// listenerEntryWithoutPortTree builds the config shape that sits between "all
// defaults" and "fully spelled out": the service is on, and its one list entry
// names an ip and omits the port.
//
// The path comes from the SCHEMA (ListenerService.Containers and ListName), not
// from a table here. A hand-written path table is a second copy of what the
// schema already states, and it goes stale the first time a YANG list moves.
func listenerEntryWithoutPortTree(svc config.ListenerService) *config.Tree {
	tree := config.NewTree()
	container := tree
	for _, name := range svc.Containers {
		container = container.GetOrCreateContainer(name)
	}
	// Every listener service whose schema declares an enabled leaf is gated on
	// it; setting it on a service without one is inert (CollectListeners reads
	// the leaf only when ListenerService.HasEnabledLeaf).
	container.Set("enabled", "true")
	entry := config.NewTree()
	entry.Set("ip", "127.0.0.1")
	container.AddListEntry(svc.ListName, "probe", entry)
	return tree
}

func TestDoctorDependencyInventory(t *testing.T) {
	for dep, code := range doctorDependencyCovered {
		meta := diagnostic.Lookup(code)
		assert.NotNilf(t, meta, "dependency %s maps to unregistered code %s", dep, code)
	}

	// The listener half of the inventory is DERIVED from the schema, not counted.
	// checkListeners probes one endpoint per ze:listener service
	// (collectSchemaListeners -> config.DiscoverListenerServices), so a service the
	// YANG gains and nobody lists is a dependency ze checks and this inventory
	// hides. expectedTotal below cannot see that: it folds the same two maps it
	// is compared against, so it only reacts to an edit of those maps. It stays
	// as the conscious-handling ratchet for the rows no registry produces --
	// external, writable, module, and the protocol listeners that come from the
	// Go extractors in checks_listener.go rather than from the schema.
	schema, err := config.YANGSchema()
	require.NoError(t, err, "doctor probes schema-discovered listeners, so the schema must load")
	schemaListeners := map[string]bool{}
	for _, svc := range config.DiscoverListenerServices(schema) {
		dep := "listener/" + svc.Name
		schemaListeners[dep] = true
		_, isCovered := doctorDependencyCovered[dep]
		_, isExcluded := doctorDependencyExcluded[dep]
		assert.Truef(t, isCovered || isExcluded,
			"ze:listener service %q is probed by ze doctor but absent from the dependency inventory; add %q to covered or excluded", svc.Name, dep)
	}

	// And the other direction: a listener/ row whose service the schema does not
	// declare is a row nothing exercises. It happens when a plugin's YANG stops
	// being linked into this binary, which would silently retire every assertion
	// below that names it while the row went on claiming coverage.
	for _, m := range []map[string]string{doctorDependencyCovered, doctorDependencyExcluded} {
		for dep := range m {
			if !strings.HasPrefix(dep, "listener/") || goExtractorListeners[dep] {
				continue
			}
			assert.Truef(t, schemaListeners[dep],
				"inventory row %q names no ze:listener the schema declares and no extractor in goExtractorListeners; either its YANG is not linked here or the row is stale", dep)
		}
	}

	const expectedTotal = 63
	total := len(doctorDependencyCovered) + len(doctorDependencyExcluded)
	assert.Equal(t, expectedTotal, total,
		"dependency inventory changed; update covered or excluded map (got %d)", total)
}

// TestDoctorProbesEveryCoveredListener closes the direction the inventory's name
// check cannot see. That loop proves every schema-declared service is CLASSIFIED;
// it says nothing about whether a service classified `covered` is still probed.
// Deleting one RegisterListenerDefault line
// (internal/component/config/listener_defaults.go) stops ze doctor probing that
// service entirely and left the inventory green.
//
// The tree it drives is also the shape ISSUE-5 named: an entry that gives an ip
// and omits the port, which the daemon binds at the service's default port and
// which parseListenerEntry drops.
//
// VALIDATES: every `doctor-listen-unavailable` listener row names a probe that
// collectSchemaListeners actually produces.
// PREVENTS: a covered row outliving the registration that produces its endpoint,
// and the ip-only entry falling through both default fills.
func TestDoctorProbesEveryCoveredListener(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err, "doctor probes schema-discovered listeners, so the schema must load")

	checked := 0
	for _, svc := range config.DiscoverListenerServices(schema) {
		name := svc.Name
		code, isCovered := doctorDependencyCovered["listener/"+name]
		// Only the schema-listener path runs through here. An excluded service has
		// no default endpoint to probe, and a covered service reported by some
		// other code would be proven by whatever produces that code, not by
		// collectSchemaListeners. No service is in the second case today: every
		// covered schema listener maps to doctor-listen-unavailable.
		if !isCovered || code != "doctor-listen-unavailable" {
			continue
		}
		// The config path is DERIVED from the schema, so it cannot go stale. It
		// must still name a container: a ze:listener list at the tree root has no
		// service block to enable and nothing below could configure it.
		require.NotEmptyf(t, svc.Containers,
			"ze:listener service %q sits at the config root, so there is no container to enable it in", name)

		wantNetworks, hasNetworks := listenerProbeNetworks[name]
		require.Truef(t, hasNetworks,
			"ze:listener service %q is covered but listenerProbeNetworks does not say which transport it binds; read its binder and add a row", name)

		listeners := collectSchemaListeners(listenerEntryWithoutPortTree(svc))
		require.NotEmptyf(t, listeners,
			"ze doctor probes nothing at all for %q with an ip-only entry; the inventory says %s reports it, so either its listener default is gone (internal/component/config/listener_defaults.go) or the entry fill in CollectListenersWithDefaults is", name, code)
		// The entry's ip identifies its own probe. A sibling service under the
		// same container (as112 declares one list per family) contributes its
		// default endpoint to the same call, and that endpoint is not evidence
		// about this service.
		var gotNetworks []string
		for _, l := range listeners {
			if l.host != "127.0.0.1" {
				continue
			}
			assert.NotEqualf(t, "0", l.port, "probe for %q has no port; the registered default did not reach the endpoint", name)
			gotNetworks = append(gotNetworks, l.network)
		}
		assert.NotEmptyf(t, gotNetworks,
			"the ip-only entry for %q produced no probe at the ip it names; the entry fill in CollectListenersWithDefaults did not run for it", name)
		// The TRANSPORT is the half a presence check cannot see. probeListener
		// calls ListenPacket only for "udp", so a UDP service probed as TCP binds
		// a socket nothing contends for: the probe passes whatever holds the port
		// the daemon actually needs, and the coverage claimed does not exist.
		slices.Sort(gotNetworks)
		want := append([]string(nil), wantNetworks...)
		slices.Sort(want)
		assert.Equalf(t, want, gotNetworks,
			"ze doctor probes %v for %q but it binds %v; a probe on the wrong transport cannot detect the conflict it exists to detect", gotNetworks, name, wantNetworks)
		checked++
	}
	assert.NotZero(t, checked, "no covered schema listener was exercised; the inventory or the schema stopped naming any")
}

func TestRunDoctorChecksIncludesPluginRegistryChecks(t *testing.T) {
	// VALIDATES: plugin doctor checks declared via registry.Registration.DoctorChecks
	// are executed by the doctor runner through the bridge.
	// PREVENTS: plugin checks registered via the new Registration field being silently ignored.
	t.Cleanup(func() { registry.Restore(registry.Snapshot()) })
	snap := registry.Snapshot()
	registry.Reset()

	called := false
	require.NoError(t, registry.Register(registry.Registration{
		Name:        "test-doctor-bridge",
		Description: "bridge test plugin",
		RunEngine:   func(net.Conn) int { return 0 },
		CLIHandler:  func([]string) int { return 0 },
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "bridge-test-check",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        900,
			Dependencies: []string{"config-loaded"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-bridge-test"},
			Check: func(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
				called = true
				return []rpc.DoctorCheckDiagnostic{{
					Code:     "doctor-bridge-test",
					Severity: "warning",
					Message:  "bridge test fired",
				}}
			},
		}},
	}))

	tree := config.NewTree()
	platform := &host.PlatformInfo{Type: host.PlatformDarwin}
	ctx := doctorCheckContext{
		Tree:      tree,
		ConfigDir: t.TempDir(),
		Platform:  platform,
	}
	diags := runDoctorChecks(doctorCheckPhasePostConfig, ctx)

	assert.True(t, called, "plugin registry doctor check was not called")
	found := false
	for _, d := range diags {
		if d.Code == "doctor-bridge-test" {
			found = true
			assert.Equal(t, diagnostic.SeverityWarning, d.Severity)
			assert.Equal(t, "bridge test fired", d.Message)
		}
	}
	assert.True(t, found, "expected doctor-bridge-test diagnostic from plugin registry bridge")
	registry.Restore(snap)
}
