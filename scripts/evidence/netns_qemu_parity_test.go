//go:build linux

package main

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/le/qemu"
)

type netnsPythonParity struct {
	Selections map[string][]string `json:"selections"`
	Defaults   []string            `json:"defaults"`
	Environment map[string]string  `json:"environment"`
	Argv       map[string][]string `json:"argv"`
	Commands   int                 `json:"commands"`
}

func TestNetnsPythonAndGoShareFullSelectionsArgvAndEnvironment(t *testing.T) {
	t.Parallel()
	code := `import importlib.util,json,pathlib,os
os.environ.pop("ZE_NETNS_QEMU_SUITES",None)
p=pathlib.Path("netns_qemu.py"); s=importlib.util.spec_from_file_location("netns",p); m=importlib.util.module_from_spec(s); s.loader.exec_module(m)
m.ZE_QEMU_TEST_BIN="/tmp/ze-test"
env={"ZE_TEST_NO_BUILD":"1","ZE_QEMU":"1","ZE_BIN":"/tmp/zebin/ze","ZE_STRIPPED_BIN":"/tmp/zebin/ze-stripped","ZE_TEST_BIN":m.ZE_QEMU_TEST_BIN,"ZE_TEST_NETNS":"1","ZE_TEST_UID":"1000","ZE_TEST_GID":"1000","ze.config.dir":"/tmp/zestate"}
argv={name:[m.ZE_QEMU_TEST_BIN,name,"-p","1",*ids] for name,ids in m.SUITE_REGISTRY.items()}
print(json.dumps({"selections":m.SUITE_REGISTRY,"defaults":list(m.DEFAULT_SUITES),"environment":env,"argv":argv,"commands":sum(len(v) for v in m.SUITE_REGISTRY.values())}))`
	command := exec.CommandContext(t.Context(), "python3", "-c", code)
	command.Dir = "."
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run Python producer: %v", err)
	}
	var python netnsPythonParity
	if err := json.Unmarshal(output, &python); err != nil {
		t.Fatalf("decode Python payload: %v\n%s", err, output)
	}
	goSelections := qemu.NetnsParitySelections()
	if !reflect.DeepEqual(python.Selections, goSelections) {
		t.Fatalf("selector population differs\nPython %#v\nGo %#v", python.Selections, goSelections)
	}
	if !reflect.DeepEqual(python.Defaults, []string{"firewall", "policy", "ospf", "ospfv3"}) {
		t.Fatalf("defaults changed: %v", python.Defaults)
	}
	for suite, ids := range goSelections {
		want := append([]string{"/tmp/ze-test", suite, "-p", "1"}, ids...)
		if !reflect.DeepEqual(python.Argv[suite], want) {
			t.Errorf("%s argv differs: %v != %v", suite, python.Argv[suite], want)
		}
	}
	if python.Environment["ze.config.dir"] != "/tmp/zestate" || python.Environment["ZE_TEST_UID"] != "1000" {
		t.Fatalf("environment changed: %#v", python.Environment)
	}
	if python.Commands != 42 {
		t.Fatalf("absolute selector count = %d, want 42", python.Commands)
	}
}
