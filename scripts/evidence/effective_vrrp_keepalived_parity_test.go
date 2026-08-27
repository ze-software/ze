//go:build linux

package main

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/ze-software/ze/internal/le/qemu"
)

type vrrpPythonParity struct {
	Values struct {
		VIP, ZeAddress, KAAddress, OBAddress, VirtualMAC, Multicast string
		VRID, ZePriority, KAPriority, AdvertMS, AdvertCS, AdvertTTL int
		QS2PromoteMin, QS2PromoteMax, QS2PreemptMin, QS2PreemptMax  float64
		QS3PromoteMax, QS3NoSkewPath                                float64
	} `json:"values"`
	ZeConfig         string   `json:"ze_config"`
	KeepalivedConfig string   `json:"keepalived_config"`
	Scenarios        []string `json:"scenarios"`
	Commands         int      `json:"commands"`
}

func TestVRRPPythonAndGoSharePayloadsAndAbsolutePopulation(t *testing.T) {
	t.Parallel()
	code := `import importlib.util,json,pathlib,sys
p=pathlib.Path("effective-vrrp-keepalived.py")
s=importlib.util.spec_from_file_location("vrrp",p); m=importlib.util.module_from_spec(s); sys.modules[s.name]=m; s.loader.exec_module(m)
m.ZE_VETH="zvz123"; m.KA_VETH="zvk123"
v={"VIP":m.VIP,"ZeAddress":m.ZE_ADDR,"KAAddress":m.KA_ADDR,"OBAddress":m.OB_ADDR,"VirtualMAC":m.VIRTUAL_MAC,"Multicast":m.VRRP_MCAST_V4,"VRID":m.VRID,"ZePriority":m.ZE_PRIORITY,"KAPriority":m.KA_PRIORITY,"AdvertMS":m.ADVERT_MS,"AdvertCS":m.ADVERT_CS,"AdvertTTL":m.ADVERT_TTL,"QS2PromoteMin":m.QS2_PROMOTE_MIN_S,"QS2PromoteMax":m.QS2_PROMOTE_MAX_S,"QS2PreemptMin":m.QS2_PREEMPT_MIN_S,"QS2PreemptMax":m.QS2_PREEMPT_MAX_S,"QS3PromoteMax":m.QS3_PRIO0_PROMOTE_MAX_S,"QS3NoSkewPath":m.QS3_NO_SKEW_PATH_S}
print(json.dumps({"values":v,"ze_config":m.ze_conf_v3_ipv4(m.ZE_PRIORITY),"keepalived_config":m.keepalived_conf_v3_ipv4(pathlib.Path("/root/notify.sh"),pathlib.Path("/root/ka-state.log"),m.KA_PRIORITY),"scenarios":list(m.SCENARIOS),"commands":3}))`
	command := exec.CommandContext(t.Context(), "python3", "-c", code)
	command.Dir = "."
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run Python producer: %v", err)
	}
	var python vrrpPythonParity
	if err := json.Unmarshal(output, &python); err != nil {
		t.Fatalf("decode Python payload: %v\n%s", err, output)
	}
	goValues := qemu.VRRPParitySnapshot()
	if python.Values.VIP != goValues.VIP || python.Values.ZeAddress != goValues.ZeAddress ||
		python.Values.KAAddress != goValues.KAAddress || python.Values.OBAddress != goValues.OBAddress ||
		python.Values.VirtualMAC != goValues.VirtualMAC || python.Values.Multicast != goValues.Multicast ||
		python.Values.VRID != goValues.VRID || python.Values.ZePriority != goValues.ZePriority ||
		python.Values.KAPriority != goValues.KAPriority || python.Values.AdvertMS != goValues.AdvertMS ||
		python.Values.AdvertCS != goValues.AdvertCS || python.Values.AdvertTTL != goValues.AdvertTTL ||
		python.Values.QS2PromoteMin != goValues.QS2PromoteMin || python.Values.QS2PromoteMax != goValues.QS2PromoteMax ||
		python.Values.QS2PreemptMin != goValues.QS2PreemptMin || python.Values.QS2PreemptMax != goValues.QS2PreemptMax ||
		python.Values.QS3PromoteMax != goValues.QS3PromoteMax || python.Values.QS3NoSkewPath != goValues.QS3NoSkewPath {
		t.Fatalf("VRRP values differ: Python %#v, Go %#v", python.Values, goValues)
	}
	zeConfig, keepalivedConfig := qemu.VRRPParityConfigs("zvz123", "zvk123", "/root/notify.sh", "/root/ka-state.log")
	if python.ZeConfig != string(zeConfig) || python.KeepalivedConfig != string(keepalivedConfig) {
		t.Fatalf("generated config bytes differ\nPython ze:\n%s\nGo ze:\n%s\nPython keepalived:\n%s\nGo keepalived:\n%s", python.ZeConfig, zeConfig, python.KeepalivedConfig, keepalivedConfig)
	}
	if python.Commands != 3 || len(python.Scenarios) != 3 {
		t.Fatalf("scenario population changed: %#v", python)
	}
}
