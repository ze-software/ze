//go:build linux

package main

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/ze-software/ze/internal/le/qemu"
)

type pppoePythonParity struct {
	AccelConfig string            `json:"accel_config"`
	ZeConfig    string            `json:"ze_config"`
	Chap        string            `json:"chap"`
	Environment map[string]string `json:"environment"`
	Commands    int               `json:"commands"`
}

func TestPPPoEPythonAndGoShareEveryConfigByteAndDottedEnvironment(t *testing.T) {
	t.Parallel()
	code := `import importlib.util,json,pathlib,tempfile
p=pathlib.Path("effective-pppoe-accel.py"); s=importlib.util.spec_from_file_location("pppoe",p); m=importlib.util.module_from_spec(s); s.loader.exec_module(m)
m.VETH_ZE="zpoez1"; m.VETH_AC="zpoea1"
w=pathlib.Path("/tmp/effective-pppoe-parity")
files={}
orig=pathlib.Path.write_text
orig_chmod=pathlib.Path.chmod
pathlib.Path.write_text=lambda self,data,**kw: files.__setitem__(str(self),data) or len(data)
pathlib.Path.chmod=lambda self,mode: None
try:
 m.write_accel_conf(w); m.write_ze_conf(w)
finally:
 pathlib.Path.write_text=orig; pathlib.Path.chmod=orig_chmod
print(json.dumps({"accel_config":files[str(w/"accel-ppp.conf")],"ze_config":files[str(w/"ze.conf")],"chap":files[str(w/"chap-secrets")],"environment":{"ze.log.interface":"debug","ZE_STORAGE_BLOB":"false","ZE_CONFIG_DIR":str(w/"ze")},"commands":2}))`
	command := exec.CommandContext(t.Context(), "python3", "-c", code)
	command.Dir = "."
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run Python producer: %v", err)
	}
	var python pppoePythonParity
	if err := json.Unmarshal(output, &python); err != nil {
		t.Fatalf("decode Python payload: %v\n%s", err, output)
	}
	accel, ze, chap := qemu.PPPoEParityConfigs("/tmp/effective-pppoe-parity", "zpoez1", "zpoea1")
	if python.AccelConfig != string(accel) || python.ZeConfig != string(ze) || python.Chap != string(chap) {
		t.Fatalf("config payload differs\nPython accel:\n%s\nGo accel:\n%s\nPython ze:\n%s\nGo ze:\n%s\nPython chap:%q Go chap:%q", python.AccelConfig, accel, python.ZeConfig, ze, python.Chap, chap)
	}
	if python.Environment["ze.log.interface"] != "debug" {
		t.Fatalf("Python dotted environment lost: %#v", python.Environment)
	}
	if python.Commands != 2 {
		t.Fatalf("expected both daemon process commands, got %d", python.Commands)
	}
}
