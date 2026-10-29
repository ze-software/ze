// Tests for the kernel feature table: how a kernel config is read, how each
// feature is classified, what a failing test is told, and that Ze's own kernel
// builds every feature in.

package kernelfeature

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleConfig carries one line of each shape a kernel config holds.
const sampleConfig = `#
# Automatically generated file; DO NOT EDIT.
#
CONFIG_NF_FLOW_TABLE=y
CONFIG_NF_FLOW_TABLE_INET=m
# CONFIG_NFT_FLOW_OFFLOAD is not set
CONFIG_NET_ACT_POLICE=m
CONFIG_LOCALVERSION="-ze"
`

// TestParseConfigKeepsValuesAndDropsUnsetSymbols pins the reader.
// VALIDATES: a set symbol keeps its value, quotes included, and a symbol the
// config marks "is not set" is absent from the map, like one it never names.
// PREVENTS: "# CONFIG_X is not set" read as a symbol the kernel provides.
func TestParseConfigKeepsValuesAndDropsUnsetSymbols(t *testing.T) {
	config, err := ParseConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{
		"CONFIG_NF_FLOW_TABLE":      "y",
		"CONFIG_NF_FLOW_TABLE_INET": "m",
		"CONFIG_NET_ACT_POLICE":     "m",
		"CONFIG_LOCALVERSION":       `"-ze"`,
	}
	for symbol, value := range want {
		if config[symbol] != value {
			t.Errorf("%s = %q, want %q", symbol, config[symbol], value)
		}
	}
	if _, ok := config["CONFIG_NFT_FLOW_OFFLOAD"]; ok {
		t.Errorf("an unset symbol was kept: %q", config["CONFIG_NFT_FLOW_OFFLOAD"])
	}
	if len(config) != len(want) {
		t.Errorf("read %d symbols, want %d: %v", len(config), len(want), config)
	}
}

// TestStateTellsBuiltInLoadedNotLoadedAndAbsentApart pins the four answers.
func TestStateTellsBuiltInLoadedNotLoadedAndAbsentApart(t *testing.T) {
	config, err := ParseConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	kernel := New("7.0.0-test", config, []string{"act_police"})
	for _, tc := range []struct {
		symbol string
		want   State
	}{
		{"CONFIG_NF_FLOW_TABLE", BuiltIn},
		{"CONFIG_NET_ACT_POLICE", Loaded},
		{"CONFIG_NF_FLOW_TABLE_INET", NotLoaded},
		{"CONFIG_NFT_FLOW_OFFLOAD", Absent},
		{"CONFIG_NET_ACT_GACT", Absent},
	} {
		if got := kernel.State(MustLookup(tc.symbol)); got != tc.want {
			t.Errorf("%s is %s, want %s", tc.symbol, got, tc.want)
		}
	}
}

// TestExplainNamesTheFixForEachMissingFeature pins what a failing test prints.
// VALIDATES: a feature the kernel lacks names Ze's kernel and the command that
// installs it; a module that is not loaded names Ze's kernel and the modprobe
// line; a usable feature adds nothing.
// PREVENTS: a bare ENOENT from the kernel with no word on which option is
// missing or how to get it.
func TestExplainNamesTheFixForEachMissingFeature(t *testing.T) {
	config, err := ParseConfig(strings.NewReader(sampleConfig))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	kernel := New("7.0.0-test", config, []string{"act_police"})

	if got := kernel.Explain("CONFIG_NF_FLOW_TABLE", "CONFIG_NET_ACT_POLICE"); got != "" {
		t.Errorf("usable features explained as %q, want nothing", got)
	}

	absent := kernel.Explain("CONFIG_NET_ACT_GACT")
	for _, part := range []string{"7.0.0-test", "lacks CONFIG_NET_ACT_GACT", "Ze's kernel", "./le setup docker kernel install"} {
		if !strings.Contains(absent, part) {
			t.Errorf("the absent answer lacks %q: %q", part, absent)
		}
	}

	unloaded := kernel.Explain("CONFIG_NF_FLOW_TABLE_INET")
	for _, part := range []string{"CONFIG_NF_FLOW_TABLE_INET", "nf_flow_table_inet", "not loaded", "Ze's kernel", "sudo modprobe nf_flow_table_inet"} {
		if !strings.Contains(unloaded, part) {
			t.Errorf("the not-loaded answer lacks %q: %q", part, unloaded)
		}
	}
}

// TestZesKernelBuildsEveryFeatureIn compares the table with its source.
// VALIDATES: every feature the table names is =y in a fragment of Ze's kernel
// (gokrazy/kernel/kernel.config, the base every profile carries, or
// runtime.config).
// PREVENTS: a feature added here that Ze's own kernel does not build, which
// would send a developer to boot a kernel that still lacks it.
func TestZesKernelBuildsEveryFeatureIn(t *testing.T) {
	built := map[string]string{}
	for _, name := range []string{"kernel.config", "runtime.config"} {
		file, err := os.Open(filepath.Join("..", "..", "..", "gokrazy", "kernel", name))
		if err != nil {
			t.Fatalf("open fragment: %v", err)
		}
		config, err := ParseConfig(file)
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close fragment %s: %v", name, closeErr)
		}
		if err != nil {
			t.Fatalf("parse fragment %s: %v", name, err)
		}
		maps.Copy(built, config)
	}
	for _, feature := range Features {
		if built[feature.Symbol] != "y" {
			t.Errorf("Ze's kernel fragments set %s to %q, want y", feature.Symbol, built[feature.Symbol])
		}
	}
}
