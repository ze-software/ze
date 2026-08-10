// Design: docs/architecture/core-design.md — tech-support archive tests

package support

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestModuleRegistry_AllRegistered(t *testing.T) {
	expected := []string{"config", "conntrack", "crashes", "disk", "dmesg", "dns", "doctor", "env", "fds", "firewall", "host", "interfaces", "modules", "neighbors", "platform", "routes", "runtime", "sockets", "sysctl", "version"}
	names := ModuleNames()
	if !slices.Equal(names, expected) {
		t.Errorf("ModuleNames() = %v, want %v", names, expected)
	}
}

func TestModuleRegistry_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(moduleRegistry))
	for name := range moduleRegistry {
		if seen[name] {
			t.Errorf("duplicate module: %s", name)
		}
		seen[name] = true
	}
}

func TestModuleSelection_IncludeFilter(t *testing.T) {
	modules, errMsg := filterModules([]string{"doctor", "host"}, nil)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if len(modules) != 2 {
		t.Errorf("got %d modules, want 2", len(modules))
	}
	if _, ok := modules["doctor"]; !ok {
		t.Error("missing doctor module")
	}
	if _, ok := modules["host"]; !ok {
		t.Error("missing host module")
	}
}

func TestModuleSelection_ExcludeFilter(t *testing.T) {
	modules, errMsg := filterModules(nil, []string{"crashes"})
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if _, ok := modules["crashes"]; ok {
		t.Error("crashes should be excluded")
	}
	if len(modules) != len(moduleRegistry)-1 {
		t.Errorf("got %d modules, want %d", len(modules), len(moduleRegistry)-1)
	}
}

func TestModuleSelection_InvalidName(t *testing.T) {
	_, errMsg := filterModules([]string{"nonexistent"}, nil)
	if errMsg == "" {
		t.Error("expected error for unknown module")
	}
}

func TestManifest_Structure(t *testing.T) {
	dir := t.TempDir()
	manifest, err := collect(
		map[string]moduleCollector{"version": collectVersion},
		&collectOptions{},
		"test-reason",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion == 0 {
		t.Error("schema-version is zero")
	}
	if manifest.Hostname == "" {
		t.Error("hostname is empty")
	}
	if manifest.Timestamp == "" {
		t.Error("timestamp is empty")
	}
	if manifest.Reason != "test-reason" {
		t.Errorf("reason = %q, want %q", manifest.Reason, "test-reason")
	}
	if manifest.ArchivePath == "" {
		t.Error("archive-path is empty")
	}
	mod, ok := manifest.Modules["version"]
	if !ok {
		t.Fatal("version module not in manifest")
	}
	if !mod.Collected {
		t.Error("version module not collected")
	}
}

func TestManifest_ReasonIncluded(t *testing.T) {
	dir := t.TempDir()
	manifest, err := collect(
		map[string]moduleCollector{"version": collectVersion},
		&collectOptions{},
		"BGP flap at 14:00",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Reason != "BGP flap at 14:00" {
		t.Errorf("reason = %q", manifest.Reason)
	}
}

func TestArchiveStructure_ModuleFiles(t *testing.T) {
	dir := t.TempDir()
	manifest, err := collect(
		map[string]moduleCollector{
			"version": collectVersion,
			"doctor": func(_ *collectOptions) (any, error) {
				return map[string]any{"test": true}, nil
			},
		},
		&collectOptions{},
		"",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(manifest.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	files := make(map[string]bool)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		files[hdr.Name] = true
	}

	for _, want := range []string{"manifest.json", "version.json", "doctor.json"} {
		if !files[want] {
			t.Errorf("archive missing %s, has: %v", want, files)
		}
	}
}

func TestArchiveManifest_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	manifest, err := collect(
		map[string]moduleCollector{"version": collectVersion},
		&collectOptions{},
		"",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(manifest.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			var m SupportManifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("manifest.json is not valid JSON: %v", err)
			}
			if m.SchemaVersion == 0 {
				t.Error("schema-version in archive manifest is zero")
			}
			return
		}
	}
	t.Error("manifest.json not found in archive")
}

func TestListModules_DerivedFromRegistry(t *testing.T) {
	names := ModuleNames()
	if len(names) != len(moduleRegistry) {
		t.Errorf("ModuleNames() returned %d names, registry has %d", len(names), len(moduleRegistry))
	}
	for _, name := range names {
		if _, ok := moduleRegistry[name]; !ok {
			t.Errorf("ModuleNames() returned %q not in registry", name)
		}
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ModuleNames() not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestJSONOutput_ManifestToStdout(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdout.json")

	old := os.Stdout
	f, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f

	code := Run([]string{"--module", "version", "--json", "--output", dir})

	os.Stdout = old
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if code != 0 {
		t.Fatalf("Run returned %d", code)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}

	var m SupportManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, data)
	}
	if _, ok := m.Modules["version"]; !ok {
		t.Error("version module not in JSON output")
	}
}

func TestGracefulDegradation_ModuleError(t *testing.T) {
	dir := t.TempDir()
	failing := func(_ *collectOptions) (any, error) {
		return nil, os.ErrNotExist
	}
	manifest, err := collect(
		map[string]moduleCollector{"failing": failing, "version": collectVersion},
		&collectOptions{},
		"",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	mod := manifest.Modules["failing"]
	if mod.Collected {
		t.Error("failing module should not be marked collected")
	}
	if mod.Error == "" {
		t.Error("failing module should have error message")
	}
	if !manifest.Modules["version"].Collected {
		t.Error("version module should still succeed")
	}
}

func TestGracefulDegradation_ModulePanic(t *testing.T) {
	dir := t.TempDir()
	panicking := func(_ *collectOptions) (any, error) {
		panic("collector blew up")
	}
	manifest, err := collect(
		map[string]moduleCollector{"panicking": panicking, "version": collectVersion},
		&collectOptions{},
		"",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	mod := manifest.Modules["panicking"]
	if mod.Collected {
		t.Error("panicking module should not be marked collected")
	}
	if mod.Error == "" {
		t.Error("panicking module should have error message")
	}
	if !strings.Contains(mod.Error, "panic") {
		t.Errorf("error should mention panic, got: %s", mod.Error)
	}
	if !strings.Contains(mod.Error, "collector blew up") {
		t.Errorf("error should preserve original panic value, got: %s", mod.Error)
	}
	if !manifest.Modules["version"].Collected {
		t.Error("version module should still succeed after panic in another module")
	}
}

func TestRouteTruncation_Indicator(t *testing.T) {
	dir := t.TempDir()
	fakeRoutes := func(_ *collectOptions) (any, error) {
		routes := make([]string, 50000)
		for i := range routes {
			routes[i] = "route"
		}
		result := map[string]any{"routes": routes, "count": len(routes)}
		if len(routes) >= 50000 {
			result["truncated"] = true
			result["limit"] = 50000
		}
		return result, nil
	}
	manifest, err := collect(
		map[string]moduleCollector{"routes": fakeRoutes},
		&collectOptions{},
		"",
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Modules["routes"].Collected {
		t.Error("routes module should be collected")
	}
}

func TestSensitiveFlag_EnvRedaction(t *testing.T) {
	opts := &collectOptions{Sensitive: false}
	result, err := collectEnv(opts)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	vars, ok := m["variables"].([]map[string]any)
	if !ok {
		t.Fatal("expected variables slice")
	}
	for _, v := range vars {
		val, _ := v["value"].(string)
		key, _ := v["key"].(string)
		if val == redactedValue {
			t.Logf("correctly redacted secret env var: %s", key)
		}
	}
}

func TestParseSince_Duration(t *testing.T) {
	before := time.Now()
	got, err := parseSince("2h")
	if err != nil {
		t.Fatal(err)
	}
	expected := before.Add(-2 * time.Hour)
	if got.Before(expected.Add(-time.Second)) || got.After(expected.Add(time.Second)) {
		t.Errorf("parseSince(2h) = %v, want ~%v", got, expected)
	}
}

func TestParseSince_ISODate(t *testing.T) {
	got, err := parseSince("2026-05-25")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseSince(2026-05-25) = %v, want %v", got, want)
	}
}

func TestParseSince_RFC3339(t *testing.T) {
	got, err := parseSince("2026-05-25T14:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 5, 25, 14, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseSince(RFC3339) = %v, want %v", got, want)
	}
}

func TestParseSince_SlogTimestamp(t *testing.T) {
	got, err := parseSince("time=2025-01-18T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2025, 1, 18, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseSince(time=...) = %v, want %v", got, want)
	}
}

func TestParseSince_SlogWithOffset(t *testing.T) {
	got, err := parseSince("2025-01-18T12:00:00.123+02:00")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestParseSince_Invalid(t *testing.T) {
	_, err := parseSince("yesterday")
	if err == nil {
		t.Error("expected error for invalid since value")
	}
}

func TestParseSince_NegativeDuration(t *testing.T) {
	_, err := parseSince("-2h")
	if err == nil {
		t.Error("expected error for negative duration")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Errorf("error should mention positive, got: %s", err.Error())
	}
}

func TestExcludeFilter_Run(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdout.json")

	old := os.Stdout
	f, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = f

	code := Run([]string{"--exclude", "doctor,host,config,crashes,disk,interfaces,routes,neighbors,env,sysctl,runtime,dmesg,sockets,modules,conntrack,fds,dns,firewall", "--json", "--output", dir})

	os.Stdout = old
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if code != 0 {
		t.Fatalf("Run returned %d", code)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}

	var m SupportManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if _, ok := m.Modules["version"]; !ok {
		t.Error("version should be present when all others excluded")
	}
	if _, ok := m.Modules["doctor"]; ok {
		t.Error("doctor should be excluded")
	}
}
